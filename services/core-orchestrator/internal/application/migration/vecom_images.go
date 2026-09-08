package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime"
	"net/http"
	neturl "net/url"
	"path"
	"strings"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// VecomImagesMigrationService is a TEMPORARY, one-off migration helper: it
// reads the previous system's legacy vecom_images table (joined to
// vecom_products so vecom_products.code can be matched against
// ecom_products.sku), HEAD-validates each image URL, creates the matching
// ecom_files row and links it into ecom_product_images — the first image per
// product (by image_position) marked is_first. Remove this service, its
// handler method and its route once the migration is done.
type VecomImagesMigrationService struct {
	db                *sql.DB
	productRepo       *mysqlInfra.ProductRepository
	filesRepo         *mysqlInfra.FilesRepository
	productImagesRepo *mysqlInfra.ProductImagesRepository
	httpClient        *http.Client
}

func NewVecomImagesMigrationService(
	db *sql.DB,
	productRepo *mysqlInfra.ProductRepository,
	filesRepo *mysqlInfra.FilesRepository,
	productImagesRepo *mysqlInfra.ProductImagesRepository,
) *VecomImagesMigrationService {
	return &VecomImagesMigrationService{
		db:                db,
		productRepo:       productRepo,
		filesRepo:         filesRepo,
		productImagesRepo: productImagesRepo,
		httpClient:        &http.Client{Timeout: 15 * time.Second},
	}
}

// vecomImagesDiskID is the only ecom_storage_disks row that exists today —
// every migrated file is attached to it.
const vecomImagesDiskID int64 = 1

// MigrateVecomImagesInput narrows a run either to one vecom_products.code (for
// re-running the migration against a single product missed on the original
// pass — e.g. one whose ecom_products row didn't exist yet because its sku
// shared a part_number with another product, before that unique constraint
// was dropped) or to a page of vecom_products ordered by id (for a large
// batched migration). Code takes precedence over Limit/Offset when both are
// set. A batch always carries every image of every product it selects — never
// a partial product — so image_position ordering / is_first stay correct
// within a single call. A zero Limit with no Code migrates every product's
// images in one call.
type MigrateVecomImagesInput struct {
	Code   string
	Limit  int
	Offset int
}

// VecomImageOutcome is the per-image report. An image failing never stops
// the batch.
type VecomImageOutcome struct {
	ProductCode    string `json:"productCode"`
	URL            string `json:"url"`
	Position       int    `json:"position"`
	IsFirst        bool   `json:"isFirst"`
	Success        bool   `json:"success"`
	FileID         *int64 `json:"fileId,omitempty"`
	ProductImageID *int64 `json:"productImageId,omitempty"`
	Skipped        bool   `json:"skipped,omitempty"`
	SkipReason     string `json:"skipReason,omitempty"`
	Error          string `json:"error,omitempty"`
}

type MigrateVecomImagesResult struct {
	TotalImages int                  `json:"totalImages"`
	Imported    int                  `json:"imported"`
	Skipped     int                  `json:"skipped"`
	Errored     int                  `json:"errored"`
	Results     []VecomImageOutcome  `json:"results"`
}

type vecomImageRow struct {
	ProductID     int64
	Code          string
	ImagePosition int
	URL           string
}

// Migrate walks the selected vecom_images rows, grouped by product and
// ordered by image_position. actorID is recorded as created_by on every
// ecom_files / ecom_product_images row written.
func (s *VecomImagesMigrationService) Migrate(ctx context.Context, input MigrateVecomImagesInput, actorID int64) (*MigrateVecomImagesResult, error) {
	if actorID <= 0 {
		return nil, fmt.Errorf("actorID is required")
	}

	rows, err := s.loadRows(ctx, input)
	if err != nil {
		return nil, err
	}

	result := &MigrateVecomImagesResult{TotalImages: len(rows), Results: make([]VecomImageOutcome, 0, len(rows))}

	// currentIndex is the position of the image within its product's
	// sequence as actually walked here (1, 2, 3, ...) — not
	// vecom_images.image_position itself, which isn't guaranteed to start
	// at 1 or be contiguous. It drives both the ecom_files filename suffix
	// and is_first.
	var currentProductID int64 = -1
	var currentIndex int

	// productStates is resolved once per vecom product_id (not once per
	// image row): the sku lookup and the already-has-images check only need
	// to run the first time a product is seen in this batch.
	productStates := make(map[int64]vecomProductState)

	for _, row := range rows {
		if row.ProductID != currentProductID {
			currentProductID = row.ProductID
			currentIndex = 0
		}
		currentIndex++
		isFirst := currentIndex == 1

		state, ok := productStates[row.ProductID]
		if !ok {
			state = s.resolveProductState(ctx, row)
			productStates[row.ProductID] = state
		}

		outcome := s.migrateImage(ctx, row, currentIndex, isFirst, actorID, state)
		result.Results = append(result.Results, outcome)

		switch {
		case outcome.Skipped:
			result.Skipped++
		case outcome.Error != "":
			result.Errored++
		default:
			result.Imported++
		}
	}

	return result, nil
}

func (s *VecomImagesMigrationService) loadRows(ctx context.Context, input MigrateVecomImagesInput) ([]vecomImageRow, error) {
	query := `
		SELECT vi.product_id, vp.code, vi.image_position, vi.url
		FROM vecom_images vi
		JOIN vecom_products vp ON vp.id = vi.product_id
	`
	args := make([]any, 0, 2)
	switch {
	case input.Code != "":
		query += " WHERE vp.code = ?"
		args = append(args, input.Code)
	case input.Limit > 0:
		query += `
			JOIN (
				SELECT DISTINCT product_id
				FROM vecom_images
				ORDER BY product_id
				LIMIT ? OFFSET ?
			) page ON page.product_id = vi.product_id
		`
		args = append(args, input.Limit, input.Offset)
	}
	query += " ORDER BY vi.product_id, vi.image_position, vi.id"

	dbRows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying vecom_images: %w", err)
	}
	defer dbRows.Close()

	out := make([]vecomImageRow, 0)
	for dbRows.Next() {
		var (
			r        vecomImageRow
			code     sql.NullString
			position sql.NullInt64
			url      sql.NullString
		)
		if err := dbRows.Scan(&r.ProductID, &code, &position, &url); err != nil {
			return nil, fmt.Errorf("error scanning vecom_images row: %w", err)
		}
		r.Code = strings.TrimSpace(code.String)
		r.ImagePosition = int(position.Int64)
		r.URL = strings.TrimSpace(url.String)
		out = append(out, r)
	}
	if err := dbRows.Err(); err != nil {
		return nil, fmt.Errorf("error reading vecom_images rows: %w", err)
	}

	return out, nil
}

// vecomProductState is the outcome of resolving a vecom product (by code ->
// sku) once per batch: either a product ready to receive images, a reason to
// skip every one of its images (not found, or already has images), or a
// lookup error.
type vecomProductState struct {
	product    *mysqlInfra.ProductDTO
	skipReason string
	err        error
}

// resolveProductState looks up the ecom_products row for row.Code and, when
// found, checks whether it already has images in ecom_product_images — if it
// does, the whole product is skipped rather than appending more images to
// it, since this migration is meant to run once against products with none.
func (s *VecomImagesMigrationService) resolveProductState(ctx context.Context, row vecomImageRow) vecomProductState {
	if row.Code == "" {
		return vecomProductState{skipReason: "vecom_products.code vacío"}
	}

	product, err := s.productRepo.FindBySKU(ctx, row.Code)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return vecomProductState{skipReason: "producto no encontrado para sku " + row.Code}
		}
		return vecomProductState{err: fmt.Errorf("error buscando producto por sku %s: %w", row.Code, err)}
	}

	existingImages, err := s.productImagesRepo.FindAllByProductID(ctx, product.ID)
	if err != nil {
		return vecomProductState{err: fmt.Errorf("error verificando imágenes existentes del producto %d: %w", product.ID, err)}
	}
	if len(existingImages) > 0 {
		return vecomProductState{skipReason: "el producto ya tiene imágenes en ecom_product_images"}
	}

	return vecomProductState{product: product}
}

func (s *VecomImagesMigrationService) migrateImage(ctx context.Context, row vecomImageRow, position int, isFirst bool, actorID int64, state vecomProductState) VecomImageOutcome {
	outcome := VecomImageOutcome{
		ProductCode: row.Code,
		URL:         row.URL,
		Position:    position,
		IsFirst:     isFirst,
	}

	if state.err != nil {
		outcome.Error = state.err.Error()
		return outcome
	}
	if state.skipReason != "" {
		outcome.Skipped = true
		outcome.SkipReason = state.skipReason
		return outcome
	}
	if row.URL == "" {
		outcome.Skipped = true
		outcome.SkipReason = "url vacía"
		return outcome
	}

	product := state.product

	file, err := s.filesRepo.FindByPath(ctx, row.URL)
	if err != nil && !errors.Is(err, mysqlInfra.ErrFileNotFound) {
		outcome.Error = fmt.Sprintf("error buscando archivo por url: %v", err)
		return outcome
	}

	if file == nil {
		size, mimeType, extension, checkErr := s.checkImageURL(ctx, row.URL)
		if checkErr != nil {
			outcome.Error = checkErr.Error()
			return outcome
		}

		created, createErr := s.filesRepo.Create(ctx, mysqlInfra.CreateFileInput{
			DiskID:    vecomImagesDiskID,
			Path:      row.URL,
			Filename:  fmt.Sprintf("%s_%d", row.Code, position),
			MimeType:  mimeType,
			FileType:  fileTypeFromMimeType(mimeType),
			Extension: extension,
			Size:      size,
			IsPublic:  true,
			CreatedBy: actorID,
		})
		if createErr != nil {
			outcome.Error = fmt.Sprintf("error creando archivo: %v", createErr)
			return outcome
		}
		file = created
	}

	outcome.FileID = &file.ID

	existingProductImage, err := s.productImagesRepo.FindByProductAndFile(ctx, product.ID, file.ID)
	if err != nil && !errors.Is(err, mysqlInfra.ErrProductImageNotFound) {
		outcome.Error = fmt.Sprintf("error buscando product_image existente: %v", err)
		return outcome
	}

	if existingProductImage != nil {
		outcome.Success = true
		outcome.ProductImageID = &existingProductImage.ID
		outcome.IsFirst = existingProductImage.IsFirst
		return outcome
	}

	productImage, err := s.productImagesRepo.Create(ctx, mysqlInfra.CreateProductImageInput{
		ProductID: product.ID,
		FileID:    file.ID,
		IsFirst:   isFirst,
		CreatedBy: actorID,
	})
	if err != nil {
		outcome.Error = fmt.Sprintf("error creando product_image: %v", err)
		return outcome
	}

	outcome.Success = true
	outcome.ProductImageID = &productImage.ID
	return outcome
}

// checkImageURL issues a HEAD request to confirm the image exists at the
// given URL, returning the reported size (from Content-Length, 0 when the
// server doesn't send one), mime type and extension so the caller doesn't
// have to guess them.
func (s *VecomImagesMigrationService) checkImageURL(ctx context.Context, rawURL string) (size int64, mimeType, extension string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, "", "", fmt.Errorf("url de imagen inválida: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, "", "", fmt.Errorf("no se pudo alcanzar la url de la imagen: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, "", "", errors.New("imagen no encontrada en la url (404)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "", "", fmt.Errorf("status code inesperado %d verificando la url de la imagen", resp.StatusCode)
	}

	mimeType = strings.TrimSpace(strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0])
	extension = extensionFromURL(rawURL)
	if extension == "" {
		extension = extensionFromMimeType(mimeType)
	}
	if mimeType == "" {
		mimeType = mimeTypeFromExtension(extension)
	}
	if extension == "" || mimeType == "" {
		return 0, "", "", fmt.Errorf("no se pudo determinar el tipo de imagen para %s", rawURL)
	}

	if resp.ContentLength < 0 {
		return 0, mimeType, extension, nil
	}
	return resp.ContentLength, mimeType, extension, nil
}

func extensionFromURL(rawURL string) string {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(parsed.Path), "."))
	if ext == "" || strings.ContainsAny(ext, "/?&=") {
		return ""
	}
	return ext
}

func extensionFromMimeType(mimeType string) string {
	if mimeType == "" {
		return ""
	}
	exts, err := mime.ExtensionsByType(mimeType)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(exts[0], "."))
}

func mimeTypeFromExtension(extension string) string {
	if extension == "" {
		return ""
	}
	return mime.TypeByExtension("." + extension)
}

func fileTypeFromMimeType(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	default:
		return "image"
	}
}

package product_image_import

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"path"
	"strings"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidImportPayload = errors.New("invalid product image import payload")

// systemImportUserID is the fixed actor recorded as created_by for records
// written by this bulk import flow, regardless of the authenticated caller.
const systemImportUserID int64 = 1

const (
	importedFileType  = "image"
	importedExtension = "webp"
	importedMimeType  = "image/webp"
)

type ProductImageURLsInput struct {
	SKU  string
	URLs []string
}

type ImportProductImagesInput struct {
	StorageDiskID int64
	Products      []ProductImageURLsInput
}

type ImageImportResult struct {
	SKU            string `json:"sku"`
	URL            string `json:"url"`
	Position       int    `json:"position"`
	Success        bool   `json:"success"`
	FileID         *int64 `json:"fileId,omitempty"`
	ProductImageID *int64 `json:"productImageId,omitempty"`
	IsFirst        bool   `json:"isFirst,omitempty"`
	Error          string `json:"error,omitempty"`
}

type ImportProductImagesResult struct {
	Results []ImageImportResult `json:"results"`
}

type Service struct {
	db                *sql.DB
	productRepo       *mysqlInfra.ProductRepository
	filesRepo         *mysqlInfra.FilesRepository
	productImagesRepo *mysqlInfra.ProductImagesRepository
	productVideosRepo *mysqlInfra.ProductVideosRepository
	productMediaRepo  *mysqlInfra.ProductMediaRepository
	storageDiskRepo   *mysqlInfra.StorageDiskRepository
	httpClient        *http.Client
}

func NewService(
	db *sql.DB,
	productRepo *mysqlInfra.ProductRepository,
	filesRepo *mysqlInfra.FilesRepository,
	productImagesRepo *mysqlInfra.ProductImagesRepository,
	productVideosRepo *mysqlInfra.ProductVideosRepository,
	productMediaRepo *mysqlInfra.ProductMediaRepository,
	storageDiskRepo *mysqlInfra.StorageDiskRepository,
) *Service {
	return &Service{
		db:                db,
		productRepo:       productRepo,
		filesRepo:         filesRepo,
		productImagesRepo: productImagesRepo,
		productVideosRepo: productVideosRepo,
		productMediaRepo:  productMediaRepo,
		storageDiskRepo:   storageDiskRepo,
		httpClient:        &http.Client{Timeout: 15 * time.Second},
	}
}

// TODO(persistence): la creación de ecom_files + ecom_product_images por URL es
// multi-sentencia. Los repos ya están listos para tx; falta reestructurar el
// loop (lecturas/HTTP intercaladas) para envolver ambos inserts en
// mysqlInfra.WithinTx. Hoy es recuperable: FindByPath deduplica, un retry
// reengancha la fila de ecom_files huérfana.
func (s *Service) Import(ctx context.Context, input ImportProductImagesInput) (*ImportProductImagesResult, error) {
	if input.StorageDiskID <= 0 || len(input.Products) == 0 {
		return nil, ErrInvalidImportPayload
	}

	if _, err := s.storageDiskRepo.FindByID(ctx, input.StorageDiskID); err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return nil, mysqlInfra.ErrStorageDiskNotFound
		}
		return nil, fmt.Errorf("error validating storage disk: %w", err)
	}

	results := make([]ImageImportResult, 0)

	for _, productInput := range input.Products {
		sku := strings.TrimSpace(productInput.SKU)
		if sku == "" {
			results = append(results, ImageImportResult{Success: false, Error: "sku is required"})
			continue
		}

		product, err := s.productRepo.FindBySKU(ctx, sku)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrProductNotFound) {
				results = append(results, ImageImportResult{SKU: sku, Success: false, Error: "product not found for sku"})
				continue
			}
			return nil, fmt.Errorf("error looking up product by sku %q: %w", sku, err)
		}

		for i, rawURL := range productInput.URLs {
			position := i + 1
			url := strings.TrimSpace(rawURL)
			isFirst := i == 0

			result := ImageImportResult{
				SKU:      sku,
				URL:      url,
				Position: position,
				IsFirst:  isFirst,
			}

			if url == "" {
				result.Error = "url is required"
				results = append(results, result)
				continue
			}

			file, err := s.filesRepo.FindByPath(ctx, url)
			if err != nil && !errors.Is(err, mysqlInfra.ErrFileNotFound) {
				return nil, fmt.Errorf("error looking up file by path %q: %w", url, err)
			}

			if file == nil {
				size, _, checkErr := s.inspectURL(ctx, url)
				if checkErr != nil {
					result.Error = checkErr.Error()
					results = append(results, result)
					continue
				}

				filename := fmt.Sprintf("%s_%d", sku, position)

				file, err = s.filesRepo.Create(ctx, mysqlInfra.CreateFileInput{
					DiskID:    input.StorageDiskID,
					Path:      url,
					Filename:  filename,
					MimeType:  importedMimeType,
					FileType:  importedFileType,
					Extension: importedExtension,
					Size:      size,
					IsPublic:  true,
					CreatedBy: systemImportUserID,
				})
				if err != nil {
					result.Error = fmt.Sprintf("error creating file record: %v", err)
					results = append(results, result)
					continue
				}
			}

			// The url already had a matching ecom_files row: don't
			// re-download/re-insert it, just make sure it is linked to
			// this product in ecom_product_images.
			existingProductImage, err := s.productImagesRepo.FindByProductAndFile(ctx, product.ID, file.ID)
			if err != nil && !errors.Is(err, mysqlInfra.ErrProductImageNotFound) {
				return nil, fmt.Errorf("error looking up product image for file %d: %w", file.ID, err)
			}

			if existingProductImage != nil {
				result.Success = true
				result.FileID = &file.ID
				result.ProductImageID = &existingProductImage.ID
				result.IsFirst = existingProductImage.IsFirst
				results = append(results, result)
				continue
			}

			productImage, err := s.productImagesRepo.Create(ctx, mysqlInfra.CreateProductImageInput{
				ProductID: product.ID,
				FileID:    file.ID,
				IsFirst:   isFirst,
				CreatedBy: systemImportUserID,
			})
			if err != nil {
				result.Error = fmt.Sprintf("error creating product image record: %v", err)
				result.FileID = &file.ID
				results = append(results, result)
				continue
			}

			result.Success = true
			result.FileID = &file.ID
			result.ProductImageID = &productImage.ID
			results = append(results, result)
		}
	}

	return &ImportProductImagesResult{Results: results}, nil
}

// --- Per-product flows (admin dashboard, product detail → Multimedia) --------

var (
	ErrProductNotFoundForImport = errors.New("product not found")
	ErrProductImageNotFound     = errors.New("product image not found")
	ErrProductVideoNotFound     = errors.New("product video not found")
	ErrProductMediaNotFound     = errors.New("product attachment not found")
)

type ProductImageItemInput struct {
	URL     string
	IsFirst bool
}

type ImportForProductInput struct {
	ProductID     int64
	StorageDiskID int64
	Images        []ProductImageItemInput
	ActorID       int64
}

// pendingImage carries a URL that passed the HEAD check into the transaction
// that writes ecom_files / ecom_product_images.
type pendingImage struct {
	url       string
	isFirst   bool
	size      int64
	mimeType  string
	extension string
	result    *ImageImportResult
	// productImageID is filled inside the tx once the row exists.
	productImageID int64
}

// ImportForProduct attaches one or more image URLs to a single product (by id,
// not SKU). URLs are HEAD-checked outside the transaction so a broken URL is a
// per-item error, not a whole-batch failure; the ecom_files + ecom_product_images
// writes and the cover bookkeeping then run atomically in one transaction.
func (s *Service) ImportForProduct(ctx context.Context, input ImportForProductInput) (*ImportProductImagesResult, error) {
	if input.ProductID <= 0 || input.StorageDiskID <= 0 || len(input.Images) == 0 {
		return nil, ErrInvalidImportPayload
	}

	if _, err := s.storageDiskRepo.FindByID(ctx, input.StorageDiskID); err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return nil, mysqlInfra.ErrStorageDiskNotFound
		}
		return nil, fmt.Errorf("error validating storage disk: %w", err)
	}

	product, err := s.productRepo.FindByID(ctx, input.ProductID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFoundForImport
		}
		return nil, fmt.Errorf("error looking up product %d: %w", input.ProductID, err)
	}

	actorID := input.ActorID
	if actorID <= 0 {
		actorID = systemImportUserID
	}

	// Phase 1: validate every URL with a HEAD request. Broken URLs are
	// reported per item and dropped from the batch.
	results := make([]ImageImportResult, len(input.Images))
	pending := make([]pendingImage, 0, len(input.Images))
	for i, img := range input.Images {
		url := strings.TrimSpace(img.URL)
		results[i] = ImageImportResult{URL: url, Position: i + 1, IsFirst: img.IsFirst}

		if url == "" {
			results[i].Error = "url is required"
			continue
		}

		size, contentType, checkErr := s.inspectURL(ctx, url)
		if checkErr != nil {
			results[i].Error = checkErr.Error()
			continue
		}

		mimeType, extension, metaErr := resolveImageMeta(url, contentType)
		if metaErr != nil {
			results[i].Error = metaErr.Error()
			continue
		}

		pending = append(pending, pendingImage{
			url:       url,
			isFirst:   img.IsFirst,
			size:      size,
			mimeType:  mimeType,
			extension: extension,
			result:    &results[i],
		})
	}

	if len(pending) == 0 {
		return &ImportProductImagesResult{Results: results}, nil
	}

	// Phase 2: one transaction for all the DB writes + cover bookkeeping.
	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		files := mysqlInfra.NewFilesRepository(tx)
		images := mysqlInfra.NewProductImagesRepository(tx)

		for idx := range pending {
			p := &pending[idx]

			file, ferr := files.FindByPath(ctx, p.url)
			if ferr != nil && !errors.Is(ferr, mysqlInfra.ErrFileNotFound) {
				return fmt.Errorf("error looking up file by path %q: %w", p.url, ferr)
			}

			if file == nil {
				filename := fmt.Sprintf("%d_%d", product.ID, p.result.Position)
				file, ferr = files.Create(ctx, mysqlInfra.CreateFileInput{
					DiskID:    input.StorageDiskID,
					Path:      p.url,
					Filename:  filename,
					MimeType:  p.mimeType,
					FileType:  importedFileType,
					Extension: p.extension,
					Size:      p.size,
					IsPublic:  true,
					CreatedBy: actorID,
				})
				if ferr != nil {
					return fmt.Errorf("error creating file record for %q: %w", p.url, ferr)
				}
			}
			p.result.FileID = &file.ID

			link, lerr := images.FindByProductAndFile(ctx, product.ID, file.ID)
			if lerr != nil && !errors.Is(lerr, mysqlInfra.ErrProductImageNotFound) {
				return fmt.Errorf("error looking up product image for file %d: %w", file.ID, lerr)
			}

			if link == nil {
				link, lerr = images.Create(ctx, mysqlInfra.CreateProductImageInput{
					ProductID: product.ID,
					FileID:    file.ID,
					IsFirst:   false,
					CreatedBy: actorID,
				})
				if lerr != nil {
					return fmt.Errorf("error linking file %d to product %d: %w", file.ID, product.ID, lerr)
				}
			}

			p.productImageID = link.ID
			p.result.ProductImageID = &link.ID
			p.result.Success = true
		}

		// Cover resolution: an explicit isFirst flag wins; otherwise promote
		// the first newly attached image only when the product has no cover.
		coverID := int64(0)
		for _, p := range pending {
			if p.isFirst && p.result.Success {
				coverID = p.productImageID
			}
		}
		if coverID == 0 {
			hasCover, herr := images.ProductHasCover(ctx, product.ID)
			if herr != nil {
				return fmt.Errorf("error checking product cover: %w", herr)
			}
			if !hasCover {
				for _, p := range pending {
					if p.result.Success {
						coverID = p.productImageID
						break
					}
				}
			}
		}

		if coverID != 0 {
			if cerr := images.ClearCover(ctx, product.ID, actorID); cerr != nil {
				return fmt.Errorf("error clearing previous cover: %w", cerr)
			}
			if cerr := images.SetCover(ctx, coverID, product.ID, actorID); cerr != nil {
				return fmt.Errorf("error setting cover: %w", cerr)
			}
			for idx := range pending {
				pending[idx].result.IsFirst = pending[idx].productImageID == coverID
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &ImportProductImagesResult{Results: results}, nil
}

type ReplaceProductImageInput struct {
	ProductID     int64
	ImageID       int64
	StorageDiskID int64
	URL           string
	ActorID       int64
}

// ReplaceProductImage swaps the file behind an existing ecom_product_images row
// for a new URL: it attaches the new URL and soft-deletes the old row, keeping
// its cover flag. All of it runs in one transaction.
func (s *Service) ReplaceProductImage(ctx context.Context, input ReplaceProductImageInput) (*ImageImportResult, error) {
	if input.ProductID <= 0 || input.ImageID <= 0 || input.StorageDiskID <= 0 {
		return nil, ErrInvalidImportPayload
	}

	url := strings.TrimSpace(input.URL)
	if url == "" {
		return nil, ErrInvalidImportPayload
	}

	if _, err := s.storageDiskRepo.FindByID(ctx, input.StorageDiskID); err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return nil, mysqlInfra.ErrStorageDiskNotFound
		}
		return nil, fmt.Errorf("error validating storage disk: %w", err)
	}

	product, err := s.productRepo.FindByID(ctx, input.ProductID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFoundForImport
		}
		return nil, fmt.Errorf("error looking up product %d: %w", input.ProductID, err)
	}

	actorID := input.ActorID
	if actorID <= 0 {
		actorID = systemImportUserID
	}

	size, contentType, checkErr := s.inspectURL(ctx, url)
	if checkErr != nil {
		return &ImageImportResult{URL: url, Error: checkErr.Error()}, nil
	}
	mimeType, extension, metaErr := resolveImageMeta(url, contentType)
	if metaErr != nil {
		return &ImageImportResult{URL: url, Error: metaErr.Error()}, nil
	}

	result := &ImageImportResult{URL: url}

	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		files := mysqlInfra.NewFilesRepository(tx)
		images := mysqlInfra.NewProductImagesRepository(tx)

		old, oerr := images.FindByID(ctx, input.ImageID)
		if oerr != nil {
			if errors.Is(oerr, mysqlInfra.ErrProductImageNotFound) {
				return ErrProductImageNotFound
			}
			return oerr
		}
		if old.ProductID != product.ID {
			return ErrProductImageNotFound
		}

		file, ferr := files.FindByPath(ctx, url)
		if ferr != nil && !errors.Is(ferr, mysqlInfra.ErrFileNotFound) {
			return fmt.Errorf("error looking up file by path %q: %w", url, ferr)
		}
		if file == nil {
			filename := fmt.Sprintf("%d_%d", product.ID, input.ImageID)
			file, ferr = files.Create(ctx, mysqlInfra.CreateFileInput{
				DiskID:    input.StorageDiskID,
				Path:      url,
				Filename:  filename,
				MimeType:  mimeType,
				FileType:  importedFileType,
				Extension: extension,
				Size:      size,
				IsPublic:  true,
				CreatedBy: actorID,
			})
			if ferr != nil {
				return fmt.Errorf("error creating file record for %q: %w", url, ferr)
			}
		}
		result.FileID = &file.ID

		if old.FileID == file.ID {
			// Same file already — nothing to swap.
			result.Success = true
			result.ProductImageID = &old.ID
			result.IsFirst = old.IsFirst
			return nil
		}

		link, lerr := images.FindByProductAndFile(ctx, product.ID, file.ID)
		if lerr != nil && !errors.Is(lerr, mysqlInfra.ErrProductImageNotFound) {
			return fmt.Errorf("error looking up product image for file %d: %w", file.ID, lerr)
		}
		if link == nil {
			link, lerr = images.Create(ctx, mysqlInfra.CreateProductImageInput{
				ProductID: product.ID,
				FileID:    file.ID,
				IsFirst:   old.IsFirst,
				CreatedBy: actorID,
			})
			if lerr != nil {
				return fmt.Errorf("error linking file %d to product %d: %w", file.ID, product.ID, lerr)
			}
		}

		if derr := images.SoftDelete(ctx, old.ID); derr != nil {
			return fmt.Errorf("error removing replaced image %d: %w", old.ID, derr)
		}

		if old.IsFirst {
			if cerr := images.ClearCover(ctx, product.ID, actorID); cerr != nil {
				return fmt.Errorf("error clearing previous cover: %w", cerr)
			}
			if cerr := images.SetCover(ctx, link.ID, product.ID, actorID); cerr != nil {
				return fmt.Errorf("error setting cover: %w", cerr)
			}
		}

		result.Success = true
		result.ProductImageID = &link.ID
		result.IsFirst = old.IsFirst
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// inspectURL issues a HEAD request to confirm the image exists at the
// given URL. It returns the reported size (from Content-Length, or 0 when the
// server does not report one) and the Content-Type header (may be empty).
func (s *Service) inspectURL(ctx context.Context, url string) (size int64, contentType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, "", fmt.Errorf("invalid image url: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("unable to reach image url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, "", errors.New("image not found at url (404)")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "", fmt.Errorf("unexpected status code %d checking image url", resp.StatusCode)
	}

	contentType = strings.TrimSpace(strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0])

	if resp.ContentLength < 0 {
		return 0, contentType, nil
	}

	return resp.ContentLength, contentType, nil
}

// imageExtensions maps the image MIME types we expect to the file extension
// stored on ecom_files.
var imageExtensions = map[string]string{
	"image/jpeg":    "jpg",
	"image/jpg":     "jpg",
	"image/png":     "png",
	"image/webp":    "webp",
	"image/gif":     "gif",
	"image/avif":    "avif",
	"image/bmp":     "bmp",
	"image/svg+xml": "svg",
	"image/tiff":    "tiff",
}

// resolveImageMeta derives (mimeType, extension) for a URL image. It trusts the
// Content-Type header when it names a known image type; otherwise it falls back
// to the extension in the URL path. It returns an error when neither source
// identifies the resource as an image, so callers reject non-image URLs the
// same way the SKU bulk import does.
func resolveImageMeta(rawURL, contentType string) (mimeType, extension string, err error) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if ext, ok := imageExtensions[contentType]; ok {
		return contentType, ext, nil
	}

	urlExt := ""
	if parsed, perr := neturl.Parse(rawURL); perr == nil {
		urlExt = strings.ToLower(strings.TrimPrefix(path.Ext(parsed.Path), "."))
	}
	for mt, ext := range imageExtensions {
		if ext == urlExt || (urlExt == "jpeg" && ext == "jpg") {
			return mt, ext, nil
		}
	}

	if strings.HasPrefix(contentType, "image/") {
		// An image type we don't have mapped — keep it, best-effort extension.
		ext := strings.TrimPrefix(contentType, "image/")
		if ext == "" {
			ext = "img"
		}
		return contentType, ext, nil
	}

	return "", "", errors.New("the url does not point to an image")
}

// --- Archivos adjuntos por URL (ecom_product_media) --------------------------

// validAttachmentTypes are the ecom_product_media.type enum values the admin
// dashboard can set from the Multimedia → "Archivos adjuntos" card.
var validAttachmentTypes = map[string]bool{
	"image":       true,
	"manual":      true,
	"datasheet":   true,
	"certificate": true,
}

// attachmentMimeToExt / extToMime cover the non-image file types that show up
// as product attachments (manuals, datasheets, certificates — mostly PDF and
// office docs). Image types fall back to imageExtensions.
var attachmentMimeToExt = map[string]string{
	"application/pdf":    "pdf",
	"application/msword": "doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"application/vnd.ms-excel": "xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "xlsx",
	"application/vnd.ms-powerpoint":                                             "ppt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
	"application/zip":              "zip",
	"application/x-zip-compressed": "zip",
	"application/rtf":              "rtf",
	"text/plain":                   "txt",
	"text/csv":                     "csv",
}

var extToMime = map[string]string{
	"jpg": "image/jpeg", "jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp",
	"gif": "image/gif", "avif": "image/avif", "bmp": "image/bmp", "svg": "image/svg+xml",
	"tiff": "image/tiff",
	"pdf":  "application/pdf", "doc": "application/msword",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"xls":  "application/vnd.ms-excel",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"ppt":  "application/vnd.ms-powerpoint",
	"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"zip":  "application/zip", "rtf": "application/rtf", "txt": "text/plain", "csv": "text/csv",
}

// resolveAttachmentMeta derives (mimeType, extension, fileType) for an
// attachment URL. Unlike resolveImageMeta it never errors — an attachment can
// be any file type; it just does its best from the Content-Type header and the
// URL extension. fileType is the ecom_files enum ("image" for image/*, else
// "document").
func resolveAttachmentMeta(rawURL, contentType, mediaType string) (mimeType, extension, fileType string) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	generic := contentType == "" ||
		contentType == "application/octet-stream" ||
		contentType == "binary/octet-stream"

	urlExt := ""
	if parsed, perr := neturl.Parse(rawURL); perr == nil {
		urlExt = strings.ToLower(strings.TrimPrefix(path.Ext(parsed.Path), "."))
	}
	if urlExt == "jpeg" {
		urlExt = "jpg"
	}

	switch {
	case !generic:
		mimeType = contentType
	case extToMime[urlExt] != "":
		mimeType = extToMime[urlExt]
	default:
		mimeType = "application/octet-stream"
	}

	switch {
	case urlExt != "":
		extension = urlExt
	case imageExtensions[mimeType] != "":
		extension = imageExtensions[mimeType]
	case attachmentMimeToExt[mimeType] != "":
		extension = attachmentMimeToExt[mimeType]
	default:
		extension = "bin"
	}

	fileType = "document"
	if mediaType == "image" || strings.HasPrefix(mimeType, "image/") {
		fileType = "image"
	}
	return mimeType, extension, fileType
}

type ProductAttachmentItemInput struct {
	URL  string
	Type string
}

type ImportAttachmentsForProductInput struct {
	ProductID     int64
	StorageDiskID int64
	Attachments   []ProductAttachmentItemInput
	ActorID       int64
}

type AttachmentImportResult struct {
	URL      string `json:"url"`
	Type     string `json:"type"`
	Position int    `json:"position"`
	Success  bool   `json:"success"`
	FileID   *int64 `json:"fileId,omitempty"`
	Error    string `json:"error,omitempty"`
}

type ImportAttachmentsResult struct {
	Results []AttachmentImportResult `json:"results"`
}

type pendingAttachment struct {
	url       string
	mediaType string
	size      int64
	mimeType  string
	extension string
	fileType  string
	result    *AttachmentImportResult
}

// ImportAttachmentsForProduct attaches file URLs (manuals, datasheets,
// certificates, images) to a product's ecom_product_media, each with its own
// type. Same shape as ImportForProduct: URLs are HEAD-checked per item outside
// the transaction, the ecom_files + ecom_product_media writes run in one tx.
func (s *Service) ImportAttachmentsForProduct(ctx context.Context, input ImportAttachmentsForProductInput) (*ImportAttachmentsResult, error) {
	if input.ProductID <= 0 || input.StorageDiskID <= 0 || len(input.Attachments) == 0 {
		return nil, ErrInvalidImportPayload
	}

	if _, err := s.storageDiskRepo.FindByID(ctx, input.StorageDiskID); err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return nil, mysqlInfra.ErrStorageDiskNotFound
		}
		return nil, fmt.Errorf("error validating storage disk: %w", err)
	}

	product, err := s.productRepo.FindByID(ctx, input.ProductID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFoundForImport
		}
		return nil, fmt.Errorf("error looking up product %d: %w", input.ProductID, err)
	}

	actorID := input.ActorID
	if actorID <= 0 {
		actorID = systemImportUserID
	}

	results := make([]AttachmentImportResult, len(input.Attachments))
	pending := make([]pendingAttachment, 0, len(input.Attachments))
	for i, item := range input.Attachments {
		url := strings.TrimSpace(item.URL)
		mediaType := strings.TrimSpace(item.Type)
		results[i] = AttachmentImportResult{URL: url, Type: mediaType, Position: i + 1}

		if url == "" {
			results[i].Error = "url is required"
			continue
		}
		if !validAttachmentTypes[mediaType] {
			results[i].Error = "invalid attachment type"
			continue
		}

		size, contentType, checkErr := s.inspectURL(ctx, url)
		if checkErr != nil {
			results[i].Error = checkErr.Error()
			continue
		}

		mimeType, extension, fileType := resolveAttachmentMeta(url, contentType, mediaType)
		pending = append(pending, pendingAttachment{
			url:       url,
			mediaType: mediaType,
			size:      size,
			mimeType:  mimeType,
			extension: extension,
			fileType:  fileType,
			result:    &results[i],
		})
	}

	if len(pending) == 0 {
		return &ImportAttachmentsResult{Results: results}, nil
	}

	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		files := mysqlInfra.NewFilesRepository(tx)
		media := mysqlInfra.NewProductMediaRepository(tx)

		for idx := range pending {
			p := &pending[idx]

			file, ferr := files.FindByPath(ctx, p.url)
			if ferr != nil && !errors.Is(ferr, mysqlInfra.ErrFileNotFound) {
				return fmt.Errorf("error looking up file by path %q: %w", p.url, ferr)
			}
			if file == nil {
				filename := fmt.Sprintf("%d_%s_%d", product.ID, p.mediaType, p.result.Position)
				file, ferr = files.Create(ctx, mysqlInfra.CreateFileInput{
					DiskID:    input.StorageDiskID,
					Path:      p.url,
					Filename:  filename,
					MimeType:  p.mimeType,
					FileType:  p.fileType,
					Extension: p.extension,
					Size:      p.size,
					IsPublic:  true,
					CreatedBy: actorID,
				})
				if ferr != nil {
					return fmt.Errorf("error creating file record for %q: %w", p.url, ferr)
				}
			}
			p.result.FileID = &file.ID

			if _, merr := media.Create(ctx, mysqlInfra.CreateProductMediaInput{
				ProductID: product.ID,
				Type:      p.mediaType,
				FileID:    file.ID,
				CreatedBy: actorID,
			}); merr != nil {
				return fmt.Errorf("error linking file %d to product %d: %w", file.ID, product.ID, merr)
			}
			p.result.Success = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &ImportAttachmentsResult{Results: results}, nil
}

type ReplaceProductAttachmentInput struct {
	ProductID     int64
	FileID        int64
	StorageDiskID int64
	URL           string
	Type          string
	ActorID       int64
}

// ReplaceProductAttachment swaps the file behind an ecom_product_media row for
// a new URL (and optionally a new type). The old row is soft-deleted.
func (s *Service) ReplaceProductAttachment(ctx context.Context, input ReplaceProductAttachmentInput) (*AttachmentImportResult, error) {
	if input.ProductID <= 0 || input.FileID <= 0 || input.StorageDiskID <= 0 {
		return nil, ErrInvalidImportPayload
	}

	url := strings.TrimSpace(input.URL)
	mediaType := strings.TrimSpace(input.Type)
	if url == "" {
		return nil, ErrInvalidImportPayload
	}
	if !validAttachmentTypes[mediaType] {
		return &AttachmentImportResult{URL: url, Type: mediaType, Error: "invalid attachment type"}, nil
	}

	if _, err := s.storageDiskRepo.FindByID(ctx, input.StorageDiskID); err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return nil, mysqlInfra.ErrStorageDiskNotFound
		}
		return nil, fmt.Errorf("error validating storage disk: %w", err)
	}

	product, err := s.productRepo.FindByID(ctx, input.ProductID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFoundForImport
		}
		return nil, fmt.Errorf("error looking up product %d: %w", input.ProductID, err)
	}

	actorID := input.ActorID
	if actorID <= 0 {
		actorID = systemImportUserID
	}

	size, contentType, checkErr := s.inspectURL(ctx, url)
	if checkErr != nil {
		return &AttachmentImportResult{URL: url, Type: mediaType, Error: checkErr.Error()}, nil
	}
	mimeType, extension, fileType := resolveAttachmentMeta(url, contentType, mediaType)

	result := &AttachmentImportResult{URL: url, Type: mediaType}

	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		files := mysqlInfra.NewFilesRepository(tx)
		media := mysqlInfra.NewProductMediaRepository(tx)

		old, oerr := media.FindByProductAndFile(ctx, product.ID, input.FileID)
		if oerr != nil {
			if errors.Is(oerr, mysqlInfra.ErrProductMediaNotFound) {
				return ErrProductMediaNotFound
			}
			return oerr
		}
		if old.DeletedAt != nil {
			return ErrProductMediaNotFound
		}

		file, ferr := files.FindByPath(ctx, url)
		if ferr != nil && !errors.Is(ferr, mysqlInfra.ErrFileNotFound) {
			return fmt.Errorf("error looking up file by path %q: %w", url, ferr)
		}
		if file == nil {
			filename := fmt.Sprintf("%d_%s_%d", product.ID, mediaType, input.FileID)
			file, ferr = files.Create(ctx, mysqlInfra.CreateFileInput{
				DiskID:    input.StorageDiskID,
				Path:      url,
				Filename:  filename,
				MimeType:  mimeType,
				FileType:  fileType,
				Extension: extension,
				Size:      size,
				IsPublic:  true,
				CreatedBy: actorID,
			})
			if ferr != nil {
				return fmt.Errorf("error creating file record for %q: %w", url, ferr)
			}
		}
		result.FileID = &file.ID

		if _, merr := media.Create(ctx, mysqlInfra.CreateProductMediaInput{
			ProductID: product.ID,
			Type:      mediaType,
			FileID:    file.ID,
			CreatedBy: actorID,
		}); merr != nil {
			return fmt.Errorf("error linking file %d to product %d: %w", file.ID, product.ID, merr)
		}

		if file.ID != input.FileID {
			if derr := media.SoftDelete(ctx, product.ID, input.FileID, actorID); derr != nil {
				return fmt.Errorf("error removing replaced attachment: %w", derr)
			}
		}

		result.Success = true
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// DeleteProductAttachment soft-deletes one ecom_product_media row
// (product_id + file_id).
func (s *Service) DeleteProductAttachment(ctx context.Context, productID, fileID, actorID int64) error {
	if productID <= 0 || fileID <= 0 {
		return ErrInvalidImportPayload
	}
	if actorID <= 0 {
		actorID = systemImportUserID
	}
	err := s.productMediaRepo.SoftDelete(ctx, productID, fileID, actorID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductMediaNotFound) {
			return ErrProductMediaNotFound
		}
		return err
	}
	return nil
}

// --- Videos por URL (ecom_product_videos) -----------------------------------

var videoMimeToExt = map[string]string{
	"video/mp4":        "mp4",
	"video/webm":       "webm",
	"video/ogg":        "ogv",
	"video/quicktime":  "mov",
	"video/x-msvideo":  "avi",
	"video/x-matroska": "mkv",
	"video/mpeg":       "mpeg",
	"video/3gpp":       "3gp",
	"video/x-flv":      "flv",
	"video/x-ms-wmv":   "wmv",
}

var videoExtToMime = map[string]string{
	"mp4": "video/mp4", "m4v": "video/mp4", "webm": "video/webm",
	"ogv": "video/ogg", "ogg": "video/ogg", "mov": "video/quicktime",
	"avi": "video/x-msvideo", "mkv": "video/x-matroska", "mpeg": "video/mpeg",
	"mpg": "video/mpeg", "3gp": "video/3gpp", "flv": "video/x-flv", "wmv": "video/x-ms-wmv",
}

// resolveVideoMeta derives (mimeType, extension) for a video URL — best-effort,
// never errors. The ecom_files.file_type is always "video" for this flow.
func resolveVideoMeta(rawURL, contentType string) (mimeType, extension string) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	generic := contentType == "" ||
		contentType == "application/octet-stream" ||
		contentType == "binary/octet-stream"

	urlExt := ""
	if parsed, perr := neturl.Parse(rawURL); perr == nil {
		urlExt = strings.ToLower(strings.TrimPrefix(path.Ext(parsed.Path), "."))
	}

	switch {
	case !generic && strings.HasPrefix(contentType, "video/"):
		mimeType = contentType
	case videoExtToMime[urlExt] != "":
		mimeType = videoExtToMime[urlExt]
	case !generic:
		mimeType = contentType
	default:
		mimeType = "video/mp4"
	}

	switch {
	case urlExt != "":
		extension = urlExt
	case videoMimeToExt[mimeType] != "":
		extension = videoMimeToExt[mimeType]
	case strings.HasPrefix(mimeType, "video/"):
		extension = strings.TrimPrefix(mimeType, "video/")
	default:
		extension = "mp4"
	}
	return mimeType, extension
}

type ImportVideosForProductInput struct {
	ProductID     int64
	StorageDiskID int64
	URLs          []string
	ActorID       int64
}

type VideoImportResult struct {
	URL      string `json:"url"`
	Position int    `json:"position"`
	Success  bool   `json:"success"`
	FileID   *int64 `json:"fileId,omitempty"`
	VideoID  *int64 `json:"videoId,omitempty"`
	Error    string `json:"error,omitempty"`
}

type ImportVideosResult struct {
	Results []VideoImportResult `json:"results"`
}

type pendingVideo struct {
	url       string
	size      int64
	mimeType  string
	extension string
	result    *VideoImportResult
}

// ImportVideosForProduct attaches video URLs to a product's ecom_product_videos.
// The simplest of the media flows: no order, no cover, no type — just the disk
// and the URLs. URLs are HEAD-checked per item outside the transaction; the
// ecom_files + ecom_product_videos writes run in one transaction.
func (s *Service) ImportVideosForProduct(ctx context.Context, input ImportVideosForProductInput) (*ImportVideosResult, error) {
	if input.ProductID <= 0 || input.StorageDiskID <= 0 || len(input.URLs) == 0 {
		return nil, ErrInvalidImportPayload
	}

	if _, err := s.storageDiskRepo.FindByID(ctx, input.StorageDiskID); err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return nil, mysqlInfra.ErrStorageDiskNotFound
		}
		return nil, fmt.Errorf("error validating storage disk: %w", err)
	}

	product, err := s.productRepo.FindByID(ctx, input.ProductID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFoundForImport
		}
		return nil, fmt.Errorf("error looking up product %d: %w", input.ProductID, err)
	}

	actorID := input.ActorID
	if actorID <= 0 {
		actorID = systemImportUserID
	}

	results := make([]VideoImportResult, len(input.URLs))
	pending := make([]pendingVideo, 0, len(input.URLs))
	for i, rawURL := range input.URLs {
		url := strings.TrimSpace(rawURL)
		results[i] = VideoImportResult{URL: url, Position: i + 1}

		if url == "" {
			results[i].Error = "url is required"
			continue
		}

		size, contentType, checkErr := s.inspectURL(ctx, url)
		if checkErr != nil {
			results[i].Error = checkErr.Error()
			continue
		}

		mimeType, extension := resolveVideoMeta(url, contentType)
		pending = append(pending, pendingVideo{
			url:       url,
			size:      size,
			mimeType:  mimeType,
			extension: extension,
			result:    &results[i],
		})
	}

	if len(pending) == 0 {
		return &ImportVideosResult{Results: results}, nil
	}

	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		files := mysqlInfra.NewFilesRepository(tx)
		videos := mysqlInfra.NewProductVideosRepository(tx)

		for idx := range pending {
			p := &pending[idx]

			file, ferr := files.FindByPath(ctx, p.url)
			if ferr != nil && !errors.Is(ferr, mysqlInfra.ErrFileNotFound) {
				return fmt.Errorf("error looking up file by path %q: %w", p.url, ferr)
			}
			if file == nil {
				filename := fmt.Sprintf("%d_video_%d", product.ID, p.result.Position)
				file, ferr = files.Create(ctx, mysqlInfra.CreateFileInput{
					DiskID:    input.StorageDiskID,
					Path:      p.url,
					Filename:  filename,
					MimeType:  p.mimeType,
					FileType:  "video",
					Extension: p.extension,
					Size:      p.size,
					IsPublic:  true,
					CreatedBy: actorID,
				})
				if ferr != nil {
					return fmt.Errorf("error creating file record for %q: %w", p.url, ferr)
				}
			}
			p.result.FileID = &file.ID

			link, lerr := videos.FindByProductAndFile(ctx, product.ID, file.ID)
			if lerr != nil && !errors.Is(lerr, mysqlInfra.ErrProductVideoNotFound) {
				return fmt.Errorf("error looking up product video for file %d: %w", file.ID, lerr)
			}
			if link == nil {
				link, lerr = videos.Create(ctx, mysqlInfra.CreateProductVideoInput{
					ProductID: product.ID,
					FileID:    file.ID,
					CreatedBy: actorID,
				})
				if lerr != nil {
					return fmt.Errorf("error linking file %d to product %d: %w", file.ID, product.ID, lerr)
				}
			}

			p.result.VideoID = &link.ID
			p.result.Success = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &ImportVideosResult{Results: results}, nil
}

type ReplaceProductVideoInput struct {
	ProductID     int64
	VideoID       int64
	StorageDiskID int64
	URL           string
	ActorID       int64
}

// ReplaceProductVideo swaps the file behind an ecom_product_videos row for a new
// URL. The old row is soft-deleted.
func (s *Service) ReplaceProductVideo(ctx context.Context, input ReplaceProductVideoInput) (*VideoImportResult, error) {
	if input.ProductID <= 0 || input.VideoID <= 0 || input.StorageDiskID <= 0 {
		return nil, ErrInvalidImportPayload
	}

	url := strings.TrimSpace(input.URL)
	if url == "" {
		return nil, ErrInvalidImportPayload
	}

	if _, err := s.storageDiskRepo.FindByID(ctx, input.StorageDiskID); err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return nil, mysqlInfra.ErrStorageDiskNotFound
		}
		return nil, fmt.Errorf("error validating storage disk: %w", err)
	}

	product, err := s.productRepo.FindByID(ctx, input.ProductID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFoundForImport
		}
		return nil, fmt.Errorf("error looking up product %d: %w", input.ProductID, err)
	}

	actorID := input.ActorID
	if actorID <= 0 {
		actorID = systemImportUserID
	}

	size, contentType, checkErr := s.inspectURL(ctx, url)
	if checkErr != nil {
		return &VideoImportResult{URL: url, Error: checkErr.Error()}, nil
	}
	mimeType, extension := resolveVideoMeta(url, contentType)

	result := &VideoImportResult{URL: url}

	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		files := mysqlInfra.NewFilesRepository(tx)
		videos := mysqlInfra.NewProductVideosRepository(tx)

		old, oerr := videos.FindByID(ctx, input.VideoID)
		if oerr != nil {
			if errors.Is(oerr, mysqlInfra.ErrProductVideoNotFound) {
				return ErrProductVideoNotFound
			}
			return oerr
		}
		if old.ProductID != product.ID {
			return ErrProductVideoNotFound
		}

		file, ferr := files.FindByPath(ctx, url)
		if ferr != nil && !errors.Is(ferr, mysqlInfra.ErrFileNotFound) {
			return fmt.Errorf("error looking up file by path %q: %w", url, ferr)
		}
		if file == nil {
			filename := fmt.Sprintf("%d_video_%d", product.ID, input.VideoID)
			file, ferr = files.Create(ctx, mysqlInfra.CreateFileInput{
				DiskID:    input.StorageDiskID,
				Path:      url,
				Filename:  filename,
				MimeType:  mimeType,
				FileType:  "video",
				Extension: extension,
				Size:      size,
				IsPublic:  true,
				CreatedBy: actorID,
			})
			if ferr != nil {
				return fmt.Errorf("error creating file record for %q: %w", url, ferr)
			}
		}
		result.FileID = &file.ID

		if old.FileID == file.ID {
			result.Success = true
			result.VideoID = &old.ID
			return nil
		}

		link, lerr := videos.FindByProductAndFile(ctx, product.ID, file.ID)
		if lerr != nil && !errors.Is(lerr, mysqlInfra.ErrProductVideoNotFound) {
			return fmt.Errorf("error looking up product video for file %d: %w", file.ID, lerr)
		}
		if link == nil {
			link, lerr = videos.Create(ctx, mysqlInfra.CreateProductVideoInput{
				ProductID: product.ID,
				FileID:    file.ID,
				CreatedBy: actorID,
			})
			if lerr != nil {
				return fmt.Errorf("error linking file %d to product %d: %w", file.ID, product.ID, lerr)
			}
		}

		if derr := videos.SoftDelete(ctx, old.ID); derr != nil {
			return fmt.Errorf("error removing replaced video %d: %w", old.ID, derr)
		}

		result.Success = true
		result.VideoID = &link.ID
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

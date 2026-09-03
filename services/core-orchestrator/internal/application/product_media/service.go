package product_media

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidProductMedia = errors.New("invalid product media")
)

// Todos los servicios de este paquete reciben *sql.DB por consistencia con el
// resto de la aplicación (ver ADR 0001). Hoy ninguna operación es
// multi-sentencia, así que ninguno abre transacción; el campo queda listo para
// cuando alguna la necesite (p. ej. reordenar imágenes = borrar + insertar).

type ProductImagesService struct {
	db         *sql.DB
	repository *mysqlInfra.ProductImagesRepository
}

func NewProductImagesService(db *sql.DB, repository *mysqlInfra.ProductImagesRepository) *ProductImagesService {
	return &ProductImagesService{db: db, repository: repository}
}

func (s *ProductImagesService) GetPaginatedImages(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedProductImages, error) {
	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *ProductImagesService) GetImageByID(ctx context.Context, id int64) (*mysqlInfra.ProductImageDTO, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *ProductImagesService) GetImagesByProduct(ctx context.Context, productID int64, offset, pageSize int) (*mysqlInfra.PaginatedProductImages, error) {
	return s.repository.FindByProductID(ctx, productID, offset, pageSize)
}

func (s *ProductImagesService) CreateImage(ctx context.Context, input mysqlInfra.CreateProductImageInput) (*mysqlInfra.ProductImageDTO, error) {
	if input.ProductID == 0 || input.FileID == 0 {
		return nil, ErrInvalidProductMedia
	}

	return s.repository.Create(ctx, input)
}

func (s *ProductImagesService) UpdateImage(ctx context.Context, id int64, input mysqlInfra.UpdateProductImageInput) (*mysqlInfra.ProductImageDTO, error) {
	return s.repository.Update(ctx, id, input)
}

func (s *ProductImagesService) DeleteImage(ctx context.Context, id int64) error {
	return s.repository.SoftDelete(ctx, id)
}

type ProductVideosService struct {
	db         *sql.DB
	repository *mysqlInfra.ProductVideosRepository
}

func NewProductVideosService(db *sql.DB, repository *mysqlInfra.ProductVideosRepository) *ProductVideosService {
	return &ProductVideosService{db: db, repository: repository}
}

func (s *ProductVideosService) GetPaginatedVideos(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedProductVideos, error) {
	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *ProductVideosService) GetVideoByID(ctx context.Context, id int64) (*mysqlInfra.ProductVideoDTO, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *ProductVideosService) GetVideosByProduct(ctx context.Context, productID int64, offset, pageSize int) (*mysqlInfra.PaginatedProductVideos, error) {
	return s.repository.FindByProductID(ctx, productID, offset, pageSize)
}

func (s *ProductVideosService) CreateVideo(ctx context.Context, input mysqlInfra.CreateProductVideoInput) (*mysqlInfra.ProductVideoDTO, error) {
	if input.ProductID == 0 || input.FileID == 0 {
		return nil, ErrInvalidProductMedia
	}

	return s.repository.Create(ctx, input)
}

func (s *ProductVideosService) UpdateVideo(ctx context.Context, id int64, input mysqlInfra.UpdateProductVideoInput) (*mysqlInfra.ProductVideoDTO, error) {
	return s.repository.Update(ctx, id, input)
}

func (s *ProductVideosService) DeleteVideo(ctx context.Context, id int64) error {
	return s.repository.SoftDelete(ctx, id)
}

type ProductPartNumbersService struct {
	db         *sql.DB
	repository *mysqlInfra.ProductPartNumbersRepository
}

func NewProductPartNumbersService(db *sql.DB, repository *mysqlInfra.ProductPartNumbersRepository) *ProductPartNumbersService {
	return &ProductPartNumbersService{db: db, repository: repository}
}

func (s *ProductPartNumbersService) GetPaginatedPartNumbers(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedProductPartNumbers, error) {
	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *ProductPartNumbersService) GetPartNumbersByProduct(ctx context.Context, productID int64, offset, pageSize int) (*mysqlInfra.PaginatedProductPartNumbers, error) {
	return s.repository.FindByProductID(ctx, productID, offset, pageSize)
}

func (s *ProductPartNumbersService) CreatePartNumber(ctx context.Context, input mysqlInfra.CreateProductPartNumberInput) (*mysqlInfra.ProductPartNumberDTO, error) {
	if input.ProductID == 0 || input.PartNumber == "" {
		return nil, ErrInvalidProductMedia
	}

	return s.repository.Create(ctx, input)
}

func (s *ProductPartNumbersService) UpdatePartNumber(ctx context.Context, productID int64, partNumber string, input mysqlInfra.UpdateProductPartNumberInput) (*mysqlInfra.ProductPartNumberDTO, error) {
	return s.repository.Update(ctx, productID, partNumber, input)
}

func (s *ProductPartNumbersService) DeletePartNumber(ctx context.Context, productID int64, partNumber string) error {
	return s.repository.SoftDelete(ctx, productID, partNumber)
}

type ProductDimensionsService struct {
	db                *sql.DB
	repository        *mysqlInfra.ProductDimensionsRepository
	productRepository *mysqlInfra.ProductRepository
}

func NewProductDimensionsService(db *sql.DB, repository *mysqlInfra.ProductDimensionsRepository, productRepository *mysqlInfra.ProductRepository) *ProductDimensionsService {
	return &ProductDimensionsService{db: db, repository: repository, productRepository: productRepository}
}

func (s *ProductDimensionsService) GetDimensionsByProduct(ctx context.Context, productID int64) (*mysqlInfra.ProductDimensionsDTO, error) {
	return s.repository.FindByProductID(ctx, productID)
}

func (s *ProductDimensionsService) CreateDimensions(ctx context.Context, input mysqlInfra.CreateProductDimensionsInput) (*mysqlInfra.ProductDimensionsDTO, error) {
	if input.ProductID == 0 {
		return nil, ErrInvalidProductMedia
	}

	return s.repository.Create(ctx, input)
}

func (s *ProductDimensionsService) UpdateDimensions(ctx context.Context, productID int64, input mysqlInfra.UpdateProductDimensionsInput) (*mysqlInfra.ProductDimensionsDTO, error) {
	return s.repository.Update(ctx, productID, input)
}

func (s *ProductDimensionsService) DeleteDimensions(ctx context.Context, productID int64) error {
	return s.repository.SoftDelete(ctx, productID)
}

type BulkDimensionItem struct {
	SKU      string
	Weight   *float64
	Length   *float64
	Width    *float64
	Height   *float64
	Diameter *float64
}

type BulkDimensionStatus string

const (
	BulkDimensionCreated  BulkDimensionStatus = "created"
	BulkDimensionUpdated  BulkDimensionStatus = "updated"
	BulkDimensionNotFound BulkDimensionStatus = "not_found"
	BulkDimensionError    BulkDimensionStatus = "error"
)

type BulkDimensionResult struct {
	Index      int                              `json:"index"`
	SKU        string                           `json:"sku"`
	Status     BulkDimensionStatus              `json:"status"`
	ProductID  int64                            `json:"productId,omitempty"`
	Dimensions *mysqlInfra.ProductDimensionsDTO `json:"dimensions,omitempty"`
	Error      string                           `json:"error,omitempty"`
}

// BulkUpsertDimensions resolves each item's product by SKU and creates or
// updates its dimensions row. Weight/length/width/height default to 1 when
// missing or null; diameter defaults to width since it doesn't apply to all
// products. Volume is never accepted from the caller — it's always
// recalculated from length/width/height (measurements are assumed to always
// be uploaded in cm/kg).
//
// Cada ítem hace dos lecturas (producto por SKU, dimensiones por producto) y
// UNA sola escritura (create o update): una escritura es atómica por sí sola,
// así que no se abre transacción. Mantiene la semántica de éxito parcial.
func (s *ProductDimensionsService) BulkUpsertDimensions(ctx context.Context, items []BulkDimensionItem, actorID int64) ([]BulkDimensionResult, error) {
	results := make([]BulkDimensionResult, 0, len(items))

	for i, item := range items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" {
			results = append(results, BulkDimensionResult{Index: i, SKU: sku, Status: BulkDimensionError, Error: "sku is required"})
			continue
		}

		product, err := s.productRepository.FindBySKU(ctx, sku)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrProductNotFound) {
				results = append(results, BulkDimensionResult{Index: i, SKU: sku, Status: BulkDimensionNotFound, Error: "product not found for sku"})
				continue
			}
			results = append(results, BulkDimensionResult{Index: i, SKU: sku, Status: BulkDimensionError, Error: err.Error()})
			continue
		}

		weight := dimensionOrDefault(item.Weight, 1)
		length := dimensionOrDefault(item.Length, 1)
		width := dimensionOrDefault(item.Width, 1)
		height := dimensionOrDefault(item.Height, 1)
		diameter := width
		if item.Diameter != nil {
			diameter = *item.Diameter
		}
		volume := length * width * height

		_, err = s.repository.FindByProductID(ctx, product.ID)
		if err != nil {
			if !errors.Is(err, mysqlInfra.ErrProductDimensionsNotFound) {
				results = append(results, BulkDimensionResult{Index: i, SKU: sku, ProductID: product.ID, Status: BulkDimensionError, Error: err.Error()})
				continue
			}

			created, createErr := s.repository.Create(ctx, mysqlInfra.CreateProductDimensionsInput{
				ProductID: product.ID,
				Weight:    formatDimension(weight),
				Length:    formatDimension(length),
				Width:     formatDimension(width),
				Height:    formatDimension(height),
				Diameter:  formatDimension(diameter),
				Volume:    formatDimension(volume),
				CreatedBy: actorID,
			})
			if createErr != nil {
				results = append(results, BulkDimensionResult{Index: i, SKU: sku, ProductID: product.ID, Status: BulkDimensionError, Error: createErr.Error()})
				continue
			}
			results = append(results, BulkDimensionResult{Index: i, SKU: sku, ProductID: product.ID, Status: BulkDimensionCreated, Dimensions: created})
			continue
		}

		updated, updateErr := s.repository.Update(ctx, product.ID, mysqlInfra.UpdateProductDimensionsInput{
			Weight:    formatDimension(weight),
			Length:    formatDimension(length),
			Width:     formatDimension(width),
			Height:    formatDimension(height),
			Diameter:  formatDimension(diameter),
			Volume:    formatDimension(volume),
			UpdatedBy: actorID,
		})
		if updateErr != nil {
			results = append(results, BulkDimensionResult{Index: i, SKU: sku, ProductID: product.ID, Status: BulkDimensionError, Error: updateErr.Error()})
			continue
		}
		results = append(results, BulkDimensionResult{Index: i, SKU: sku, ProductID: product.ID, Status: BulkDimensionUpdated, Dimensions: updated})
	}

	return results, nil
}

func dimensionOrDefault(v *float64, def float64) float64 {
	if v == nil {
		return def
	}
	return *v
}

func formatDimension(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

type ProductSEOService struct {
	db         *sql.DB
	repository *mysqlInfra.ProductSEORepository
}

func NewProductSEOService(db *sql.DB, repository *mysqlInfra.ProductSEORepository) *ProductSEOService {
	return &ProductSEOService{db: db, repository: repository}
}

func (s *ProductSEOService) GetSEOByProduct(ctx context.Context, productID int64) (*mysqlInfra.ProductSEODTO, error) {
	return s.repository.FindByProductID(ctx, productID)
}

func (s *ProductSEOService) CreateSEO(ctx context.Context, input mysqlInfra.CreateProductSEOInput) (*mysqlInfra.ProductSEODTO, error) {
	if input.ProductID == 0 {
		return nil, ErrInvalidProductMedia
	}

	return s.repository.Create(ctx, input)
}

func (s *ProductSEOService) UpdateSEO(ctx context.Context, productID int64, input mysqlInfra.UpdateProductSEOInput) (*mysqlInfra.ProductSEODTO, error) {
	return s.repository.Update(ctx, productID, input)
}

func (s *ProductSEOService) DeleteSEO(ctx context.Context, productID int64) error {
	return s.repository.SoftDelete(ctx, productID)
}

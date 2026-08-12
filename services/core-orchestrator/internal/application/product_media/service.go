package product_media

import (
	"errors"
	"strconv"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidProductMedia = errors.New("invalid product media")
)

type ProductImagesService struct {
	repository *mysqlInfra.ProductImagesRepository
}

func NewProductImagesService(repository *mysqlInfra.ProductImagesRepository) *ProductImagesService {
	return &ProductImagesService{repository: repository}
}

func (s *ProductImagesService) GetPaginatedImages(offset, pageSize int) (*mysqlInfra.PaginatedProductImages, error) {
	return s.repository.FindPaginated(offset, pageSize)
}

func (s *ProductImagesService) GetImageByID(id int64) (*mysqlInfra.ProductImageDTO, error) {
	return s.repository.FindByID(id)
}

func (s *ProductImagesService) GetImagesByProduct(productID int64, offset, pageSize int) (*mysqlInfra.PaginatedProductImages, error) {
	return s.repository.FindByProductID(productID, offset, pageSize)
}

func (s *ProductImagesService) CreateImage(input mysqlInfra.CreateProductImageInput) (*mysqlInfra.ProductImageDTO, error) {
	if input.ProductID == 0 || input.FileID == 0 {
		return nil, ErrInvalidProductMedia
	}

	return s.repository.Create(input)
}

func (s *ProductImagesService) UpdateImage(id int64, input mysqlInfra.UpdateProductImageInput) (*mysqlInfra.ProductImageDTO, error) {
	return s.repository.Update(id, input)
}

func (s *ProductImagesService) DeleteImage(id int64) error {
	return s.repository.SoftDelete(id)
}

type ProductVideosService struct {
	repository *mysqlInfra.ProductVideosRepository
}

func NewProductVideosService(repository *mysqlInfra.ProductVideosRepository) *ProductVideosService {
	return &ProductVideosService{repository: repository}
}

func (s *ProductVideosService) GetPaginatedVideos(offset, pageSize int) (*mysqlInfra.PaginatedProductVideos, error) {
	return s.repository.FindPaginated(offset, pageSize)
}

func (s *ProductVideosService) GetVideoByID(id int64) (*mysqlInfra.ProductVideoDTO, error) {
	return s.repository.FindByID(id)
}

func (s *ProductVideosService) GetVideosByProduct(productID int64, offset, pageSize int) (*mysqlInfra.PaginatedProductVideos, error) {
	return s.repository.FindByProductID(productID, offset, pageSize)
}

func (s *ProductVideosService) CreateVideo(input mysqlInfra.CreateProductVideoInput) (*mysqlInfra.ProductVideoDTO, error) {
	if input.ProductID == 0 || input.FileID == 0 {
		return nil, ErrInvalidProductMedia
	}

	return s.repository.Create(input)
}

func (s *ProductVideosService) UpdateVideo(id int64, input mysqlInfra.UpdateProductVideoInput) (*mysqlInfra.ProductVideoDTO, error) {
	return s.repository.Update(id, input)
}

func (s *ProductVideosService) DeleteVideo(id int64) error {
	return s.repository.SoftDelete(id)
}

type ProductPartNumbersService struct {
	repository *mysqlInfra.ProductPartNumbersRepository
}

func NewProductPartNumbersService(repository *mysqlInfra.ProductPartNumbersRepository) *ProductPartNumbersService {
	return &ProductPartNumbersService{repository: repository}
}

func (s *ProductPartNumbersService) GetPaginatedPartNumbers(offset, pageSize int) (*mysqlInfra.PaginatedProductPartNumbers, error) {
	return s.repository.FindPaginated(offset, pageSize)
}

func (s *ProductPartNumbersService) GetPartNumbersByProduct(productID int64, offset, pageSize int) (*mysqlInfra.PaginatedProductPartNumbers, error) {
	return s.repository.FindByProductID(productID, offset, pageSize)
}

func (s *ProductPartNumbersService) CreatePartNumber(input mysqlInfra.CreateProductPartNumberInput) (*mysqlInfra.ProductPartNumberDTO, error) {
	if input.ProductID == 0 || input.PartNumber == "" {
		return nil, ErrInvalidProductMedia
	}

	return s.repository.Create(input)
}

func (s *ProductPartNumbersService) UpdatePartNumber(productID int64, partNumber string, input mysqlInfra.UpdateProductPartNumberInput) (*mysqlInfra.ProductPartNumberDTO, error) {
	return s.repository.Update(productID, partNumber, input)
}

func (s *ProductPartNumbersService) DeletePartNumber(productID int64, partNumber string) error {
	return s.repository.SoftDelete(productID, partNumber)
}

type ProductDimensionsService struct {
	repository        *mysqlInfra.ProductDimensionsRepository
	productRepository *mysqlInfra.ProductRepository
}

func NewProductDimensionsService(repository *mysqlInfra.ProductDimensionsRepository, productRepository *mysqlInfra.ProductRepository) *ProductDimensionsService {
	return &ProductDimensionsService{repository: repository, productRepository: productRepository}
}

func (s *ProductDimensionsService) GetDimensionsByProduct(productID int64) (*mysqlInfra.ProductDimensionsDTO, error) {
	return s.repository.FindByProductID(productID)
}

func (s *ProductDimensionsService) CreateDimensions(input mysqlInfra.CreateProductDimensionsInput) (*mysqlInfra.ProductDimensionsDTO, error) {
	if input.ProductID == 0 {
		return nil, ErrInvalidProductMedia
	}

	return s.repository.Create(input)
}

func (s *ProductDimensionsService) UpdateDimensions(productID int64, input mysqlInfra.UpdateProductDimensionsInput) (*mysqlInfra.ProductDimensionsDTO, error) {
	return s.repository.Update(productID, input)
}

func (s *ProductDimensionsService) DeleteDimensions(productID int64) error {
	return s.repository.SoftDelete(productID)
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
func (s *ProductDimensionsService) BulkUpsertDimensions(items []BulkDimensionItem, actorID int64) ([]BulkDimensionResult, error) {
	results := make([]BulkDimensionResult, 0, len(items))

	for i, item := range items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" {
			results = append(results, BulkDimensionResult{Index: i, SKU: sku, Status: BulkDimensionError, Error: "sku is required"})
			continue
		}

		product, err := s.productRepository.FindBySKU(sku)
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

		_, err = s.repository.FindByProductID(product.ID)
		if err != nil {
			if !errors.Is(err, mysqlInfra.ErrProductDimensionsNotFound) {
				results = append(results, BulkDimensionResult{Index: i, SKU: sku, ProductID: product.ID, Status: BulkDimensionError, Error: err.Error()})
				continue
			}

			created, createErr := s.repository.Create(mysqlInfra.CreateProductDimensionsInput{
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

		updated, updateErr := s.repository.Update(product.ID, mysqlInfra.UpdateProductDimensionsInput{
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
	repository *mysqlInfra.ProductSEORepository
}

func NewProductSEOService(repository *mysqlInfra.ProductSEORepository) *ProductSEOService {
	return &ProductSEOService{repository: repository}
}

func (s *ProductSEOService) GetSEOByProduct(productID int64) (*mysqlInfra.ProductSEODTO, error) {
	return s.repository.FindByProductID(productID)
}

func (s *ProductSEOService) CreateSEO(input mysqlInfra.CreateProductSEOInput) (*mysqlInfra.ProductSEODTO, error) {
	if input.ProductID == 0 {
		return nil, ErrInvalidProductMedia
	}

	return s.repository.Create(input)
}

func (s *ProductSEOService) UpdateSEO(productID int64, input mysqlInfra.UpdateProductSEOInput) (*mysqlInfra.ProductSEODTO, error) {
	return s.repository.Update(productID, input)
}

func (s *ProductSEOService) DeleteSEO(productID int64) error {
	return s.repository.SoftDelete(productID)
}

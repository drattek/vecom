package product_image_import

import (
	"errors"
	"fmt"
	"net/http"
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
	productRepo       *mysqlInfra.ProductRepository
	filesRepo         *mysqlInfra.FilesRepository
	productImagesRepo *mysqlInfra.ProductImagesRepository
	storageDiskRepo   *mysqlInfra.StorageDiskRepository
	httpClient        *http.Client
}

func NewService(
	productRepo *mysqlInfra.ProductRepository,
	filesRepo *mysqlInfra.FilesRepository,
	productImagesRepo *mysqlInfra.ProductImagesRepository,
	storageDiskRepo *mysqlInfra.StorageDiskRepository,
) *Service {
	return &Service{
		productRepo:       productRepo,
		filesRepo:         filesRepo,
		productImagesRepo: productImagesRepo,
		storageDiskRepo:   storageDiskRepo,
		httpClient:        &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Service) Import(input ImportProductImagesInput) (*ImportProductImagesResult, error) {
	if input.StorageDiskID <= 0 || len(input.Products) == 0 {
		return nil, ErrInvalidImportPayload
	}

	if _, err := s.storageDiskRepo.FindByID(input.StorageDiskID); err != nil {
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

		product, err := s.productRepo.FindBySKU(sku)
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

			file, err := s.filesRepo.FindByPath(url)
			if err != nil && !errors.Is(err, mysqlInfra.ErrFileNotFound) {
				return nil, fmt.Errorf("error looking up file by path %q: %w", url, err)
			}

			if file == nil {
				size, checkErr := s.checkImageURL(url)
				if checkErr != nil {
					result.Error = checkErr.Error()
					results = append(results, result)
					continue
				}

				filename := fmt.Sprintf("%s_%d", sku, position)

				file, err = s.filesRepo.Create(mysqlInfra.CreateFileInput{
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
			existingProductImage, err := s.productImagesRepo.FindByProductAndFile(product.ID, file.ID)
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

			productImage, err := s.productImagesRepo.Create(mysqlInfra.CreateProductImageInput{
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

// checkImageURL issues a HEAD request to confirm the image exists at the
// given URL. It returns the reported size (from Content-Length) when
// available, or 0 if the server does not report one.
func (s *Service) checkImageURL(url string) (int64, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return 0, fmt.Errorf("invalid image url: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("unable to reach image url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, errors.New("image not found at url (404)")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("unexpected status code %d checking image url", resp.StatusCode)
	}

	if resp.ContentLength < 0 {
		return 0, nil
	}

	return resp.ContentLength, nil
}

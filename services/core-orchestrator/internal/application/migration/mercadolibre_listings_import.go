package migration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	productCategorySelectionApp "core-orchestrator/internal/application/product_category_selection"
	productImageImportApp "core-orchestrator/internal/application/product_image_import"
	syncApp "core-orchestrator/internal/application/sync"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// Defaults stamped on an ecom_products row auto-created by this import.
// Mirrors auditProductBrandID/auditProductSourceID/auditProductType in
// sync_mercadolibre_listings_audit.go — same product decision (brand_id 1,
// source_id 13, product_type "part"), fixed rather than configurable.
const (
	meliListingsImportBrandID     int64 = 1
	meliListingsImportSourceID    int64 = 13
	meliListingsImportProductType       = "part"
	// meliListingsImportStorageDiskID mirrors vecomImagesDiskID in
	// vecom_images.go — the only ecom_storage_disks row that exists today.
	meliListingsImportStorageDiskID int64 = 1
)

var ErrInvalidListingsImportInput = errors.New("connectionId and products are required")

// MercadoLibreListingsImportService is a TEMPORARY, one-off import endpoint:
// given a connectionId and a batch of listings that already exist on
// MercadoLibre but have no local ecom_products row yet, it, per item:
//
//  1. resolves the local product by sku, creating it (part_number, name,
//     description from the payload; brand_id 1, product_type "part",
//     source_id 13) when it doesn't exist yet;
//  2. replaces the product's images with the ones provided (soft-deleting
//     any existing ecom_product_images row first), marking the first URL as
//     the cover — see product_image_import.Service.ImportForProduct;
//  3. records the MercadoLibre category for this connection via
//     product_category_selection.Service.Select (ADR 0005), which also seeds
//     the category's required-attribute slots;
//  4. downloads the listing's vehicle compatibilities from MercadoLibre
//     (by meli_id) and links them locally — see
//     MercadoLibreCompatibilityService.CopyCompatibilitiesForProduct.
//
// Nothing is ever written back to MercadoLibre. Remove this service, its
// handler method and its route once the backlog it's for has been imported.
type MercadoLibreListingsImportService struct {
	productRepository               *mysqlInfra.ProductRepository
	productImagesRepository         *mysqlInfra.ProductImagesRepository
	productImageImportService       *productImageImportApp.Service
	productCategorySelectionService *productCategorySelectionApp.Service
	channelConnectionRepository     *mysqlInfra.ChannelConnectionRepository
	channelRepository               *mysqlInfra.ChannelRepository
	compatibilityService            *syncApp.MercadoLibreCompatibilityService
	tokenService                    *syncApp.MercadoLibreTokenService
}

func NewMercadoLibreListingsImportService(
	productRepository *mysqlInfra.ProductRepository,
	productImagesRepository *mysqlInfra.ProductImagesRepository,
	productImageImportService *productImageImportApp.Service,
	productCategorySelectionService *productCategorySelectionApp.Service,
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	compatibilityService *syncApp.MercadoLibreCompatibilityService,
	tokenService *syncApp.MercadoLibreTokenService,
) *MercadoLibreListingsImportService {
	return &MercadoLibreListingsImportService{
		productRepository:               productRepository,
		productImagesRepository:         productImagesRepository,
		productImageImportService:       productImageImportService,
		productCategorySelectionService: productCategorySelectionService,
		channelConnectionRepository:     channelConnectionRepository,
		channelRepository:               channelRepository,
		compatibilityService:            compatibilityService,
		tokenService:                    tokenService,
	}
}

// ImportListingItemInput is one MercadoLibre listing to import — field names
// mirror the marketplace's own vocabulary (meli_id, category_id) rather than
// this system's, since that's what the caller has on hand.
type ImportListingItemInput struct {
	SKU         string
	Name        string
	MeliID      string
	CategoryID  string
	PartNumber  string
	Description string
	Images      []string
}

type ImportListingsInput struct {
	ConnectionID int64
	Items        []ImportListingItemInput
	ActorID      int64
}

// ListingImportOutcome reports what happened for one item. A batch never
// fails wholesale on one bad item.
type ListingImportOutcome struct {
	SKU            string `json:"sku"`
	MeliID         string `json:"meliId,omitempty"`
	ProductID      *int64 `json:"productId,omitempty"`
	ProductCreated bool   `json:"productCreated,omitempty"`

	ImagesImported int      `json:"imagesImported,omitempty"`
	ImagesReplaced bool     `json:"imagesReplaced,omitempty"`
	ImageErrors    []string `json:"imageErrors,omitempty"`

	CategorySelected bool   `json:"categorySelected,omitempty"`
	LocalCategoryID  *int64 `json:"localCategoryId,omitempty"`

	CompatibilitiesLinked int                                     `json:"compatibilitiesLinked,omitempty"`
	Compatibilities       []syncApp.CopyLocalCompatibilityOutcome `json:"compatibilities,omitempty"`
	NewVehicleFitments    []syncApp.CreatedVehicleFitment         `json:"newVehicleFitments,omitempty"`

	Error string `json:"error,omitempty"`
}

type ImportListingsResult struct {
	Total    int                    `json:"total"`
	Imported int                    `json:"imported"`
	Errored  int                    `json:"errored"`
	Results  []ListingImportOutcome `json:"results"`
}

// Import validates connectionID is a live MERCADOLIBRE connection, then walks
// input.Items one at a time (see the service doc comment for the four steps
// per item).
func (s *MercadoLibreListingsImportService) Import(ctx context.Context, input ImportListingsInput) (*ImportListingsResult, error) {
	if input.ConnectionID <= 0 || len(input.Items) == 0 {
		return nil, ErrInvalidListingsImportInput
	}
	if input.ActorID <= 0 {
		return nil, ErrInvalidListingsImportInput
	}

	connection, err := s.channelConnectionRepository.FindByID(ctx, input.ConnectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return nil, mysqlInfra.ErrChannelConnectionNotFound
		}
		return nil, fmt.Errorf("error loading connection %d: %w", input.ConnectionID, err)
	}

	channel, err := s.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}
	if !strings.EqualFold(strings.TrimSpace(channel.Code), "MERCADOLIBRE") {
		return nil, syncApp.ErrNotMercadoLibreConnection
	}

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, input.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("error getting mercadolibre access token: %w", err)
	}

	result := &ImportListingsResult{Total: len(input.Items), Results: make([]ListingImportOutcome, 0, len(input.Items))}
	for _, item := range input.Items {
		outcome := s.importOne(ctx, input.ConnectionID, accessToken, item, input.ActorID)
		if outcome.Error != "" {
			result.Errored++
		} else {
			result.Imported++
		}
		result.Results = append(result.Results, outcome)
	}

	return result, nil
}

func (s *MercadoLibreListingsImportService) importOne(ctx context.Context, connectionID int64, accessToken string, item ImportListingItemInput, actorID int64) ListingImportOutcome {
	sku := strings.TrimSpace(item.SKU)
	meliID := strings.TrimSpace(item.MeliID)
	outcome := ListingImportOutcome{SKU: sku, MeliID: meliID}

	partNumber := strings.TrimSpace(item.PartNumber)
	name := strings.TrimSpace(item.Name)
	if sku == "" || partNumber == "" || name == "" {
		outcome.Error = "sku, part_number and name are required"
		return outcome
	}

	product, created, err := s.resolveOrCreateProduct(ctx, item, sku, partNumber, name, actorID)
	if err != nil {
		outcome.Error = err.Error()
		return outcome
	}
	outcome.ProductID = &product.ID
	outcome.ProductCreated = created

	if len(item.Images) > 0 {
		imported, ierr := s.replaceProductImages(ctx, product.ID, item.Images, actorID)
		if ierr != nil {
			outcome.ImageErrors = append(outcome.ImageErrors, ierr.Error())
		} else {
			outcome.ImagesImported = imported
			outcome.ImagesReplaced = true
		}
	}

	categoryID := strings.TrimSpace(item.CategoryID)
	if categoryID != "" {
		selection, serr := s.productCategorySelectionService.Select(ctx, product.ID, connectionID, categoryID, actorID)
		if serr != nil {
			outcome.Error = fmt.Sprintf("error selecting category %q: %v", categoryID, serr)
			return outcome
		}
		outcome.CategorySelected = true
		outcome.LocalCategoryID = &selection.CategoryID
	}

	if meliID != "" {
		results, newFitments, cerr := s.compatibilityService.CopyCompatibilitiesForProduct(ctx, accessToken, meliID, product.ID, actorID)
		if cerr != nil {
			outcome.Error = fmt.Sprintf("error copying compatibilities for %q: %v", meliID, cerr)
			return outcome
		}
		linked := 0
		for _, r := range results {
			if r.CompatibilityID != 0 {
				linked++
			}
		}
		outcome.CompatibilitiesLinked = linked
		outcome.Compatibilities = results
		outcome.NewVehicleFitments = newFitments
	}

	return outcome
}

// resolveOrCreateProduct returns the ecom_products row for sku, creating it
// from the payload (part_number/name/description, brand_id/product_type/
// source_id fixed per meliListingsImport* constants) when it doesn't exist
// yet. created reports whether this call inserted it.
func (s *MercadoLibreListingsImportService) resolveOrCreateProduct(ctx context.Context, item ImportListingItemInput, sku, partNumber, name string, actorID int64) (*mysqlInfra.ProductDTO, bool, error) {
	existing, err := s.productRepository.FindBySKU(ctx, sku)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, mysqlInfra.ErrProductNotFound) {
		return nil, false, fmt.Errorf("error loading product by sku %s: %w", sku, err)
	}

	brandID := meliListingsImportBrandID
	var description *string
	if d := strings.TrimSpace(item.Description); d != "" {
		description = &d
	}

	newProduct, err := s.productRepository.Create(ctx, mysqlInfra.CreateProductInput{
		SKU:         sku,
		PartNumber:  partNumber,
		Name:        name,
		Description: description,
		BrandID:     &brandID,
		ProductType: meliListingsImportProductType,
		IsSellable:  true,
		IsStockable: true,
		SourceID:    meliListingsImportSourceID,
		CreatedBy:   actorID,
	})
	if err != nil {
		if recovered, recErr := s.productRepository.FindBySKU(ctx, sku); recErr == nil {
			return recovered, false, nil
		}
		return nil, false, fmt.Errorf("error creating product for sku %s: %w", sku, err)
	}

	return newProduct, true, nil
}

// replaceProductImages soft-deletes every existing ecom_product_images row
// for productID and imports urls in order, marking the first one as the
// cover — see product_image_import.Service.ImportForProduct. It returns the
// count of images successfully imported.
func (s *MercadoLibreListingsImportService) replaceProductImages(ctx context.Context, productID int64, urls []string, actorID int64) (int, error) {
	existing, err := s.productImagesRepository.FindAllByProductID(ctx, productID)
	if err != nil {
		return 0, fmt.Errorf("error loading existing product images: %w", err)
	}
	for _, img := range existing {
		if err := s.productImagesRepository.SoftDelete(ctx, img.ID); err != nil {
			return 0, fmt.Errorf("error removing existing product image %d: %w", img.ID, err)
		}
	}

	images := make([]productImageImportApp.ProductImageItemInput, 0, len(urls))
	for _, rawURL := range urls {
		url := strings.TrimSpace(rawURL)
		if url == "" {
			continue
		}
		images = append(images, productImageImportApp.ProductImageItemInput{URL: url, IsFirst: len(images) == 0})
	}
	if len(images) == 0 {
		return 0, nil
	}

	result, err := s.productImageImportService.ImportForProduct(ctx, productImageImportApp.ImportForProductInput{
		ProductID:     productID,
		StorageDiskID: meliListingsImportStorageDiskID,
		Images:        images,
		ActorID:       actorID,
	})
	if err != nil {
		return 0, err
	}

	imported := 0
	for _, r := range result.Results {
		if !r.Success {
			return imported, fmt.Errorf("error importing image %s: %s", r.URL, r.Error)
		}
		imported++
	}
	return imported, nil
}

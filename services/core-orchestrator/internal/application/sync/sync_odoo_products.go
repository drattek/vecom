package sync

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	pricingApp "core-orchestrator/internal/application/pricing"
	odooInfra "core-orchestrator/internal/infrastructure/marketplace/odoo"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/workers"
)

// systemOdooSyncActorID is recorded as the actor for ecom_channel_product_map
// writes made by this background flow, since there is no authenticated user
// behind an automated sync (mirrors product_image_import's systemImportUserID).
const systemOdooSyncActorID int64 = 1

const (
	odooProductType   = "consu"
	odooTaxID         = int64(19)
	odooDefaultWeight = 1.0
	odooDefaultVolume = 1.0
	odooSyncCurrency  = "MXN" // marketplace channels always receive prices in MXN
	odooSyncedStatus  = "synced"

	// odooCM3PerM3 converts ecom_product_dimensions.volume — always stored in
	// cm³, since length/width/height are always uploaded in cm — into Odoo's
	// product.template volume field, which Odoo stores in m³.
	odooCM3PerM3 = 1_000_000.0
)

// odooRefreshPriceStockGateEnabled gates Refresh so it only calls out to Odoo
// when product's price or stock actually changed since listing.LastSyncedAt
// (priceOrStockChangedSince below) — see mercadoLibreRefreshPriceStockGateEnabled
// in sync_mercadolibre_products.go for the same gate on MercadoLibre's
// Refresh, toggled independently.
const odooRefreshPriceStockGateEnabled = true

// OdooProductSyncService pushes a single product's current price/stock/
// images/category to Odoo, creating the product.template on first sync and
// only refreshing qty_available/list_price on later ones. It implements
// workers.ProductSyncer (duck-typed: no import from workers is needed here).
type OdooProductSyncService struct {
	credentialsRepository        *mysqlInfra.ConnectionCredentialsRepository
	settingsRepository           *mysqlInfra.ConnectionSettingsRepository
	productRepository            *mysqlInfra.ProductRepository
	productPricesRepository      *mysqlInfra.ProductPricesRepository
	productStockRepository       *mysqlInfra.ProductStockRepository
	productDimensionsRepository  *mysqlInfra.ProductDimensionsRepository
	productImagesRepository      *mysqlInfra.ProductImagesRepository
	filesRepository              *mysqlInfra.FilesRepository
	currenciesRepository         *mysqlInfra.CurrenciesRepository
	categoriesRepository         *mysqlInfra.CategoriesRepository
	channelCategoryMapRepository *mysqlInfra.ChannelCategoryMapRepository
	channelProductMapRepository  *mysqlInfra.ChannelProductMapRepository
	rateLimiter                  *odooInfra.RateLimiter
	httpClient                   *http.Client
	formulaCalculator            *pricingApp.PricingFormulaCalculator
	effectivePriceResolver       *pricingApp.EffectivePriceResolver
}

func NewOdooProductSyncService(
	credentialsRepository *mysqlInfra.ConnectionCredentialsRepository,
	settingsRepository *mysqlInfra.ConnectionSettingsRepository,
	productRepository *mysqlInfra.ProductRepository,
	productPricesRepository *mysqlInfra.ProductPricesRepository,
	productStockRepository *mysqlInfra.ProductStockRepository,
	productDimensionsRepository *mysqlInfra.ProductDimensionsRepository,
	productImagesRepository *mysqlInfra.ProductImagesRepository,
	filesRepository *mysqlInfra.FilesRepository,
	currenciesRepository *mysqlInfra.CurrenciesRepository,
	categoriesRepository *mysqlInfra.CategoriesRepository,
	channelCategoryMapRepository *mysqlInfra.ChannelCategoryMapRepository,
	channelProductMapRepository *mysqlInfra.ChannelProductMapRepository,
	rateLimiter *odooInfra.RateLimiter,
	formulaCalculator *pricingApp.PricingFormulaCalculator,
	effectivePriceResolver *pricingApp.EffectivePriceResolver,
) *OdooProductSyncService {
	return &OdooProductSyncService{
		credentialsRepository:        credentialsRepository,
		settingsRepository:           settingsRepository,
		productRepository:            productRepository,
		productPricesRepository:      productPricesRepository,
		productStockRepository:       productStockRepository,
		productDimensionsRepository:  productDimensionsRepository,
		productImagesRepository:      productImagesRepository,
		filesRepository:              filesRepository,
		currenciesRepository:         currenciesRepository,
		categoriesRepository:         categoriesRepository,
		channelCategoryMapRepository: channelCategoryMapRepository,
		channelProductMapRepository:  channelProductMapRepository,
		rateLimiter:                  rateLimiter,
		httpClient:                   &http.Client{Timeout: 20 * time.Second},
		formulaCalculator:            formulaCalculator,
		effectivePriceResolver:       effectivePriceResolver,
	}
}

// Sync resolves productID's current MXN price and total stock, then either
// creates it in Odoo (first time on this connection) or refreshes its
// price/stock (already synced), recording the external id in
// ecom_channel_product_map either way.
func (s *OdooProductSyncService) Sync(ctx context.Context, productID, connectionID int64) error {
	product, err := s.productRepository.FindByID(ctx, productID)
	if err != nil {
		return fmt.Errorf("error loading product %d: %w", productID, err)
	}

	values, err := LoadOdooConnectionValues(ctx, s.credentialsRepository, s.settingsRepository, connectionID)
	if err != nil {
		return fmt.Errorf("error loading odoo connection %d: %w", connectionID, err)
	}

	odooURL, ok := values["odoo_url"]
	if !ok || odooURL == "" {
		return fmt.Errorf("%w: odoo_url", ErrMissingOdooSettings)
	}
	apiKey, ok := values["apikey"]
	if !ok || apiKey == "" {
		return fmt.Errorf("%w: ApiKey", ErrMissingOdooCredentials)
	}
	database := values["x-odoo-database"]
	credentials := odooInfra.Credentials{APIKey: apiKey, Database: database}

	client := odooInfra.NewClient(nil, odooURL, s.rateLimiter)
	productsHandler := odooInfra.NewProductsHandler(client)
	imagesHandler := odooInfra.NewImagesHandler(client)
	categoriesHandler := odooInfra.NewCategoriesHandler(client)

	mxnCurrency, err := s.currenciesRepository.FindByCode(ctx, odooSyncCurrency)
	if err != nil {
		return fmt.Errorf("error loading %s currency: %w", odooSyncCurrency, err)
	}

	basePrice, _, priceListID, err := s.effectivePriceResolver.ResolveInCurrency(ctx, productID, mxnCurrency.ID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductPriceNotFound) {
			return fmt.Errorf("%w: no active price list entry for product %d", workers.ErrSyncNotReady, productID)
		}
		return fmt.Errorf("error loading effective price for product %d: %w", productID, err)
	}
	listPrice, err := s.formulaCalculator.CalculatePrice(ctx, product.BrandID, connectionID, priceListID, basePrice)
	if err != nil {
		return fmt.Errorf("error calculating final price for product %d: %w", productID, err)
	}

	qtyAvailable, err := s.sumAvailableStock(ctx, productID)
	if err != nil {
		return fmt.Errorf("error summing stock for product %d: %w", productID, err)
	}

	existingMap, err := s.channelProductMapRepository.FindByProductAndConnection(ctx, productID, connectionID)
	if err != nil && !errors.Is(err, mysqlInfra.ErrChannelProductMapNotFound) {
		return fmt.Errorf("error loading channel product map for product %d: %w", productID, err)
	}

	if existingMap != nil && existingMap.ExternalID != nil && strings.TrimSpace(*existingMap.ExternalID) != "" {
		title := resolveOdooRefreshTitle(existingMap, product)
		return s.update(ctx, productsHandler, categoriesHandler, credentials, connectionID, product, existingMap, title, listPrice, qtyAvailable)
	}

	_, err = s.create(ctx, productsHandler, imagesHandler, categoriesHandler, credentials, product, connectionID, product.Name, nil, listPrice, qtyAvailable, product.ID)
	return err
}

// Publish implements channel_listings.Publisher (duck-typed: no import from
// that package is needed here) by creating a brand-new Odoo product.template
// for product titled title, against vehicleFitmentID, with images pulled
// from imageSourceProductID (product.ID itself, unless channel_listings
// resolved product's own cover image to be missing and borrowed one from
// another product in its succession chain — see
// channel_listings.resolveImageSource). It is independent from Sync/the
// ecom_channel_sync_queue worker flow above: it never checks
// ecom_channel_product_map for an existing row itself and never falls back
// to an update — the channel_listings orchestrator already decided this
// (product, connection, vehicleFitmentID) has no listing yet before calling
// Publish. officialStoreID is a MercadoLibre-only concept and is ignored
// here — it exists solely to satisfy channel_listings.Publisher's signature.
// The returned missingRequiredAttributes/missingOptionalAttributes are always
// nil: Odoo has no equivalent of ecom_channel_attributes-tracked attributes
// today.
func (s *OdooProductSyncService) Publish(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, title string, vehicleFitmentID *int64, imageSourceProductID int64, officialStoreID *int64) (string, []string, []string, error) {
	values, err := LoadOdooConnectionValues(ctx, s.credentialsRepository, s.settingsRepository, connectionID)
	if err != nil {
		return "", nil, nil, fmt.Errorf("error loading odoo connection %d: %w", connectionID, err)
	}

	odooURL, ok := values["odoo_url"]
	if !ok || odooURL == "" {
		return "", nil, nil, fmt.Errorf("%w: odoo_url", ErrMissingOdooSettings)
	}
	apiKey, ok := values["apikey"]
	if !ok || apiKey == "" {
		return "", nil, nil, fmt.Errorf("%w: ApiKey", ErrMissingOdooCredentials)
	}
	database := values["x-odoo-database"]
	credentials := odooInfra.Credentials{APIKey: apiKey, Database: database}

	client := odooInfra.NewClient(nil, odooURL, s.rateLimiter)
	productsHandler := odooInfra.NewProductsHandler(client)
	imagesHandler := odooInfra.NewImagesHandler(client)
	categoriesHandler := odooInfra.NewCategoriesHandler(client)

	mxnCurrency, err := s.currenciesRepository.FindByCode(ctx, odooSyncCurrency)
	if err != nil {
		return "", nil, nil, fmt.Errorf("error loading %s currency: %w", odooSyncCurrency, err)
	}

	basePrice, _, priceListID, err := s.effectivePriceResolver.ResolveInCurrency(ctx, product.ID, mxnCurrency.ID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductPriceNotFound) {
			return "", nil, nil, fmt.Errorf("no active price list entry for product %d", product.ID)
		}
		return "", nil, nil, fmt.Errorf("error loading effective price for product %d: %w", product.ID, err)
	}
	listPrice, err := s.formulaCalculator.CalculatePrice(ctx, product.BrandID, connectionID, priceListID, basePrice)
	if err != nil {
		return "", nil, nil, fmt.Errorf("error calculating final price for product %d: %w", product.ID, err)
	}

	qtyAvailable, err := s.sumAvailableStock(ctx, product.ID)
	if err != nil {
		return "", nil, nil, fmt.Errorf("error summing stock for product %d: %w", product.ID, err)
	}

	externalID, err := s.create(ctx, productsHandler, imagesHandler, categoriesHandler, credentials, product, connectionID, title, vehicleFitmentID, listPrice, qtyAvailable, imageSourceProductID)
	return externalID, nil, nil, err
}

// Refresh implements channel_listings.Refresher (duck-typed: no import from
// that package is needed here) for an already-published Odoo listing. It
// only calls out to Odoo at all when product's effective price or total
// stock changed since listing.LastSyncedAt — comparing
// ecom_product_prices.updated_at / ecom_product_stock.last_sync_at against
// it, the same signal ecom_channel_product_map already carries for this
// purpose (see odooRefreshPriceStockGateEnabled) — and, when it does, sends
// only qty_available and list_price (see updatePriceAndStock /
// odooInfra.UpdateProductPriceStockVals): no name, weight/volume, or
// category. Those full-field pushes stay exclusive to Sync/update (the
// channel-agnostic queue path) — a plain refresh must never touch anything
// besides price and stock.
func (s *OdooProductSyncService) Refresh(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO) (bool, error) {
	externalIDStr := derefString(listing.ExternalID)
	if externalIDStr == "" {
		return false, nil
	}

	mxnCurrency, err := s.currenciesRepository.FindByCode(ctx, odooSyncCurrency)
	if err != nil {
		return false, fmt.Errorf("error loading %s currency: %w", odooSyncCurrency, err)
	}

	basePrice, priceUpdatedAt, priceListID, err := s.effectivePriceResolver.ResolveInCurrency(ctx, product.ID, mxnCurrency.ID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductPriceNotFound) {
			return false, fmt.Errorf("no active price list entry for product %d", product.ID)
		}
		return false, fmt.Errorf("error loading effective price for product %d: %w", product.ID, err)
	}

	stocks, err := s.productStockRepository.FindByProductID(ctx, product.ID)
	if err != nil {
		return false, fmt.Errorf("error loading stock for product %d: %w", product.ID, err)
	}

	if odooRefreshPriceStockGateEnabled && !priceOrStockChangedSince(priceUpdatedAt, stocks, listing.LastSyncedAt) {
		return false, nil
	}

	values, err := LoadOdooConnectionValues(ctx, s.credentialsRepository, s.settingsRepository, connectionID)
	if err != nil {
		return false, fmt.Errorf("error loading odoo connection %d: %w", connectionID, err)
	}

	odooURL, ok := values["odoo_url"]
	if !ok || odooURL == "" {
		return false, fmt.Errorf("%w: odoo_url", ErrMissingOdooSettings)
	}
	apiKey, ok := values["apikey"]
	if !ok || apiKey == "" {
		return false, fmt.Errorf("%w: ApiKey", ErrMissingOdooCredentials)
	}
	database := values["x-odoo-database"]
	credentials := odooInfra.Credentials{APIKey: apiKey, Database: database}

	client := odooInfra.NewClient(nil, odooURL, s.rateLimiter)
	productsHandler := odooInfra.NewProductsHandler(client)

	listPrice, err := s.formulaCalculator.CalculatePrice(ctx, product.BrandID, connectionID, priceListID, basePrice)
	if err != nil {
		return false, fmt.Errorf("error calculating final price for product %d: %w", product.ID, err)
	}

	qtyAvailable := 0.0
	for _, stock := range stocks {
		qtyAvailable += float64(stock.AvailableQty)
	}

	if err := s.updatePriceAndStock(ctx, productsHandler, credentials, connectionID, listing, listPrice, qtyAvailable); err != nil {
		return false, err
	}

	return true, nil
}

// updatePriceAndStock pushes only qty_available/list_price to an
// already-published Odoo product.template (via
// odooInfra.UpdateProductPriceStock) — no name, weight/volume, or category —
// and records the refresh in ecom_channel_product_map, passing through
// listing_title/external_category_id unchanged since neither was touched on
// the Odoo side. Used exclusively by Refresh; see update below for the
// full-field version Sync uses.
func (s *OdooProductSyncService) updatePriceAndStock(
	ctx context.Context,
	productsHandler *odooInfra.ProductsHandler,
	credentials odooInfra.Credentials,
	connectionID int64,
	existingMap *mysqlInfra.ChannelProductMapDTO,
	listPrice, qtyAvailable float64,
) error {
	productID := existingMap.ProductID
	externalIDStr := derefString(existingMap.ExternalID)

	externalID, err := strconv.ParseInt(strings.TrimSpace(externalIDStr), 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing external id %q for product %d: %w", externalIDStr, productID, err)
	}

	if err := productsHandler.UpdateProductPriceStock(ctx, odooInfra.UpdateProductPriceStockRequest{
		Credentials: credentials,
		ExternalID:  externalID,
		Vals: odooInfra.UpdateProductPriceStockVals{
			QtyAvailable: qtyAvailable,
			ListPrice:    listPrice,
		},
	}); err != nil {
		return fmt.Errorf("error updating odoo product %d (local product %d): %w", externalID, productID, err)
	}

	if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
		ProductID:          productID,
		ConnectionID:       connectionID,
		VehicleFitmentID:   existingMap.VehicleFitmentID,
		ListingTitle:       derefString(existingMap.ListingTitle),
		ExternalID:         externalIDStr,
		ExternalCategoryID: derefString(existingMap.ExternalCategoryID),
		Status:             odooSyncedStatus,
		ActorID:            systemOdooSyncActorID,
	}); err != nil {
		return fmt.Errorf("error recording channel product map for product %d: %w", productID, err)
	}

	return nil
}

// update refreshes an already-synced product's stock, price, name (always
// sent together — see normalizeOdooTitle), weight/volume, and category per
// Odoo's product.template/write, and touches the map row's last_synced_at.
// Category is resolved via resolveOdooCategory — the same lookup-or-create
// chain a brand-new listing goes through in create — whenever product still
// has a local category (product.CategoryID != nil): a category with no
// ecom_channel_category_map row yet for connectionID is created in Odoo
// hierarchically (walking up every missing ancestor) before its id is sent
// here, so an already-published listing whose category was assigned or
// changed after it was first synced picks up the correct one on the next
// refresh instead of staying wired to whatever it had at publish time. A
// product with no local category at all leaves PublicCategIDs unset
// (omitempty), which Odoo's write leaves untouched rather than clearing it.
// existingMap's own listing_title/external_category_id are passed back
// through the Upsert unchanged (other than listing_title itself now being
// refreshed to title, and external_category_id when a category was
// resolved) instead of being left out and nulled — Upsert's UPDATE branch
// sets whatever it's given, so omitting a field here would blank it out.
func (s *OdooProductSyncService) update(
	ctx context.Context,
	productsHandler *odooInfra.ProductsHandler,
	categoriesHandler *odooInfra.CategoriesHandler,
	credentials odooInfra.Credentials,
	connectionID int64,
	product *mysqlInfra.ProductDTO,
	existingMap *mysqlInfra.ChannelProductMapDTO,
	title string,
	listPrice, qtyAvailable float64,
) error {
	productID := existingMap.ProductID
	externalIDStr := derefString(existingMap.ExternalID)

	externalID, err := strconv.ParseInt(strings.TrimSpace(externalIDStr), 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing external id %q for product %d: %w", externalIDStr, productID, err)
	}

	weight, volume := s.resolveDimensions(ctx, productID)

	vals := odooInfra.UpdateProductVals{
		QtyAvailable: qtyAvailable,
		ListPrice:    listPrice,
		Name:         title,
		Weight:       weight,
		Volume:       volume,
	}

	externalCategoryID := derefString(existingMap.ExternalCategoryID)
	if product.CategoryID != nil {
		resolvedCategoryID, err := s.resolveOdooCategory(ctx, categoriesHandler, credentials, *product.CategoryID, connectionID)
		if err != nil {
			return fmt.Errorf("error resolving odoo category for product %d: %w", productID, err)
		}
		vals.PublicCategIDs = odooInfra.X2ManyReplace(resolvedCategoryID)
		externalCategoryID = strconv.FormatInt(resolvedCategoryID, 10)
	}

	if err := productsHandler.UpdateProduct(ctx, odooInfra.UpdateProductRequest{
		Credentials: credentials,
		ExternalID:  externalID,
		Vals:        vals,
	}); err != nil {
		return fmt.Errorf("error updating odoo product %d (local product %d): %w", externalID, productID, err)
	}

	if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
		ProductID:          productID,
		ConnectionID:       connectionID,
		VehicleFitmentID:   existingMap.VehicleFitmentID,
		ListingTitle:       title,
		ExternalID:         externalIDStr,
		ExternalCategoryID: externalCategoryID,
		Status:             odooSyncedStatus,
		ActorID:            systemOdooSyncActorID,
	}); err != nil {
		return fmt.Errorf("error recording channel product map for product %d: %w", productID, err)
	}

	return nil
}

// resolveOdooRefreshTitle picks the title to (re-)send to Odoo on a
// price/stock refresh: the listing's own recorded title when there is one
// (so a per-vehicle-fitment listing keeps its fitment-specific name), or the
// product's own name as a fallback for rows where no title was ever
// recorded. Either way it's run through normalizeOdooTitle, which is
// idempotent, so a title normalized once before just normalizes to itself.
func resolveOdooRefreshTitle(existingMap *mysqlInfra.ChannelProductMapDTO, product *mysqlInfra.ProductDTO) string {
	raw := derefString(existingMap.ListingTitle)
	if strings.TrimSpace(raw) == "" {
		raw = product.Name
	}
	return normalizeOdooTitle(raw, product.PartNumber)
}

// create builds the full product.template payload and creates it in Odoo,
// requiring a cover image first (a missing one leaves the queue entry
// pending via workers.ErrSyncNotReady instead of failing it), then uploads
// every remaining image. title is used for the listing's
// Name/DescriptionEcommerce instead of product.Name so the same product can
// be listed more than once under a different title (e.g. one per vehicle
// compatibility via Publish); vehicleFitmentID is recorded alongside it in
// ecom_channel_product_map. Images are loaded from imageSourceProductID
// rather than product.ID: normally the same product, but channel_listings
// can point this at a different product in product's succession chain
// (ecom_part_number_supersessions) when product itself has no cover image of
// its own.
func (s *OdooProductSyncService) create(
	ctx context.Context,
	productsHandler *odooInfra.ProductsHandler,
	imagesHandler *odooInfra.ImagesHandler,
	categoriesHandler *odooInfra.CategoriesHandler,
	credentials odooInfra.Credentials,
	product *mysqlInfra.ProductDTO,
	connectionID int64,
	title string,
	vehicleFitmentID *int64,
	listPrice, qtyAvailable float64,
	imageSourceProductID int64,
) (string, error) {
	title = normalizeOdooTitle(title, product.PartNumber)

	images, err := s.productImagesRepository.FindAllByProductID(ctx, imageSourceProductID)
	if err != nil {
		return "", fmt.Errorf("error loading images for product %d: %w", imageSourceProductID, err)
	}

	var coverImage *mysqlInfra.ProductImageDTO
	extraImages := make([]mysqlInfra.ProductImageDTO, 0, len(images))
	for i := range images {
		if images[i].IsFirst && coverImage == nil {
			coverImage = &images[i]
			continue
		}
		extraImages = append(extraImages, images[i])
	}

	if coverImage == nil {
		return "", fmt.Errorf("%w: product %d has no cover image (is_first) (image source product %d)", workers.ErrSyncNotReady, product.ID, imageSourceProductID)
	}

	coverImageBase64, err := s.downloadImageAsBase64(ctx, coverImage.FileID)
	if err != nil {
		return "", fmt.Errorf("error downloading cover image for product %d: %w", product.ID, err)
	}

	if product.CategoryID == nil {
		return "", fmt.Errorf("%w: product %d has no category", workers.ErrSyncNotReady, product.ID)
	}

	externalCategoryID, err := s.resolveOdooCategory(ctx, categoriesHandler, credentials, *product.CategoryID, connectionID)
	if err != nil {
		return "", fmt.Errorf("error resolving odoo category for product %d: %w", product.ID, err)
	}

	weight, volume := s.resolveDimensions(ctx, product.ID)

	// DescriptionEcommerce is only populated from ecom_products.description
	// when one exists — an empty ecom_products.description leaves the field
	// unset rather than falling back to title.
	var descriptionEcommerce string
	if product.Description != nil && strings.TrimSpace(*product.Description) != "" {
		descriptionEcommerce = *product.Description
	}

	vals := odooInfra.CreateProductVals{
		Name:                 title,
		Type:                 odooProductType,
		ListPrice:            listPrice,
		SaleOK:               true,
		IsPublished:          true,
		PurchaseOK:           true,
		Image1920:            coverImageBase64,
		DefaultCode:          product.PartNumber,
		ShowAvailability:     true,
		IsStorable:           true,
		AllowOutOfStockOrder: false,
		PublicCategIDs:       odooInfra.X2ManyReplace(externalCategoryID),
		DescriptionEcommerce: descriptionEcommerce,
		TaxesID:              odooInfra.X2ManyReplace(odooTaxID),
		QtyAvailable:         qtyAvailable,
		Weight:               weight,
		Volume:               volume,
	}

	externalID, err := productsHandler.CreateProduct(ctx, odooInfra.CreateProductRequest{Credentials: credentials, Vals: vals})
	if err != nil {
		return "", fmt.Errorf("error creating odoo product for %d: %w", product.ID, err)
	}
	externalIDStr := strconv.FormatInt(externalID, 10)

	if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
		ProductID:        product.ID,
		ConnectionID:     connectionID,
		VehicleFitmentID: vehicleFitmentID,
		ListingTitle:     title,
		ExternalID:       externalIDStr,
		Status:           odooSyncedStatus,
		ActorID:          systemOdooSyncActorID,
	}); err != nil {
		return "", fmt.Errorf("error recording channel product map for product %d: %w", product.ID, err)
	}

	for i, image := range extraImages {
		imageBase64, err := s.downloadImageAsBase64(ctx, image.FileID)
		if err != nil {
			return "", fmt.Errorf("error downloading image %d for product %d: %w", image.ID, product.ID, err)
		}

		if err := imagesHandler.CreateProductImage(ctx, odooInfra.CreateProductImageRequest{
			Credentials: credentials,
			Vals: odooInfra.CreateProductImageVals{
				ProductTemplateID: externalID,
				Image1920:         imageBase64,
				Name:              fmt.Sprintf("%s_%d", product.SKU, i+1),
			},
		}); err != nil {
			return "", fmt.Errorf("error uploading image %d for odoo product %d: %w", image.ID, externalID, err)
		}
	}

	return externalIDStr, nil
}

// normalizeOdooTitle applies Odoo's own naming convention to a listing
// title: each word capitalized (rest of the word lowercased, so an
// all-caps ERP name like "FILTRO DE CABINA NISSAN" becomes "Filtro De
// Cabina Nissan" rather than staying shouty), followed by the product's own
// part number in upper case — e.g. "Filtro De Cabina Nissan SAR1234". This
// only affects what's sent to Odoo; other channels (MercadoLibre) keep
// whatever title channel_listings.Service built. Idempotent: normalizing an
// already-normalized title (e.g. re-running this on the stored
// ecom_channel_product_map.listing_title during a refresh) strips the
// trailing part number first instead of capitalizing it as another word and
// appending a second copy.
func normalizeOdooTitle(title, partNumber string) string {
	partNumber = strings.ToUpper(strings.TrimSpace(partNumber))

	base := strings.TrimSpace(title)
	if partNumber != "" {
		base = strings.TrimSpace(strings.TrimSuffix(base, partNumber))
	}

	words := strings.Fields(base)
	for i, word := range words {
		runes := []rune(strings.ToLower(word))
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	capitalized := strings.Join(words, " ")

	if partNumber == "" {
		return capitalized
	}

	return strings.TrimSpace(capitalized + " " + partNumber)
}

// priceOrStockChangedSince reports whether a product's effective price or
// any of its stock rows changed after sinceUTC — comparing against
// ecom_product_prices.updated_at (MySQL-managed, stamped on every write) and
// ecom_product_stock.last_sync_at (stamped only when a sync actually wrote a
// different available_qty — see sync_nissan.go's syncProductStock/
// syncNissanPrice), the same timestamps already used to skip redundant
// writes there. Shared by both MercadoLibre's and Odoo's Refresh so a
// channel refresh only ever calls out to a marketplace for a listing whose
// price or stock could plausibly have moved. A nil sinceUTC (the listing has
// no last_synced_at yet) always reports changed, since there is nothing to
// compare against.
func priceOrStockChangedSince(priceUpdatedAt time.Time, stocks []mysqlInfra.ProductStockDTO, sinceUTC *time.Time) bool {
	if sinceUTC == nil {
		return true
	}
	if priceUpdatedAt.After(*sinceUTC) {
		return true
	}
	for _, stock := range stocks {
		if stock.LastSyncAt != nil && stock.LastSyncAt.After(*sinceUTC) {
			return true
		}
	}
	return false
}

// resolveOdooCategory returns the Odoo product.public.category id that maps
// to categoryID on connectionID, creating it (and, recursively, whatever
// ancestor up to the root is also missing) in Odoo first if no
// ecom_channel_category_map row exists for it yet. Unlike MercadoLibre's
// EnsureLocalCategory (external category -> local hierarchy), this resolves
// the opposite direction: ecom_categories already has the full hierarchy
// (category_id is assigned directly on ecom_products), so it's replicated
// upward into Odoo instead of downloaded from it.
func (s *OdooProductSyncService) resolveOdooCategory(
	ctx context.Context,
	categoriesHandler *odooInfra.CategoriesHandler,
	credentials odooInfra.Credentials,
	categoryID, connectionID int64,
) (int64, error) {
	existing, err := s.channelCategoryMapRepository.FindByCategoryAndConnection(ctx, categoryID, connectionID)
	if err != nil && !errors.Is(err, mysqlInfra.ErrChannelCategoryMapNotFound) {
		return 0, fmt.Errorf("error loading channel category map for category %d: %w", categoryID, err)
	}
	if existing != nil {
		externalID, err := strconv.ParseInt(strings.TrimSpace(existing.ExternalCategoryID), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("error parsing external category id %q: %w", existing.ExternalCategoryID, err)
		}
		return externalID, nil
	}

	category, err := s.categoriesRepository.FindByID(ctx, categoryID)
	if err != nil {
		return 0, fmt.Errorf("error loading category %d: %w", categoryID, err)
	}

	var parentExternalID *int64
	if category.ParentID != nil {
		resolvedParentID, err := s.resolveOdooCategory(ctx, categoriesHandler, credentials, *category.ParentID, connectionID)
		if err != nil {
			return 0, err
		}
		parentExternalID = &resolvedParentID
	}

	externalID, err := categoriesHandler.CreateCategory(ctx, odooInfra.CreateCategoryRequest{
		Credentials: credentials,
		Vals: odooInfra.CreateCategoryVals{
			Name:     category.Name,
			ParentID: parentExternalID,
		},
	})
	if err != nil {
		return 0, fmt.Errorf("error creating odoo category %q: %w", category.Name, err)
	}

	name := category.Name
	if _, err := s.channelCategoryMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelCategoryMapInput{
		CategoryID:           categoryID,
		ConnectionID:         connectionID,
		ExternalCategoryID:   strconv.FormatInt(externalID, 10),
		ExternalCategoryName: &name,
		ActorID:              systemOdooSyncActorID,
	}); err != nil {
		return 0, fmt.Errorf("error recording channel category map for category %d: %w", categoryID, err)
	}

	return externalID, nil
}

func (s *OdooProductSyncService) sumAvailableStock(ctx context.Context, productID int64) (float64, error) {
	stocks, err := s.productStockRepository.FindByProductID(ctx, productID)
	if err != nil {
		return 0, err
	}

	total := 0
	for _, stock := range stocks {
		total += stock.AvailableQty
	}

	return float64(total), nil
}

// resolveDimensions returns a product's weight (kg, same unit Odoo and
// ecom_product_dimensions both use — no conversion needed) and volume
// converted from ecom_product_dimensions' cm³ into Odoo's m³, falling back
// to 1/1 when the product has no ecom_product_dimensions row at all, or when
// a stored value fails to parse.
func (s *OdooProductSyncService) resolveDimensions(ctx context.Context, productID int64) (weight, volume float64) {
	dimensions, err := s.productDimensionsRepository.FindByProductID(ctx, productID)
	if err != nil {
		return odooDefaultWeight, odooDefaultVolume
	}

	weight, err = strconv.ParseFloat(strings.TrimSpace(dimensions.Weight), 64)
	if err != nil {
		weight = odooDefaultWeight
	}

	volumeCM3, err := strconv.ParseFloat(strings.TrimSpace(dimensions.Volume), 64)
	if err != nil {
		return weight, odooDefaultVolume
	}

	return weight, volumeCM3 / odooCM3PerM3
}

func (s *OdooProductSyncService) downloadImageAsBase64(ctx context.Context, fileID int64) (string, error) {
	file, err := s.filesRepository.FindByID(ctx, fileID)
	if err != nil {
		return "", fmt.Errorf("error loading file %d: %w", fileID, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, file.Path, nil)
	if err != nil {
		return "", fmt.Errorf("invalid image url %q: %w", file.Path, err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error downloading image %q: %w", file.Path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d downloading image %q", resp.StatusCode, file.Path)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading image %q: %w", file.Path, err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

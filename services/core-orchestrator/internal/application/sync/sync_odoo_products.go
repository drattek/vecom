package sync

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
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

// odooChannelCode is ecom_channels.code for Odoo — used to load the channel's
// custom-attribute vocabulary (ecom_channel_attributes) when building the
// website.sale.product.info rows. Mirrors the literal main.go registers the
// Publisher/Refresher under and channel_attribute_values.mercadoLibreChannelCode.
const odooChannelCode = "ODOO"

// odooProductInfoSequenceStep spaces out website.sale.product.info.sequence
// values (10, 20, 30…) so an admin can later slot a manual row between two
// synced ones in Odoo without renumbering.
const odooProductInfoSequenceStep = 10

// odooVehicleMachineTypeName is the machine.type every vehicle fitment is
// filed under in Odoo: vehicles have no type table in core-orchestrator (unlike
// ecom_equipment_types), so this fixed name — flagged is_vehicle, which makes
// the storefront require a year — stands in for it. See ADR 0006.
const odooVehicleMachineTypeName = "Vehículo"

// Prefixes of Fitment/FitmentBrand/FitmentType.EcomRef: the identifier of each
// record in core-orchestrator, stamped on the Odoo record so later syncs match
// it without relying on names.
const (
	odooBrandRefPrefix            = "brand:"
	odooEquipmentTypeRefPrefix    = "equipment_type:"
	odooVehicleFitmentRefPrefix   = "vehicle_fitment:"
	odooEquipmentFitmentRefPrefix = "equipment_fitment:"
)

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

// OdooProductSyncService creates and refreshes Odoo product.templates. It
// implements channel_listings.Publisher and channel_listings.Refresher
// (duck-typed: no import from that package is needed here) — Publish creates
// the template, Refresh pushes qty_available/list_price changes.
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
	// categorySelectionRepository resolves the per-connection category the
	// user picked in the "Sincronización" tab before this product had any
	// listing there (ADR 0005) — see resolveConnectionCategoryID.
	categorySelectionRepository *mysqlInfra.ChannelProductCategorySelectionRepository
	// channelRepository/channelAttributesRepository/channelAttributeMapRepository/
	// productAttributesRepository/attributeOptionsRepository back
	// resolveProductInfoEntries: reading the custom attributes the ODOO channel
	// exposes (ecom_channel_attributes with category_id NULL — see ADR 0004) and
	// whatever value the product already has for each, to push them into Odoo's
	// website.sale.product.info key/value model.
	channelRepository             *mysqlInfra.ChannelRepository
	channelAttributesRepository   *mysqlInfra.ChannelAttributesRepository
	channelAttributeMapRepository *mysqlInfra.ChannelAttributeMapRepository
	productAttributesRepository   *mysqlInfra.ProductAttributesRepository
	attributeOptionsRepository    *mysqlInfra.AttributeOptionsRepository
	// fitmentsRepository backs syncProductFitments: the machine/vehicle
	// compatibilities pushed to the website_vegusa module (ADR 0006).
	fitmentsRepository *mysqlInfra.ProductFitmentsExportRepository
	rateLimiter        *odooInfra.RateLimiter
	// httpClient downloads image files from wherever ecom_files.path points
	// (this integration's own storage, not Odoo) — see downloadImageAsBase64.
	httpClient *http.Client
	// odooAPIClient is handed to every odooInfra.NewClient(...) call in this
	// file instead of nil, so every Odoo API call (product.template create/
	// write, product.image create/search_read/unlink,
	// website.sale.product.info create/write/unlink) gets a longer timeout
	// than odooInfra.NewClient's own 15s default — Resync in particular can
	// chain several of these sequentially (update, search, unlink, N image
	// creates), and Odoo has been observed taking longer than 15s on a plain
	// product.image.unlink under load.
	odooAPIClient          *http.Client
	formulaCalculator      *pricingApp.PricingFormulaCalculator
	effectivePriceResolver *pricingApp.EffectivePriceResolver
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
	categorySelectionRepository *mysqlInfra.ChannelProductCategorySelectionRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	channelAttributesRepository *mysqlInfra.ChannelAttributesRepository,
	channelAttributeMapRepository *mysqlInfra.ChannelAttributeMapRepository,
	productAttributesRepository *mysqlInfra.ProductAttributesRepository,
	attributeOptionsRepository *mysqlInfra.AttributeOptionsRepository,
	fitmentsRepository *mysqlInfra.ProductFitmentsExportRepository,
	rateLimiter *odooInfra.RateLimiter,
	formulaCalculator *pricingApp.PricingFormulaCalculator,
	effectivePriceResolver *pricingApp.EffectivePriceResolver,
) *OdooProductSyncService {
	return &OdooProductSyncService{
		credentialsRepository:         credentialsRepository,
		settingsRepository:            settingsRepository,
		productRepository:             productRepository,
		productPricesRepository:       productPricesRepository,
		productStockRepository:        productStockRepository,
		productDimensionsRepository:   productDimensionsRepository,
		productImagesRepository:       productImagesRepository,
		filesRepository:               filesRepository,
		currenciesRepository:          currenciesRepository,
		categoriesRepository:          categoriesRepository,
		channelCategoryMapRepository:  channelCategoryMapRepository,
		channelProductMapRepository:   channelProductMapRepository,
		categorySelectionRepository:   categorySelectionRepository,
		channelRepository:             channelRepository,
		channelAttributesRepository:   channelAttributesRepository,
		channelAttributeMapRepository: channelAttributeMapRepository,
		productAttributesRepository:   productAttributesRepository,
		attributeOptionsRepository:    attributeOptionsRepository,
		fitmentsRepository:            fitmentsRepository,
		rateLimiter:                   rateLimiter,
		httpClient:                    &http.Client{Timeout: 20 * time.Second},
		odooAPIClient:                 &http.Client{Timeout: 60 * time.Second},
		formulaCalculator:             formulaCalculator,
		effectivePriceResolver:        effectivePriceResolver,
	}
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
// The returned missingRequiredAttributes/missingOptionalAttributes list the
// ODOO channel's custom attributes (ecom_channel_attributes) the product has no
// value for; a non-empty missingRequiredAttributes also aborts the publish
// before the product.template is created (see create / ADR 0004).
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

	client := odooInfra.NewClient(s.odooAPIClient, odooURL, s.rateLimiter)
	productsHandler := odooInfra.NewProductsHandler(client)
	imagesHandler := odooInfra.NewImagesHandler(client)
	categoriesHandler := odooInfra.NewCategoriesHandler(client)
	productInfoHandler := odooInfra.NewProductInfoHandler(client)
	fitmentsHandler := odooInfra.NewFitmentsHandler(client)

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

	return s.create(ctx, productsHandler, imagesHandler, categoriesHandler, productInfoHandler, fitmentsHandler, credentials, product, connectionID, title, vehicleFitmentID, listPrice, qtyAvailable, imageSourceProductID)
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

	client := odooInfra.NewClient(s.odooAPIClient, odooURL, s.rateLimiter)
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

// Resync implements channel_listings.FullRefresher (duck-typed: no import
// from that package is needed here) for an already-published Odoo listing.
// Unlike Refresh, it always calls out (no price/stock-changed gate) and
// delegates to update — the full-field push (title, category, weight/
// volume, description, images, custom attributes) that already existed for
// the now-removed channel-agnostic Sync/queue flow, revived here instead of
// being duplicated. Odoo, unlike MercadoLibre, allows changing every one of
// these fields on an already-created product.template, so nothing here is
// held back the way MercadoLibre's Resync holds back title/category.
func (s *OdooProductSyncService) Resync(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO) (bool, error) {
	externalIDStr := derefString(listing.ExternalID)
	if externalIDStr == "" {
		return false, nil
	}

	mxnCurrency, err := s.currenciesRepository.FindByCode(ctx, odooSyncCurrency)
	if err != nil {
		return false, fmt.Errorf("error loading %s currency: %w", odooSyncCurrency, err)
	}

	basePrice, _, priceListID, err := s.effectivePriceResolver.ResolveInCurrency(ctx, product.ID, mxnCurrency.ID)
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

	client := odooInfra.NewClient(s.odooAPIClient, odooURL, s.rateLimiter)
	productsHandler := odooInfra.NewProductsHandler(client)
	imagesHandler := odooInfra.NewImagesHandler(client)
	categoriesHandler := odooInfra.NewCategoriesHandler(client)
	productInfoHandler := odooInfra.NewProductInfoHandler(client)
	fitmentsHandler := odooInfra.NewFitmentsHandler(client)

	listPrice, err := s.formulaCalculator.CalculatePrice(ctx, product.BrandID, connectionID, priceListID, basePrice)
	if err != nil {
		return false, fmt.Errorf("error calculating final price for product %d: %w", product.ID, err)
	}

	qtyAvailable := 0.0
	for _, stock := range stocks {
		qtyAvailable += float64(stock.AvailableQty)
	}

	title := resolveOdooRefreshTitle(listing, product)

	if err := s.update(ctx, productsHandler, imagesHandler, categoriesHandler, productInfoHandler, fitmentsHandler, credentials, connectionID, product, listing, title, listPrice, qtyAvailable); err != nil {
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
	imagesHandler *odooInfra.ImagesHandler,
	categoriesHandler *odooInfra.CategoriesHandler,
	productInfoHandler *odooInfra.ProductInfoHandler,
	fitmentsHandler *odooInfra.FitmentsHandler,
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

	// DescriptionEcommerce mirrors create's own rule: an empty
	// ecom_products.description clears the field in Odoo too (UpdateProductVals
	// sends it with no omitempty), rather than leaving a stale one behind.
	var descriptionEcommerce string
	if product.Description != nil && strings.TrimSpace(*product.Description) != "" {
		descriptionEcommerce = *product.Description
	}

	vals := odooInfra.UpdateProductVals{
		QtyAvailable:         qtyAvailable,
		ListPrice:            listPrice,
		Name:                 title,
		Weight:               weight,
		Volume:               volume,
		DescriptionEcommerce: descriptionEcommerce,
	}

	externalCategoryID := derefString(existingMap.ExternalCategoryID)
	categoryID, err := s.resolveConnectionCategoryID(ctx, product, connectionID)
	if err != nil {
		return err
	}
	if categoryID != nil {
		resolvedCategoryID, err := s.resolveOdooCategory(ctx, categoriesHandler, credentials, *categoryID, connectionID)
		if err != nil {
			return fmt.Errorf("error resolving odoo category for product %d: %w", productID, err)
		}
		vals.PublicCategIDs = odooInfra.X2ManyReplace(resolvedCategoryID)
		externalCategoryID = strconv.FormatInt(resolvedCategoryID, 10)
	}

	// Images are reloaded from product.ID's current ecom_product_images —
	// resync only ever targets a product's own already-published listing, so
	// there's no image-source borrowing to consider here (unlike create/
	// channel_listings.resolveImageSource). A missing cover image is left
	// alone (Image1920 stays unset — omitempty) rather than blanking out
	// whatever cover Odoo already has; extra images are always resynced when
	// there is a cover to key them to.
	images, err := s.productImagesRepository.FindAllByProductID(ctx, productID)
	if err != nil {
		return fmt.Errorf("error loading images for product %d: %w", productID, err)
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

	if coverImage != nil {
		coverImageBase64, err := s.downloadImageAsBase64(ctx, coverImage.FileID)
		if err != nil {
			return fmt.Errorf("error downloading cover image for product %d: %w", productID, err)
		}
		vals.Image1920 = coverImageBase64
	}

	if err := productsHandler.UpdateProduct(ctx, odooInfra.UpdateProductRequest{
		Credentials: credentials,
		ExternalID:  externalID,
		Vals:        vals,
	}); err != nil {
		return fmt.Errorf("error updating odoo product %d (local product %d): %w", externalID, productID, err)
	}

	if coverImage != nil {
		// Clear out every product.image row already attached to this template
		// before recreating them from ecom_product_images' current state — the
		// simplest way to keep a repeated resync idempotent (no duplicate
		// photos accumulating across runs) without diffing old vs new.
		existingImageIDs, err := imagesHandler.SearchProductImageIDs(ctx, credentials, externalID)
		if err != nil {
			return fmt.Errorf("error listing existing odoo images for product %d (odoo template %d): %w", productID, externalID, err)
		}
		if err := imagesHandler.DeleteProductImages(ctx, credentials, existingImageIDs); err != nil {
			return fmt.Errorf("error deleting existing odoo images for product %d (odoo template %d): %w", productID, externalID, err)
		}
		for i, image := range extraImages {
			imageBase64, err := s.downloadImageAsBase64(ctx, image.FileID)
			if err != nil {
				return fmt.Errorf("error downloading image %d for product %d: %w", image.ID, productID, err)
			}
			if err := imagesHandler.CreateProductImage(ctx, odooInfra.CreateProductImageRequest{
				Credentials: credentials,
				Vals: odooInfra.CreateProductImageVals{
					ProductTemplateID: externalID,
					Image1920:         imageBase64,
					Name:              fmt.Sprintf("%s_%d", product.SKU, i+1),
				},
			}); err != nil {
				return fmt.Errorf("error uploading image %d for odoo product %d: %w", image.ID, externalID, err)
			}
		}
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

	// Re-sync website.sale.product.info on a full update, same as the category is
	// re-resolved above. Unlike create this never aborts on a missing required
	// attribute — the listing already exists; a cleared value just drops its row.
	infoEntries, managedInfoKeys, missingRequired, _, err := s.resolveProductInfoEntries(ctx, product, connectionID)
	if err != nil {
		return fmt.Errorf("error resolving odoo custom attributes for product %d: %w", productID, err)
	}
	if len(missingRequired) > 0 {
		log.Printf("odoo update: product %d (odoo template %d) — missing required attributes not sent: %s", productID, externalID, strings.Join(missingRequired, ", "))
	}
	if err := s.syncProductInfo(ctx, productInfoHandler, credentials, externalID, infoEntries, managedInfoKeys); err != nil {
		return fmt.Errorf("error syncing odoo product info for product %d (odoo template %d): %w", productID, externalID, err)
	}

	if err := s.syncProductFitments(ctx, fitmentsHandler, credentials, externalID, productID); err != nil {
		return err
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
//
// Custom attributes: the ODOO channel's ecom_channel_attributes are resolved
// against the product's values (resolveProductInfoEntries) and, once the
// product.template exists, written into Odoo's website.sale.product.info
// key/value model (syncProductInfo). A required attribute with no value aborts
// the whole publish before the product.template is created — same criterion as
// MercadoLibre's createNewItem. See ADR 0004.
func (s *OdooProductSyncService) create(
	ctx context.Context,
	productsHandler *odooInfra.ProductsHandler,
	imagesHandler *odooInfra.ImagesHandler,
	categoriesHandler *odooInfra.CategoriesHandler,
	productInfoHandler *odooInfra.ProductInfoHandler,
	fitmentsHandler *odooInfra.FitmentsHandler,
	credentials odooInfra.Credentials,
	product *mysqlInfra.ProductDTO,
	connectionID int64,
	title string,
	vehicleFitmentID *int64,
	listPrice, qtyAvailable float64,
	imageSourceProductID int64,
) (string, []string, []string, error) {
	title = normalizeOdooTitle(title, product.PartNumber)

	infoEntries, managedInfoKeys, missingRequired, missingOptional, err := s.resolveProductInfoEntries(ctx, product, connectionID)
	if err != nil {
		return "", nil, nil, fmt.Errorf("error resolving odoo custom attributes for product %d: %w", product.ID, err)
	}
	if len(missingRequired) > 0 {
		return "", missingRequired, missingOptional,
			fmt.Errorf("%w for product %d: %s", ErrMissingRequiredOdooAttributes, product.ID, strings.Join(missingRequired, ", "))
	}

	images, err := s.productImagesRepository.FindAllByProductID(ctx, imageSourceProductID)
	if err != nil {
		return "", missingRequired, missingOptional, fmt.Errorf("error loading images for product %d: %w", imageSourceProductID, err)
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
		return "", missingRequired, missingOptional, fmt.Errorf("%w: product %d has no cover image (is_first) (image source product %d)", workers.ErrSyncNotReady, product.ID, imageSourceProductID)
	}

	coverImageBase64, err := s.downloadImageAsBase64(ctx, coverImage.FileID)
	if err != nil {
		return "", missingRequired, missingOptional, fmt.Errorf("error downloading cover image for product %d: %w", product.ID, err)
	}

	categoryID, err := s.resolveConnectionCategoryID(ctx, product, connectionID)
	if err != nil {
		return "", missingRequired, missingOptional, err
	}
	if categoryID == nil {
		return "", missingRequired, missingOptional, fmt.Errorf("%w: product %d has no category", workers.ErrSyncNotReady, product.ID)
	}

	externalCategoryID, err := s.resolveOdooCategory(ctx, categoriesHandler, credentials, *categoryID, connectionID)
	if err != nil {
		return "", missingRequired, missingOptional, fmt.Errorf("error resolving odoo category for product %d: %w", product.ID, err)
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
		return "", missingRequired, missingOptional, fmt.Errorf("error creating odoo product for %d: %w", product.ID, err)
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
		return "", missingRequired, missingOptional, fmt.Errorf("error recording channel product map for product %d: %w", product.ID, err)
	}

	// website.sale.product.info is synced after the map row is recorded (like the
	// extra-images loop below): a failure here is reported so the queue row is
	// marked failed, but the listing itself already exists — the next full Sync
	// (update) re-syncs the key/value rows.
	if err := s.syncProductInfo(ctx, productInfoHandler, credentials, externalID, infoEntries, managedInfoKeys); err != nil {
		return "", missingRequired, missingOptional, fmt.Errorf("error syncing odoo product info for product %d (odoo template %d): %w", product.ID, externalID, err)
	}

	// Same failure semantics as website.sale.product.info above: the listing
	// already exists, the next full Sync (update) retries the fitments.
	if err := s.syncProductFitments(ctx, fitmentsHandler, credentials, externalID, product.ID); err != nil {
		return "", missingRequired, missingOptional, err
	}

	for i, image := range extraImages {
		imageBase64, err := s.downloadImageAsBase64(ctx, image.FileID)
		if err != nil {
			return "", missingRequired, missingOptional, fmt.Errorf("error downloading image %d for product %d: %w", image.ID, product.ID, err)
		}

		if err := imagesHandler.CreateProductImage(ctx, odooInfra.CreateProductImageRequest{
			Credentials: credentials,
			Vals: odooInfra.CreateProductImageVals{
				ProductTemplateID: externalID,
				Image1920:         imageBase64,
				Name:              fmt.Sprintf("%s_%d", product.SKU, i+1),
			},
		}); err != nil {
			return "", missingRequired, missingOptional, fmt.Errorf("error uploading image %d for odoo product %d: %w", image.ID, externalID, err)
		}
	}

	return externalIDStr, missingRequired, missingOptional, nil
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

// resolveConnectionCategoryID returns the local ecom_categories id to
// resolve product's Odoo category from on connectionID: a per-connection
// selection picked in the "Sincronización" tab before this product had any
// listing there (ecom_channel_product_category_selection — see ADR 0005)
// takes priority when present; otherwise product's own catalog category is
// used as before (nil if it has none), preserving prior behavior for
// connections that never went through the picker.
func (s *OdooProductSyncService) resolveConnectionCategoryID(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64) (*int64, error) {
	selection, err := s.categorySelectionRepository.FindByProductAndConnection(ctx, product.ID, connectionID)
	if err != nil && !errors.Is(err, mysqlInfra.ErrChannelProductCategorySelectionNotFound) {
		return nil, fmt.Errorf("error loading category selection for product %d: %w", product.ID, err)
	}
	if selection != nil {
		categoryID := selection.CategoryID
		return &categoryID, nil
	}
	return product.CategoryID, nil
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

// odooProductInfoEntry is one resolved website.sale.product.info key/value pair
// ready to push to Odoo.
type odooProductInfoEntry struct {
	Key   string
	Value string
}

// resolveProductInfoEntries reads every custom attribute the ODOO channel
// exposes (ecom_channel_attributes — channel-wide category_id NULL slots, plus
// any scoped to product's own category — see ADR 0004) and builds the
// website.sale.product.info rows to push:
//
//   - source_type 'custom_attribute' → the product's own ecom_product_attributes
//     value, rendered as text; a slot the product has no value for is simply
//     skipped (the slots are channel-wide, so this is expected), unless it's
//     explicitly is_required, which goes to missingRequired and aborts.
//   - source_type 'static_value'     → the fixed value, same for every product.
//   - source_type 'system_field'     → skipped: that data already reaches Odoo
//     through product.template fields (default_code, name, category, dimensions),
//     matching resolveCustomAttributes on the MercadoLibre side.
//
// managedKeys is the set of external_key values across every applicable slot —
// syncProductInfo only ever deletes rows whose key is in this set, so keys added
// by hand in Odoo survive.
func (s *OdooProductSyncService) resolveProductInfoEntries(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64) (entries []odooProductInfoEntry, managedKeys map[string]bool, missingRequired, missingOptional []string, err error) {
	channel, err := s.channelRepository.FindByCode(ctx, odooChannelCode)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error loading %s channel: %w", odooChannelCode, err)
	}

	slots, err := s.channelAttributesRepository.FindApplicable(ctx, channel.ID, product.CategoryID)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error loading %s channel attributes: %w", odooChannelCode, err)
	}

	productAttrs, err := s.productAttributesRepository.FindByProductID(ctx, product.ID)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error loading attributes for product %d: %w", product.ID, err)
	}
	valueByAttr := make(map[int64]mysqlInfra.ProductAttributeDTO, len(productAttrs))
	for _, pa := range productAttrs {
		valueByAttr[pa.AttributeID] = pa
	}

	managedKeys = make(map[string]bool)
	entries = make([]odooProductInfoEntry, 0, len(slots))
	seenKey := make(map[string]bool)

	for _, slot := range slots {
		if slot.ExternalKey == nil {
			continue
		}
		key := strings.TrimSpace(*slot.ExternalKey)
		if key == "" || seenKey[key] {
			// FindApplicable returns category-specific rows before channel-wide
			// ones; the first slot seen for a key wins, mirroring the checklist.
			continue
		}
		seenKey[key] = true
		managedKeys[key] = true

		m, mapErr := s.channelAttributeMapRepository.FindByChannelAttributeAndConnection(ctx, slot.ID, connectionID)
		if mapErr != nil {
			if errors.Is(mapErr, mysqlInfra.ErrChannelAttributeMapNotFound) {
				continue
			}
			return nil, nil, nil, nil, fmt.Errorf("error resolving channel attribute map for slot %d: %w", slot.ID, mapErr)
		}

		switch m.SourceType {
		case "static_value":
			if m.StaticValue != nil && strings.TrimSpace(*m.StaticValue) != "" {
				entries = append(entries, odooProductInfoEntry{Key: key, Value: strings.TrimSpace(*m.StaticValue)})
			}
		case "custom_attribute":
			if m.AttributeID == nil {
				continue
			}
			rendered := ""
			if value, ok := valueByAttr[*m.AttributeID]; ok {
				r, renderErr := s.renderProductAttributeValue(ctx, value)
				if renderErr != nil {
					return nil, nil, nil, nil, fmt.Errorf("error rendering attribute %d for product %d: %w", *m.AttributeID, product.ID, renderErr)
				}
				rendered = r
			}
			if strings.TrimSpace(rendered) == "" {
				// The slots are channel-wide, not per product (ADR 0004): a
				// product simply not having a value for one is the norm, not a
				// gap to report. Only a slot explicitly flagged is_required (rare;
				// SetValue never sets it) blocks the publish.
				if slot.IsRequired {
					missingRequired = append(missingRequired, key)
				}
				continue
			}
			entries = append(entries, odooProductInfoEntry{Key: key, Value: rendered})
		default:
			// system_field (and anything else): not pushed to website.sale.product.info.
			continue
		}
	}

	return entries, managedKeys, missingRequired, missingOptional, nil
}

// renderProductAttributeValue renders whichever typed column an
// ecom_product_attributes row has set as the plain text Odoo's
// website.sale.product.info.info_value (a Char) expects — mirrors
// product_attributes_checklist.renderAttributeValue. An enum option is sent by
// its display value: Odoo's key/value model has no closed lists or value ids.
func (s *OdooProductSyncService) renderProductAttributeValue(ctx context.Context, pa mysqlInfra.ProductAttributeDTO) (string, error) {
	switch {
	case pa.OptionID != nil:
		option, err := s.attributeOptionsRepository.FindByID(ctx, *pa.OptionID)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrAttributeOptionNotFound) {
				return "", nil
			}
			return "", err
		}
		return strings.TrimSpace(option.Value), nil
	case pa.ValueText != nil:
		return strings.TrimSpace(*pa.ValueText), nil
	case pa.ValueNumber != nil:
		return strconv.FormatFloat(*pa.ValueNumber, 'f', -1, 64), nil
	case pa.ValueBool != nil:
		if *pa.ValueBool {
			return "Sí", nil
		}
		return "No", nil
	case pa.ValueDate != nil:
		return pa.ValueDate.Format(time.DateOnly), nil
	}
	return "", nil
}

// resolveProductFitments builds the complete list of machine/vehicle
// compatibilities of productID as Odoo's machine.model rows: one per distinct
// vehicle fitment (years included; motor/position/side stay out) and one per
// equipment fitment. See ADR 0006.
func (s *OdooProductSyncService) resolveProductFitments(ctx context.Context, productID int64) ([]odooInfra.Fitment, error) {
	vehicleFitments, err := s.fitmentsRepository.FindVehicleFitmentsByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}
	equipmentFitments, err := s.fitmentsRepository.FindEquipmentFitmentsByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	fitments := make([]odooInfra.Fitment, 0, len(vehicleFitments)+len(equipmentFitments))

	for _, vehicle := range vehicleFitments {
		yearEnd := 0
		if vehicle.YearEnd != nil {
			yearEnd = *vehicle.YearEnd
		}
		fitments = append(fitments, odooInfra.Fitment{
			EcomRef:   odooVehicleFitmentRefPrefix + strconv.FormatInt(vehicle.FitmentID, 10),
			Name:      vehicle.Model,
			YearStart: vehicle.YearStart,
			YearEnd:   yearEnd,
			Brand: odooInfra.FitmentBrand{
				EcomRef: odooBrandRefPrefix + strconv.FormatInt(vehicle.BrandID, 10),
				Name:    vehicle.BrandName,
			},
			Type: odooInfra.FitmentType{Name: odooVehicleMachineTypeName, IsVehicle: true},
		})
	}

	for _, equipment := range equipmentFitments {
		fitments = append(fitments, odooInfra.Fitment{
			EcomRef: odooEquipmentFitmentRefPrefix + strconv.FormatInt(equipment.FitmentID, 10),
			// ecom_equipment_fitment requires a model or a serie (or both); Odoo has
			// a single name column.
			Name: strings.TrimSpace(equipment.Model + " " + equipment.Serie),
			Brand: odooInfra.FitmentBrand{
				EcomRef: odooBrandRefPrefix + strconv.FormatInt(equipment.BrandID, 10),
				Name:    equipment.BrandName,
			},
			Type: odooInfra.FitmentType{
				EcomRef: odooEquipmentTypeRefPrefix + strconv.FormatInt(equipment.EquipmentTypeID, 10),
				Name:    equipment.EquipmentTypeName,
			},
		})
	}

	return fitments, nil
}

// syncProductFitments pushes productID's compatibilities to Odoo's
// website_vegusa module for template productTmplID. It always
// sends the complete list, an empty one included, so a compatibility removed
// in core-orchestrator is removed from Odoo too; the module only ever removes
// models it previously received (those with an ecom_ref), never ones loaded by
// hand. Requires the module to be upgraded to 1.1.0 first. See ADR 0006.
func (s *OdooProductSyncService) syncProductFitments(
	ctx context.Context,
	handler *odooInfra.FitmentsHandler,
	credentials odooInfra.Credentials,
	productTmplID int64,
	productID int64,
) error {
	fitments, err := s.resolveProductFitments(ctx, productID)
	if err != nil {
		return fmt.Errorf("error resolving fitments for product %d: %w", productID, err)
	}

	result, err := handler.SyncProductFitments(ctx, credentials, productTmplID, fitments)
	if err != nil {
		return fmt.Errorf("error syncing odoo fitments for product %d (odoo template %d): %w", productID, productTmplID, err)
	}

	if result.Linked > 0 || result.Unlinked > 0 {
		log.Printf("odoo fitments: product %d (odoo template %d) — %d linked, %d unlinked", productID, productTmplID, result.Linked, result.Unlinked)
	}

	return nil
}

// syncProductInfo reconciles Odoo's website.sale.product.info rows for
// productTmplID against entries: values that changed are written, missing keys
// are created, and rows whose key is in managedKeys but no longer in entries (a
// cleared attribute value) are deleted. Rows with keys outside managedKeys —
// added by hand in Odoo — are never touched. The unique (product_tmpl_id,
// info_key) in the Odoo plugin makes the create/write split safe. See ADR 0004.
func (s *OdooProductSyncService) syncProductInfo(
	ctx context.Context,
	handler *odooInfra.ProductInfoHandler,
	credentials odooInfra.Credentials,
	productTmplID int64,
	entries []odooProductInfoEntry,
	managedKeys map[string]bool,
) error {
	current, err := handler.SearchReadByProductTemplate(ctx, credentials, productTmplID)
	if err != nil {
		return fmt.Errorf("error loading website.sale.product.info for template %d: %w", productTmplID, err)
	}

	currentByKey := make(map[string]odooInfra.ProductInfoRecord, len(current))
	for _, row := range current {
		currentByKey[string(row.InfoKey)] = row
	}

	desiredKeys := make(map[string]bool, len(entries))
	toCreate := make([]odooInfra.CreateProductInfoVals, 0)

	for i, entry := range entries {
		desiredKeys[entry.Key] = true
		sequence := (i + 1) * odooProductInfoSequenceStep

		existing, ok := currentByKey[entry.Key]
		if !ok {
			toCreate = append(toCreate, odooInfra.CreateProductInfoVals{
				ProductTmplID: productTmplID,
				InfoKey:       entry.Key,
				InfoValue:     entry.Value,
				Sequence:      sequence,
			})
			continue
		}
		if string(existing.InfoValue) != entry.Value {
			if err := handler.WriteProductInfo(ctx, credentials, existing.ID, odooInfra.UpdateProductInfoVals{InfoValue: entry.Value}); err != nil {
				return fmt.Errorf("error updating website.sale.product.info %q for template %d: %w", entry.Key, productTmplID, err)
			}
		}
	}

	if _, err := handler.CreateProductInfo(ctx, credentials, toCreate); err != nil {
		return fmt.Errorf("error creating website.sale.product.info rows for template %d: %w", productTmplID, err)
	}

	staleIDs := make([]int64, 0)
	for _, row := range current {
		key := string(row.InfoKey)
		if managedKeys[key] && !desiredKeys[key] {
			staleIDs = append(staleIDs, row.ID)
		}
	}
	if err := handler.UnlinkProductInfo(ctx, credentials, staleIDs); err != nil {
		return fmt.Errorf("error deleting stale website.sale.product.info rows for template %d: %w", productTmplID, err)
	}

	return nil
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

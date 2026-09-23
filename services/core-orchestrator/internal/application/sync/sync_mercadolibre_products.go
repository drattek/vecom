package sync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	channelAttributeValuesApp "core-orchestrator/internal/application/channel_attribute_values"
	pricingApp "core-orchestrator/internal/application/pricing"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// systemMercadoLibreSyncActorID is recorded as the actor for
// ecom_channel_product_map writes made by this flow, since MercadoLibre
// upload/update requests aren't threaded through an authenticated user
// today (mirrors sync_odoo_products.go's systemOdooSyncActorID).
const systemMercadoLibreSyncActorID int64 = 1

const (
	mercadoLibreItemCurrencyID       = "MXN" // marketplace channels always receive prices in MXN
	mercadoLibreItemCondition        = "new"
	mercadoLibreItemBuyingMode       = "buy_it_now"
	mercadoLibreItemListingTypeID    = "gold_pro"
	mercadoLibreItemConditionValue   = "2230284" // ITEM_CONDITION attribute value for "new"
	mercadoLibreItemPackageSide      = "10 cm"
	mercadoLibreItemPackageWeight    = "1000 g"
	mercadoLibreCategoryPredictLimit = 1

	// Raw MercadoLibre item statuses this package branches on. They show up
	// in ItemsHandler.GetItemsStatus responses and get folded into
	// ecom_channel_product_map's own status enum by foldMercadoLibreStatus.
	mercadoLibreStatusUnderReview = "under_review"
	mercadoLibreStatusPaused      = "paused"

	// mercadoLibreSubStatusForbidden is a sub_status MercadoLibre reports
	// alongside status "under_review" for a listing that will never clear
	// review through the normal flow — per MercadoLibre's own docs, the only
	// valid next step for one of these is exclusion, not a status update to
	// "closed". foldMercadoLibreStatus treats it as closed locally (see
	// mercadoLibreMapStatusClosed) precisely because, from this app's
	// perspective, it's just as dead as a closed listing and should never be
	// retried.
	mercadoLibreSubStatusForbidden = "forbidden"

	// ecom_channel_product_map.status values. under_review, paused and closed
	// each mean "leave this listing alone on the next sync" (see
	// isPausedOrUnderReview), but are kept as distinct values (rather than
	// all folding into one) so the map's status column mirrors MercadoLibre's
	// own vocabulary.
	mercadoLibreMapStatusSynced      = "synced"
	mercadoLibreMapStatusPaused      = "paused"
	mercadoLibreMapStatusUnderReview = "under_review"
	mercadoLibreMapStatusClosed      = "closed"
)

var (
	ErrEmptyMercadoLibreUpload = errors.New("at least one product is required")
	ErrEmptyMercadoLibreUpdate = errors.New("at least one sku is required")
	// errMissingRequiredMercadoLibreAttributes is returned by createNewItem
	// before it calls POST /items when the resolved category has required
	// custom attributes the product has no value for (or that couldn't be
	// provisioned). Wrapped with the attribute keys; callers surface it as the
	// listing outcome's error.
	errMissingRequiredMercadoLibreAttributes = errors.New("product is missing MercadoLibre-required attribute values")
)

// UploadItemInput is one {sku, name, vehicleFitmentId} entry from the
// request: which local product to pull price/stock/images/brand from, the
// listing name to use instead of ecom_products.name, and — on connections
// where ecom_channel_connections.allows_multiple_listings is true — which
// vehicle compatibility this particular listing is for (nil for a general
// listing, e.g. a product with no vehicle compatibilities at all).
type UploadItemInput struct {
	SKU              string
	Name             string
	VehicleFitmentID *int64
}

type UploadInput struct {
	ConnectionID int64
	Products     []UploadItemInput
}

// UploadResultItem reports the outcome for a single (sku, name) pair. A
// batch never fails wholesale on one bad pair.
type UploadResultItem struct {
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	Success    bool   `json:"success"`
	ExternalID string `json:"externalId,omitempty"`
	Error      string `json:"error,omitempty"`
}

type UploadResult struct {
	Results []UploadResultItem `json:"results"`
}

// MercadoLibreProductSyncService lists products as new items on
// MercadoLibre and keeps them in sync, recording every listing in
// ecom_channel_product_map. Whether a product may have more than one
// listing on a given connection is governed by that connection's
// allows_multiple_listings flag — Upload enforces it before ever calling
// out to MercadoLibre.
type MercadoLibreProductSyncService struct {
	productRepository           *mysqlInfra.ProductRepository
	brandsRepository            *mysqlInfra.BrandsRepository
	productPricesRepository     *mysqlInfra.ProductPricesRepository
	productStockRepository      *mysqlInfra.ProductStockRepository
	productImagesRepository     *mysqlInfra.ProductImagesRepository
	productDimensionsRepository *mysqlInfra.ProductDimensionsRepository
	filesRepository             *mysqlInfra.FilesRepository
	currenciesRepository        *mysqlInfra.CurrenciesRepository
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository
	channelProductMapRepository *mysqlInfra.ChannelProductMapRepository
	// categorySelectionRepository resolves the per-connection category the
	// user picked in the "Sincronización" tab before this product had any
	// listing there (ADR 0005) — checked in resolveExternalCategoryID ahead
	// of the predictor. There is deliberately no ecom_channel_category_map
	// (local-category-level) fallback here anymore — see resolveExternalCategoryID.
	categorySelectionRepository *mysqlInfra.ChannelProductCategorySelectionRepository
	tokenService                *MercadoLibreTokenService
	categoryPredictorService    *MercadoLibreCategoryPredictorService
	itemsHandler                *mercadoLibreInfra.ItemsHandler
	formulaCalculator           *pricingApp.PricingFormulaCalculator
	effectivePriceResolver      *pricingApp.EffectivePriceResolver
	// channelAttributeValuesService/productAttributesRepository/
	// attributeOptionsRepository back resolveCustomAttributes: provisioning
	// the required-attribute slots for a listing's category and reading
	// whatever value the product already has for each one (see
	// resolveCustomAttributes's own doc comment).
	channelAttributeValuesService *channelAttributeValuesApp.Service
	productAttributesRepository   *mysqlInfra.ProductAttributesRepository
	attributeOptionsRepository    *mysqlInfra.AttributeOptionsRepository
	// compatibilityService pushes a product's ecom_product_vehicle_compatibility
	// rows to a listing right after it's created (createNewItem), so an
	// autopart listing starts life with its vehicle compatibilities already
	// reported instead of waiting for the daily CompatibilitiesFixScheduler to
	// catch the incomplete_compatibilities tag. nil-safe: a nil here disables
	// the at-creation push (the scheduler still covers it later).
	compatibilityService *MercadoLibreCompatibilityService
}

func NewMercadoLibreProductSyncService(
	productRepository *mysqlInfra.ProductRepository,
	brandsRepository *mysqlInfra.BrandsRepository,
	productPricesRepository *mysqlInfra.ProductPricesRepository,
	productStockRepository *mysqlInfra.ProductStockRepository,
	productImagesRepository *mysqlInfra.ProductImagesRepository,
	productDimensionsRepository *mysqlInfra.ProductDimensionsRepository,
	filesRepository *mysqlInfra.FilesRepository,
	currenciesRepository *mysqlInfra.CurrenciesRepository,
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	channelProductMapRepository *mysqlInfra.ChannelProductMapRepository,
	categorySelectionRepository *mysqlInfra.ChannelProductCategorySelectionRepository,
	tokenService *MercadoLibreTokenService,
	categoryPredictorService *MercadoLibreCategoryPredictorService,
	rateLimiter *mercadoLibreInfra.RateLimiter,
	channelAttributeValuesService *channelAttributeValuesApp.Service,
	productAttributesRepository *mysqlInfra.ProductAttributesRepository,
	attributeOptionsRepository *mysqlInfra.AttributeOptionsRepository,
	formulaCalculator *pricingApp.PricingFormulaCalculator,
	effectivePriceResolver *pricingApp.EffectivePriceResolver,
	compatibilityService *MercadoLibreCompatibilityService,
) *MercadoLibreProductSyncService {
	client := mercadoLibreInfra.NewClient(nil, "", rateLimiter)

	return &MercadoLibreProductSyncService{
		productRepository:             productRepository,
		brandsRepository:              brandsRepository,
		productPricesRepository:       productPricesRepository,
		productStockRepository:        productStockRepository,
		productImagesRepository:       productImagesRepository,
		productDimensionsRepository:   productDimensionsRepository,
		filesRepository:               filesRepository,
		currenciesRepository:          currenciesRepository,
		channelConnectionRepository:   channelConnectionRepository,
		channelProductMapRepository:   channelProductMapRepository,
		categorySelectionRepository:   categorySelectionRepository,
		tokenService:                  tokenService,
		categoryPredictorService:      categoryPredictorService,
		itemsHandler:                  mercadoLibreInfra.NewItemsHandler(client),
		formulaCalculator:             formulaCalculator,
		effectivePriceResolver:        effectivePriceResolver,
		channelAttributeValuesService: channelAttributeValuesService,
		productAttributesRepository:   productAttributesRepository,
		attributeOptionsRepository:    attributeOptionsRepository,
		compatibilityService:          compatibilityService,
	}
}

// Upload lists each (sku, name, vehicleFitmentId) entry in input.Products as
// its own new item on the input.ConnectionID MercadoLibre connection. On a
// connection with allows_multiple_listings=false, an entry is rejected once
// the product already has any listing there; on a connection with
// allows_multiple_listings=true, an entry is rejected only if that exact
// vehicle fitment (or the general listing, for a nil VehicleFitmentID) is
// already listed — so the same sku can still appear more than once, one
// listing per compatibility.
func (s *MercadoLibreProductSyncService) Upload(ctx context.Context, input UploadInput) (*UploadResult, error) {
	if input.ConnectionID <= 0 || len(input.Products) == 0 {
		return nil, ErrEmptyMercadoLibreUpload
	}

	connection, err := s.channelConnectionRepository.FindByID(ctx, input.ConnectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return nil, ErrInvalidMercadoLibreConnection
		}
		return nil, fmt.Errorf("error loading connection %d: %w", input.ConnectionID, err)
	}

	log.Printf("mercadolibre upload: starting batch of %d product(s) for connection %d (allowsMultipleListings=%v)", len(input.Products), input.ConnectionID, connection.AllowsMultipleListings)

	// Fetched once for the whole batch and handed to every createNewItem below,
	// so listing N products doesn't re-pull MercadoLibre's site-wide
	// compatibility dump N times. Non-fatal: on failure createNewItem's compat
	// push falls back to fetching its own, and the listing is created anyway.
	var compatDump []mercadoLibreInfra.DomainDumpEntry
	if s.compatibilityService != nil {
		if dump, err := s.compatibilityService.DomainDump(ctx, input.ConnectionID); err != nil {
			log.Printf("mercadolibre upload: could not prefetch compatibility dump for connection %d: %v", input.ConnectionID, err)
		} else {
			compatDump = dump
		}
	}

	results := make([]UploadResultItem, 0, len(input.Products))

	for i, item := range input.Products {
		sku := strings.TrimSpace(item.SKU)
		name := strings.TrimSpace(item.Name)
		result := UploadResultItem{SKU: sku, Name: name}
		step := fmt.Sprintf("[%d/%d sku=%q name=%q]", i+1, len(input.Products), sku, name)

		log.Printf("mercadolibre upload: %s starting", step)

		if sku == "" {
			log.Printf("mercadolibre upload: %s skipped, sku is required", step)
			result.Error = "sku is required"
			results = append(results, result)
			continue
		}
		if name == "" {
			log.Printf("mercadolibre upload: %s skipped, name is required", step)
			result.Error = "name is required"
			results = append(results, result)
			continue
		}

		log.Printf("mercadolibre upload: %s looking up product by sku", step)
		product, err := s.productRepository.FindBySKU(ctx, sku)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrProductNotFound) {
				log.Printf("mercadolibre upload: %s product not found", step)
				result.Error = "product not found for sku"
				results = append(results, result)
				continue
			}
			return nil, fmt.Errorf("error looking up product by sku %q: %w", sku, err)
		}

		existingListings, err := s.channelProductMapRepository.FindAllByProductAndConnection(ctx, product.ID, input.ConnectionID)
		if err != nil {
			return nil, fmt.Errorf("error loading existing listings for product %d: %w", product.ID, err)
		}

		// Nissan (brand id nissanBrandID) never gets more than one listing per
		// connection, even where allows_multiple_listings is true — the sole
		// exception to the per-fitment fan-out. The listing is published with no
		// vehicle fitment attached (mirrors channel_listings' publishReady).
		allowsMultiple := connection.AllowsMultipleListings
		listingVehicleFitmentID := item.VehicleFitmentID
		if product.BrandID != nil && *product.BrandID == nissanBrandID {
			allowsMultiple = false
			listingVehicleFitmentID = nil
		}

		if !allowsMultiple && len(existingListings) > 0 {
			log.Printf("mercadolibre upload: %s product %d already has a listing on connection %d, which does not allow multiple listings", step, product.ID, input.ConnectionID)
			result.Error = "product already has a listing on this connection; this connection does not allow multiple listings per product"
			results = append(results, result)
			continue
		}
		if allowsMultiple && hasExistingFitmentListing(existingListings, listingVehicleFitmentID) {
			log.Printf("mercadolibre upload: %s product %d already has a listing for this vehicle fitment on connection %d", step, product.ID, input.ConnectionID)
			result.Error = "a listing already exists for this vehicle fitment on this connection"
			results = append(results, result)
			continue
		}

		log.Printf("mercadolibre upload: %s resolved product id=%d, creating item", step, product.ID)

		// createItem turns a non-empty missingRequiredAttributes into an error
		// (it would be rejected by MercadoLibre anyway), so only the
		// non-blocking optional ones come back on the success path.
		externalID, _, missingOptionalAttributes, err := s.createItem(ctx, product, input.ConnectionID, name, listingVehicleFitmentID, product.ID, nil, compatDump)
		if err != nil {
			log.Printf("mercadolibre upload: %s failed: %v", step, err)
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		if len(missingOptionalAttributes) > 0 {
			log.Printf("mercadolibre upload: %s created with missing optional attribute(s): %v", step, missingOptionalAttributes)
		}

		log.Printf("mercadolibre upload: %s done, externalId=%s", step, externalID)
		result.Success = true
		result.ExternalID = externalID
		results = append(results, result)
	}

	log.Printf("mercadolibre upload: batch finished, %d result(s)", len(results))

	return &UploadResult{Results: results}, nil
}

// Publish implements channel_listings.Publisher (duck-typed: no import from
// that package is needed here) by creating title as a brand-new MercadoLibre
// item for product, against vehicleFitmentID, with pictures pulled from
// imageSourceProductID (product.ID itself, unless channel_listings resolved
// product's own cover image to be missing and borrowed one from another
// product in its succession chain — see channel_listings.resolveImageSource).
// Unlike Upload, it performs no existing-listing check of its own — the
// channel_listings orchestrator already decided this (product, connection,
// vehicleFitmentID) has no listing yet before calling Publish.
// officialStoreID is sent through to MercadoLibre's official_store_id item
// field as-is, nil included (so a nil here reaches MercadoLibre as an
// explicit JSON null, not an omitted field — see CreateItemVals). The
// returned missingRequiredAttributes/missingOptionalAttributes list any
// attribute (see resolveCustomAttributes) the product had no value for, split
// by MercadoLibre's own Tags.Required for that attribute. A non-empty
// missingRequiredAttributes fails the publish before the POST /items call
// (errMissingRequiredMercadoLibreAttributes) — MercadoLibre would reject the
// item anyway; missingOptionalAttributes never blocks it.
func (s *MercadoLibreProductSyncService) Publish(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, title string, vehicleFitmentID *int64, imageSourceProductID int64, officialStoreID *int64) (string, []string, []string, error) {
	// compatDump nil: the channel_listings orchestrator calls Publish once per
	// (product, fitment); createNewItem's compat push fetches its own dump.
	return s.createItem(ctx, product, connectionID, title, vehicleFitmentID, imageSourceProductID, officialStoreID, nil)
}

// SyncListingStatus implements channel_listings.StatusSyncer (duck-typed: no
// import from that package is needed here). channel_listings.RefreshListings
// calls this before it walks ecom_channel_product_map, so that by the time
// Refresh (below) runs, every row's status already reflects MercadoLibre's
// live state and Refresh's isPausedOrUnderReview gate correctly leaves alone
// a listing the marketplace itself has since put under review/paused —
// otherwise a listing MercadoLibre silently moved to under_review since its
// last local sync would still look "synced" here and get an update call that
// MercadoLibre would reject anyway. It also reconciles category drift the
// same pass (see RefreshChannelProductMapStatusAndCategory), which is why
// Refresh itself no longer needs to detect or send category. Per-row errors
// are carried in RefreshChannelProductMapStatusAndCategory's own result items
// and don't fail this call or stop RefreshListings' batch; only an
// infrastructure-level failure (token, listing load, MercadoLibre status
// fetch) does.
func (s *MercadoLibreProductSyncService) SyncListingStatus(ctx context.Context, connectionID int64) error {
	_, err := s.RefreshChannelProductMapStatusAndCategory(ctx, connectionID)
	return err
}

// Refresh implements channel_listings.Refresher (duck-typed: no import from
// that package is needed here) for an already-published MercadoLibre
// listing. It fires when product's effective price or total stock changed
// since listing.LastSyncedAt (see priceOrStockChangedSince, shared with
// Odoo's Refresh in sync_odoo_products.go) — or unconditionally when the
// listing is currently under_review, since that status is otherwise a dead
// end: MercadoLibre sometimes reactivates a listing without this system's
// local status ever finding out, and the only way to detect that is to just
// retry the update and see whether MercadoLibre now accepts it. The PUT
// /items call sends only price, stock, and attributes — MercadoLibre's
// update endpoint requires price/stock together regardless of which one
// actually moved, and attributes ride along for free so a custom attribute
// value added to the product after the listing was first published (see
// resolveCustomAttributes) reaches MercadoLibre without a separate call.
// Category is deliberately never sent here: channel_listings.RefreshListings
// calls SyncListingStatus right before this, which already reconciles any
// category drift against MercadoLibre's live state (see
// RefreshChannelProductMapStatusAndCategory) — resending it here would just
// repeat that work with a possibly stale local value. Listings currently
// paused/closed are left untouched, same as updateExistingListings — only
// under_review gets this extra retry. A successful retry falls through to
// the same Upsert every other successful update goes through, which always
// writes status "synced" — exactly what should happen once MercadoLibre
// confirms the listing is no longer under review. A retry MercadoLibre still
// rejects is treated as the expected outcome for a listing that's still
// genuinely under review, not a batch error: the row is left completely
// untouched (already recorded as under_review) so the next refresh run
// simply tries again.
func (s *MercadoLibreProductSyncService) Refresh(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO) (bool, error) {
	externalID := derefString(listing.ExternalID)
	if externalID == "" {
		return false, nil
	}
	if listing.Status == mercadoLibreMapStatusPaused || listing.Status == mercadoLibreMapStatusClosed {
		return false, nil
	}
	isUnderReview := listing.Status == mercadoLibreMapStatusUnderReview
	// El sku de pinnedSKUPriceOverrideSKU se refresca siempre, sin pasar por el
	// gate de "solo si cambió precio/stock" (ver mercadoLibreRefreshPriceStockGateEnabled
	// más abajo): su precio fijo debe mantenerse en MercadoLibre incluso cuando
	// nada más del producto cambió desde el último sync.
	isPinnedSKU := isPinnedSKUPriceOverride(product.SKU)

	mxnCurrency, err := s.currenciesRepository.FindByCode(ctx, mercadoLibreItemCurrencyID)
	if err != nil {
		return false, fmt.Errorf("error loading %s currency: %w", mercadoLibreItemCurrencyID, err)
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

	if mercadoLibreRefreshPriceStockGateEnabled && !isUnderReview && !isPinnedSKU && !priceOrStockChangedSince(priceUpdatedAt, stocks, listing.LastSyncedAt) {
		return false, nil
	}

	finalPrice, err := s.formulaCalculator.CalculatePrice(ctx, product.BrandID, connectionID, priceListID, basePrice)
	if err != nil {
		return false, fmt.Errorf("error calculating final price for product %d: %w", product.ID, err)
	}
	finalPrice = applyPinnedSKUPriceOverride(product.SKU, finalPrice)

	qtyAvailable := 0
	for _, stock := range stocks {
		qtyAvailable += stock.AvailableQty
	}

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return false, fmt.Errorf("error getting mercadolibre access token for connection %d: %w", connectionID, err)
	}

	externalCategoryID := derefString(listing.ExternalCategoryID)

	// mercadoLibreRefreshAttributesEnabled temporarily disabled: attributes
	// only need to be sent once, on Publish (item creation) — Refresh no
	// longer resends them on every price/stock push.
	var attributes []mercadoLibreInfra.ItemAttribute
	if mercadoLibreRefreshAttributesEnabled {
		attributes = s.resolvePackageAttributes(ctx, product.ID)
		if externalCategoryID != "" {
			customAttributes, _, _, err := s.resolveCustomAttributes(ctx, product, connectionID, externalCategoryID)
			if err != nil {
				return false, fmt.Errorf("error resolving custom attributes for product %d: %w", product.ID, err)
			}
			attributes = append(attributes, customAttributes...)
		}
	}

	// mercadoLibreRefreshFamilyNameEnabled temporarily enabled: see its doc
	// comment. Reconstructs the same family_name Publish would send today
	// ("<listing.ListingTitle> Original <brand name>" for Nissan, "<title>
	// <brand name>" otherwise) from what's already on
	// ecom_channel_product_map/ecom_brands — no need to touch listingName
	// input, since Refresh never receives it. Left empty (omitted from the
	// request) when the flag is off or the brand can't be resolved, so a
	// missing brand never blanks out an existing title.
	var familyName string
	if mercadoLibreRefreshFamilyNameEnabled && product.BrandID != nil {
		brand, err := s.brandsRepository.FindByID(ctx, *product.BrandID)
		if err != nil {
			log.Printf("mercadolibre: product %d — error loading brand %d for family_name refresh, leaving title unchanged: %v", product.ID, *product.BrandID, err)
		} else {
			familyName = strings.TrimSpace(withNissanOriginalSuffix(product.BrandID, derefString(listing.ListingTitle)) + " " + brand.Name)
		}
	}

	if err := s.itemsHandler.UpdateItem(ctx, mercadoLibreInfra.UpdateItemRequest{
		AccessToken: accessToken,
		ExternalID:  externalID,
		Vals: mercadoLibreInfra.UpdateItemVals{
			Price:             finalPrice,
			AvailableQuantity: qtyAvailable,
			FamilyName:        familyName,
			Attributes:        attributes,
		},
	}); err != nil {
		if isUnderReview {
			log.Printf("mercadolibre: listing %s still under review, leaving status unchanged: %v", externalID, err)
			return false, nil
		}
		return false, fmt.Errorf("error updating mercadolibre item %s for product %d: %w", externalID, product.ID, err)
	}

	if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
		ProductID:          product.ID,
		ConnectionID:       connectionID,
		VehicleFitmentID:   listing.VehicleFitmentID,
		ListingTitle:       derefString(listing.ListingTitle),
		ExternalID:         externalID,
		ExternalCategoryID: externalCategoryID,
		Status:             mercadoLibreMapStatusSynced,
		ActorID:            systemMercadoLibreSyncActorID,
	}); err != nil {
		return false, fmt.Errorf("error recording channel product map for product %d: %w", product.ID, err)
	}

	return true, nil
}

// Resync implements channel_listings.FullRefresher (duck-typed: no import
// from that package is needed here) for an already-published MercadoLibre
// listing. Unlike Refresh, it always calls out (no price/stock-changed gate)
// and always resends attributes (package dimensions plus category-specific
// custom attributes — the same building blocks Refresh uses behind
// mercadoLibreRefreshAttributesEnabled, here unconditional) plus description
// and pictures. It deliberately never sends FamilyName (title) or
// CategoryID: MercadoLibre rejects changing either once a listing already
// exists (title outright on items with sales — see
// mercadoLibreRefreshFamilyNameEnabled's doc comment; category is treated
// the same way here, left to RefreshChannelProductMapStatusAndCategory to
// keep in sync from MercadoLibre's own side instead).
func (s *MercadoLibreProductSyncService) Resync(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO) (bool, error) {
	externalID := derefString(listing.ExternalID)
	if externalID == "" {
		return false, nil
	}
	if listing.Status == mercadoLibreMapStatusPaused || listing.Status == mercadoLibreMapStatusClosed {
		return false, nil
	}
	isUnderReview := listing.Status == mercadoLibreMapStatusUnderReview

	mxnCurrency, err := s.currenciesRepository.FindByCode(ctx, mercadoLibreItemCurrencyID)
	if err != nil {
		return false, fmt.Errorf("error loading %s currency: %w", mercadoLibreItemCurrencyID, err)
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

	finalPrice, err := s.formulaCalculator.CalculatePrice(ctx, product.BrandID, connectionID, priceListID, basePrice)
	if err != nil {
		return false, fmt.Errorf("error calculating final price for product %d: %w", product.ID, err)
	}
	finalPrice = applyPinnedSKUPriceOverride(product.SKU, finalPrice)

	qtyAvailable := 0
	for _, stock := range stocks {
		qtyAvailable += stock.AvailableQty
	}

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return false, fmt.Errorf("error getting mercadolibre access token for connection %d: %w", connectionID, err)
	}

	externalCategoryID := derefString(listing.ExternalCategoryID)

	attributes := s.resolvePackageAttributes(ctx, product.ID)
	if externalCategoryID != "" {
		customAttributes, _, _, err := s.resolveCustomAttributes(ctx, product, connectionID, externalCategoryID)
		if err != nil {
			return false, fmt.Errorf("error resolving custom attributes for product %d: %w", product.ID, err)
		}
		attributes = append(attributes, customAttributes...)
	}

	if err := s.itemsHandler.UpdateItem(ctx, mercadoLibreInfra.UpdateItemRequest{
		AccessToken: accessToken,
		ExternalID:  externalID,
		Vals: mercadoLibreInfra.UpdateItemVals{
			Price:             finalPrice,
			AvailableQuantity: qtyAvailable,
			Attributes:        attributes,
		},
	}); err != nil {
		if isUnderReview {
			log.Printf("mercadolibre: resync — listing %s still under review, leaving status unchanged: %v", externalID, err)
			return false, nil
		}
		return false, fmt.Errorf("error updating mercadolibre item %s for product %d: %w", externalID, product.ID, err)
	}

	if product.Description != nil && strings.TrimSpace(*product.Description) != "" {
		if err := s.itemsHandler.UpdateItemDescription(ctx, mercadoLibreInfra.UpdateItemDescriptionRequest{
			AccessToken: accessToken,
			ExternalID:  externalID,
			PlainText:   *product.Description,
		}); err != nil {
			return false, fmt.Errorf("error updating mercadolibre item description %s for product %d: %w", externalID, product.ID, err)
		}
	}

	images, err := s.productImagesRepository.FindAllByProductID(ctx, product.ID)
	if err != nil {
		return false, fmt.Errorf("error loading images for product %d: %w", product.ID, err)
	}
	if len(images) > 0 {
		pictures := make([]mercadoLibreInfra.ItemPicture, 0, len(images))
		for _, image := range images {
			file, err := s.filesRepository.FindByID(ctx, image.FileID)
			if err != nil {
				return false, fmt.Errorf("error loading file %d for product %d: %w", image.FileID, product.ID, err)
			}
			pictures = append(pictures, mercadoLibreInfra.ItemPicture{Source: file.Path})
		}
		if err := s.itemsHandler.UpdateItemPictures(ctx, mercadoLibreInfra.UpdateItemPicturesRequest{
			AccessToken: accessToken,
			ExternalID:  externalID,
			Pictures:    pictures,
		}); err != nil {
			return false, fmt.Errorf("error updating mercadolibre item pictures %s for product %d: %w", externalID, product.ID, err)
		}
	}

	if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
		ProductID:          product.ID,
		ConnectionID:       connectionID,
		VehicleFitmentID:   listing.VehicleFitmentID,
		ListingTitle:       derefString(listing.ListingTitle),
		ExternalID:         externalID,
		ExternalCategoryID: externalCategoryID,
		Status:             mercadoLibreMapStatusSynced,
		ActorID:            systemMercadoLibreSyncActorID,
	}); err != nil {
		return false, fmt.Errorf("error recording channel product map for product %d: %w", product.ID, err)
	}

	return true, nil
}

// hasExistingFitmentListing reports whether existing already contains a row
// for fitmentID — nil matching nil (the general listing) or an equal
// non-nil id (the same vehicle compatibility).
func hasExistingFitmentListing(existing []mysqlInfra.ChannelProductMapDTO, fitmentID *int64) bool {
	for _, e := range existing {
		if fitmentIDsEqual(e.VehicleFitmentID, fitmentID) {
			return true
		}
	}
	return false
}

func fitmentIDsEqual(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// foldMercadoLibreStatus maps a raw MercadoLibre item status (plus its
// sub_status entries) onto ecom_channel_product_map's status column.
// sub_status is checked first and wins over status: a listing reported as
// "under_review" with sub_status "forbidden" folds to
// mercadoLibreMapStatusClosed, not "under_review" — MercadoLibre's own docs
// confirm this specific combination never recovers through the normal
// review flow, so treating it as merely "under review" would make
// RefreshChannelProductMapStatus (and anything gating on
// isPausedOrUnderReview) keep expecting it to come back on its own. Absent
// that, under_review and paused each map to their own distinct value (see
// isPausedOrUnderReview); every other MercadoLibre status (active, closed,
// inactive, ...) folds into mercadoLibreMapStatusSynced since none of them
// are branched on today.
func foldMercadoLibreStatus(rawStatus string, subStatus []string) string {
	for _, s := range subStatus {
		if strings.EqualFold(strings.TrimSpace(s), mercadoLibreSubStatusForbidden) {
			return mercadoLibreMapStatusClosed
		}
	}

	switch strings.ToLower(strings.TrimSpace(rawStatus)) {
	case mercadoLibreStatusUnderReview:
		return mercadoLibreMapStatusUnderReview
	case mercadoLibreStatusPaused:
		return mercadoLibreMapStatusPaused
	default:
		return mercadoLibreMapStatusSynced
	}
}

// isPausedOrUnderReview reports whether a channel_product_map status means
// "leave this listing alone on the next sync" — true for paused,
// under_review, and closed (which also covers a forbidden listing folded to
// closed by foldMercadoLibreStatus — there's nothing to recover, so it
// should never be retried either).
func isPausedOrUnderReview(status string) bool {
	return status == mercadoLibreMapStatusPaused || status == mercadoLibreMapStatusUnderReview || status == mercadoLibreMapStatusClosed
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// createItem resolves product's current price/stock/access token and
// creates a brand new MercadoLibre listing under listingName (rather than
// product.Name, so the same product can be listed more than once with
// different names), recording it against vehicleFitmentID. Pictures come
// from imageSourceProductID, not necessarily product.ID (see Publish).
func (s *MercadoLibreProductSyncService) createItem(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listingName string, vehicleFitmentID *int64, imageSourceProductID int64, officialStoreID *int64, compatDump []mercadoLibreInfra.DomainDumpEntry) (string, []string, []string, error) {
	finalPrice, qtyAvailable, accessToken, err := s.resolvePriceStockAndToken(ctx, product.ID, product.SKU, product.BrandID, connectionID)
	if err != nil {
		return "", nil, nil, err
	}

	return s.createNewItem(ctx, product, connectionID, listingName, vehicleFitmentID, accessToken, finalPrice, qtyAvailable, imageSourceProductID, officialStoreID, compatDump)
}

// TEMPORARY: sku pinnedSKUPriceOverrideSKU is pinned to a fixed MXN price on
// MercadoLibre only (Odoo is untouched), overriding whatever its pricing
// formula would otherwise compute. Every listing for this sku ends up at
// this price: resolvePriceStockAndToken's single computed finalPrice is
// applied to every one of a product's existing listings by
// updateExistingListings, and a connection with allow_multi=true publishes
// one listing per vehicle fitment via a separate createItem call each —
// each going through this same override independently. Remove
// pinnedSKUPriceOverrideSKU/pinnedSKUPriceOverridePrice,
// applyPinnedSKUPriceOverride, and its two call sites (here and in Refresh)
// once no longer needed.
const (
	pinnedSKUPriceOverrideSKU   = "22042021OM"
	pinnedSKUPriceOverridePrice = 110.00
)

// applyPinnedSKUPriceOverride returns pinnedSKUPriceOverridePrice when sku
// matches pinnedSKUPriceOverrideSKU (case-insensitive), otherwise price
// unchanged.
func applyPinnedSKUPriceOverride(sku string, price float64) float64 {
	if isPinnedSKUPriceOverride(sku) {
		return pinnedSKUPriceOverridePrice
	}
	return price
}

// isPinnedSKUPriceOverride reports whether sku is pinnedSKUPriceOverrideSKU
// (case-insensitive) — shared by applyPinnedSKUPriceOverride and Refresh's
// price/stock-changed gate bypass, so the pinned sku's fixed MercadoLibre
// price keeps being pushed on every refresh even when nothing else about the
// product changed.
func isPinnedSKUPriceOverride(sku string) bool {
	return strings.EqualFold(strings.TrimSpace(sku), pinnedSKUPriceOverrideSKU)
}

// nissanBrandID is ecom_brands.id for "Nissan" in this deployment's seed
// data — hardcoded rather than resolved by name/code since no
// BrandsRepository.FindByCode/FindByName exists today (mirrors
// pinnedSKUPriceOverrideSKU's hardcoded-identifier approach above).
const nissanBrandID int64 = 1

// omnipartsBrandID is ecom_brands.id for "Omniparts" in this deployment's
// seed data — hardcoded rather than resolved by name/code, same as
// nissanBrandID above. Brand-new MercadoLibre listings for this brand must be
// assigned to Omniparts' Official Store: a publish request sends
// official_store_id = null by default (no store), and createNewItem forces it
// to omnipartsOfficialStoreID for this brand, overriding whatever the request
// carried.
const omnipartsBrandID int64 = 36

// omnipartsOfficialStoreID is the MercadoLibre official_store_id for the
// Omniparts Official Store in this deployment.
const omnipartsOfficialStoreID int64 = 71022

// Nissan no longer has an Official Store in this deployment: the store that
// used to back Nissan-branded listings was removed on MercadoLibre's side.
// Brand-new Nissan listings must now be created with official_store_id = null,
// so createNewItem forces officialStoreID back to nil for this brand,
// overriding whatever the publish request carried. See nissanBrandID.

// nissanOriginalSuffix is appended to the title portion of Nissan-branded
// listings' MercadoLibre family_name — producing "<title> Original <brand>"
// — to flag them as genuine Nissan parts, distinct from third-party
// compatible parts sold under other brands. It only affects what's sent to
// MercadoLibre — never ecom_products.name or the category prediction query,
// which stays title+brand-name-only.
const nissanOriginalSuffix = "Original"

// withNissanOriginalSuffix appends nissanOriginalSuffix to name when
// brandID is nissanBrandID, otherwise returns name unchanged. Callers apply
// this to the title alone, then append the brand name afterward, so the
// suffix always lands between the two ("<title> Original <brand>").
func withNissanOriginalSuffix(brandID *int64, name string) string {
	if brandID != nil && *brandID == nissanBrandID {
		return strings.TrimSpace(name + " " + nissanOriginalSuffix)
	}
	return name
}

// mercadoLibreRefreshPriceStockGateEnabled gates Refresh so it only calls out
// to MercadoLibre when product's price or stock actually changed since
// listing.LastSyncedAt (priceOrStockChangedSince below) — see
// odooRefreshPriceStockGateEnabled in sync_odoo_products.go for the same gate
// on Odoo's Refresh, toggled independently. Note: this alone does NOT resend
// package attributes (SELLER_PACKAGE_*) — Refresh only includes those when
// mercadoLibreRefreshAttributesEnabled is also true.
const mercadoLibreRefreshPriceStockGateEnabled = true

// mercadoLibreRefreshAttributesEnabled controls whether Refresh sends item
// attributes (package dimensions plus category-specific custom attributes)
// on every price/stock push. Kept disabled: attributes only need to reach
// MercadoLibre once, at Publish (item creation) — a plain refresh must only
// ever send price and stock, never attributes.
const mercadoLibreRefreshAttributesEnabled = false

// mercadoLibreRefreshFamilyNameEnabled controls whether Refresh resends
// family_name (title) on every price/stock push. Kept disabled: title only
// needs to reach MercadoLibre once, at Publish — same reason as
// mercadoLibreRefreshAttributesEnabled. MercadoLibre also rejects
// family_name changes outright on items that already have sales
// (sold_quantity > 0), so a plain refresh must never send it.
const mercadoLibreRefreshFamilyNameEnabled = false

// resolvePriceStockAndToken computes the current MXN price (via
// formulaCalculator, resolved against brandID, then
// applyPinnedSKUPriceOverride) and total stock for a product, and ensures a
// valid MercadoLibre access token for connectionID — the inputs both
// createItem and UpdatePricesAndStock need before talking to MercadoLibre.
func (s *MercadoLibreProductSyncService) resolvePriceStockAndToken(ctx context.Context, productID int64, sku string, brandID *int64, connectionID int64) (finalPrice float64, qtyAvailable int, accessToken string, err error) {
	log.Printf("mercadolibre: product %d — resolving %s currency", productID, mercadoLibreItemCurrencyID)
	mxnCurrency, err := s.currenciesRepository.FindByCode(ctx, mercadoLibreItemCurrencyID)
	if err != nil {
		return 0, 0, "", fmt.Errorf("error loading %s currency: %w", mercadoLibreItemCurrencyID, err)
	}

	log.Printf("mercadolibre: product %d — resolving effective price", productID)
	basePrice, _, priceListID, err := s.effectivePriceResolver.ResolveInCurrency(ctx, productID, mxnCurrency.ID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductPriceNotFound) {
			return 0, 0, "", fmt.Errorf("no active price list entry for product %d", productID)
		}
		return 0, 0, "", fmt.Errorf("error loading effective price for product %d: %w", productID, err)
	}
	finalPrice, err = s.formulaCalculator.CalculatePrice(ctx, brandID, connectionID, priceListID, basePrice)
	if err != nil {
		return 0, 0, "", fmt.Errorf("error calculating final price for product %d: %w", productID, err)
	}
	finalPrice = applyPinnedSKUPriceOverride(sku, finalPrice)
	log.Printf("mercadolibre: product %d — base price=%.2f, final price=%.2f", productID, basePrice, finalPrice)

	log.Printf("mercadolibre: product %d — summing stock", productID)
	qtyAvailable, err = s.sumAvailableStock(ctx, productID)
	if err != nil {
		return 0, 0, "", fmt.Errorf("error summing stock for product %d: %w", productID, err)
	}
	log.Printf("mercadolibre: product %d — qty available=%d", productID, qtyAvailable)

	log.Printf("mercadolibre: product %d — ensuring valid access token for connection %d", productID, connectionID)
	accessToken, err = s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return 0, 0, "", fmt.Errorf("error getting mercadolibre access token for connection %d: %w", connectionID, err)
	}
	log.Printf("mercadolibre: product %d — access token ready", productID)

	return finalPrice, qtyAvailable, accessToken, nil
}

// updateListingsOutcome reports what UpdatePricesAndStock did for a
// product's existing listings: which ones got price/stock refreshed, and
// which were left alone because of their status.
type updateListingsOutcome struct {
	UpdatedExternalIDs []string
	SkippedExternalIDs []string
}

// updateExistingListings refreshes price/stock on every one of a product's
// existing listings — no pictures/attributes/category/title, since those
// only apply when creating a new listing. Listings whose last known status
// is paused or under_review (see isPausedOrUnderReview) are left untouched.
// A single listing's update failing does
// not stop the others from being attempted; the first such error is still
// returned (after every listing has been tried) so the batch caller can
// report it.
func (s *MercadoLibreProductSyncService) updateExistingListings(
	ctx context.Context,
	product *mysqlInfra.ProductDTO,
	existingListings []mysqlInfra.ChannelProductMapDTO,
	accessToken string,
	finalPrice float64,
	qtyAvailable int,
) (*updateListingsOutcome, error) {
	outcome := &updateListingsOutcome{}
	var firstErr error

	for _, listing := range existingListings {
		externalID := derefString(listing.ExternalID)
		if externalID == "" {
			continue
		}

		if isPausedOrUnderReview(listing.Status) {
			log.Printf("mercadolibre update: product %d — listing %s is paused/under_review, skipping", product.ID, externalID)
			outcome.SkippedExternalIDs = append(outcome.SkippedExternalIDs, externalID)
			continue
		}

		log.Printf("mercadolibre update: product %d — calling PUT /items/%s (price/stock/package dimensions)", product.ID, externalID)
		if err := s.itemsHandler.UpdateItem(ctx, mercadoLibreInfra.UpdateItemRequest{
			AccessToken: accessToken,
			ExternalID:  externalID,
			Vals: mercadoLibreInfra.UpdateItemVals{
				Price:             finalPrice,
				AvailableQuantity: qtyAvailable,
				Attributes:        s.resolvePackageAttributes(ctx, product.ID),
			},
		}); err != nil {
			log.Printf("mercadolibre update: product %d — error updating listing %s: %v", product.ID, externalID, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("error updating mercadolibre item %s for product %d: %w", externalID, product.ID, err)
			}
			continue
		}

		if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
			ProductID:          product.ID,
			ConnectionID:       listing.ConnectionID,
			VehicleFitmentID:   listing.VehicleFitmentID,
			ListingTitle:       derefString(listing.ListingTitle),
			ExternalID:         externalID,
			ExternalCategoryID: derefString(listing.ExternalCategoryID),
			Status:             mercadoLibreMapStatusSynced,
			ActorID:            systemMercadoLibreSyncActorID,
		}); err != nil {
			log.Printf("mercadolibre update: product %d — error recording update for listing %s: %v", product.ID, externalID, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("error recording channel product map update for product %d (listing %s): %w", product.ID, externalID, err)
			}
			continue
		}

		log.Printf("mercadolibre update: product %d — listing %s updated", product.ID, externalID)
		outcome.UpdatedExternalIDs = append(outcome.UpdatedExternalIDs, externalID)
	}

	if firstErr != nil {
		return nil, firstErr
	}

	return outcome, nil
}

// resolvePackageAttributes returns the SELLER_PACKAGE_* attributes for
// productID's current ecom_product_dimensions row: weight converted from kg
// to grams and length/width/height passed through as cm (the shared cm/kg
// convention every ecom_product_dimensions value is uploaded in — see
// product_media.BulkUpsertDimensions). Diameter isn't part of MercadoLibre's
// package attributes and is never read here. Falls back to the generic
// mercadoLibreItemPackageSide/Weight defaults when productID has no
// dimensions row yet, mirroring resolveDimensions' fallback for Odoo.
func (s *MercadoLibreProductSyncService) resolvePackageAttributes(ctx context.Context, productID int64) []mercadoLibreInfra.ItemAttribute {
	fallback := []mercadoLibreInfra.ItemAttribute{
		{ID: "SELLER_PACKAGE_HEIGHT", ValueName: mercadoLibreItemPackageSide},
		{ID: "SELLER_PACKAGE_LENGTH", ValueName: mercadoLibreItemPackageSide},
		{ID: "SELLER_PACKAGE_WEIGHT", ValueName: mercadoLibreItemPackageWeight},
		{ID: "SELLER_PACKAGE_WIDTH", ValueName: mercadoLibreItemPackageSide},
	}

	dimensions, err := s.productDimensionsRepository.FindByProductID(ctx, productID)
	if err != nil {
		return fallback
	}

	return []mercadoLibreInfra.ItemAttribute{
		{ID: "SELLER_PACKAGE_HEIGHT", ValueName: formatPackageCM(dimensions.Height)},
		{ID: "SELLER_PACKAGE_LENGTH", ValueName: formatPackageCM(dimensions.Length)},
		{ID: "SELLER_PACKAGE_WEIGHT", ValueName: formatPackageGrams(dimensions.Weight)},
		{ID: "SELLER_PACKAGE_WIDTH", ValueName: formatPackageCM(dimensions.Width)},
	}
}

// formatPackageCM renders an ecom_product_dimensions cm value (e.g. "12.50")
// as MercadoLibre's "<value> cm" package attribute format. MercadoLibre only
// accepts integers for these attributes — sending a decimal (its own
// stored precision) is rejected with item.attribute.invalid.format.seller.
// package.dimensions — so the value is rounded to the nearest whole
// centimeter and floored at 1 (0 cm isn't a valid package side). Falls back
// to the generic package side default when raw fails to parse.
func formatPackageCM(raw string) string {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return mercadoLibreItemPackageSide
	}
	return strconv.FormatFloat(roundPackageValue(value), 'f', 0, 64) + " cm"
}

// formatPackageGrams converts an ecom_product_dimensions weight value (kg)
// into MercadoLibre's "<value> g" package attribute format, rounded to the
// nearest whole gram and floored at 1 for the same integer-only reason as
// formatPackageCM. Falls back to the generic package weight default when
// raw fails to parse.
func formatPackageGrams(raw string) string {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return mercadoLibreItemPackageWeight
	}
	return strconv.FormatFloat(roundPackageValue(value*1000), 'f', 0, 64) + " g"
}

// roundPackageValue rounds to the nearest integer and floors at 1, since
// MercadoLibre rejects both non-integer and zero/negative package
// dimensions/weight.
func roundPackageValue(value float64) float64 {
	rounded := math.Round(value)
	if rounded < 1 {
		return 1
	}
	return rounded
}

// createNewItem resolves brand/images, predicts a category from listingName
// + brand name, creates a brand new listing with the full payload, and
// records it in ecom_channel_product_map against vehicleFitmentID. Images
// are loaded from imageSourceProductID rather than product.ID: normally the
// same product, but channel_listings can point this at a different product
// in product's succession chain (ecom_part_number_supersessions) when
// product itself has no cover image of its own.
func (s *MercadoLibreProductSyncService) createNewItem(
	ctx context.Context,
	product *mysqlInfra.ProductDTO,
	connectionID int64,
	listingName string,
	vehicleFitmentID *int64,
	accessToken string,
	finalPrice float64,
	qtyAvailable int,
	imageSourceProductID int64,
	officialStoreID *int64,
	compatDump []mercadoLibreInfra.DomainDumpEntry,
) (string, []string, []string, error) {
	log.Printf("mercadolibre upload: product %d — resolving brand", product.ID)
	if product.BrandID == nil {
		return "", nil, nil, fmt.Errorf("product %d has no brand", product.ID)
	}

	brand, err := s.brandsRepository.FindByID(ctx, *product.BrandID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrBrandNotFound) {
			return "", nil, nil, fmt.Errorf("brand %d not found for product %d", *product.BrandID, product.ID)
		}
		return "", nil, nil, fmt.Errorf("error loading brand for product %d: %w", product.ID, err)
	}
	log.Printf("mercadolibre upload: product %d — brand resolved: %s", product.ID, brand.Name)

	// Certain brands are pinned to a specific Official Store, regardless of the
	// official_store_id the publish request carried (nil by default). Nissan is
	// pinned the other way: its Official Store no longer exists, so its listings
	// must always go out with official_store_id = null even if the request
	// carried a value. Any other brand keeps the request's value (nil = no
	// store). See omnipartsBrandID / omnipartsOfficialStoreID and nissanBrandID.
	if product.BrandID != nil {
		switch *product.BrandID {
		case omnipartsBrandID:
			storeID := omnipartsOfficialStoreID
			officialStoreID = &storeID
			log.Printf("mercadolibre upload: product %d — brand is Omniparts, forcing official_store_id=%d", product.ID, omnipartsOfficialStoreID)
		case nissanBrandID:
			officialStoreID = nil
			log.Printf("mercadolibre upload: product %d — brand is Nissan, forcing official_store_id=null (no Official Store)", product.ID)
		}
	}

	log.Printf("mercadolibre upload: product %d — loading images (source product %d)", product.ID, imageSourceProductID)
	images, err := s.productImagesRepository.FindAllByProductID(ctx, imageSourceProductID)
	if err != nil {
		return "", nil, nil, fmt.Errorf("error loading images for product %d: %w", imageSourceProductID, err)
	}
	if len(images) == 0 {
		return "", nil, nil, fmt.Errorf("product %d has no images (image source product %d)", product.ID, imageSourceProductID)
	}
	log.Printf("mercadolibre upload: product %d — %d image(s) found, resolving file urls", product.ID, len(images))

	pictures := make([]mercadoLibreInfra.ItemPicture, 0, len(images))
	for _, image := range images {
		file, err := s.filesRepository.FindByID(ctx, image.FileID)
		if err != nil {
			return "", nil, nil, fmt.Errorf("error loading file %d for product %d: %w", image.FileID, product.ID, err)
		}
		pictures = append(pictures, mercadoLibreInfra.ItemPicture{Source: file.Path})
	}
	log.Printf("mercadolibre upload: product %d — pictures ready", product.ID)

	query := strings.TrimSpace(listingName + " " + brand.Name)
	externalCategoryID, err := s.resolveExternalCategoryID(ctx, product, connectionID, query)
	if err != nil {
		return "", nil, nil, err
	}

	attributes := []mercadoLibreInfra.ItemAttribute{
		{ID: "BRAND", ValueName: brand.Name},
		{ID: "PART_NUMBER", ValueName: product.PartNumber},
		{ID: "ITEM_CONDITION", ValueID: mercadoLibreItemConditionValue},
		{ID: "MODEL", ValueName: product.PartNumber},
		{ID: "SELLER_SKU", ValueName: product.SKU},
	}
	attributes = append(attributes, s.resolvePackageAttributes(ctx, product.ID)...)

	customAttributes, missingRequiredAttributes, missingOptionalAttributes, err := s.resolveCustomAttributes(ctx, product, connectionID, externalCategoryID)
	if err != nil {
		return "", nil, nil, err
	}
	attributes = append(attributes, customAttributes...)

	// Stop before the POST when a required custom attribute has no value (or
	// couldn't be provisioned): MercadoLibre rejects the item with
	// item.attributes.missing_required anyway, and failing here names exactly
	// which attributes to fill in (via the product's Atributos checklist or
	// POST /api/marketplaces/mercadolibre/product-attributes) instead of
	// surfacing MercadoLibre's opaque 400.
	if len(missingRequiredAttributes) > 0 {
		return "", missingRequiredAttributes, missingOptionalAttributes,
			fmt.Errorf("%w for category %s: %s", errMissingRequiredMercadoLibreAttributes, externalCategoryID, strings.Join(missingRequiredAttributes, ", "))
	}

	vals := mercadoLibreInfra.CreateItemVals{
		Price:             finalPrice,
		AvailableQuantity: qtyAvailable,
		CurrencyID:        mercadoLibreItemCurrencyID,
		Condition:         mercadoLibreItemCondition,
		BuyingMode:        mercadoLibreItemBuyingMode,
		ListingTypeID:     mercadoLibreItemListingTypeID,
		CategoryID:        externalCategoryID,
		OfficialStoreID:   officialStoreID,
		FamilyName:        strings.TrimSpace(withNissanOriginalSuffix(product.BrandID, listingName) + " " + brand.Name),
		Pictures:          pictures,
		Attributes:        attributes,
	}

	log.Printf("mercadolibre upload: product %d — calling POST /items", product.ID)
	created, err := s.itemsHandler.CreateItem(ctx, mercadoLibreInfra.CreateItemRequest{
		AccessToken: accessToken,
		Vals:        vals,
	})
	if err != nil {
		return "", nil, nil, fmt.Errorf("error creating mercadolibre item for product %d: %w", product.ID, err)
	}
	log.Printf("mercadolibre upload: product %d — item created, externalId=%s, categoryId=%s", product.ID, created.ID, created.CategoryID)

	if _, err := s.syncCategoryMapping(ctx, product, connectionID, created.CategoryID); err != nil {
		return "", nil, nil, err
	}

	if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
		ProductID:          product.ID,
		ConnectionID:       connectionID,
		VehicleFitmentID:   vehicleFitmentID,
		ListingTitle:       listingName,
		ExternalID:         created.ID,
		ExternalCategoryID: created.CategoryID,
		Status:             mercadoLibreMapStatusSynced,
		ActorID:            systemMercadoLibreSyncActorID,
	}); err != nil {
		return "", nil, nil, fmt.Errorf("error recording channel product map for product %d: %w", product.ID, err)
	}
	log.Printf("mercadolibre upload: product %d — channel product map saved", product.ID)

	if product.Description != nil && strings.TrimSpace(*product.Description) != "" {
		if err := s.itemsHandler.CreateItemDescription(ctx, mercadoLibreInfra.CreateItemDescriptionRequest{
			AccessToken: accessToken,
			ExternalID:  created.ID,
			PlainText:   *product.Description,
		}); err != nil {
			return "", nil, nil, fmt.Errorf("error creating mercadolibre item description for product %d (externalId=%s): %w", product.ID, created.ID, err)
		}
		log.Printf("mercadolibre upload: product %d — description created for externalId=%s", product.ID, created.ID)
	}

	// Report the product's vehicle compatibilities up front (when the category
	// supports them and we have data) so the listing doesn't get the
	// incomplete_compatibilities tag and wait for the daily
	// CompatibilitiesFixScheduler. Non-fatal: the item is already created and
	// recorded as synced; a failure here just leaves the scheduler to retry.
	if s.compatibilityService != nil {
		outcome, err := s.compatibilityService.PushForNewListing(ctx, accessToken, created.ID, product, compatDump)
		switch {
		case err != nil:
			log.Printf("mercadolibre upload: product %d — compatibilities push failed for externalId=%s: %v", product.ID, created.ID, err)
		case outcome.Error != "":
			log.Printf("mercadolibre upload: product %d — compatibilities push errored for externalId=%s: %s", product.ID, created.ID, outcome.Error)
		case outcome.Skipped:
			log.Printf("mercadolibre upload: product %d — compatibilities push skipped for externalId=%s: %s", product.ID, created.ID, outcome.SkipReason)
		default:
			log.Printf("mercadolibre upload: product %d — %d vehicle compatibilit(ies) reported at creation for externalId=%s", product.ID, outcome.CreatedCompatibilitiesCount, created.ID)
		}
	}

	return created.ID, missingRequiredAttributes, missingOptionalAttributes, nil
}

// resolveCustomAttributes provisions (via
// channelAttributeValuesService.ProvisionCategoryAttributes, reusing the same
// find-or-create chain /product-attributes/provision uses) every attribute
// MercadoLibre exposes for externalCategoryID — required and optional alike,
// so optional slots exist to fill in later for better listing exposure —
// then builds the ItemAttribute entries for the ones product already has a
// value for. system_field-sourced slots (BRAND, PART_NUMBER, MODEL,
// SELLER_SKU, SELLER_PACKAGE_*) are skipped here — createNewItem's caller
// already sends those directly from product/brand/ecom_product_dimensions
// data, so adding them again would just duplicate the same key. A
// custom_attribute slot with no ecom_product_attributes value yet — and a slot
// whose provisioning errored (e.g. a data-type conflict), which is otherwise
// unsendable — is left out of the payload and reported back in
// missingRequiredAttributes or missingOptionalAttributes instead (split by
// outcome.IsRequired, i.e. MercadoLibre's own Tags.Required for that
// attribute). createNewItem then refuses to publish when
// missingRequiredAttributes is non-empty, since MercadoLibre would reject the
// item with item.attributes.missing_required anyway.
func (s *MercadoLibreProductSyncService) resolveCustomAttributes(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, externalCategoryID string) (attributes []mercadoLibreInfra.ItemAttribute, missingRequiredAttributes []string, missingOptionalAttributes []string, err error) {
	provisioned, err := s.channelAttributeValuesService.ProvisionCategoryAttributes(ctx, channelAttributeValuesApp.ProvisionCategoryAttributesInput{
		SKU:          product.SKU,
		CategoryID:   externalCategoryID,
		ConnectionID: &connectionID,
		ActorID:      systemMercadoLibreSyncActorID,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error provisioning attributes for category %s: %w", externalCategoryID, err)
	}

	for _, outcome := range provisioned.Results {
		if outcome.Skipped || outcome.SourceType == "system_field" {
			continue
		}

		// A slot that didn't resolve to a usable custom_attribute (provisioning
		// errored — e.g. ErrAttributeDataTypeConflict) can't be sent. Don't drop
		// it silently: if MercadoLibre marks it required, report it so the
		// publish is blocked with a message that names it, instead of surfacing
		// MercadoLibre's opaque 400.
		if outcome.SourceType != "custom_attribute" || outcome.AttributeID == nil {
			if outcome.IsRequired {
				missingRequiredAttributes = append(missingRequiredAttributes, outcome.ExternalKey)
			}
			continue
		}

		value, err := s.productAttributesRepository.FindByProductAndAttribute(ctx, product.ID, *outcome.AttributeID)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrProductAttributeNotFound) {
				if outcome.IsRequired {
					missingRequiredAttributes = append(missingRequiredAttributes, outcome.ExternalKey)
				} else {
					missingOptionalAttributes = append(missingOptionalAttributes, outcome.ExternalKey)
				}
				continue
			}
			return nil, nil, nil, fmt.Errorf("error loading product attribute %s for product %d: %w", outcome.ExternalKey, product.ID, err)
		}

		formatted, isValueID, ok := s.formatProductAttributeValue(ctx, value)
		if !ok {
			if outcome.IsRequired {
				missingRequiredAttributes = append(missingRequiredAttributes, outcome.ExternalKey)
			} else {
				missingOptionalAttributes = append(missingOptionalAttributes, outcome.ExternalKey)
			}
			continue
		}

		item := mercadoLibreInfra.ItemAttribute{ID: outcome.ExternalKey}
		if isValueID {
			item.ValueID = formatted
		} else {
			item.ValueName = formatted
		}
		attributes = append(attributes, item)
	}

	return attributes, missingRequiredAttributes, missingOptionalAttributes, nil
}

// formatProductAttributeValue renders whichever typed column value has set
// (matching its attribute's data_type — see attributes.valueMatchesDataType)
// as the plain string MercadoLibre expects, plus whether that string is a
// value_id (true) or a value_name (true means the caller must set
// ItemAttribute.ValueID instead of ValueName — see resolveCustomAttributes).
// Only an enum value resolved to an option provisioned from MercadoLibre's
// own closed list (ecom_attribute_options.external_value_id set — see
// provisionAttributeOptions) is ever a value_id; every other type, and a
// free-text enum option with no external_value_id (created before this
// attribute was list-provisioned, or through the plain CRUD endpoint), is
// sent as value_name same as before. ok is false when value has no usable
// content (e.g. an empty ValueText, or an OptionID that no longer resolves)
// — resolveCustomAttributes treats that the same as no value at all.
func (s *MercadoLibreProductSyncService) formatProductAttributeValue(ctx context.Context, value *mysqlInfra.ProductAttributeDTO) (formatted string, isValueID bool, ok bool) {
	switch {
	case value.ValueText != nil:
		text := strings.TrimSpace(*value.ValueText)
		return text, false, text != ""
	case value.ValueNumber != nil:
		return strconv.FormatFloat(*value.ValueNumber, 'f', -1, 64), false, true
	case value.ValueBool != nil:
		return formatMercadoLibreBoolean(*value.ValueBool), false, true
	case value.ValueDate != nil:
		return value.ValueDate.Format(time.DateOnly), false, true
	case value.OptionID != nil:
		option, err := s.attributeOptionsRepository.FindByID(ctx, *value.OptionID)
		if err != nil {
			return "", false, false
		}
		if option.ExternalValueID != nil && strings.TrimSpace(*option.ExternalValueID) != "" {
			return *option.ExternalValueID, true, true
		}
		return option.Value, false, true
	default:
		return "", false, false
	}
}

// formatMercadoLibreBoolean renders value as the value_name MercadoLibre's
// item-attribute validation actually accepts for a boolean attribute — Go's
// own "true"/"false" (strconv.FormatBool) is rejected with
// invalid.item.attribute.values, matching parseMercadoLibreBoolean's own
// accepted spelling on the read side.
func formatMercadoLibreBoolean(value bool) string {
	if value {
		return "Sí"
	}
	return "No"
}

// resolveExternalCategoryID returns the MercadoLibre category id to list
// product under. It first checks for a per-(product, connectionID) category
// picked in the "Sincronización" tab before this product had any listing on
// connectionID (ecom_channel_product_category_selection — see ADR 0005),
// returning that directly when present. Deliberately does NOT fall back to
// ecom_channel_category_map (the local-category-level mapping): that map is
// keyed only by (local category, connection), so reusing it here is exactly
// the bug ADR 0005 fixes — once any product under a shared local category
// (e.g. "Frenos") resolved to some ML category, every other product under
// that same local category would silently inherit it too, even a wrong one
// (e.g. balatas getting listed under "Frenos"). With no selection on record,
// query is always sent to the predictor and its best match is used — this
// covers the automated discovery/queue publish path (workers.MarketplaceWorker),
// which never goes through the manual picker.
func (s *MercadoLibreProductSyncService) resolveExternalCategoryID(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, query string) (string, error) {
	selection, err := s.categorySelectionRepository.FindByProductAndConnection(ctx, product.ID, connectionID)
	if err != nil && !errors.Is(err, mysqlInfra.ErrChannelProductCategorySelectionNotFound) {
		return "", fmt.Errorf("error loading category selection for product %d: %w", product.ID, err)
	}
	if selection != nil {
		log.Printf("mercadolibre upload: product %d — using category %s selected for connection %d, skipping category predictor", product.ID, selection.ExternalCategoryID, connectionID)
		return selection.ExternalCategoryID, nil
	}

	log.Printf("mercadolibre upload: product %d — calling category predictor (query=%q)", product.ID, query)
	predictorResult, err := s.categoryPredictorService.PredictCategories(ctx, connectionID, query, "", mercadoLibreCategoryPredictLimit)
	if err != nil {
		return "", fmt.Errorf("error predicting category for product %d: %w", product.ID, err)
	}
	if len(predictorResult.Predictions) == 0 {
		return "", fmt.Errorf("no category prediction for product %d (query %q)", product.ID, query)
	}
	bestPrediction := predictorResult.Predictions[0]
	log.Printf("mercadolibre upload: product %d — category resolved: %s (%s)", product.ID, bestPrediction.CategoryID, bestPrediction.CategoryName)

	return bestPrediction.CategoryID, nil
}

// syncCategoryMapping ensures externalCategoryID (MercadoLibre's own
// category id, as returned by item creation) has a local ecom_categories
// hierarchy and ecom_channel_category_map row for connectionID — replicating
// MercadoLibre's own category tree the first time this external category is
// seen for this connection — then assigns that hierarchy's leaf category to
// product only when product has no category yet. ecom_products.category_id
// is a single column shared by every channel (Odoo, MercadoLibre, etc.), so a
// product that already has a category — curated manually, assigned by
// another channel, or by a previous MercadoLibre publish — is never
// reassigned here just because THIS connection didn't have it mapped yet:
// doing so would silently move the product away from a category some other
// connection's ecom_channel_category_map row still points at. The
// channel_category_map link for externalCategoryID is still recorded (via
// EnsureLocalCategory above) regardless, so category prediction keeps
// working; it just doesn't get written back onto the product.
// The returned int64 is EnsureLocalCategory's leaf ecom_categories.id for
// externalCategoryID regardless of which branch returns it — including the
// "product already has a category" early return — so callers that only care
// about the local category id (see RefreshChannelProductMapStatusAndCategory)
// don't need to re-derive it themselves.
func (s *MercadoLibreProductSyncService) syncCategoryMapping(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, externalCategoryID string) (int64, error) {
	if strings.TrimSpace(externalCategoryID) == "" {
		return 0, nil
	}

	leafCategoryID, err := s.categoryPredictorService.EnsureLocalCategory(ctx, connectionID, externalCategoryID, systemMercadoLibreSyncActorID)
	if err != nil {
		return 0, fmt.Errorf("error ensuring local category for external category %q: %w", externalCategoryID, err)
	}

	if product.CategoryID != nil {
		return leafCategoryID, nil
	}

	if err := s.productRepository.UpdateCategoryID(ctx, product.ID, leafCategoryID, systemMercadoLibreSyncActorID); err != nil {
		return leafCategoryID, fmt.Errorf("error assigning category %d to product %d: %w", leafCategoryID, product.ID, err)
	}

	// publishReady reuses this same product pointer across every vehicle
	// compatibility of one publish batch; without updating it here,
	// resolveExternalCategoryID would keep seeing a nil CategoryID on later
	// iterations and call the MercadoLibre predictor again for each one.
	product.CategoryID = &leafCategoryID

	return leafCategoryID, nil
}

// UpdatePricesStockInput carries the skus whose already-listed MercadoLibre
// items should have their price/stock refreshed.
type UpdatePricesStockInput struct {
	ConnectionID int64
	SKUs         []string
}

// UpdatePricesStockResultItem reports the outcome for a single sku. A batch
// never fails wholesale on one bad sku.
type UpdatePricesStockResultItem struct {
	SKU                string   `json:"sku"`
	Success            bool     `json:"success"`
	UpdatedExternalIDs []string `json:"updatedExternalIds,omitempty"`
	SkippedExternalIDs []string `json:"skippedExternalIds,omitempty"`
	Error              string   `json:"error,omitempty"`
}

type UpdatePricesStockResult struct {
	Results []UpdatePricesStockResultItem `json:"results"`
}

// UpdatePricesAndStock refreshes price/stock on every existing MercadoLibre
// listing for each sku on input.ConnectionID — every row already recorded
// for that (product, connection) pair in ecom_channel_product_map — without
// creating anything new. Skus with no existing listing on that connection
// are reported as errors rather than silently creating one (use Upload for
// that). Listings that are under_review or paused are left untouched.
func (s *MercadoLibreProductSyncService) UpdatePricesAndStock(ctx context.Context, input UpdatePricesStockInput) (*UpdatePricesStockResult, error) {
	if input.ConnectionID <= 0 || len(input.SKUs) == 0 {
		return nil, ErrEmptyMercadoLibreUpdate
	}

	log.Printf("mercadolibre update: starting batch of %d sku(s) for connection %d", len(input.SKUs), input.ConnectionID)

	results := make([]UpdatePricesStockResultItem, 0, len(input.SKUs))

	for i, rawSKU := range input.SKUs {
		sku := strings.TrimSpace(rawSKU)
		result := UpdatePricesStockResultItem{SKU: sku}
		step := fmt.Sprintf("[%d/%d sku=%q]", i+1, len(input.SKUs), sku)

		log.Printf("mercadolibre update: %s starting", step)

		if sku == "" {
			log.Printf("mercadolibre update: %s skipped, sku is required", step)
			result.Error = "sku is required"
			results = append(results, result)
			continue
		}

		product, err := s.productRepository.FindBySKU(ctx, sku)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrProductNotFound) {
				log.Printf("mercadolibre update: %s product not found", step)
				result.Error = "product not found for sku"
				results = append(results, result)
				continue
			}
			return nil, fmt.Errorf("error looking up product by sku %q: %w", sku, err)
		}

		existingListings, err := s.channelProductMapRepository.FindAllByProductAndConnection(ctx, product.ID, input.ConnectionID)
		if err != nil {
			return nil, fmt.Errorf("error loading channel product map for product %d: %w", product.ID, err)
		}
		if len(existingListings) == 0 {
			log.Printf("mercadolibre update: %s product %d has no existing listing on connection %d", step, product.ID, input.ConnectionID)
			result.Error = "product has no existing mercadolibre listing on this connection"
			results = append(results, result)
			continue
		}

		finalPrice, qtyAvailable, accessToken, err := s.resolvePriceStockAndToken(ctx, product.ID, product.SKU, product.BrandID, input.ConnectionID)
		if err != nil {
			log.Printf("mercadolibre update: %s failed: %v", step, err)
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		outcome, err := s.updateExistingListings(ctx, product, existingListings, accessToken, finalPrice, qtyAvailable)
		if err != nil {
			log.Printf("mercadolibre update: %s failed: %v", step, err)
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		log.Printf("mercadolibre update: %s done, updated=%v skipped=%v", step, outcome.UpdatedExternalIDs, outcome.SkippedExternalIDs)
		result.Success = true
		result.UpdatedExternalIDs = outcome.UpdatedExternalIDs
		result.SkippedExternalIDs = outcome.SkippedExternalIDs
		results = append(results, result)
	}

	log.Printf("mercadolibre update: batch finished, %d result(s)", len(results))

	return &UpdatePricesStockResult{Results: results}, nil
}

func (s *MercadoLibreProductSyncService) sumAvailableStock(ctx context.Context, productID int64) (int, error) {
	stocks, err := s.productStockRepository.FindByProductID(ctx, productID)
	if err != nil {
		return 0, err
	}

	total := 0
	for _, stock := range stocks {
		total += stock.AvailableQty
	}

	return total, nil
}

// ChannelProductMapStatusResultItem reports the outcome of refreshing a
// single ecom_channel_product_map row's status from MercadoLibre.
type ChannelProductMapStatusResultItem struct {
	ProductID int64  `json:"productId"`
	SKU       string `json:"sku,omitempty"`
	// VehicleFitmentID mirrors ecom_channel_product_map.vehicle_fitment_id —
	// nil for a general listing, set for one specific vehicle compatibility
	// on a connection where allows_multiple_listings is true.
	VehicleFitmentID *int64 `json:"vehicleFitmentId,omitempty"`
	ExternalID       string `json:"externalId"`
	// PreviousStatus/Status are both ecom_channel_product_map's own status
	// vocabulary (see foldMercadoLibreStatus), not MercadoLibre's raw one —
	// Changed is true only when they differ, so a caller can tell "checked,
	// nothing moved" apart from "checked, updated" at a glance.
	PreviousStatus string `json:"previousStatus,omitempty"`
	Status         string `json:"status,omitempty"`
	Changed        bool   `json:"changed,omitempty"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

type ChannelProductMapStatusResult struct {
	Results []ChannelProductMapStatusResultItem `json:"results"`
}

// RefreshChannelProductMapStatus queries MercadoLibre for the current status
// of every ecom_channel_product_map row recorded for connectionID — across
// every product, general listings and per-vehicle-fitment ones alike — and
// persists any change back into that row's status/last_synced_at (via
// ChannelProductMapRepository.UpdateStatus). This is the complement to
// RefreshListings/Refresh above: those only ever *push* a locally-detected
// price/stock change out to MercadoLibre and never re-check MercadoLibre's
// own state, so a listing paused or put under review directly on
// MercadoLibre (by the seller from its own site, or by MercadoLibre's policy
// enforcement) would otherwise go unnoticed here indefinitely — including
// isPausedOrUnderReview continuing to see the stale local status and (not)
// skip it accordingly. Rows with no external_id yet (never successfully
// published) are skipped, same as RefreshListings' refreshOne.
func (s *MercadoLibreProductSyncService) RefreshChannelProductMapStatus(ctx context.Context, connectionID int64) (*ChannelProductMapStatusResult, error) {
	if connectionID <= 0 {
		return nil, ErrInvalidMercadoLibreConnection
	}

	listings, err := s.channelProductMapRepository.FindAllByConnectionID(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel product map for connection %d: %w", connectionID, err)
	}

	type pendingListing struct {
		listing    mysqlInfra.ChannelProductMapDTO
		externalID string
		sku        string
	}

	pending := make([]pendingListing, 0, len(listings))
	externalIDs := make([]string, 0, len(listings))
	seen := make(map[string]bool, len(listings))

	for _, listing := range listings {
		externalID := derefString(listing.ExternalID)
		if externalID == "" {
			continue
		}

		sku := ""
		if product, err := s.productRepository.FindByID(ctx, listing.ProductID); err == nil {
			sku = product.SKU
		}

		pending = append(pending, pendingListing{listing: listing, externalID: externalID, sku: sku})
		if !seen[externalID] {
			seen[externalID] = true
			externalIDs = append(externalIDs, externalID)
		}
	}

	results := make([]ChannelProductMapStatusResultItem, 0, len(pending))

	if len(externalIDs) == 0 {
		return &ChannelProductMapStatusResult{Results: results}, nil
	}

	log.Printf("mercadolibre channel product map status refresh: connection %d — checking %d listing(s), %d distinct external id(s)", connectionID, len(pending), len(externalIDs))

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error getting mercadolibre access token for connection %d: %w", connectionID, err)
	}

	statuses, err := s.itemsHandler.GetItemsStatus(ctx, accessToken, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("error fetching mercadolibre item statuses: %w", err)
	}

	statusByExternalID := make(map[string]mercadoLibreInfra.ItemStatus, len(statuses))
	for _, itemStatus := range statuses {
		statusByExternalID[itemStatus.ID] = itemStatus
	}

	for _, item := range pending {
		result := ChannelProductMapStatusResultItem{
			ProductID:        item.listing.ProductID,
			SKU:              item.sku,
			VehicleFitmentID: item.listing.VehicleFitmentID,
			ExternalID:       item.externalID,
			PreviousStatus:   item.listing.Status,
		}

		itemStatus, ok := statusByExternalID[item.externalID]
		if !ok {
			result.Error = "status not returned by mercadolibre"
			results = append(results, result)
			continue
		}

		mappedStatus := foldMercadoLibreStatus(itemStatus.Status, itemStatus.SubStatus)
		result.Status = mappedStatus

		if mappedStatus == item.listing.Status {
			result.Success = true
			results = append(results, result)
			continue
		}

		if err := s.channelProductMapRepository.UpdateStatus(ctx, item.listing.ID, mappedStatus); err != nil {
			result.Error = fmt.Sprintf("error saving status: %v", err)
			results = append(results, result)
			continue
		}

		result.Changed = true
		result.Success = true
		results = append(results, result)
	}

	log.Printf("mercadolibre channel product map status refresh: connection %d — done, %d result(s)", connectionID, len(results))

	return &ChannelProductMapStatusResult{Results: results}, nil
}

// ChannelProductMapSyncResultItem reports the outcome of reconciling a
// single ecom_channel_product_map row against MercadoLibre's current status
// and category (see RefreshChannelProductMapStatusAndCategory).
type ChannelProductMapSyncResultItem struct {
	ProductID        int64  `json:"productId"`
	SKU              string `json:"sku,omitempty"`
	VehicleFitmentID *int64 `json:"vehicleFitmentId,omitempty"`
	ExternalID       string `json:"externalId"`
	// PreviousStatus/Status follow ecom_channel_product_map's own status
	// vocabulary, same as ChannelProductMapStatusResultItem.
	PreviousStatus string `json:"previousStatus,omitempty"`
	Status         string `json:"status,omitempty"`
	StatusChanged  bool   `json:"statusChanged,omitempty"`
	// PreviousExternalCategoryID/ExternalCategoryID are MercadoLibre's own
	// category ids (ecom_channel_product_map.external_category_id), not local
	// ecom_categories ids.
	PreviousExternalCategoryID string `json:"previousExternalCategoryId,omitempty"`
	ExternalCategoryID         string `json:"externalCategoryId,omitempty"`
	CategoryChanged            bool   `json:"categoryChanged,omitempty"`
	// LocalCategoryID is the ecom_categories leaf id syncCategoryMapping
	// resolved/created for ExternalCategoryID — only set when CategoryChanged.
	LocalCategoryID int64  `json:"localCategoryId,omitempty"`
	Success         bool   `json:"success"`
	Error           string `json:"error,omitempty"`
}

type ChannelProductMapSyncResult struct {
	Results []ChannelProductMapSyncResultItem `json:"results"`
}

// RefreshChannelProductMapStatusAndCategory is
// RefreshChannelProductMapStatus plus category drift reconciliation:
// MercadoLibre is known to silently reclassify a listing into a different
// category over time (its own auto-categorization, policy enforcement,
// etc.), which otherwise goes unnoticed here indefinitely since nothing else
// re-checks a listing's category after publish. For every
// ecom_channel_product_map row on connectionID with an external id, this
// fetches MercadoLibre's current status and category_id in the same
// GET /items?ids=... call RefreshChannelProductMapStatus uses, and:
//
//   - persists a changed status the same way (folded through
//     foldMercadoLibreStatus), and
//   - when the reported category_id no longer matches the row's own
//     external_category_id, runs the exact same category-provisioning
//     procedure a first-time listing creation uses (syncCategoryMapping,
//     which itself calls EnsureLocalCategory) to replicate whatever's missing
//     of MercadoLibre's category tree into ecom_categories and upsert
//     ecom_channel_category_map for it — reassigning the product's own
//     category only under the same condition syncCategoryMapping already
//     applies at publish time (product has no category yet at all).
//
// Either change is persisted back into the row via Upsert (title/external id
// passed through unchanged) so external_category_id keeps reflecting
// MercadoLibre's own state even on a run where the product's category
// itself wasn't reassigned. This is a separate, explicitly-invoked endpoint
// rather than folded into RefreshChannelProductMapStatus so it can be
// exercised on its own before anything schedules it automatically. Rows with
// no external_id yet (never successfully published) are skipped, same as
// RefreshChannelProductMapStatus.
func (s *MercadoLibreProductSyncService) RefreshChannelProductMapStatusAndCategory(ctx context.Context, connectionID int64) (*ChannelProductMapSyncResult, error) {
	if connectionID <= 0 {
		return nil, ErrInvalidMercadoLibreConnection
	}

	listings, err := s.channelProductMapRepository.FindAllByConnectionID(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel product map for connection %d: %w", connectionID, err)
	}

	type pendingListing struct {
		listing    mysqlInfra.ChannelProductMapDTO
		externalID string
		product    *mysqlInfra.ProductDTO
	}

	pending := make([]pendingListing, 0, len(listings))
	externalIDs := make([]string, 0, len(listings))
	seen := make(map[string]bool, len(listings))

	for _, listing := range listings {
		externalID := derefString(listing.ExternalID)
		if externalID == "" {
			continue
		}

		// product may end up nil (lookup failure) — still queued so its
		// result item can report the lookup error rather than being silently
		// dropped from the batch.
		product, _ := s.productRepository.FindByID(ctx, listing.ProductID)

		pending = append(pending, pendingListing{listing: listing, externalID: externalID, product: product})
		if !seen[externalID] {
			seen[externalID] = true
			externalIDs = append(externalIDs, externalID)
		}
	}

	results := make([]ChannelProductMapSyncResultItem, 0, len(pending))

	if len(externalIDs) == 0 {
		return &ChannelProductMapSyncResult{Results: results}, nil
	}

	log.Printf("mercadolibre channel product map status+category sync: connection %d — checking %d listing(s), %d distinct external id(s)", connectionID, len(pending), len(externalIDs))

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error getting mercadolibre access token for connection %d: %w", connectionID, err)
	}

	statuses, err := s.itemsHandler.GetItemsStatus(ctx, accessToken, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("error fetching mercadolibre item statuses: %w", err)
	}

	statusByExternalID := make(map[string]mercadoLibreInfra.ItemStatus, len(statuses))
	for _, itemStatus := range statuses {
		statusByExternalID[itemStatus.ID] = itemStatus
	}

	for _, item := range pending {
		sku := ""
		if item.product != nil {
			sku = item.product.SKU
		}

		result := ChannelProductMapSyncResultItem{
			ProductID:                  item.listing.ProductID,
			SKU:                        sku,
			VehicleFitmentID:           item.listing.VehicleFitmentID,
			ExternalID:                 item.externalID,
			PreviousStatus:             item.listing.Status,
			PreviousExternalCategoryID: derefString(item.listing.ExternalCategoryID),
		}

		itemStatus, ok := statusByExternalID[item.externalID]
		if !ok {
			result.Error = "status not returned by mercadolibre"
			results = append(results, result)
			continue
		}

		mappedStatus := foldMercadoLibreStatus(itemStatus.Status, itemStatus.SubStatus)
		result.Status = mappedStatus
		result.StatusChanged = mappedStatus != item.listing.Status

		remoteCategoryID := strings.TrimSpace(itemStatus.CategoryID)
		result.ExternalCategoryID = remoteCategoryID
		result.CategoryChanged = remoteCategoryID != "" && remoteCategoryID != result.PreviousExternalCategoryID

		if !result.StatusChanged && !result.CategoryChanged {
			result.Success = true
			results = append(results, result)
			continue
		}

		newExternalCategoryID := result.PreviousExternalCategoryID
		if result.CategoryChanged {
			if item.product == nil {
				result.Error = fmt.Sprintf("error loading product %d to sync category", item.listing.ProductID)
				results = append(results, result)
				continue
			}

			leafCategoryID, err := s.syncCategoryMapping(ctx, item.product, connectionID, remoteCategoryID)
			if err != nil {
				result.Error = err.Error()
				results = append(results, result)
				continue
			}
			result.LocalCategoryID = leafCategoryID
			newExternalCategoryID = remoteCategoryID
		}

		if _, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
			ProductID:          item.listing.ProductID,
			ConnectionID:       connectionID,
			VehicleFitmentID:   item.listing.VehicleFitmentID,
			ListingTitle:       derefString(item.listing.ListingTitle),
			ExternalID:         item.externalID,
			ExternalCategoryID: newExternalCategoryID,
			Status:             mappedStatus,
			ActorID:            systemMercadoLibreSyncActorID,
		}); err != nil {
			result.Error = fmt.Sprintf("error saving channel product map: %v", err)
			results = append(results, result)
			continue
		}

		result.Success = true
		results = append(results, result)
	}

	log.Printf("mercadolibre channel product map status+category sync: connection %d — done, %d result(s)", connectionID, len(results))

	return &ChannelProductMapSyncResult{Results: results}, nil
}

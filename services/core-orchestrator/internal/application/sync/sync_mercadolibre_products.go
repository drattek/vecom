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
	productRepository            *mysqlInfra.ProductRepository
	brandsRepository             *mysqlInfra.BrandsRepository
	productPricesRepository      *mysqlInfra.ProductPricesRepository
	productStockRepository       *mysqlInfra.ProductStockRepository
	productImagesRepository      *mysqlInfra.ProductImagesRepository
	productDimensionsRepository  *mysqlInfra.ProductDimensionsRepository
	filesRepository              *mysqlInfra.FilesRepository
	currenciesRepository         *mysqlInfra.CurrenciesRepository
	channelConnectionRepository  *mysqlInfra.ChannelConnectionRepository
	channelProductMapRepository  *mysqlInfra.ChannelProductMapRepository
	channelCategoryMapRepository *mysqlInfra.ChannelCategoryMapRepository
	tokenService                 *MercadoLibreTokenService
	categoryPredictorService     *MercadoLibreCategoryPredictorService
	itemsHandler                 *mercadoLibreInfra.ItemsHandler
	// channelAttributeValuesService/productAttributesRepository/
	// attributeOptionsRepository back resolveCustomAttributes: provisioning
	// the required-attribute slots for a listing's category and reading
	// whatever value the product already has for each one (see
	// resolveCustomAttributes's own doc comment).
	channelAttributeValuesService *channelAttributeValuesApp.Service
	productAttributesRepository   *mysqlInfra.ProductAttributesRepository
	attributeOptionsRepository    *mysqlInfra.AttributeOptionsRepository
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
	channelCategoryMapRepository *mysqlInfra.ChannelCategoryMapRepository,
	tokenService *MercadoLibreTokenService,
	categoryPredictorService *MercadoLibreCategoryPredictorService,
	rateLimiter *mercadoLibreInfra.RateLimiter,
	channelAttributeValuesService *channelAttributeValuesApp.Service,
	productAttributesRepository *mysqlInfra.ProductAttributesRepository,
	attributeOptionsRepository *mysqlInfra.AttributeOptionsRepository,
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
		channelCategoryMapRepository:  channelCategoryMapRepository,
		tokenService:                  tokenService,
		categoryPredictorService:      categoryPredictorService,
		itemsHandler:                  mercadoLibreInfra.NewItemsHandler(client),
		channelAttributeValuesService: channelAttributeValuesService,
		productAttributesRepository:   productAttributesRepository,
		attributeOptionsRepository:    attributeOptionsRepository,
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

	connection, err := s.channelConnectionRepository.FindByID(input.ConnectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return nil, ErrInvalidMercadoLibreConnection
		}
		return nil, fmt.Errorf("error loading connection %d: %w", input.ConnectionID, err)
	}

	log.Printf("mercadolibre upload: starting batch of %d product(s) for connection %d (allowsMultipleListings=%v)", len(input.Products), input.ConnectionID, connection.AllowsMultipleListings)

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
		product, err := s.productRepository.FindBySKU(sku)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrProductNotFound) {
				log.Printf("mercadolibre upload: %s product not found", step)
				result.Error = "product not found for sku"
				results = append(results, result)
				continue
			}
			return nil, fmt.Errorf("error looking up product by sku %q: %w", sku, err)
		}

		existingListings, err := s.channelProductMapRepository.FindAllByProductAndConnection(product.ID, input.ConnectionID)
		if err != nil {
			return nil, fmt.Errorf("error loading existing listings for product %d: %w", product.ID, err)
		}

		if !connection.AllowsMultipleListings && len(existingListings) > 0 {
			log.Printf("mercadolibre upload: %s product %d already has a listing on connection %d, which does not allow multiple listings", step, product.ID, input.ConnectionID)
			result.Error = "product already has a listing on this connection; this connection does not allow multiple listings per product"
			results = append(results, result)
			continue
		}
		if connection.AllowsMultipleListings && hasExistingFitmentListing(existingListings, item.VehicleFitmentID) {
			log.Printf("mercadolibre upload: %s product %d already has a listing for this vehicle fitment on connection %d", step, product.ID, input.ConnectionID)
			result.Error = "a listing already exists for this vehicle fitment on this connection"
			results = append(results, result)
			continue
		}

		log.Printf("mercadolibre upload: %s resolved product id=%d, creating item", step, product.ID)

		externalID, missingRequiredAttributes, missingOptionalAttributes, err := s.createItem(ctx, product, input.ConnectionID, name, item.VehicleFitmentID, product.ID, nil)
		if err != nil {
			log.Printf("mercadolibre upload: %s failed: %v", step, err)
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		if len(missingRequiredAttributes) > 0 {
			log.Printf("mercadolibre upload: %s created with missing required attribute(s): %v", step, missingRequiredAttributes)
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
// by MercadoLibre's own Tags.Required for that attribute — the item is still
// created regardless.
func (s *MercadoLibreProductSyncService) Publish(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, title string, vehicleFitmentID *int64, imageSourceProductID int64, officialStoreID *int64) (string, []string, []string, error) {
	return s.createItem(ctx, product, connectionID, title, vehicleFitmentID, imageSourceProductID, officialStoreID)
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
// listing. It fires only when product's effective price or total stock
// changed since listing.LastSyncedAt (see priceOrStockChangedSince, shared
// with Odoo's Refresh in sync_odoo_products.go). The PUT /items call sends
// only price, stock, and attributes — MercadoLibre's update endpoint requires
// price/stock together regardless of which one actually moved, and
// attributes ride along for free so a custom attribute value added to the
// product after the listing was first published (see resolveCustomAttributes)
// reaches MercadoLibre without a separate call. Category is deliberately
// never sent here: channel_listings.RefreshListings calls SyncListingStatus
// right before this, which already reconciles any category drift against
// MercadoLibre's live state (see RefreshChannelProductMapStatusAndCategory) —
// resending it here would just repeat that work with a possibly stale local
// value. Listings currently paused/under_review are left untouched, same as
// updateExistingListings.
func (s *MercadoLibreProductSyncService) Refresh(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO) (bool, error) {
	externalID := derefString(listing.ExternalID)
	if externalID == "" {
		return false, nil
	}
	if isPausedOrUnderReview(listing.Status) {
		return false, nil
	}

	mxnCurrency, err := s.currenciesRepository.FindByCode(mercadoLibreItemCurrencyID)
	if err != nil {
		return false, fmt.Errorf("error loading %s currency: %w", mercadoLibreItemCurrencyID, err)
	}

	price, err := s.productPricesRepository.FindEffectivePrice(product.ID, mxnCurrency.ID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductPriceNotFound) {
			return false, fmt.Errorf("no active %s price list entry for product %d", mercadoLibreItemCurrencyID, product.ID)
		}
		return false, fmt.Errorf("error loading effective price for product %d: %w", product.ID, err)
	}

	stocks, err := s.productStockRepository.FindByProductID(product.ID)
	if err != nil {
		return false, fmt.Errorf("error loading stock for product %d: %w", product.ID, err)
	}

	if !priceOrStockChangedSince(price.UpdatedAt, stocks, listing.LastSyncedAt) {
		return false, nil
	}

	basePrice, err := strconv.ParseFloat(price.Price, 64)
	if err != nil {
		return false, fmt.Errorf("error parsing price %q for product %d: %w", price.Price, product.ID, err)
	}
	finalPrice := math.Round((((basePrice*1.13)/0.85)+90)*1.16*100) / 100

	qtyAvailable := 0
	for _, stock := range stocks {
		qtyAvailable += stock.AvailableQty
	}

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return false, fmt.Errorf("error getting mercadolibre access token for connection %d: %w", connectionID, err)
	}

	attributes := s.resolvePackageAttributes(product.ID)
	externalCategoryID := derefString(listing.ExternalCategoryID)
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
		return false, fmt.Errorf("error updating mercadolibre item %s for product %d: %w", externalID, product.ID, err)
	}

	if _, err := s.channelProductMapRepository.Upsert(mysqlInfra.UpsertChannelProductMapInput{
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
func (s *MercadoLibreProductSyncService) createItem(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listingName string, vehicleFitmentID *int64, imageSourceProductID int64, officialStoreID *int64) (string, []string, []string, error) {
	finalPrice, qtyAvailable, accessToken, err := s.resolvePriceStockAndToken(ctx, product.ID, connectionID)
	if err != nil {
		return "", nil, nil, err
	}

	return s.createNewItem(ctx, product, connectionID, listingName, vehicleFitmentID, accessToken, finalPrice, qtyAvailable, imageSourceProductID, officialStoreID)
}

// resolvePriceStockAndToken computes the current MXN price (via the
// configured formula) and total stock for a product, and ensures a valid
// MercadoLibre access token for connectionID — the inputs both createItem
// and UpdatePricesAndStock need before talking to MercadoLibre.
func (s *MercadoLibreProductSyncService) resolvePriceStockAndToken(ctx context.Context, productID, connectionID int64) (finalPrice float64, qtyAvailable int, accessToken string, err error) {
	log.Printf("mercadolibre: product %d — resolving %s currency", productID, mercadoLibreItemCurrencyID)
	mxnCurrency, err := s.currenciesRepository.FindByCode(mercadoLibreItemCurrencyID)
	if err != nil {
		return 0, 0, "", fmt.Errorf("error loading %s currency: %w", mercadoLibreItemCurrencyID, err)
	}

	log.Printf("mercadolibre: product %d — resolving effective price", productID)
	price, err := s.productPricesRepository.FindEffectivePrice(productID, mxnCurrency.ID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductPriceNotFound) {
			return 0, 0, "", fmt.Errorf("no active %s price list entry for product %d", mercadoLibreItemCurrencyID, productID)
		}
		return 0, 0, "", fmt.Errorf("error loading effective price for product %d: %w", productID, err)
	}
	basePrice, err := strconv.ParseFloat(price.Price, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("error parsing price %q for product %d: %w", price.Price, productID, err)
	}
	finalPrice = (((basePrice * 1.13) / 0.85) + 90) * 1.16
	// MXN accepts at most 2 decimals; the raw formula result carries far
	// more precision than that (e.g. 112.80762541176469), which MercadoLibre
	// rejects with item.price.invalid.
	finalPrice = math.Round(finalPrice*100) / 100
	log.Printf("mercadolibre: product %d — base price=%s, final price=%.2f", productID, price.Price, finalPrice)

	log.Printf("mercadolibre: product %d — summing stock", productID)
	qtyAvailable, err = s.sumAvailableStock(productID)
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
				Attributes:        s.resolvePackageAttributes(product.ID),
			},
		}); err != nil {
			log.Printf("mercadolibre update: product %d — error updating listing %s: %v", product.ID, externalID, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("error updating mercadolibre item %s for product %d: %w", externalID, product.ID, err)
			}
			continue
		}

		if _, err := s.channelProductMapRepository.Upsert(mysqlInfra.UpsertChannelProductMapInput{
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
func (s *MercadoLibreProductSyncService) resolvePackageAttributes(productID int64) []mercadoLibreInfra.ItemAttribute {
	fallback := []mercadoLibreInfra.ItemAttribute{
		{ID: "SELLER_PACKAGE_HEIGHT", ValueName: mercadoLibreItemPackageSide},
		{ID: "SELLER_PACKAGE_LENGTH", ValueName: mercadoLibreItemPackageSide},
		{ID: "SELLER_PACKAGE_WEIGHT", ValueName: mercadoLibreItemPackageWeight},
		{ID: "SELLER_PACKAGE_WIDTH", ValueName: mercadoLibreItemPackageSide},
	}

	dimensions, err := s.productDimensionsRepository.FindByProductID(productID)
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
// as MercadoLibre's "<value> cm" package attribute format, trimming trailing
// zeros. Falls back to the generic package side default when raw fails to
// parse.
func formatPackageCM(raw string) string {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return mercadoLibreItemPackageSide
	}
	return strconv.FormatFloat(value, 'f', -1, 64) + " cm"
}

// formatPackageGrams converts an ecom_product_dimensions weight value (kg)
// into MercadoLibre's "<value> g" package attribute format. Falls back to
// the generic package weight default when raw fails to parse.
func formatPackageGrams(raw string) string {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return mercadoLibreItemPackageWeight
	}
	return strconv.FormatFloat(value*1000, 'f', -1, 64) + " g"
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
) (string, []string, []string, error) {
	log.Printf("mercadolibre upload: product %d — resolving brand", product.ID)
	if product.BrandID == nil {
		return "", nil, nil, fmt.Errorf("product %d has no brand", product.ID)
	}

	brand, err := s.brandsRepository.FindByID(*product.BrandID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrBrandNotFound) {
			return "", nil, nil, fmt.Errorf("brand %d not found for product %d", *product.BrandID, product.ID)
		}
		return "", nil, nil, fmt.Errorf("error loading brand for product %d: %w", product.ID, err)
	}
	log.Printf("mercadolibre upload: product %d — brand resolved: %s", product.ID, brand.Name)

	log.Printf("mercadolibre upload: product %d — loading images (source product %d)", product.ID, imageSourceProductID)
	images, err := s.productImagesRepository.FindAllByProductID(imageSourceProductID)
	if err != nil {
		return "", nil, nil, fmt.Errorf("error loading images for product %d: %w", imageSourceProductID, err)
	}
	if len(images) == 0 {
		return "", nil, nil, fmt.Errorf("product %d has no images (image source product %d)", product.ID, imageSourceProductID)
	}
	log.Printf("mercadolibre upload: product %d — %d image(s) found, resolving file urls", product.ID, len(images))

	pictures := make([]mercadoLibreInfra.ItemPicture, 0, len(images))
	for _, image := range images {
		file, err := s.filesRepository.FindByID(image.FileID)
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
	attributes = append(attributes, s.resolvePackageAttributes(product.ID)...)

	customAttributes, missingRequiredAttributes, missingOptionalAttributes, err := s.resolveCustomAttributes(ctx, product, connectionID, externalCategoryID)
	if err != nil {
		return "", nil, nil, err
	}
	attributes = append(attributes, customAttributes...)

	vals := mercadoLibreInfra.CreateItemVals{
		Price:             finalPrice,
		AvailableQuantity: qtyAvailable,
		CurrencyID:        mercadoLibreItemCurrencyID,
		Condition:         mercadoLibreItemCondition,
		BuyingMode:        mercadoLibreItemBuyingMode,
		ListingTypeID:     mercadoLibreItemListingTypeID,
		CategoryID:        externalCategoryID,
		OfficialStoreID:   officialStoreID,
		FamilyName:        query,
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

	if _, err := s.channelProductMapRepository.Upsert(mysqlInfra.UpsertChannelProductMapInput{
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
// custom_attribute slot with no ecom_product_attributes value yet is left out
// of the payload and reported back in missingRequiredAttributes or
// missingOptionalAttributes instead (split by outcome.IsRequired, i.e.
// MercadoLibre's own Tags.Required for that attribute) — MercadoLibre itself
// decides whether to reject the item over a missing required one; this never
// blocks publishing.
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
		if outcome.Skipped || outcome.SourceType != "custom_attribute" || outcome.AttributeID == nil {
			continue
		}

		value, err := s.productAttributesRepository.FindByProductAndAttribute(product.ID, *outcome.AttributeID)
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

		formatted, isValueID, ok := s.formatProductAttributeValue(value)
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
func (s *MercadoLibreProductSyncService) formatProductAttributeValue(value *mysqlInfra.ProductAttributeDTO) (formatted string, isValueID bool, ok bool) {
	switch {
	case value.ValueText != nil:
		text := strings.TrimSpace(*value.ValueText)
		return text, false, text != ""
	case value.ValueNumber != nil:
		return strconv.FormatFloat(*value.ValueNumber, 'f', -1, 64), false, true
	case value.ValueBool != nil:
		return strconv.FormatBool(*value.ValueBool), false, true
	case value.ValueDate != nil:
		return value.ValueDate.Format(time.DateOnly), false, true
	case value.OptionID != nil:
		option, err := s.attributeOptionsRepository.FindByID(*value.OptionID)
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

// resolveExternalCategoryID returns the MercadoLibre category id to list
// product under. If product already has a local category that's mapped to
// connectionID in ecom_channel_category_map, that mapping's external id is
// reused directly and the category predictor is never called; otherwise
// query is sent to the predictor and its best match is used.
func (s *MercadoLibreProductSyncService) resolveExternalCategoryID(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, query string) (string, error) {
	if product.CategoryID != nil {
		existing, err := s.channelCategoryMapRepository.FindByCategoryAndConnection(*product.CategoryID, connectionID)
		if err != nil && !errors.Is(err, mysqlInfra.ErrChannelCategoryMapNotFound) {
			return "", fmt.Errorf("error loading channel category map for product %d: %w", product.ID, err)
		}
		if existing != nil {
			log.Printf("mercadolibre upload: product %d — category %d already mapped to %s for connection %d, skipping category predictor", product.ID, *product.CategoryID, existing.ExternalCategoryID, connectionID)
			return existing.ExternalCategoryID, nil
		}
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
// product when product has no category yet, or replaces it when product's
// current category has no mapping to this connection (i.e. isn't the
// category MercadoLibre just listed the item under). A product whose current
// category is already mapped to this connection is left untouched.
// The returned int64 is EnsureLocalCategory's leaf ecom_categories.id for
// externalCategoryID regardless of which branch returns it — including the
// early "already mapped" returns — so callers that only care about the local
// category id (see RefreshChannelProductMapStatusAndCategory) don't need to
// re-derive it themselves.
func (s *MercadoLibreProductSyncService) syncCategoryMapping(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, externalCategoryID string) (int64, error) {
	if strings.TrimSpace(externalCategoryID) == "" {
		return 0, nil
	}

	leafCategoryID, err := s.categoryPredictorService.EnsureLocalCategory(ctx, connectionID, externalCategoryID, systemMercadoLibreSyncActorID)
	if err != nil {
		return 0, fmt.Errorf("error ensuring local category for external category %q: %w", externalCategoryID, err)
	}

	if product.CategoryID != nil {
		existing, err := s.channelCategoryMapRepository.FindByCategoryAndConnection(*product.CategoryID, connectionID)
		if err != nil && !errors.Is(err, mysqlInfra.ErrChannelCategoryMapNotFound) {
			return leafCategoryID, fmt.Errorf("error loading channel category map for product %d: %w", product.ID, err)
		}
		if existing != nil {
			return leafCategoryID, nil
		}
	}

	if err := s.productRepository.UpdateCategoryID(product.ID, leafCategoryID, systemMercadoLibreSyncActorID); err != nil {
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

		product, err := s.productRepository.FindBySKU(sku)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrProductNotFound) {
				log.Printf("mercadolibre update: %s product not found", step)
				result.Error = "product not found for sku"
				results = append(results, result)
				continue
			}
			return nil, fmt.Errorf("error looking up product by sku %q: %w", sku, err)
		}

		existingListings, err := s.channelProductMapRepository.FindAllByProductAndConnection(product.ID, input.ConnectionID)
		if err != nil {
			return nil, fmt.Errorf("error loading channel product map for product %d: %w", product.ID, err)
		}
		if len(existingListings) == 0 {
			log.Printf("mercadolibre update: %s product %d has no existing listing on connection %d", step, product.ID, input.ConnectionID)
			result.Error = "product has no existing mercadolibre listing on this connection"
			results = append(results, result)
			continue
		}

		finalPrice, qtyAvailable, accessToken, err := s.resolvePriceStockAndToken(ctx, product.ID, input.ConnectionID)
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

func (s *MercadoLibreProductSyncService) sumAvailableStock(productID int64) (int, error) {
	stocks, err := s.productStockRepository.FindByProductID(productID)
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

	listings, err := s.channelProductMapRepository.FindAllByConnectionID(connectionID)
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
		if product, err := s.productRepository.FindByID(listing.ProductID); err == nil {
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

		if err := s.channelProductMapRepository.UpdateStatus(item.listing.ID, mappedStatus); err != nil {
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
//     applies at publish time (no category yet, or its current one isn't
//     mapped to this connection).
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

	listings, err := s.channelProductMapRepository.FindAllByConnectionID(connectionID)
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
		product, _ := s.productRepository.FindByID(listing.ProductID)

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

		if _, err := s.channelProductMapRepository.Upsert(mysqlInfra.UpsertChannelProductMapInput{
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

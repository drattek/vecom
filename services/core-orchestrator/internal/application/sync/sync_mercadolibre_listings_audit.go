package sync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

const (
	// mercadoLibreFlaggedListingPrice/mercadoLibreFlaggedListingStock are the
	// sentinel price/stock this audit treats as "flagged": only listings
	// matching both, exactly, are processed at all — everything else is
	// skipped without being counted or touched.
	mercadoLibreFlaggedListingPrice = 999999
	mercadoLibreFlaggedListingStock = 0

	// Defaults stamped on an ecom_products row auto-created for a flagged
	// listing whose SKU has no local product yet. Only these four columns are
	// set from the listing (sku, part_number, name, brand_id) plus source_id;
	// product_type/is_sellable/is_stockable/status are left to the table
	// defaults ('part', 1, 1, NULL). brand_id 1 and source_id 13 are fixed by
	// product decision, not configurable.
	auditProductBrandID  int64 = 1
	auditProductSourceID int64 = 13
	auditProductType           = "part"
)

// MercadoLibreListingsAuditService scans every listing in a MercadoLibre
// account (via connectionID's own credentials) for ones matching the flagged
// price/stock sentinel (mercadoLibreFlaggedListingPrice/
// mercadoLibreFlaggedListingStock) and, for each match:
//
//  1. resolves the seller's SKU straight from the listing (resolveSKUFromItem)
//     and finds the matching ecom_products row — creating it when absent, with
//     sku = part_number = the listing SKU, name = the listing title,
//     brand_id = auditProductBrandID, source_id = auditProductSourceID;
//  2. updates ecom_products.description from MercadoLibre when a description
//     can be fetched (see fetchListingDescription) — and leaves it untouched
//     when it can't, so a description that couldn't be read is never
//     overwritten with nothing;
//  3. downloads the listing's vehicle compatibilities and links each one to
//     that product locally — exactly what
//     POST /api/marketplaces/mercadolibre/compatibilities/copy does, reusing
//     MercadoLibreCompatibilityService.CopyCompatibilitiesForProduct (so the
//     same position/side split from the downloaded restrictions applies).
//
// Nothing is ever written back to MercadoLibre — this only reads listings and
// writes locally.
type MercadoLibreListingsAuditService struct {
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository
	channelRepository           *mysqlInfra.ChannelRepository
	productRepository           *mysqlInfra.ProductRepository
	tokenService                *MercadoLibreTokenService
	compatibilityService        *MercadoLibreCompatibilityService
	itemsHandler                *mercadoLibreInfra.ItemsHandler
	usersHandler                *mercadoLibreInfra.UsersHandler
}

func NewMercadoLibreListingsAuditService(
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	productRepository *mysqlInfra.ProductRepository,
	tokenService *MercadoLibreTokenService,
	compatibilityService *MercadoLibreCompatibilityService,
	rateLimiter *mercadoLibreInfra.RateLimiter,
) *MercadoLibreListingsAuditService {
	client := mercadoLibreInfra.NewClient(nil, "", rateLimiter)

	return &MercadoLibreListingsAuditService{
		channelConnectionRepository: channelConnectionRepository,
		channelRepository:           channelRepository,
		productRepository:           productRepository,
		tokenService:                tokenService,
		compatibilityService:        compatibilityService,
		itemsHandler:                mercadoLibreInfra.NewItemsHandler(client),
		usersHandler:                mercadoLibreInfra.NewUsersHandler(client),
	}
}

type SyncFlaggedListingsInput struct {
	ConnectionID int64
}

// ListingAuditOutcome reports what happened for one listing that matched the
// flagged price/stock filter. A batch never fails wholesale on one bad
// listing.
type ListingAuditOutcome struct {
	ExternalID string `json:"externalId"`
	SKU        string `json:"sku"`
	ProductID  *int64 `json:"productId,omitempty"`
	// ProductCreated is true when this run inserted the ecom_products row
	// (SKU had no local product yet).
	ProductCreated bool `json:"productCreated,omitempty"`
	// DescriptionUpdated is true when ecom_products.description was written
	// this run; DescriptionSource is "item" or "catalog" in that case, empty
	// when no description could be fetched (and the column was left as-is).
	DescriptionUpdated bool   `json:"descriptionUpdated,omitempty"`
	DescriptionSource  string `json:"descriptionSource,omitempty"`

	// CompatibilitiesLinked counts the ecom_product_vehicle_compatibility rows
	// created for this listing's product; Compatibilities carries the full
	// per-entry detail (skips, errors, catalog product ids) and
	// NewVehicleFitments the ecom_vehicle_fitments rows created along the way.
	CompatibilitiesLinked int                             `json:"compatibilitiesLinked,omitempty"`
	Compatibilities       []CopyLocalCompatibilityOutcome `json:"compatibilities,omitempty"`
	NewVehicleFitments    []CreatedVehicleFitment         `json:"newVehicleFitments,omitempty"`

	SkipReason string `json:"skipReason,omitempty"`
	Error      string `json:"error,omitempty"`
}

type SyncFlaggedListingsResult struct {
	TotalListings         int                   `json:"totalListings"`
	TotalMatched          int                   `json:"totalMatched"`
	ProductsCreated       int                   `json:"productsCreated"`
	CompatibilitiesLinked int                   `json:"compatibilitiesLinked"`
	Results               []ListingAuditOutcome `json:"results"`
}

// SyncFlaggedListings is the account-wide entry point — see the service doc
// comment for what it does per matching listing. input only needs
// ConnectionID: every listing in the account is discovered by paging through
// MercadoLibre's own /users/{id}/items/search, not by anything already in the
// local database.
func (s *MercadoLibreListingsAuditService) SyncFlaggedListings(ctx context.Context, input SyncFlaggedListingsInput) (*SyncFlaggedListingsResult, error) {
	if input.ConnectionID <= 0 {
		return nil, ErrInvalidMercadoLibreConnection
	}

	connection, err := s.channelConnectionRepository.FindByID(ctx, input.ConnectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return nil, ErrInvalidMercadoLibreConnection
		}
		return nil, fmt.Errorf("error loading connection %d: %w", input.ConnectionID, err)
	}

	channel, err := s.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}
	if !strings.EqualFold(strings.TrimSpace(channel.Code), "MERCADOLIBRE") {
		return nil, ErrNotMercadoLibreConnection
	}

	// A full account scan can run long enough (many multiget batches, each
	// throttled by the shared rate limiter) that a token fetched once up front
	// could go stale before the run finishes. So no accessToken is captured
	// here: every call below re-validates it immediately beforehand via
	// s.freshAccessToken, which is cheap (a local DB read) whenever the token
	// isn't actually near expiry.
	accessToken, err := s.freshAccessToken(ctx, input.ConnectionID)
	if err != nil {
		return nil, err
	}
	me, err := s.usersHandler.GetMe(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("error resolving mercadolibre account id: %w", err)
	}

	externalIDs, err := s.listAllSellerItemIDs(ctx, input.ConnectionID, me.ID)
	if err != nil {
		return nil, fmt.Errorf("error listing mercadolibre seller items: %w", err)
	}
	log.Printf("mercadolibre listings audit: connection %d — seller %d has %d listing(s) to scan", input.ConnectionID, me.ID, len(externalIDs))

	summaries, err := s.fetchAllItemSummaries(ctx, input.ConnectionID, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("error fetching mercadolibre item summaries: %w", err)
	}

	matchedIDs := make([]string, 0)
	for _, summary := range summaries {
		if summary.Price == mercadoLibreFlaggedListingPrice && summary.AvailableQuantity == mercadoLibreFlaggedListingStock {
			matchedIDs = append(matchedIDs, summary.ID)
		}
	}
	log.Printf("mercadolibre listings audit: connection %d — %d of %d listing(s) match price=%d/stock=%d", input.ConnectionID, len(matchedIDs), len(summaries), mercadoLibreFlaggedListingPrice, mercadoLibreFlaggedListingStock)

	result := &SyncFlaggedListingsResult{TotalListings: len(summaries), TotalMatched: len(matchedIDs), Results: []ListingAuditOutcome{}}
	if len(matchedIDs) == 0 {
		return result, nil
	}

	details, err := s.fetchAllItemDetails(ctx, input.ConnectionID, matchedIDs)
	if err != nil {
		return nil, fmt.Errorf("error fetching mercadolibre item details: %w", err)
	}

	for i := range details {
		outcome := s.syncOne(ctx, input.ConnectionID, &details[i])
		if outcome.ProductCreated {
			result.ProductsCreated++
		}
		result.CompatibilitiesLinked += outcome.CompatibilitiesLinked
		result.Results = append(result.Results, outcome)
	}

	return result, nil
}

// freshAccessToken re-validates connectionID's token immediately before an
// outbound MercadoLibre call — see the comment in SyncFlaggedListings for why
// this is called at every call site here instead of once for the whole run.
func (s *MercadoLibreListingsAuditService) freshAccessToken(ctx context.Context, connectionID int64) (string, error) {
	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return "", fmt.Errorf("error getting mercadolibre access token: %w", err)
	}
	return accessToken, nil
}

// listAllSellerItemIDs pages through
// GET /users/{sellerID}/items/search?search_type=scan (MercadoLibre's
// cursor-based "scan" mode — no 1000-result cap, unlike classic offset+limit
// pagination) until a page comes back with no results, re-validating the
// access token before every page.
func (s *MercadoLibreListingsAuditService) listAllSellerItemIDs(ctx context.Context, connectionID, sellerID int64) ([]string, error) {
	all := make([]string, 0)
	scrollID := ""

	for {
		accessToken, err := s.freshAccessToken(ctx, connectionID)
		if err != nil {
			return nil, err
		}

		page, err := s.itemsHandler.ScanSellerItems(ctx, accessToken, sellerID, scrollID)
		if err != nil {
			return nil, err
		}
		if len(page.Results) == 0 {
			break
		}

		all = append(all, page.Results...)
		scrollID = page.ScrollID
		if scrollID == "" {
			break
		}
	}

	return all, nil
}

// fetchAllItemSummaries chunks externalIDs into batches of
// mercadoLibreInfra.MaxItemsBatchSize and calls GetItemsSummary once per
// batch, re-validating the access token before each one.
func (s *MercadoLibreListingsAuditService) fetchAllItemSummaries(ctx context.Context, connectionID int64, externalIDs []string) ([]mercadoLibreInfra.ItemSummary, error) {
	summaries := make([]mercadoLibreInfra.ItemSummary, 0, len(externalIDs))

	for start := 0; start < len(externalIDs); start += mercadoLibreInfra.MaxItemsBatchSize {
		end := start + mercadoLibreInfra.MaxItemsBatchSize
		if end > len(externalIDs) {
			end = len(externalIDs)
		}

		accessToken, err := s.freshAccessToken(ctx, connectionID)
		if err != nil {
			return nil, err
		}

		batch, err := s.itemsHandler.GetItemsSummary(ctx, accessToken, externalIDs[start:end])
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, batch...)
	}

	return summaries, nil
}

// fetchAllItemDetails chunks externalIDs into batches of
// mercadoLibreInfra.MaxItemsBatchSize and calls GetItemsDetail once per batch,
// re-validating the access token before each one — mirrors
// fetchAllItemSummaries.
func (s *MercadoLibreListingsAuditService) fetchAllItemDetails(ctx context.Context, connectionID int64, externalIDs []string) ([]mercadoLibreInfra.ItemDetail, error) {
	details := make([]mercadoLibreInfra.ItemDetail, 0, len(externalIDs))

	for start := 0; start < len(externalIDs); start += mercadoLibreInfra.MaxItemsBatchSize {
		end := start + mercadoLibreInfra.MaxItemsBatchSize
		if end > len(externalIDs) {
			end = len(externalIDs)
		}

		accessToken, err := s.freshAccessToken(ctx, connectionID)
		if err != nil {
			return nil, err
		}

		batch, err := s.itemsHandler.GetItemsDetail(ctx, accessToken, externalIDs[start:end])
		if err != nil {
			return nil, err
		}
		details = append(details, batch...)
	}

	return details, nil
}

// syncOne processes a single listing already confirmed to match the flagged
// price/stock filter — see the service doc comment for the two steps.
func (s *MercadoLibreListingsAuditService) syncOne(ctx context.Context, connectionID int64, item *mercadoLibreInfra.ItemDetail) ListingAuditOutcome {
	sku := resolveSKUFromItem(item)
	outcome := ListingAuditOutcome{ExternalID: item.ID, SKU: sku}

	// resolveSKUFromItem falls back to the item id when the listing carries no
	// SELLER_SKU/seller_custom_field — creating an ecom_products row keyed by a
	// MercadoLibre id (and copying compatibilities onto it) is not what's
	// wanted, so skip it and report why.
	if sku == item.ID {
		outcome.SkipReason = "listing has no SELLER_SKU / seller_custom_field on MercadoLibre"
		return outcome
	}

	accessToken, err := s.freshAccessToken(ctx, connectionID)
	if err != nil {
		outcome.Error = err.Error()
		return outcome
	}

	product, created, err := s.resolveOrCreateProduct(ctx, item, sku)
	if err != nil {
		outcome.Error = err.Error()
		return outcome
	}
	outcome.ProductID = &product.ID
	outcome.ProductCreated = created

	// Description: update it whenever one can be fetched, skip silently when it
	// can't — never overwrite an existing description with nothing.
	if text, source := s.fetchListingDescription(ctx, accessToken, item); text != nil {
		outcome.DescriptionSource = source
		if product.Description == nil || strings.TrimSpace(*product.Description) != *text {
			if err := s.productRepository.UpdateDescription(ctx, product.ID, *text, systemMercadoLibreSyncActorID); err != nil {
				outcome.Error = fmt.Sprintf("error updating product description: %v", err)
				return outcome
			}
			outcome.DescriptionUpdated = true
		}
	}

	results, newFitments, err := s.compatibilityService.CopyCompatibilitiesForProduct(ctx, accessToken, item.ID, product.ID, systemMercadoLibreSyncActorID)
	if err != nil {
		outcome.Error = err.Error()
		return outcome
	}

	linked := 0
	for _, r := range results {
		if r.CompatibilityID != 0 {
			linked++
		}
	}
	outcome.Compatibilities = results
	outcome.CompatibilitiesLinked = linked
	outcome.NewVehicleFitments = newFitments

	return outcome
}

// resolveOrCreateProduct returns the ecom_products row for sku, creating it
// from the listing (description left NULL — syncOne fills it right after)
// when it doesn't exist yet. created reports whether this call inserted it.
func (s *MercadoLibreListingsAuditService) resolveOrCreateProduct(ctx context.Context, item *mercadoLibreInfra.ItemDetail, sku string) (product *mysqlInfra.ProductDTO, created bool, err error) {
	existing, findErr := s.productRepository.FindBySKU(ctx, sku)
	if findErr == nil {
		return existing, false, nil
	}
	if !errors.Is(findErr, mysqlInfra.ErrProductNotFound) {
		return nil, false, fmt.Errorf("error loading product by sku %s: %w", sku, findErr)
	}

	brandID := auditProductBrandID
	newProduct, createErr := s.productRepository.Create(ctx, mysqlInfra.CreateProductInput{
		SKU:         sku,
		PartNumber:  sku,
		Name:        strings.TrimSpace(item.Title),
		BrandID:     &brandID,
		ProductType: auditProductType,
		IsSellable:  true,
		IsStockable: true,
		SourceID:    auditProductSourceID,
		CreatedBy:   systemMercadoLibreSyncActorID,
	})
	if createErr != nil {
		// Another flagged listing for the same seller SKU may have created the
		// row already (unique key on source_id + part_number) — re-resolve
		// rather than failing this listing.
		if recovered, recErr := s.productRepository.FindBySourceAndPartNumber(ctx, auditProductSourceID, sku); recErr == nil {
			return recovered, false, nil
		}
		if recovered, recErr := s.productRepository.FindBySKU(ctx, sku); recErr == nil {
			return recovered, false, nil
		}
		return nil, false, fmt.Errorf("error creating product for sku %s: %w", sku, createErr)
	}

	return newProduct, true, nil
}

// fetchListingDescription returns the listing's description as plain text,
// best-effort: GET /items/{id}/description first, then — since that 404s for
// catalog/user-product-linked listings (see
// mercadolibre.ErrItemDescriptionNotFound), which is exactly what the flagged
// listings tend to be — the catalog product's short_description. Returns
// (nil, "") when neither is available; never an error (a missing description
// must not block the compatibility copy).
func (s *MercadoLibreListingsAuditService) fetchListingDescription(ctx context.Context, accessToken string, item *mercadoLibreInfra.ItemDetail) (*string, string) {
	if desc, err := s.itemsHandler.GetItemDescription(ctx, accessToken, item.ID); err == nil {
		if text := pickDescriptionText(desc); text != "" {
			return &text, "item"
		}
	} else if !errors.Is(err, mercadoLibreInfra.ErrItemDescriptionNotFound) {
		log.Printf("mercadolibre listings audit: item %s description fetch failed: %v", item.ID, err)
	}

	catalogProductID := strings.TrimSpace(item.CatalogProductID)
	if catalogProductID == "" {
		return nil, ""
	}

	catalog, err := s.itemsHandler.GetCatalogProduct(ctx, accessToken, catalogProductID)
	if err != nil {
		log.Printf("mercadolibre listings audit: item %s catalog product %s fetch failed: %v", item.ID, catalogProductID, err)
		return nil, ""
	}
	if text := strings.TrimSpace(catalog.ShortDescription.Content); text != "" {
		return &text, "catalog"
	}
	return nil, ""
}

func pickDescriptionText(desc *mercadoLibreInfra.ItemDescription) string {
	if desc == nil {
		return ""
	}
	if text := strings.TrimSpace(desc.PlainText); text != "" {
		return text
	}
	return strings.TrimSpace(desc.Text)
}

// resolveSKUFromItem extracts the seller's own SKU straight from
// MercadoLibre's item payload — the SELLER_SKU attribute (current convention,
// same key channel_attribute_values.systemFieldByExternalKey maps to "sku") or
// the older top-level seller_custom_field. Falls back to the MercadoLibre item
// id itself when neither is set, so a listing with no SKU recorded on
// MercadoLibre's side still gets a usable, unique lookup key.
func resolveSKUFromItem(item *mercadoLibreInfra.ItemDetail) string {
	for _, attr := range item.Attributes {
		if attr.ID == "SELLER_SKU" {
			if sku := strings.TrimSpace(attr.ValueName); sku != "" {
				return sku
			}
		}
	}
	if sku := strings.TrimSpace(item.SellerCustomField); sku != "" {
		return sku
	}
	return item.ID
}

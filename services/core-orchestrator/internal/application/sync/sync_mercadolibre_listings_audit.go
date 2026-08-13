package sync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	channelAttributeValuesApp "core-orchestrator/internal/application/channel_attribute_values"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// mercadoLibreFlaggedListingPrice/mercadoLibreFlaggedListingStock are the
// sentinel price/stock this audit treats as "flagged for review": only
// listings matching both, exactly, are processed at all — everything else is
// skipped without being counted or touched.
const (
	mercadoLibreFlaggedListingPrice = 999999
	mercadoLibreFlaggedListingStock = 0
)

// MercadoLibreListingsAuditService scans every listing in a MercadoLibre
// account (via connectionID's own credentials) for ones matching the
// flagged price/stock sentinel (mercadoLibreFlaggedListingPrice/
// mercadoLibreFlaggedListingStock) and, for each match where its SKU
// (resolved straight from MercadoLibre's own item data — see
// resolveSKUFromItem, since these listings are not assumed to have a local
// ecom_channel_product_map row) matches an existing ecom_products row:
//  1. provisions the local category/attribute chain for the listing's
//     MercadoLibre category (reusing channel_attribute_values.Service.
//     ProvisionCategoryAttributes exactly as the regular publish flow does)
//     and writes every attribute value the listing itself carries into
//     ecom_product_attributes;
//  2. replaces ecom_products.name with the listing's own title, stripped of
//     its trailing " {sku}" suffix (MercadoLibre titles here are always
//     "{name} {sku}" — see stripSKUFromTitle).
//
// A listing whose SKU matches no local product is left entirely untouched —
// see ListingAuditOutcome.ProductFound. Image downloading used to be a third
// step here; it's handled well enough by other means now and was removed to
// keep an account-wide run fast (no image I/O, no per-listing disk writes).
type MercadoLibreListingsAuditService struct {
	channelConnectionRepository   *mysqlInfra.ChannelConnectionRepository
	channelRepository             *mysqlInfra.ChannelRepository
	productRepository             *mysqlInfra.ProductRepository
	tokenService                  *MercadoLibreTokenService
	channelAttributeValuesService *channelAttributeValuesApp.Service
	itemsHandler                  *mercadoLibreInfra.ItemsHandler
	usersHandler                  *mercadoLibreInfra.UsersHandler
}

func NewMercadoLibreListingsAuditService(
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	productRepository *mysqlInfra.ProductRepository,
	tokenService *MercadoLibreTokenService,
	channelAttributeValuesService *channelAttributeValuesApp.Service,
	rateLimiter *mercadoLibreInfra.RateLimiter,
) *MercadoLibreListingsAuditService {
	client := mercadoLibreInfra.NewClient(nil, "", rateLimiter)

	return &MercadoLibreListingsAuditService{
		channelConnectionRepository:   channelConnectionRepository,
		channelRepository:             channelRepository,
		productRepository:             productRepository,
		tokenService:                  tokenService,
		channelAttributeValuesService: channelAttributeValuesService,
		itemsHandler:                  mercadoLibreInfra.NewItemsHandler(client),
		usersHandler:                  mercadoLibreInfra.NewUsersHandler(client),
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
	// ProductFound is false when SKU matched no ecom_products row — in that
	// case the listing is otherwise left untouched.
	ProductFound  bool   `json:"productFound"`
	ProductID     *int64 `json:"productId,omitempty"`
	AttributesSet int    `json:"attributesSet,omitempty"`
	// AttributeErrors lists individual attribute values that failed to save
	// (e.g. an unparseable number, or a value not in a value_id-mode
	// attribute's provisioned option list) — never fatal to the listing.
	AttributeErrors []string `json:"attributeErrors,omitempty"`
	NameUpdated     bool     `json:"nameUpdated,omitempty"`
	NewName         string   `json:"newName,omitempty"`
	SkipReason      string   `json:"skipReason,omitempty"`
	Error           string   `json:"error,omitempty"`
}

type SyncFlaggedListingsResult struct {
	TotalListings int                   `json:"totalListings"`
	TotalMatched  int                   `json:"totalMatched"`
	Results       []ListingAuditOutcome `json:"results"`
}

// SyncFlaggedListings is the account-wide entry point — see the service doc
// comment for what it does per matching listing. input only needs
// ConnectionID: every other listing in the account is discovered by paging
// through MercadoLibre's own /users/{id}/items/search, not by anything
// already in the local database.
func (s *MercadoLibreListingsAuditService) SyncFlaggedListings(ctx context.Context, input SyncFlaggedListingsInput) (*SyncFlaggedListingsResult, error) {
	if input.ConnectionID <= 0 {
		return nil, ErrInvalidMercadoLibreConnection
	}

	connection, err := s.channelConnectionRepository.FindByID(input.ConnectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return nil, ErrInvalidMercadoLibreConnection
		}
		return nil, fmt.Errorf("error loading connection %d: %w", input.ConnectionID, err)
	}

	channel, err := s.channelRepository.FindByID(connection.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}
	if !strings.EqualFold(strings.TrimSpace(channel.Code), "MERCADOLIBRE") {
		return nil, ErrNotMercadoLibreConnection
	}

	// A full account scan can run long enough (many multiget batches, each
	// throttled by the shared 90/min rate limiter — see
	// MercadoLibreRateLimit) that a token fetched once up front could go
	// stale before the run finishes. So no accessToken is captured here:
	// every call below re-validates it immediately beforehand via
	// s.freshAccessToken, which is cheap (a local DB read) whenever the
	// token isn't actually near expiry, and only reaches out to MercadoLibre
	// when it truly needs refreshing.
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
		result.Results = append(result.Results, s.syncOne(ctx, input.ConnectionID, &details[i]))
	}

	return result, nil
}

// freshAccessToken re-validates connectionID's token immediately before an
// outbound MercadoLibre call — see the comment in SyncFlaggedListings for
// why this is called at every call site here instead of once for the whole
// run.
func (s *MercadoLibreListingsAuditService) freshAccessToken(ctx context.Context, connectionID int64) (string, error) {
	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return "", fmt.Errorf("error getting mercadolibre access token: %w", err)
	}
	return accessToken, nil
}

// listAllSellerItemIDs pages through
// GET /users/{sellerID}/items/search?search_type=scan (MercadoLibre's
// cursor-based "scan" mode — no 1000-result cap, unlike classic
// offset+limit pagination) until a page comes back with no results,
// re-validating the access token before every page.
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
// mercadoLibreInfra.MaxItemsBatchSize and calls GetItemsDetail once per
// batch, re-validating the access token before each one — mirrors
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

	product, err := s.productRepository.FindBySKU(sku)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			outcome.SkipReason = "no local ecom_products row for this sku"
			return outcome
		}
		outcome.Error = fmt.Sprintf("error loading product by sku: %v", err)
		return outcome
	}
	outcome.ProductFound = true
	outcome.ProductID = &product.ID

	// Provisions the local category (only if the product has none yet — see
	// ProvisionCategoryAttributes) plus every attribute slot/option the
	// listing's category exposes; SetValue below needs those slots to exist
	// first.
	provisioned, err := s.channelAttributeValuesService.ProvisionCategoryAttributes(ctx, channelAttributeValuesApp.ProvisionCategoryAttributesInput{
		SKU:          sku,
		CategoryID:   item.CategoryID,
		ConnectionID: &connectionID,
		ActorID:      systemMercadoLibreSyncActorID,
	})
	if err != nil {
		outcome.Error = fmt.Sprintf("error provisioning category/attributes: %v", err)
		return outcome
	}

	dataTypeByExternalKey := make(map[string]string, len(provisioned.Results))
	for _, provisionedAttr := range provisioned.Results {
		if provisionedAttr.SourceType == "custom_attribute" && provisionedAttr.DataType != "" {
			dataTypeByExternalKey[provisionedAttr.ExternalKey] = provisionedAttr.DataType
		}
	}

	attributesSet := 0
	var attributeErrors []string
	for _, attr := range item.Attributes {
		dataType, ok := dataTypeByExternalKey[attr.ID]
		if !ok {
			// Not a custom_attribute slot this run provisioned (system_field,
			// deliberately skipped, or read_only) — nothing to set.
			continue
		}

		setInput, ok := buildSetValueInput(sku, attr, dataType)
		if !ok {
			continue
		}

		if _, err := s.channelAttributeValuesService.SetValue(ctx, setInput); err != nil {
			attributeErrors = append(attributeErrors, fmt.Sprintf("%s: %v", attr.ID, err))
			continue
		}
		attributesSet++
	}
	outcome.AttributesSet = attributesSet
	outcome.AttributeErrors = attributeErrors

	name := stripSKUFromTitle(item.Title, sku)
	if name != "" && name != product.Name {
		if err := s.productRepository.UpdateName(product.ID, name, systemMercadoLibreSyncActorID); err != nil {
			outcome.Error = fmt.Sprintf("error updating product name: %v", err)
			return outcome
		}
		outcome.NameUpdated = true
		outcome.NewName = name
	}

	return outcome
}

// buildSetValueInput types attr's live MercadoLibre value against dataType
// (as channel_attribute_values.SetValue requires) and reports ok=false when
// the value is empty or doesn't parse as dataType — buildSetValueInput never
// guesses, it just leaves that one attribute unset, same "one bad value
// doesn't stop the rest" rule as everywhere else in this audit.
func buildSetValueInput(sku string, attr mercadoLibreInfra.ItemAttribute, dataType string) (channelAttributeValuesApp.SetValueInput, bool) {
	input := channelAttributeValuesApp.SetValueInput{
		SKU:         sku,
		ExternalKey: attr.ID,
		DataType:    dataType,
		ActorID:     systemMercadoLibreSyncActorID,
	}

	switch dataType {
	case "enum":
		value := strings.TrimSpace(attr.ValueName)
		if value == "" {
			return input, false
		}
		input.EnumValue = &value
		return input, true

	case "number":
		value := strings.TrimSpace(attr.ValueName)
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return input, false
		}
		input.ValueNumber = &parsed
		return input, true

	case "boolean":
		parsed, ok := parseMercadoLibreBoolean(attr.ValueName)
		if !ok {
			return input, false
		}
		input.ValueBool = &parsed
		return input, true

	case "date":
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(attr.ValueName))
		if err != nil {
			return input, false
		}
		input.ValueDate = &parsed
		return input, true

	default: // "text"
		value := strings.TrimSpace(attr.ValueName)
		if value == "" {
			return input, false
		}
		input.ValueText = &value
		return input, true
	}
}

// parseMercadoLibreBoolean maps the handful of value_name strings
// MercadoLibre actually sends for a boolean attribute (Spanish "Sí"/"No",
// their English equivalents, and the literal "true"/"false") — ok is false
// for anything else rather than guessing.
func parseMercadoLibreBoolean(raw string) (value bool, ok bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "sí", "si", "yes", "true", "verdadero":
		return true, true
	case "no", "false", "falso":
		return false, true
	default:
		return false, false
	}
}

// stripSKUFromTitle removes title's trailing " {sku}" — MercadoLibre titles
// created by this audit's target listings are always "{name} {sku}" — so
// ecom_products.name ends up holding just the name portion. If title
// doesn't actually end with sku (unexpected), it's returned unchanged rather
// than guessed at.
func stripSKUFromTitle(title, sku string) string {
	title = strings.TrimSpace(title)
	sku = strings.TrimSpace(sku)
	if sku == "" || !strings.HasSuffix(title, sku) {
		return title
	}

	trimmed := strings.TrimSuffix(title, sku)
	trimmed = strings.TrimRight(trimmed, " -_")
	return strings.TrimSpace(trimmed)
}

// resolveSKUFromItem extracts the seller's own SKU straight from
// MercadoLibre's item payload — the SELLER_SKU attribute (current
// convention, same key channel_attribute_values.systemFieldByExternalKey
// maps to "sku") or the older top-level seller_custom_field. Falls back to
// the MercadoLibre item id itself when neither is set, so a listing with no
// SKU recorded on MercadoLibre's side still gets a usable, unique lookup key
// (which simply won't match any ecom_products row).
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

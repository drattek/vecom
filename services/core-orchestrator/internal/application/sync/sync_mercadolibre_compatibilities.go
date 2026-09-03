package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

const (
	mercadoLibreCompatibilitySiteID = "MLM"

	// mercadoLibreCompatibilityTag is the MercadoLibre item tag that marks a
	// listing as needing vehicle compatibilities reported before it clears
	// moderation — see infrastructure/docs (autopartes compatibilities).
	mercadoLibreCompatibilityTag = "incomplete_compatibilities"

	// mercadoLibreCompatibilityCreationSource is stamped on every
	// products_families entry this flow creates. MercadoLibre requires one
	// of ITEM_SUGGESTIONS/NEW_VEHICLES/DEFAULT; DEFAULT is correct here since
	// this data comes from our own ecom_product_vehicle_compatibility table,
	// not from a MercadoLibre suggestion or a "new vehicle" filter.
	mercadoLibreCompatibilityCreationSource = "DEFAULT"
)

var ErrNotMercadoLibreConnection = errors.New("connection's channel is not MERCADOLIBRE")

// ErrInvalidCompatibilityCopyItems is returned by CopyCompatibilities when
// sourceItemId or sku is missing.
var ErrInvalidCompatibilityCopyItems = errors.New("sourceItemId and sku are required")

// ErrInvalidCompatibilityCopyActor is returned by CopyCompatibilities when
// called without a valid ActorID to stamp created_by/updated_by on the new
// local rows.
var ErrInvalidCompatibilityCopyActor = errors.New("actorId is required")

// mlPositionValue is one entry of MercadoLibre's closed POSITION restriction
// value set. Confirmed for real via
// GET /catalog_compatibilities/restrictions/values against
// MLM-CARS_AND_VANS_FOR_COMPATIBILITIES / MLM-VEHICLE_BRAKE_PADS — this
// dictionary is deliberately not guessed beyond that.
type mlPositionValue struct{ ID, Name string }

var mlPositionValuesByToken = map[string]mlPositionValue{
	"delantero":   {"13701104", "Delantera"},
	"delantera":   {"13701104", "Delantera"},
	"adelante":    {"13701104", "Delantera"},
	"delante":     {"13701104", "Delantera"},
	"trasero":     {"13701105", "Trasera"},
	"trasera":     {"13701105", "Trasera"},
	"izquierdo":   {"2262158", "Izquierda"},
	"izquierda":   {"2262158", "Izquierda"},
	"derecho":     {"2262160", "Derecha"},
	"derecha":     {"2262160", "Derecha"},
	"superior":    {"4774238", "Superior"},
	"inferior":    {"4774239", "Inferior"},
	"interno":     {"13373177", "Interno"},
	"externo":     {"13373178", "Externo"},
	"intermedio":  {"13373179", "Intermedio"},
	"centro":      {"13373180", "Centro"},
	"conductor":   {"13373175", "Conductor"},
	"acompañante": {"13373176", "Acompañante"},
	"acompanante": {"13373176", "Acompañante"},
	"pasajero":    {"13373176", "Acompañante"},
}

// mlSideValueIDs is the subset of mlPositionValuesByToken's value_id
// dictionary that describes laterality (left/right, or the driver/passenger-
// side equivalent) rather than where on the vehicle a part sits — see
// splitCompatibilityRestrictionValues, which uses this to decide whether a
// value downloaded from MercadoLibre belongs in
// ecom_product_vehicle_compatibility.side instead of .position.
var mlSideValueIDs = map[string]bool{
	"2262158":  true, // Izquierda
	"2262160":  true, // Derecha
	"13373175": true, // Conductor
	"13373176": true, // Acompañante
}

// MercadoLibreCompatibilityService pushes ecom_product_vehicle_compatibility
// data (brand/model/year plus motor/position/side, where the product/fitment
// pair carries them) to MercadoLibre for listings stuck under_review because
// MercadoLibre requires reported vehicle compatibilities for autopart
// categories (tag incomplete_compatibilities).
type MercadoLibreCompatibilityService struct {
	channelProductMapRepository           *mysqlInfra.ChannelProductMapRepository
	channelConnectionRepository           *mysqlInfra.ChannelConnectionRepository
	channelRepository                     *mysqlInfra.ChannelRepository
	productRepository                     *mysqlInfra.ProductRepository
	vehicleFitmentsRepository             *mysqlInfra.VehicleFitmentsRepository
	brandsRepository                      *mysqlInfra.BrandsRepository
	productVehicleCompatibilityRepository *mysqlInfra.ProductVehicleCompatibilityRepository
	tokenService                          *MercadoLibreTokenService
	itemsHandler                          *mercadoLibreInfra.ItemsHandler
	compatibilitiesHandler                *mercadoLibreInfra.CompatibilitiesHandler
}

func NewMercadoLibreCompatibilityService(
	channelProductMapRepository *mysqlInfra.ChannelProductMapRepository,
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	productRepository *mysqlInfra.ProductRepository,
	vehicleFitmentsRepository *mysqlInfra.VehicleFitmentsRepository,
	brandsRepository *mysqlInfra.BrandsRepository,
	productVehicleCompatibilityRepository *mysqlInfra.ProductVehicleCompatibilityRepository,
	tokenService *MercadoLibreTokenService,
	rateLimiter *mercadoLibreInfra.RateLimiter,
) *MercadoLibreCompatibilityService {
	client := mercadoLibreInfra.NewClient(nil, "", rateLimiter)

	return &MercadoLibreCompatibilityService{
		channelProductMapRepository:           channelProductMapRepository,
		channelConnectionRepository:           channelConnectionRepository,
		channelRepository:                     channelRepository,
		productRepository:                     productRepository,
		vehicleFitmentsRepository:             vehicleFitmentsRepository,
		brandsRepository:                      brandsRepository,
		productVehicleCompatibilityRepository: productVehicleCompatibilityRepository,
		tokenService:                          tokenService,
		itemsHandler:                          mercadoLibreInfra.NewItemsHandler(client),
		compatibilitiesHandler:                mercadoLibreInfra.NewCompatibilitiesHandler(client),
	}
}

type FixCompatibilitiesInput struct {
	ConnectionID int64
}

// CompatibilityFixOutcome reports what happened for one
// ecom_channel_product_map row. A batch never fails wholesale on one bad
// row.
type CompatibilityFixOutcome struct {
	ProductID  int64  `json:"productId"`
	SKU        string `json:"sku,omitempty"`
	ExternalID string `json:"externalId,omitempty"`
	Success    bool   `json:"success,omitempty"`
	// Skipped is true when the item no longer needs fixing (MercadoLibre
	// already cleared the incomplete_compatibilities tag) or the category
	// doesn't support compatibilities at all — Error is empty in that case,
	// SkipReason explains why.
	Skipped                     bool   `json:"skipped,omitempty"`
	SkipReason                  string `json:"skipReason,omitempty"`
	CreatedCompatibilitiesCount int    `json:"createdCompatibilitiesCount,omitempty"`
	// UnmappedPositionTokens lists any position/side words that didn't match
	// MercadoLibre's known POSITION value set (see mlPositionValuesByToken) —
	// informational only, the compatibility is still created without them
	// rather than guessing a value.
	UnmappedPositionTokens []string `json:"unmappedPositionTokens,omitempty"`
	Error                  string   `json:"error,omitempty"`
}

type FixCompatibilitiesResult struct {
	Results []CompatibilityFixOutcome `json:"results"`
}

// FixUnderReviewListings finds every ecom_channel_product_map row for
// input.ConnectionID (every status except closed — see the filter below)
// with a vehicle fitment attached, and reports each one's compatibility data
// to MercadoLibre when the real item still carries the
// incomplete_compatibilities tag. Checking synced/paused listings too, not
// only ones already under_review, catches the tag while it's still just a
// warning on an otherwise-active listing — before MercadoLibre pauses it for
// this reason. It never touches ecom_channel_product_map.status itself —
// RefreshChannelProductMapStatus (see mercadolibre_handler.go) is what polls
// MercadoLibre for the real status afterward, since MercadoLibre processes a
// compatibility submission asynchronously. Every candidate is processed in
// one call — there is no per-call cap — since MercadoLibre's shared rate
// limiter (see mercadolibre.RateLimiter) already serializes the underlying
// HTTP calls instead of failing when the budget is exhausted, so a larger
// backlog only makes this call take longer, not fail partway through.
func (s *MercadoLibreCompatibilityService) FixUnderReviewListings(ctx context.Context, input FixCompatibilitiesInput) (*FixCompatibilitiesResult, error) {
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

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, input.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("error getting mercadolibre access token: %w", err)
	}

	allMappings, err := s.channelProductMapRepository.FindAllByConnectionID(ctx, input.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel product map for connection %d: %w", input.ConnectionID, err)
	}

	// Every status except "closed" is considered, not just under_review:
	// MercadoLibre shows incomplete_compatibilities as a warning tag on an
	// otherwise-synced listing before it eventually gets paused for it, so
	// checking synced/paused listings too (not only ones already broken)
	// catches that ahead of time. fixOne itself is what actually decides
	// whether a given item needs anything, via the real tag on MercadoLibre —
	// this filter only skips "closed" listings, which have no live external
	// listing to check at all. No cap is applied here: every eligible row
	// becomes a candidate, so a backlog larger than any earlier fixed limit
	// can no longer starve rows sitting past it in id order.
	candidates := make([]mysqlInfra.ChannelProductMapDTO, 0, len(allMappings))
	for _, mapping := range allMappings {
		if mapping.Status == mercadoLibreMapStatusClosed || mapping.VehicleFitmentID == nil || mapping.ExternalID == nil {
			continue
		}
		candidates = append(candidates, mapping)
	}

	if len(candidates) == 0 {
		return &FixCompatibilitiesResult{Results: []CompatibilityFixOutcome{}}, nil
	}

	// Fetched once and reused across every candidate below — it's the same
	// site-wide dump regardless of which item is being fixed.
	domainDump, err := s.compatibilitiesHandler.GetDomainCompatibilities(ctx, accessToken, mercadoLibreCompatibilitySiteID)
	if err != nil {
		return nil, fmt.Errorf("error loading mercadolibre domain compatibilities dump: %w", err)
	}

	results := make([]CompatibilityFixOutcome, 0, len(candidates))
	for _, mapping := range candidates {
		results = append(results, s.fixOne(ctx, accessToken, domainDump, mapping))
	}

	return &FixCompatibilitiesResult{Results: results}, nil
}

type CopyCompatibilitiesInput struct {
	ConnectionID int64
	// SourceItemID is the MercadoLibre item id whose compatibilities are
	// downloaded (GET .../compatibilities?extended=true) — nothing is ever
	// written back to MercadoLibre for it.
	SourceItemID string
	// SKU resolves which local ecom_products row (via
	// ProductRepository.FindBySKU) the downloaded compatibilities are linked
	// to.
	SKU string
	// ActorID stamps created_by/updated_by on any new/changed local row.
	ActorID int64
}

type CopyCompatibilitiesResult struct {
	SourceItemID string                          `json:"sourceItemId"`
	SKU          string                          `json:"sku"`
	ProductID    int64                           `json:"productId"`
	Results      []CopyLocalCompatibilityOutcome `json:"results"`
	// NewVehicleFitments lists every ecom_vehicle_fitments row this call
	// created (i.e. the vehicle didn't already exist locally under any
	// product) — a filtered, human-readable view of the same fitments
	// already present per-entry in Results (FitmentCreated=true), collected
	// here so a caller doesn't have to scan Results to find them.
	NewVehicleFitments []CreatedVehicleFitment `json:"newVehicleFitments,omitempty"`
}

// CopyLocalCompatibilityOutcome reports what happened resolving one
// MercadoLibre compatibility entry (see mercadolibre.ItemCompatibilityEntry)
// into a local ecom_vehicle_fitments/ecom_product_vehicle_compatibility row.
// A batch never fails wholesale on one bad entry.
type CopyLocalCompatibilityOutcome struct {
	CatalogProductID   string `json:"catalogProductId,omitempty"`
	CatalogProductName string `json:"catalogProductName,omitempty"`
	VehicleFitmentID   int64  `json:"vehicleFitmentId,omitempty"`
	FitmentCreated     bool   `json:"fitmentCreated,omitempty"`
	CompatibilityID    int64  `json:"compatibilityId,omitempty"`
	// Skipped is true both for MercadoLibre catalog-managed entries that
	// carry no individual vehicle to resolve (see
	// mercadolibre.ItemCompatibilityEntry) and for a vehicle the target
	// product is already linked to — SkipReason distinguishes the two.
	Skipped    bool   `json:"skipped,omitempty"`
	SkipReason string `json:"skipReason,omitempty"`
	Error      string `json:"error,omitempty"`
}

// CreatedVehicleFitment is one ecom_vehicle_fitments row created while
// syncing local compatibilities from MercadoLibre.
type CreatedVehicleFitment struct {
	VehicleFitmentID int64  `json:"vehicleFitmentId"`
	Brand            string `json:"brand"`
	Model            string `json:"model"`
	Year             int    `json:"year"`
}

// CopyCompatibilities downloads every compatibility MercadoLibre reports for
// input.SourceItemID (GET .../compatibilities?extended=true) and links each
// one, resolving/creating its ecom_vehicle_fitments row as needed, to the
// local product input.SKU resolves to (see
// linkCompatibilityEntriesToProduct). Nothing is ever written back to
// MercadoLibre — this only reads sourceItemId and writes locally.
// input.ConnectionID resolves which MercadoLibre account's access token to
// call with, same as FixUnderReviewListings.
func (s *MercadoLibreCompatibilityService) CopyCompatibilities(ctx context.Context, input CopyCompatibilitiesInput) (*CopyCompatibilitiesResult, error) {
	sourceItemID := strings.TrimSpace(input.SourceItemID)
	sku := strings.TrimSpace(input.SKU)
	if sourceItemID == "" || sku == "" {
		return nil, ErrInvalidCompatibilityCopyItems
	}

	if input.ConnectionID <= 0 {
		return nil, ErrInvalidMercadoLibreConnection
	}

	if input.ActorID <= 0 {
		return nil, ErrInvalidCompatibilityCopyActor
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

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, input.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("error getting mercadolibre access token: %w", err)
	}

	product, err := s.productRepository.FindBySKU(ctx, sku)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, fmt.Errorf("%w: sku %s", mysqlInfra.ErrProductNotFound, sku)
		}
		return nil, fmt.Errorf("error resolving product for sku %s: %w", sku, err)
	}

	results, newFitments, err := s.CopyCompatibilitiesForProduct(ctx, accessToken, sourceItemID, product.ID, input.ActorID)
	if err != nil {
		return nil, err
	}

	return &CopyCompatibilitiesResult{
		SourceItemID:       sourceItemID,
		SKU:                sku,
		ProductID:          product.ID,
		Results:            results,
		NewVehicleFitments: newFitments,
	}, nil
}

// CopyCompatibilitiesForProduct downloads every compatibility MercadoLibre
// reports for sourceItemID (GET .../compatibilities?extended=true) and links
// each one to productID via ecom_product_vehicle_compatibility, creating the
// ecom_vehicle_fitments/ecom_brands rows it needs — the same work
// CopyCompatibilities does, but taking an already-resolved productID and a
// live accessToken directly instead of a connection + sku. Used by the
// flagged-listings audit (sync.MercadoLibreListingsAuditService), which loops
// over many listings under one token and often has just created the product.
// Nothing is ever written back to MercadoLibre.
func (s *MercadoLibreCompatibilityService) CopyCompatibilitiesForProduct(ctx context.Context, accessToken, sourceItemID string, productID, actorID int64) ([]CopyLocalCompatibilityOutcome, []CreatedVehicleFitment, error) {
	entries, err := s.compatibilitiesHandler.GetItemCompatibilitiesExtended(ctx, accessToken, sourceItemID)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading mercadolibre compatibilities for %s: %w", sourceItemID, err)
	}

	results, newFitments := s.linkCompatibilityEntriesToProduct(ctx, accessToken, entries, productID, actorID)
	return results, newFitments, nil
}

// CompatibilityDiagnosis is DiagnoseItem's read-only dump of everything that
// decides whether a listing's vehicle compatibilities can be copied locally:
// the item's catalog/user-product linkage, whether its category supports
// compatibilities at all, and the raw MercadoLibre compatibility payloads
// (both the item endpoint and — when the listing is User Product-linked — the
// user-product one). Nothing is written anywhere.
type CompatibilityDiagnosis struct {
	ItemID           string   `json:"itemId"`
	CategoryID       string   `json:"categoryId"`
	DomainID         string   `json:"domainId"`
	CatalogProductID string   `json:"catalogProductId,omitempty"`
	UserProductID    string   `json:"userProductId,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	// CompatibleDomainID is the vehicle domain the item's part domain reports
	// against, per MercadoLibre's site-wide dump; CategorySupportsCompatibilities
	// is false when the item's (domain, category) pair isn't in that dump at all.
	CompatibleDomainID              string `json:"compatibleDomainId,omitempty"`
	CategorySupportsCompatibilities bool   `json:"categorySupportsCompatibilities"`
	// RestrictionsStatus is the category's position/side restriction support
	// ("ENABLED"/"DISABLED"), empty when the category isn't in the dump.
	RestrictionsStatus string `json:"restrictionsStatus,omitempty"`
	// LinkableEntryCount is how many entries the current copy flow
	// (linkCompatibilityEntriesToProduct) could actually resolve to a vehicle —
	// i.e. entries with a non-empty catalog_product_id. Zero here with a
	// non-empty ItemCompatibilities is the "catalog-managed, brand totals only"
	// case.
	LinkableEntryCount int `json:"linkableEntryCount"`

	ItemCompatibilities        json.RawMessage `json:"itemCompatibilities,omitempty"`
	ItemCompatibilitiesError   string          `json:"itemCompatibilitiesError,omitempty"`
	UserProductCompatibilities json.RawMessage `json:"userProductCompatibilities,omitempty"`
	UserProductError           string          `json:"userProductError,omitempty"`
}

// DiagnoseItem reports, read-only, why CopyCompatibilities produced nothing
// for a given listing: it fetches the item, the site-wide domain dump, the raw
// GET /items/{id}/compatibilities?extended=true body and — when the item is
// linked to a User Product — the raw GET /user-products/{id}/compatibilities
// body, so it's clear whether the seller compatibilities live somewhere the
// current copy flow doesn't look. connectionID picks the MercadoLibre account.
func (s *MercadoLibreCompatibilityService) DiagnoseItem(ctx context.Context, connectionID int64, itemID string) (*CompatibilityDiagnosis, error) {
	itemID = strings.TrimSpace(itemID)
	if connectionID <= 0 {
		return nil, ErrInvalidMercadoLibreConnection
	}
	if itemID == "" {
		return nil, ErrInvalidCompatibilityCopyItems
	}

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error getting mercadolibre access token: %w", err)
	}

	item, err := s.itemsHandler.GetItem(ctx, accessToken, itemID)
	if err != nil {
		return nil, fmt.Errorf("error fetching mercadolibre item %s: %w", itemID, err)
	}

	diagnosis := &CompatibilityDiagnosis{
		ItemID:           item.ID,
		CategoryID:       item.CategoryID,
		DomainID:         item.DomainID,
		CatalogProductID: item.CatalogProductID,
		UserProductID:    item.UserProductID,
		Tags:             item.Tags,
	}

	if dump, err := s.compatibilitiesHandler.GetDomainCompatibilities(ctx, accessToken, mercadoLibreCompatibilitySiteID); err == nil {
		if compatibleDomainID, categoryEntry, derr := resolveCompatibleDomain(dump, item.DomainID, item.CategoryID); derr == nil {
			diagnosis.CompatibleDomainID = compatibleDomainID
			diagnosis.CategorySupportsCompatibilities = true
			diagnosis.RestrictionsStatus = categoryEntry.RestrictionsStatus
		}
	} else {
		log.Printf("mercadolibre compatibilities diagnose: item %s — domain dump fetch failed: %v", itemID, err)
	}

	if raw, err := s.compatibilitiesHandler.GetItemCompatibilitiesRaw(ctx, accessToken, itemID, true); err != nil {
		diagnosis.ItemCompatibilitiesError = err.Error()
	} else {
		diagnosis.ItemCompatibilities = raw
	}

	if entries, err := s.compatibilitiesHandler.GetItemCompatibilitiesExtended(ctx, accessToken, itemID); err == nil {
		for _, entry := range entries {
			if strings.TrimSpace(entry.CatalogProductID) != "" {
				diagnosis.LinkableEntryCount++
			}
		}
	}

	if item.UserProductID != "" {
		if raw, err := s.compatibilitiesHandler.GetUserProductCompatibilitiesRaw(ctx, accessToken, item.UserProductID, true); err != nil {
			diagnosis.UserProductError = err.Error()
		} else {
			diagnosis.UserProductCompatibilities = raw
		}
	}

	return diagnosis, nil
}

// linkCompatibilityEntriesToProduct resolves, for each entry with a
// resolvable catalog_product_id, a matching ecom_vehicle_fitments row
// (creating it — and its brand, if that's new too — when it doesn't already
// exist) and links it to productID via ecom_product_vehicle_compatibility. A
// vehicle productID is already linked to, or a catalog-managed entry with no
// individual vehicle to resolve (see mercadolibre.ItemCompatibilityEntry), is
// reported as skipped rather than an error; any other per-entry failure is
// reported too but doesn't stop the rest of the batch.
func (s *MercadoLibreCompatibilityService) linkCompatibilityEntriesToProduct(ctx context.Context, accessToken string, entries []mercadoLibreInfra.ItemCompatibilityEntry, productID, actorID int64) ([]CopyLocalCompatibilityOutcome, []CreatedVehicleFitment) {
	results := make([]CopyLocalCompatibilityOutcome, 0, len(entries))
	newFitments := make([]CreatedVehicleFitment, 0)
	for _, entry := range entries {
		outcome := CopyLocalCompatibilityOutcome{CatalogProductID: entry.CatalogProductID, CatalogProductName: entry.CatalogProductName}

		catalogProductID := strings.TrimSpace(entry.CatalogProductID)
		if catalogProductID == "" {
			outcome.Skipped = true
			outcome.SkipReason = "mercadolibre catalog-managed compatibility exposes only brand-level totals (since 2026-07-15) — no individual vehicle to resolve"
			results = append(results, outcome)
			continue
		}

		resolved, err := s.resolveOrCreateVehicleFitmentFromCatalogProduct(ctx, accessToken, catalogProductID, actorID)
		if err != nil {
			outcome.Error = err.Error()
			results = append(results, outcome)
			continue
		}
		outcome.VehicleFitmentID = resolved.FitmentID
		outcome.FitmentCreated = resolved.Created
		if resolved.Created {
			newFitments = append(newFitments, CreatedVehicleFitment{
				VehicleFitmentID: resolved.FitmentID,
				Brand:            resolved.Brand,
				Model:            resolved.Model,
				Year:             resolved.Year,
			})
		}

		position, side := splitCompatibilityRestrictionValues(entry.Restrictions)
		compat, createErr := s.productVehicleCompatibilityRepository.Create(ctx, mysqlInfra.CreateProductVehicleCompatibilityInput{
			ProductID:        productID,
			VehicleFitmentID: resolved.FitmentID,
			Position:         position,
			Side:             side,
			CreatedBy:        actorID,
		})
		switch {
		case createErr == nil:
			outcome.CompatibilityID = compat.ID
		case errors.Is(createErr, mysqlInfra.ErrProductVehicleCompatibilityAlreadyExists):
			outcome.Skipped = true
			outcome.SkipReason = "product is already linked to this vehicle fitment"
		default:
			outcome.Error = createErr.Error()
		}

		results = append(results, outcome)
	}

	return results, newFitments
}

// resolvedVehicleFitment is resolveOrCreateVehicleFitmentFromCatalogProduct's
// result: the local fitment id plus the brand/model/year it resolved to
// (needed by the caller to report newly created fitments — see
// CreatedVehicleFitment) and whether it was created just now or already
// existed.
type resolvedVehicleFitment struct {
	FitmentID int64
	Created   bool
	Brand     string
	Model     string
	Year      int
}

// resolveOrCreateVehicleFitmentFromCatalogProduct decomposes a MercadoLibre
// catalog product (GET /products/{catalogProductID}) into brand/model/year
// and finds or creates the matching ecom_vehicle_fitments row — same
// find-or-create-brand pattern as
// compatibility.VehicleFitmentService.BulkImportFitments. The fitment is
// created as a single-year range (YearEnd left nil), same convention as that
// flow, since MercadoLibre's catalog product carries one year, not a range.
// MODEL is looked up under CAR_AND_VAN_MODEL first (MLM's attribute id) and
// falls back to plain MODEL (other sites' attribute id per MercadoLibre's
// own docs).
func (s *MercadoLibreCompatibilityService) resolveOrCreateVehicleFitmentFromCatalogProduct(ctx context.Context, accessToken, catalogProductID string, actorID int64) (resolvedVehicleFitment, error) {
	product, err := s.itemsHandler.GetCatalogProduct(ctx, accessToken, catalogProductID)
	if err != nil {
		return resolvedVehicleFitment{}, fmt.Errorf("error fetching mercadolibre catalog product %s: %w", catalogProductID, err)
	}

	brandName := compatibilityCatalogAttributeValue(product.Attributes, "BRAND")
	model := compatibilityCatalogAttributeValue(product.Attributes, "CAR_AND_VAN_MODEL", "MODEL")
	yearText := compatibilityCatalogAttributeValue(product.Attributes, "YEAR")
	if brandName == "" || model == "" || yearText == "" {
		return resolvedVehicleFitment{}, fmt.Errorf("catalog product %s is missing brand/model/year attributes", catalogProductID)
	}

	year, convErr := strconv.Atoi(yearText)
	if convErr != nil {
		return resolvedVehicleFitment{}, fmt.Errorf("catalog product %s has a non-numeric year %q: %w", catalogProductID, yearText, convErr)
	}

	brand, err := s.brandsRepository.FindByName(ctx, brandName)
	if errors.Is(err, mysqlInfra.ErrBrandNotFound) {
		brand, err = s.brandsRepository.Create(ctx, mysqlInfra.CreateBrandInput{Name: brandName, CreatedBy: actorID})
	}
	if err != nil {
		return resolvedVehicleFitment{}, fmt.Errorf("error resolving brand %q: %w", brandName, err)
	}

	existing, err := s.vehicleFitmentsRepository.FindByUniqueKey(ctx, brand.ID, model, year, nil)
	if err == nil {
		return resolvedVehicleFitment{FitmentID: existing.ID, Brand: brandName, Model: model, Year: year}, nil
	}
	if !errors.Is(err, mysqlInfra.ErrVehicleFitmentNotFound) {
		return resolvedVehicleFitment{}, fmt.Errorf("error looking up vehicle fitment: %w", err)
	}

	fitment, err := s.vehicleFitmentsRepository.Create(ctx, mysqlInfra.CreateVehicleFitmentInput{
		BrandID:   brand.ID,
		Model:     model,
		YearStart: year,
		CreatedBy: actorID,
	})
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrVehicleFitmentAlreadyExists) {
			existing, findErr := s.vehicleFitmentsRepository.FindByUniqueKey(ctx, brand.ID, model, year, nil)
			if findErr != nil {
				return resolvedVehicleFitment{}, fmt.Errorf("error loading existing vehicle fitment after conflict: %w", findErr)
			}
			return resolvedVehicleFitment{FitmentID: existing.ID, Brand: brandName, Model: model, Year: year}, nil
		}
		return resolvedVehicleFitment{}, fmt.Errorf("error creating vehicle fitment: %w", err)
	}

	return resolvedVehicleFitment{FitmentID: fitment.ID, Created: true, Brand: brandName, Model: model, Year: year}, nil
}

// compatibilityCatalogAttributeValue returns the first non-empty ValueName
// among attrs for any of ids, tried in order.
func compatibilityCatalogAttributeValue(attrs []mercadoLibreInfra.ItemAttribute, ids ...string) string {
	for _, id := range ids {
		for _, attr := range attrs {
			if attr.ID == id && strings.TrimSpace(attr.ValueName) != "" {
				return strings.TrimSpace(attr.ValueName)
			}
		}
	}
	return ""
}

// splitCompatibilityRestrictionValues classifies a compatibility entry's
// position restriction values by value_id (see mlSideValueIDs) into the two
// separate columns MercadoLibre's own response doesn't distinguish between:
// position (where on the vehicle — front/back/up/down/inner/outer/center)
// and side (laterality — left/right, or the driver/passenger-side
// equivalent). An unrecognized value_id falls back into position rather
// than being dropped or guessed into side.
func splitCompatibilityRestrictionValues(restrictions []mercadoLibreInfra.ItemCompatibilityRestriction) (position, side string) {
	var positionNames, sideNames []string
	for _, restriction := range restrictions {
		for _, group := range restriction.AttributeValues {
			for _, value := range group.Values {
				name := strings.TrimSpace(value.ValueName)
				if name == "" {
					name = strings.TrimSpace(value.ValueID)
				}
				if name == "" {
					continue
				}

				if mlSideValueIDs[strings.TrimSpace(value.ValueID)] {
					sideNames = append(sideNames, name)
				} else {
					positionNames = append(positionNames, name)
				}
			}
		}
	}
	return strings.Join(positionNames, " "), strings.Join(sideNames, " ")
}

func (s *MercadoLibreCompatibilityService) fixOne(ctx context.Context, accessToken string, domainDump []mercadoLibreInfra.DomainDumpEntry, mapping mysqlInfra.ChannelProductMapDTO) CompatibilityFixOutcome {
	externalID := strings.TrimSpace(*mapping.ExternalID)
	outcome := CompatibilityFixOutcome{ProductID: mapping.ProductID, ExternalID: externalID}

	product, err := s.productRepository.FindByID(ctx, mapping.ProductID)
	if err != nil {
		outcome.Error = fmt.Sprintf("error loading product: %v", err)
		return outcome
	}
	outcome.SKU = product.SKU

	item, err := s.itemsHandler.GetItem(ctx, accessToken, externalID)
	if err != nil {
		outcome.Error = fmt.Sprintf("error fetching mercadolibre item: %v", err)
		return outcome
	}

	if !containsTag(item.Tags, mercadoLibreCompatibilityTag) {
		outcome.Skipped = true
		outcome.SkipReason = "item no longer has the incomplete_compatibilities tag"
		return outcome
	}

	return s.pushCompatibilitiesForProduct(ctx, accessToken, domainDump, item, product, false)
}

// DomainDump fetches MercadoLibre's site-wide autopart→vehicle compatibility
// dump for connectionID's account, resolving that connection's access token
// first. Callers publishing a batch of new listings fetch it once and pass
// the result to every PushForNewListing call rather than re-fetching it per
// listing (see GetDomainCompatibilities' own note).
func (s *MercadoLibreCompatibilityService) DomainDump(ctx context.Context, connectionID int64) ([]mercadoLibreInfra.DomainDumpEntry, error) {
	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error getting mercadolibre access token: %w", err)
	}
	return s.compatibilitiesHandler.GetDomainCompatibilities(ctx, accessToken, mercadoLibreCompatibilitySiteID)
}

// PushForNewListing reports product's ecom_product_vehicle_compatibility rows
// to a freshly created MercadoLibre listing (externalID), without waiting for
// the incomplete_compatibilities tag MercadoLibre only assigns during
// moderation. It is a silent no-op (outcome.Skipped, nil error) when the
// listing's category doesn't support vehicle compatibilities or the product
// has none recorded locally — so it's safe to call unconditionally right
// after creating any item. domainDump may be a cached copy shared across a
// publish batch; pass nil to have this fetch its own.
func (s *MercadoLibreCompatibilityService) PushForNewListing(ctx context.Context, accessToken, externalID string, product *mysqlInfra.ProductDTO, domainDump []mercadoLibreInfra.DomainDumpEntry) (CompatibilityFixOutcome, error) {
	externalID = strings.TrimSpace(externalID)
	outcome := CompatibilityFixOutcome{ProductID: product.ID, SKU: product.SKU, ExternalID: externalID}
	if externalID == "" {
		return outcome, fmt.Errorf("externalID is required")
	}

	if domainDump == nil {
		dump, err := s.compatibilitiesHandler.GetDomainCompatibilities(ctx, accessToken, mercadoLibreCompatibilitySiteID)
		if err != nil {
			return outcome, fmt.Errorf("error loading mercadolibre domain compatibilities dump: %w", err)
		}
		domainDump = dump
	}

	item, err := s.itemsHandler.GetItem(ctx, accessToken, externalID)
	if err != nil {
		return outcome, fmt.Errorf("error fetching mercadolibre item %s: %w", externalID, err)
	}

	return s.pushCompatibilitiesForProduct(ctx, accessToken, domainDump, item, product, true), nil
}

// pushCompatibilitiesForProduct builds one products_families entry per
// (vehicle fitment × year) product is linked to via
// ecom_product_vehicle_compatibility and reports them against item on
// MercadoLibre, chunked to its per-request limits (see
// createFamilyEntriesBisecting). When unsupportedCategoryIsSkip is true, a
// category MercadoLibre's dump doesn't list as supporting compatibilities is
// reported as Skipped rather than an error (the new-listing path, where a
// non-autopart category is expected); fixOne passes false since an item
// already carrying the incomplete_compatibilities tag is supposed to resolve.
func (s *MercadoLibreCompatibilityService) pushCompatibilitiesForProduct(ctx context.Context, accessToken string, domainDump []mercadoLibreInfra.DomainDumpEntry, item *mercadoLibreInfra.ItemDetail, product *mysqlInfra.ProductDTO, unsupportedCategoryIsSkip bool) CompatibilityFixOutcome {
	externalID := strings.TrimSpace(item.ID)
	outcome := CompatibilityFixOutcome{ProductID: product.ID, SKU: product.SKU, ExternalID: externalID}

	compatibleDomainID, categoryEntry, err := resolveCompatibleDomain(domainDump, item.DomainID, item.CategoryID)
	if err != nil {
		if unsupportedCategoryIsSkip {
			outcome.Skipped = true
			outcome.SkipReason = "item category does not support vehicle compatibilities"
			return outcome
		}
		outcome.Error = err.Error()
		return outcome
	}

	// Every vehicle fitment this product is linked to — not just the one
	// tied to this particular listing's vehicle_fitment_id. On a connection
	// with allows_multiple_listings=true, the same SKU gets one listing per
	// fitment (e.g. one titled "... Toyota Prius", another "... Toyota
	// Corolla"), but MercadoLibre's compatibility data describes what the
	// physical part fits — a buyer with a Corolla should see it as
	// compatible on the Prius-titled listing too, since it's the same part.
	links, err := s.productVehicleCompatibilityRepository.FindByProductID(ctx, product.ID, 0, maxVehicleFitmentsPerFix)
	if err != nil {
		outcome.Error = fmt.Sprintf("error loading vehicle compatibilities: %v", err)
		return outcome
	}
	if len(links.Compatibilities) == 0 {
		outcome.Skipped = true
		outcome.SkipReason = "product has no vehicle compatibilities recorded"
		return outcome
	}

	familyEntries := make([]map[string]any, 0, len(links.Compatibilities))
	var unmappedTokens []string
	for _, link := range links.Compatibilities {
		fitment, err := s.vehicleFitmentsRepository.FindByID(ctx, link.VehicleFitmentID)
		if err != nil {
			log.Printf("mercadolibre compatibilities: error loading vehicle fitment %d for product %d: %v", link.VehicleFitmentID, product.ID, err)
			continue
		}

		brand, err := s.brandsRepository.FindByID(ctx, fitment.BrandID)
		if err != nil {
			log.Printf("mercadolibre compatibilities: error loading brand %d for fitment %d: %v", fitment.BrandID, fitment.ID, err)
			continue
		}

		// A fitment's own motor/position/side would be a single set shared
		// across its year range — position/side genuinely come from the
		// link row (link.Position/link.Side), same value applied to every
		// year in the fitment below.
		for _, year := range yearsInRange(fitment.YearStart, fitment.YearEnd) {
			familyAttrs := []map[string]string{
				{"id": "BRAND", "value_name": brand.Name},
				{"id": "CAR_AND_VAN_MODEL", "value_name": fitment.Model},
				{"id": "YEAR", "value_name": strconv.Itoa(year)},
			}

			familyEntry := map[string]any{
				"domain_id":       compatibleDomainID,
				"creation_source": mercadoLibreCompatibilityCreationSource,
				"attributes":      familyAttrs,
			}

			// Only attach a POSITION restriction when the category supports
			// it — sending one otherwise gets rejected. Not gated on
			// RestrictionsRequired: even when optional, including real
			// position data makes the compatibility more precise (e.g. front
			// pads don't get offered for a rear-only search).
			if categoryEntry.RestrictionsStatus == "ENABLED" {
				restriction, unmapped := buildPositionRestriction(link.Position, link.Side)
				unmappedTokens = append(unmappedTokens, unmapped...)
				if restriction != nil {
					familyEntry["restrictions"] = []map[string]any{restriction}
				}
			}

			familyEntries = append(familyEntries, familyEntry)
		}
	}
	outcome.UnmappedPositionTokens = unmappedTokens

	if len(familyEntries) == 0 {
		outcome.Error = "could not resolve any vehicle fitment for this product's compatibilities"
		return outcome
	}

	// MercadoLibre rejects a single POST .../compatibilities call with 400
	// ("products_families: size must be between 0 and 10") once
	// products_families exceeds 10 entries — easy to hit here since
	// familyEntries has one entry per (fitment × year), and a part compatible
	// with several models and/or a wide year range quickly adds up. POST is
	// additive (confirmed against MercadoLibre's docs: "Agrega
	// compatibilidades a un item"), not a replace, so splitting into chunks
	// of at most this size and calling it repeatedly is safe.
	const maxFamilyEntriesPerRequest = 10

	createdCount := 0
	for start := 0; start < len(familyEntries); start += maxFamilyEntriesPerRequest {
		end := start + maxFamilyEntriesPerRequest
		if end > len(familyEntries) {
			end = len(familyEntries)
		}

		created, chunkErr := s.createFamilyEntriesBisecting(ctx, accessToken, item, externalID, compatibleDomainID, familyEntries[start:end])
		createdCount += created
		if chunkErr != nil {
			outcome.Error = chunkErr.Error()
			outcome.CreatedCompatibilitiesCount = createdCount
			return outcome
		}
	}

	outcome.Success = true
	outcome.CreatedCompatibilitiesCount = createdCount
	return outcome
}

// createFamilyEntriesBisecting posts entries (at most maxFamilyEntriesPerRequest
// long, already enforced by the caller) and, if MercadoLibre rejects it with
// "Maximum of 200 products for a single request was exceeded" — because a
// single family (one brand/model/year combination) can itself expand into
// several catalog products (different engines/trims for that year), a count
// unknown to us ahead of time and unrelated to the products_families size
// limit — bisects entries and retries each half. Any other error is returned
// as-is. Returns how many compatibilities were actually created even when it
// ultimately returns an error, since earlier successful halves still count.
func (s *MercadoLibreCompatibilityService) createFamilyEntriesBisecting(ctx context.Context, accessToken string, item *mercadoLibreInfra.ItemDetail, externalID, compatibleDomainID string, entries []map[string]any) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}

	var createResp *mercadoLibreInfra.CreateCompatibilitiesResponse
	var err error
	if item.UserProductID != "" {
		payload := map[string]any{
			"domain_id":         compatibleDomainID,
			"category_id":       item.CategoryID,
			"products_families": entries,
		}
		createResp, err = s.compatibilitiesHandler.CreateUserProductCompatibilities(ctx, accessToken, item.UserProductID, payload)
	} else {
		payload := map[string]any{
			"products_families": entries,
		}
		createResp, err = s.compatibilitiesHandler.CreateItemCompatibilities(ctx, accessToken, externalID, payload)
	}
	if err == nil {
		return createResp.CreatedCompatibilitiesCount, nil
	}
	if len(entries) == 1 || !strings.Contains(err.Error(), "Maximum of 200 products") {
		return 0, err
	}

	mid := len(entries) / 2
	createdLeft, errLeft := s.createFamilyEntriesBisecting(ctx, accessToken, item, externalID, compatibleDomainID, entries[:mid])
	if errLeft != nil {
		return createdLeft, errLeft
	}
	createdRight, errRight := s.createFamilyEntriesBisecting(ctx, accessToken, item, externalID, compatibleDomainID, entries[mid:])
	return createdLeft + createdRight, errRight
}

// maxVehicleFitmentsPerFix bounds how many ecom_product_vehicle_compatibility
// rows are pooled for a single product when fixing one listing — a guard
// against a corrupted/runaway data set, not an expected limit in practice
// (mirrors channel_listings.maxCompatibilitiesPerSKU).
const maxVehicleFitmentsPerFix = 500

// maxYearsPerFitment bounds how many years a single fitment's yearStart..
// yearEnd range expands into — a sanity guard against bad data (e.g. a
// missing yearEnd defaulting to something absurd), not an expected limit for
// a real vehicle generation range.
const maxYearsPerFitment = 40

// yearsInRange expands a fitment's year_start/year_end into every individual
// year MercadoLibre's YEAR attribute needs one entry per — sending only
// year_start (as this flow used to) silently dropped every other year in the
// fitment's range from the reported compatibility.
func yearsInRange(start int, end *int) []int {
	if end == nil || *end < start {
		return []int{start}
	}

	last := *end
	if last-start+1 > maxYearsPerFitment {
		last = start + maxYearsPerFitment - 1
	}

	years := make([]int, 0, last-start+1)
	for year := start; year <= last; year++ {
		years = append(years, year)
	}
	return years
}

// resolveCompatibleDomain finds, within domainDump, the entry for
// partDomainID (the autopart's own domain, e.g. "MLM-VEHICLE_BRAKE_PADS")
// and returns the vehicle domain_id it's compatible with plus the category
// settings for categoryID. Only Type "EXTENSION" compatibility blocks
// support reporting compatibilities. Falls back to the block's first
// category if categoryID isn't listed explicitly (matches the single
// autopart category most domains expose).
func resolveCompatibleDomain(domainDump []mercadoLibreInfra.DomainDumpEntry, partDomainID, categoryID string) (string, *mercadoLibreInfra.DomainCategoryEntry, error) {
	for _, entry := range domainDump {
		if entry.DomainID != partDomainID {
			continue
		}
		for _, compat := range entry.Compatibilities {
			if compat.Type != "EXTENSION" {
				continue
			}
			for _, cat := range compat.Categories {
				if cat.ID == categoryID {
					catCopy := cat
					return compat.CompatibleDomainID, &catCopy, nil
				}
			}
			if len(compat.Categories) > 0 {
				catCopy := compat.Categories[0]
				return compat.CompatibleDomainID, &catCopy, nil
			}
		}
	}

	return "", nil, fmt.Errorf("domain %q (category %q) not found in mercadolibre's compatibility dump", partDomainID, categoryID)
}

// mapPositionTokens maps each already-lowercased token in tokens against
// mlPositionValuesByToken, returning the matched value_id/value_name pairs
// (deduplicated by value_id, order preserved) plus any tokens that didn't
// match.
func mapPositionTokens(tokens []string) (values []map[string]string, unmapped []string) {
	seenIDs := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		mapped, ok := mlPositionValuesByToken[token]
		if !ok {
			unmapped = append(unmapped, token)
			continue
		}
		if seenIDs[mapped.ID] {
			continue
		}
		seenIDs[mapped.ID] = true
		values = append(values, map[string]string{"value_id": mapped.ID, "value_name": mapped.Name})
	}
	return values, unmapped
}

// capPositionValues caps a single attribute_values group at MercadoLibre's
// limit of 4 ids, dropping any extra mapped tokens beyond the first four.
func capPositionValues(values []map[string]string) []map[string]string {
	if len(values) > 4 {
		return values[:4]
	}
	return values
}

// mlPositionOppositeAxes groups mlPositionValuesByToken's value_ids into
// axes where at most one value can describe a single physical spot — e.g. a
// part can't be simultaneously Delantera (front) and Trasera (back), or
// Izquierda (left) and Derecha (right). When position/side free text maps to
// more than one value from the same axis (e.g. the homologated "Derecha
// Izquierda" that replaced the old free-text "ambos", or "Trasera Inferior
// Delantera"), that means "fits either one" (OR), not "requires all of them
// at once" (impossible via AND) — see splitContradictoryPositionValues.
var mlPositionOppositeAxes = [][]string{
	{"13701104", "13701105"}, // Delantera, Trasera
	{"4774238", "4774239"},   // Superior, Inferior
	{"13373177", "13373178"}, // Interno, Externo
	{"2262158", "2262160"},   // Izquierda, Derecha
}

// splitContradictoryPositionValues expands values into one or more
// alternative groups (OR) whenever it holds more than one value from the
// same axis in mlPositionOppositeAxes — every value belonging to no
// conflicting axis is carried unchanged into every resulting group. Returns
// a single group (the original values, untouched) when there's no conflict
// at all, same shape as a plain AND restriction.
func splitContradictoryPositionValues(values []map[string]string) [][]map[string]string {
	consumed := make(map[int]bool, len(values))
	var conflictingAxisGroups [][]map[string]string

	for _, axis := range mlPositionOppositeAxes {
		axisIDs := make(map[string]bool, len(axis))
		for _, id := range axis {
			axisIDs[id] = true
		}

		var present []map[string]string
		var idxs []int
		for i, v := range values {
			if axisIDs[v["value_id"]] {
				present = append(present, v)
				idxs = append(idxs, i)
			}
		}

		if len(present) > 1 {
			conflictingAxisGroups = append(conflictingAxisGroups, present)
			for _, i := range idxs {
				consumed[i] = true
			}
		}
	}

	shared := make([]map[string]string, 0, len(values))
	for i, v := range values {
		if !consumed[i] {
			shared = append(shared, v)
		}
	}

	groups := [][]map[string]string{shared}
	for _, axisValues := range conflictingAxisGroups {
		next := make([][]map[string]string, 0, len(groups)*len(axisValues))
		for _, g := range groups {
			for _, v := range axisValues {
				combined := make([]map[string]string, len(g), len(g)+1)
				copy(combined, g)
				next = append(next, append(combined, v))
			}
		}
		groups = next
	}

	return groups
}

// buildPositionRestriction tokenizes position and side (free text, e.g.
// "trasero inferior delante", "Derecha Izquierda") on whitespace and maps
// each word against mlPositionValuesByToken, then expands any contradictory
// combination (see splitContradictoryPositionValues) into alternate
// attribute_values groups — MercadoLibre ANDs values within a single group
// (e.g. "Trasera"+"Izquierda" — one specific corner) and ORs separate
// groups within the same restriction. Unrecognized tokens are reported back
// but never invented a value for — a restriction is only returned when at
// least one token mapped. MercadoLibre caps a restriction group at 4 ids, so
// any extra mapped tokens beyond the first four (per group) are dropped.
func buildPositionRestriction(position, side string) (restriction map[string]any, unmapped []string) {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(position)))
	tokens = append(tokens, strings.Fields(strings.ToLower(strings.TrimSpace(side)))...)
	if len(tokens) == 0 {
		return nil, nil
	}

	values, unmapped := mapPositionTokens(tokens)
	if len(values) == 0 {
		return nil, unmapped
	}

	groups := splitContradictoryPositionValues(values)
	attributeValues := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		attributeValues = append(attributeValues, map[string]any{"values": capPositionValues(g)})
	}
	if len(attributeValues) == 0 {
		return nil, unmapped
	}

	return map[string]any{
		"attribute_id":     "POSITION",
		"attribute_values": attributeValues,
	}, unmapped
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

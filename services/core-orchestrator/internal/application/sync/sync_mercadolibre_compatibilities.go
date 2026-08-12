package sync

import (
	"context"
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

	// defaultFixCompatibilitiesLimit/maxFixCompatibilitiesLimit bound how
	// many ecom_channel_product_map rows FixUnderReviewListings processes
	// per call. MercadoLibre's shared rate limiter budgets 10 requests/min
	// for the whole app, and each row costs ~2 calls (GetItem + create) —
	// a large limit would make a single HTTP call run for many minutes.
	// Callers with a bigger backlog call this endpoint again to continue.
	defaultFixCompatibilitiesLimit = 10
	maxFixCompatibilitiesLimit     = 50
)

var ErrNotMercadoLibreConnection = errors.New("connection's channel is not MERCADOLIBRE")

// mlPositionValue is one entry of MercadoLibre's closed POSITION restriction
// value set. Confirmed for real via
// GET /catalog_compatibilities/restrictions/values against
// MLM-CARS_AND_VANS_FOR_COMPATIBILITIES / MLM-VEHICLE_BRAKE_PADS — this
// dictionary is deliberately not guessed beyond that.
type mlPositionValue struct{ ID, Name string }

var mlPositionValuesByToken = map[string]mlPositionValue{
	"delantero":   {"13701104", "Delantera"},
	"delantera":   {"13701104", "Delantera"},
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
	// Limit caps how many under_review rows are processed in this call.
	// Defaults to defaultFixCompatibilitiesLimit when <= 0, capped at
	// maxFixCompatibilitiesLimit.
	Limit int
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

// FixUnderReviewListings finds up to input.Limit ecom_channel_product_map
// rows for input.ConnectionID (every status except closed — see the filter
// below) with a vehicle fitment attached, and reports each one's
// compatibility data to MercadoLibre when the real item still carries the
// incomplete_compatibilities tag. Checking synced/paused listings too, not
// only ones already under_review, catches the tag while it's still just a
// warning on an otherwise-active listing — before MercadoLibre pauses it for
// this reason. It never touches ecom_channel_product_map.status itself —
// RefreshChannelProductMapStatus (see mercadolibre_handler.go) is what polls
// MercadoLibre for the real status afterward, since MercadoLibre processes a
// compatibility submission asynchronously.
func (s *MercadoLibreCompatibilityService) FixUnderReviewListings(ctx context.Context, input FixCompatibilitiesInput) (*FixCompatibilitiesResult, error) {
	if input.ConnectionID <= 0 {
		return nil, ErrInvalidMercadoLibreConnection
	}

	limit := input.Limit
	if limit <= 0 {
		limit = defaultFixCompatibilitiesLimit
	}
	if limit > maxFixCompatibilitiesLimit {
		limit = maxFixCompatibilitiesLimit
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

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, input.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("error getting mercadolibre access token: %w", err)
	}

	allMappings, err := s.channelProductMapRepository.FindAllByConnectionID(input.ConnectionID)
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
	// listing to check at all.
	candidates := make([]mysqlInfra.ChannelProductMapDTO, 0, limit)
	for _, mapping := range allMappings {
		if mapping.Status == mercadoLibreMapStatusClosed || mapping.VehicleFitmentID == nil || mapping.ExternalID == nil {
			continue
		}
		candidates = append(candidates, mapping)
		if len(candidates) >= limit {
			break
		}
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

func (s *MercadoLibreCompatibilityService) fixOne(ctx context.Context, accessToken string, domainDump []mercadoLibreInfra.DomainDumpEntry, mapping mysqlInfra.ChannelProductMapDTO) CompatibilityFixOutcome {
	externalID := strings.TrimSpace(*mapping.ExternalID)
	outcome := CompatibilityFixOutcome{ProductID: mapping.ProductID, ExternalID: externalID}

	product, err := s.productRepository.FindByID(mapping.ProductID)
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

	compatibleDomainID, categoryEntry, err := resolveCompatibleDomain(domainDump, item.DomainID, item.CategoryID)
	if err != nil {
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
	links, err := s.productVehicleCompatibilityRepository.FindByProductID(mapping.ProductID, 0, maxVehicleFitmentsPerFix)
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
		fitment, err := s.vehicleFitmentsRepository.FindByID(link.VehicleFitmentID)
		if err != nil {
			log.Printf("mercadolibre compatibilities: error loading vehicle fitment %d for product %d: %v", link.VehicleFitmentID, mapping.ProductID, err)
			continue
		}

		brand, err := s.brandsRepository.FindByID(fitment.BrandID)
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

	var createResp *mercadoLibreInfra.CreateCompatibilitiesResponse
	if item.UserProductID != "" {
		payload := map[string]any{
			"domain_id":         compatibleDomainID,
			"category_id":       item.CategoryID,
			"products_families": familyEntries,
		}
		createResp, err = s.compatibilitiesHandler.CreateUserProductCompatibilities(ctx, accessToken, item.UserProductID, payload)
	} else {
		payload := map[string]any{
			"products_families": familyEntries,
		}
		createResp, err = s.compatibilitiesHandler.CreateItemCompatibilities(ctx, accessToken, externalID, payload)
	}
	if err != nil {
		outcome.Error = err.Error()
		return outcome
	}

	outcome.Success = true
	outcome.CreatedCompatibilitiesCount = createResp.CreatedCompatibilitiesCount
	return outcome
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

// buildPositionRestriction tokenizes position/side (free text, e.g.
// "trasero inferior delante", "izquierdo") on whitespace and maps each word
// against mlPositionValuesByToken. side "ambos" is treated as "no side
// restriction" (applies regardless of side) rather than guessed into a
// specific value. Unrecognized tokens are reported back but never invented a
// value for — a restriction is only returned when at least one token
// mapped. MercadoLibre caps a restriction group at 4 ids, so any extra
// mapped tokens beyond the first four are dropped.
func buildPositionRestriction(position, side string) (restriction map[string]any, unmapped []string) {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(position)))
	trimmedSide := strings.ToLower(strings.TrimSpace(side))
	if trimmedSide != "" && trimmedSide != "ambos" {
		tokens = append(tokens, strings.Fields(trimmedSide)...)
	}
	if len(tokens) == 0 {
		return nil, nil
	}

	values := make([]map[string]string, 0, len(tokens))
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

	if len(values) == 0 {
		return nil, unmapped
	}
	if len(values) > 4 {
		values = values[:4]
	}

	return map[string]any{
		"attribute_id": "POSITION",
		"attribute_values": []map[string]any{
			{"values": values},
		},
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

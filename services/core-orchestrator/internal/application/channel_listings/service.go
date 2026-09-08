// Package channel_listings publishes brand-new marketplace listings for a
// batch of SKUs against a single channel connection, dispatching to
// whichever Publisher is registered for that connection's channel code
// (ecom_channels.code). The same publish path is driven two ways: directly by
// the CreateListings endpoint, and one (product, connection) pair at a time
// by workers.MarketplaceWorker via PublishQueuedProduct, for the rows
// ListingDiscoveryScheduler enqueues. It only ever creates listings: a
// (product, connection, vehicle fitment) combination that already has a row
// in ecom_channel_product_map is skipped, never updated — except when that
// row's status is channelProductMapStatusClosed (see publishOne), which is
// treated as if no listing existed at all, so a permanently dead listing
// (e.g. MercadoLibre "under_review"/"forbidden", folded to "closed" by
// application/sync's foldMercadoLibreStatus) can be republished. It is
// entirely independent from the MercadoLibre Upload/UpdatePricesAndStock
// flows in application/sync.
package channel_listings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// maxCompatibilitiesPerSKU bounds how many vehicle compatibilities are
// fanned out into listings for a single SKU in one call.
const maxCompatibilitiesPerSKU = 500

// nissanBrandID is ecom_brands.id for "Nissan" in this deployment's seed data
// — hardcoded rather than resolved by name/code, mirroring application/sync's
// identically-named constant. It is the one exception to per-vehicle-fitment
// listing fan-out: a Nissan-branded product always publishes as a single
// general listing, even on a connection with allows_multiple_listings=true
// (see publishReady).
const nissanBrandID int64 = 1

// maxSuccessionChainNodes bounds how many part numbers resolveSuccessionChain
// will visit walking ecom_part_number_supersessions (both directions) from a
// starting product, guarding against a corrupted/cyclic chain in the data.
const maxSuccessionChainNodes = 50

const (
	reasonNoStock = "product has no available stock"
	reasonNoImage = "product has no cover image (ecom_product_images row with is_first=true)"
)

// channelProductMapStatusClosed mirrors ecom_channel_product_map.status'
// "closed" enum value (see application/sync's mercadoLibreMapStatusClosed —
// duplicated here rather than imported, since sync itself imports this
// package to implement Publisher and importing it back would cycle) —
// stamped on a listing MercadoLibre reports as permanently dead (e.g.
// "under_review" with sub_status "forbidden") that will never recover
// through a normal refresh. publishOne treats a row in this status as if it
// didn't exist, so CreateListings can create a fresh listing for it instead
// of skipping forever.
const channelProductMapStatusClosed = "closed"

var (
	ErrEmptyChannelListingsInput = errors.New("connectionId and at least one sku are required")
	ErrInvalidChannelConnection  = errors.New("invalid connectionId")
	ErrNoPublisherForChannel     = errors.New("no listing publisher registered for this connection's channel")
	ErrNoRefresherForChannel     = errors.New("no listing refresher registered for this connection's channel")
	// ErrChannelListingsNotReady is returned by PublishQueuedProduct, wrapped
	// with reasonNoStock, when the product has lost its stock between being
	// enqueued and being processed.
	ErrChannelListingsNotReady = errors.New("product still not ready to publish")
)

// Publisher is implemented by each marketplace-specific sync service
// (MercadoLibre, Odoo today; others later, duck-typed — no import of this
// package is needed on their side). Publish always creates a brand-new
// external listing titled title for product, against vehicleFitmentID (nil
// for a general listing with no vehicle compatibility), and records it in
// ecom_channel_product_map. imageSourceProductID is which product's images to
// list under (see resolveImageSource): equal to product.ID unless product has
// no cover image of its own and a product connected to it through
// ecom_part_number_supersessions does. officialStoreID is passed through
// as-is (nil included) to channels that support it — MercadoLibre — and
// ignored by ones that don't (Odoo). missingRequiredAttributes/
// missingOptionalAttributes report, on success, any channel-tracked attribute
// (MercadoLibre's ecom_channel_attributes slots today — see
// channel_attribute_values.Service.ProvisionCategoryAttributes) the product
// had no value for at publish time, split by whether the channel marks that
// attribute mandatory (is_required) or merely available to improve listing
// quality; a channel that doesn't track this (Odoo) always returns nil for
// both. Callers are responsible for checking whether a listing already
// exists before calling Publish — it never checks itself and never falls
// back to an update.
type Publisher interface {
	Publish(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, title string, vehicleFitmentID *int64, imageSourceProductID int64, officialStoreID *int64) (externalID string, missingRequiredAttributes []string, missingOptionalAttributes []string, err error)
}

// Refresher is implemented by each marketplace-specific sync service
// (MercadoLibre, Odoo today — the same struct that implements Publisher, duck-
// typed the same way). Refresh pushes product's current price/stock (plus
// whatever else that channel wants kept in lockstep on every push — e.g.
// MercadoLibre's category, Odoo's normalized name) to listing's already-
// published external id, but only actually calls out to the marketplace when
// something changed; it reports back whether it did (changed) so
// RefreshListings can tell a real update apart from a no-op skip.
type Refresher interface {
	Refresh(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO) (changed bool, err error)
}

// StatusSyncer is optionally implemented by a channel's registered Refresher
// (duck-typed, same as Publisher/Refresher — no import cycle) to pull each
// listing's live status/category from the marketplace before RefreshListings
// walks ecom_channel_product_map, so the price/stock push that follows sees
// up-to-date state and correctly leaves alone a listing the marketplace
// itself has since put under review/paused (see Refresh/isPausedOrUnderReview
// on the MercadoLibre side, and
// MercadoLibreProductSyncService.RefreshChannelProductMapStatusAndCategory,
// the only implementation of this today). A channel with nothing equivalent
// (Odoo) simply doesn't implement it, so RefreshListings skips the step for
// it.
type StatusSyncer interface {
	SyncListingStatus(ctx context.Context, connectionID int64) error
}

// ListingOutcome reports what happened for one (sku, vehicleFitmentId)
// combination. A batch never fails wholesale on one bad entry.
type ListingOutcome struct {
	SKU              string `json:"sku"`
	VehicleFitmentID *int64 `json:"vehicleFitmentId,omitempty"`
	Title            string `json:"title,omitempty"`
	Success          bool   `json:"success"`
	Skipped          bool   `json:"skipped,omitempty"`
	ExternalID       string `json:"externalId,omitempty"`
	// MissingRequiredAttributes lists the marketplace-required attribute
	// external keys (MercadoLibre's ecom_channel_attributes slots with
	// is_required=true) the product had no value for at publish time. The
	// listing is still created — the marketplace itself decides whether to
	// reject it for a missing mandatory attribute — this is purely
	// informational, so the caller knows exactly which ones to fill in (e.g.
	// via /api/marketplaces/mercadolibre/product-attributes) and retry.
	MissingRequiredAttributes []string `json:"missingRequiredAttributes,omitempty"`
	// MissingOptionalAttributes is the same, for attributes the channel
	// exposes but doesn't require (is_required=false) — filling these in
	// doesn't unblock anything, it only improves the listing's exposure/
	// completeness on the marketplace.
	MissingOptionalAttributes []string `json:"missingOptionalAttributes,omitempty"`
	Error                     string   `json:"error,omitempty"`
}

type CreateListingsInput struct {
	ConnectionID int64
	SKUs         []string
	// OfficialStoreID is forwarded to every Publisher.Publish call in this
	// batch — nil (the default) publishes with no official store assigned;
	// a value assigns every created listing to that store. Channels that
	// don't support the concept (Odoo) ignore it.
	OfficialStoreID *int64
}

type CreateListingsResult struct {
	Results []ListingOutcome `json:"results"`
}

// Service resolves a connection's channel and dispatches to the Publisher
// registered for it. Register additional marketplaces by calling Register
// with their ecom_channels.code — the batch/dispatch logic never changes.
type Service struct {
	db                                    *sql.DB
	productRepository                     *mysqlInfra.ProductRepository
	productImagesRepository               *mysqlInfra.ProductImagesRepository
	productStockRepository                *mysqlInfra.ProductStockRepository
	brandsRepository                      *mysqlInfra.BrandsRepository
	channelRepository                     *mysqlInfra.ChannelRepository
	channelConnectionRepository           *mysqlInfra.ChannelConnectionRepository
	channelProductMapRepository           *mysqlInfra.ChannelProductMapRepository
	productVehicleCompatibilityRepository *mysqlInfra.ProductVehicleCompatibilityRepository
	vehicleFitmentsRepository             *mysqlInfra.VehicleFitmentsRepository
	partNumberSupersessionsRepository     *mysqlInfra.PartNumberSupersessionsRepository
	publishers                            map[string]Publisher
	refreshers                            map[string]Refresher
}

func NewService(
	db *sql.DB,
	productRepository *mysqlInfra.ProductRepository,
	productImagesRepository *mysqlInfra.ProductImagesRepository,
	productStockRepository *mysqlInfra.ProductStockRepository,
	brandsRepository *mysqlInfra.BrandsRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	channelProductMapRepository *mysqlInfra.ChannelProductMapRepository,
	productVehicleCompatibilityRepository *mysqlInfra.ProductVehicleCompatibilityRepository,
	vehicleFitmentsRepository *mysqlInfra.VehicleFitmentsRepository,
	partNumberSupersessionsRepository *mysqlInfra.PartNumberSupersessionsRepository,
) *Service {
	return &Service{
		db:                                    db,
		productRepository:                     productRepository,
		productImagesRepository:               productImagesRepository,
		productStockRepository:                productStockRepository,
		brandsRepository:                      brandsRepository,
		channelRepository:                     channelRepository,
		channelConnectionRepository:           channelConnectionRepository,
		channelProductMapRepository:           channelProductMapRepository,
		productVehicleCompatibilityRepository: productVehicleCompatibilityRepository,
		vehicleFitmentsRepository:             vehicleFitmentsRepository,
		partNumberSupersessionsRepository:     partNumberSupersessionsRepository,
		publishers:                            make(map[string]Publisher),
		refreshers:                            make(map[string]Refresher),
	}
}

// Register associates a Publisher with a channel code (ecom_channels.code,
// matched case-insensitively).
func (s *Service) Register(channelCode string, publisher Publisher) {
	s.publishers[normalizeChannelCode(channelCode)] = publisher
}

// RegisterRefresher associates a Refresher with a channel code
// (ecom_channels.code, matched case-insensitively) — separate from Register/
// Publisher since not every channel that can create listings necessarily
// supports refreshing them, though MercadoLibre and Odoo's sync services
// implement both today.
func (s *Service) RegisterRefresher(channelCode string, refresher Refresher) {
	s.refreshers[normalizeChannelCode(channelCode)] = refresher
}

// CreateListings cleans input.SKUs (trims/strips whitespace, drops
// duplicates and blanks), resolves input.ConnectionID's channel, and creates
// one new listing per SKU — or, on a connection with
// allows_multiple_listings=true, one new listing per vehicle compatibility
// the product has, titled "<product name> <vehicle brand> <model>
// <yearStart>-<yearEnd>". Products with no vehicle compatibility at all
// (apparel, accessories — anything not tied to a specific vehicle) always
// get a single listing titled "<product name> <product brand>", even on a
// connection that allows multiple listings. Every SKU is required to have a
// usable cover image (its own, or borrowed from another product in its
// succession chain — see resolveImageSource) before it is processed at all.
//
// Each cleaned SKU is expanded into its full succession chain (see
// resolveSuccessionChain) — every product connected to it through
// ecom_part_number_supersessions, in both directions — and every distinct
// product across every SKU's chain is processed independently: one SKU with
// no stock is reported as not-ready while another in the same chain that does
// have stock still gets published. On a connection that allows multiple
// listings, the vehicle compatibilities considered for each chain member are
// the union across the whole chain, not just that member's own (see
// pooledCompatibilityFitmentIDs), so a superseded/superseding sibling's
// compatibilities are picked up too. A product reached through more than one
// input SKU's chain (or explicitly listed twice) is only ever processed
// once.
func (s *Service) CreateListings(ctx context.Context, input CreateListingsInput) (*CreateListingsResult, error) {
	skus := cleanSKUs(input.SKUs)
	if input.ConnectionID <= 0 || len(skus) == 0 {
		return nil, ErrEmptyChannelListingsInput
	}

	connection, publisher, err := s.resolveConnectionPublisher(ctx, input.ConnectionID)
	if err != nil {
		return nil, err
	}

	results := make([]ListingOutcome, 0, len(skus))
	processedProductIDs := make(map[int64]bool, len(skus))

	for _, sku := range skus {
		product, err := s.productRepository.FindBySKU(ctx, sku)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrProductNotFound) {
				results = append(results, ListingOutcome{SKU: sku, Error: "product not found for sku"})
				continue
			}
			results = append(results, ListingOutcome{SKU: sku, Error: fmt.Sprintf("error loading product: %v", err)})
			continue
		}
		if processedProductIDs[product.ID] {
			continue
		}

		chain, err := s.resolveSuccessionChain(ctx, product)
		if err != nil {
			results = append(results, ListingOutcome{SKU: sku, Error: fmt.Sprintf("error resolving succession chain: %v", err)})
			continue
		}

		for _, member := range chain {
			if processedProductIDs[member.ID] {
				continue
			}
			processedProductIDs[member.ID] = true
			results = append(results, s.processChainMember(ctx, publisher, connection, member, chain, input.OfficialStoreID)...)
		}
	}

	return &CreateListingsResult{Results: results}, nil
}

// PublishQueuedProduct publishes every listing for productID on connectionID —
// the entry point the marketplace consumer (internal/workers.MarketplaceWorker)
// calls for each 'listing' row ListingDiscoveryScheduler enqueued. It
// re-validates only stock live: everything else (price, cover image,
// category, brand) was checked when the row was enqueued and doesn't
// realistically change in the poll window; if it did, the publish call itself
// fails and the caller marks the row failed. When stock has dropped to 0 it
// returns ErrChannelListingsNotReady so the caller releases the *same* queue
// row back to pending instead of failing it. It resolves productID's
// succession chain the same way CreateListings does (so a cover image
// borrowed from a chain sibling and pooled compatibilities both apply), but
// only ever publishes productID itself, never fanning out to the rest of the
// chain.
func (s *Service) PublishQueuedProduct(ctx context.Context, productID, connectionID int64) (*CreateListingsResult, error) {
	product, err := s.productRepository.FindByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("error loading product %d: %w", productID, err)
	}

	connection, publisher, err := s.resolveConnectionPublisher(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	availableStock, err := s.sumAvailableStock(ctx, product.ID)
	if err != nil {
		return nil, fmt.Errorf("error checking product stock: %w", err)
	}
	if availableStock <= 0 {
		return nil, fmt.Errorf("%w: %s", ErrChannelListingsNotReady, reasonNoStock)
	}

	chain, err := s.resolveSuccessionChain(ctx, product)
	if err != nil {
		return nil, fmt.Errorf("error resolving succession chain for product %d: %w", product.ID, err)
	}

	// The cover image was verified at enqueue time; it isn't re-gated here.
	// resolveImageSource still runs to pick which product's images to list
	// under (its own, or a chain sibling's) — if somehow none has one now,
	// fall back to the product itself and let publisher.Publish fail, so the
	// row is marked failed rather than silently released.
	imageSourceProductID, hasImage, err := s.resolveImageSource(ctx, product, chain)
	if err != nil {
		return nil, fmt.Errorf("error checking product images: %w", err)
	}
	if !hasImage {
		imageSourceProductID = product.ID
	}

	// officialStoreID is always nil here: a queued publish has no access to an
	// original CreateListings request (there isn't one — the row came from the
	// discovery scan).
	return &CreateListingsResult{Results: s.publishReady(ctx, publisher, connection, product, chain, imageSourceProductID, nil)}, nil
}

// resolveConnectionPublisher loads connectionID, its channel, and the
// Publisher registered for that channel's code — shared by CreateListings
// and PublishQueuedProduct.
func (s *Service) resolveConnectionPublisher(ctx context.Context, connectionID int64) (*mysqlInfra.ChannelConnectionDTO, Publisher, error) {
	connection, err := s.channelConnectionRepository.FindByID(ctx, connectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return nil, nil, ErrInvalidChannelConnection
		}
		return nil, nil, fmt.Errorf("error loading connection %d: %w", connectionID, err)
	}

	channel, err := s.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}

	publisher, ok := s.publishers[normalizeChannelCode(channel.Code)]
	if !ok {
		return nil, nil, fmt.Errorf("%w: %s", ErrNoPublisherForChannel, channel.Code)
	}

	return connection, publisher, nil
}

// RefreshListingsInput carries the connection whose already-published
// listings should be checked for a price/stock change and pushed if so.
type RefreshListingsInput struct {
	ConnectionID int64
}

// RefreshOutcome reports what happened for one ecom_channel_product_map row.
type RefreshOutcome struct {
	ProductID  int64  `json:"productId"`
	SKU        string `json:"sku,omitempty"`
	ExternalID string `json:"externalId,omitempty"`
	// Updated is true only when the refresher actually found a price/stock
	// change and pushed it; Skipped covers everything else that isn't an
	// Error (no change detected, no external id yet, or a status like
	// paused/under_review the refresher itself leaves alone).
	Updated bool   `json:"updated,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
	Error   string `json:"error,omitempty"`
}

type RefreshListingsResult struct {
	Results []RefreshOutcome `json:"results"`
}

// RefreshListings first gives the connection's channel a chance to pull live
// status/category from the marketplace (see StatusSyncer — MercadoLibre only,
// today), then reads every ecom_channel_product_map row already recorded for
// input.ConnectionID and dispatches each one to the Refresher registered for
// that connection's channel (see RegisterRefresher). A row is left alone
// unless its refresher independently decides the product's price or stock
// actually changed since the row's last_synced_at — RefreshListings itself
// runs no diffing of its own, since what counts as "changed enough to call
// the marketplace" (and what else to bundle into that same call — attributes
// for MercadoLibre, name for Odoo) is channel-specific. One row failing
// doesn't stop the rest of the batch.
func (s *Service) RefreshListings(ctx context.Context, input RefreshListingsInput) (*RefreshListingsResult, error) {
	if input.ConnectionID <= 0 {
		return nil, ErrInvalidChannelConnection
	}

	connection, err := s.channelConnectionRepository.FindByID(ctx, input.ConnectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return nil, ErrInvalidChannelConnection
		}
		return nil, fmt.Errorf("error loading connection %d: %w", input.ConnectionID, err)
	}

	channel, err := s.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}

	refresher, ok := s.refreshers[normalizeChannelCode(channel.Code)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoRefresherForChannel, channel.Code)
	}

	if syncer, ok := refresher.(StatusSyncer); ok {
		if err := syncer.SyncListingStatus(ctx, input.ConnectionID); err != nil {
			return nil, fmt.Errorf("error syncing listing status for connection %d: %w", input.ConnectionID, err)
		}
	}

	listings, err := s.channelProductMapRepository.FindAllByConnectionID(ctx, input.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel product map for connection %d: %w", input.ConnectionID, err)
	}

	results := make([]RefreshOutcome, 0, len(listings))
	for i := range listings {
		results = append(results, s.refreshOne(ctx, refresher, &listings[i]))
	}

	return &RefreshListingsResult{Results: results}, nil
}

// refreshOne loads listing's product and delegates to refresher.Refresh —
// shared by every row RefreshListings processes so one bad row (product
// missing, refresher error) never stops the batch.
func (s *Service) refreshOne(ctx context.Context, refresher Refresher, listing *mysqlInfra.ChannelProductMapDTO) RefreshOutcome {
	outcome := RefreshOutcome{ProductID: listing.ProductID}

	externalID := ""
	if listing.ExternalID != nil {
		externalID = strings.TrimSpace(*listing.ExternalID)
	}
	if externalID == "" {
		outcome.Skipped = true
		return outcome
	}
	outcome.ExternalID = externalID

	product, err := s.productRepository.FindByID(ctx, listing.ProductID)
	if err != nil {
		outcome.Error = fmt.Sprintf("error loading product: %v", err)
		return outcome
	}
	outcome.SKU = product.SKU

	updated, err := refresher.Refresh(ctx, product, listing.ConnectionID, listing)
	if err != nil {
		outcome.Error = err.Error()
		return outcome
	}

	outcome.Updated = updated
	outcome.Skipped = !updated
	return outcome
}

// resolveSuccessionChain returns product and every other product connected
// to it through ecom_part_number_supersessions, walking the chain in both
// directions (a part product itself superseded, and any part superseding
// product) breadth-first until no further link is found or
// maxSuccessionChainNodes distinct part numbers have been visited — a guard
// against a corrupted/cyclic chain in the data, not an expected limit in
// practice. A visited part number with no ecom_products row yet (still
// unresolved — see ecom_part_number_supersessions.old_product_id/
// new_product_id) is skipped rather than stopping the walk: the string chain
// in ecom_part_number_supersessions doesn't require every link to already
// have a product. The returned slice always has product itself as its first
// element.
func (s *Service) resolveSuccessionChain(ctx context.Context, product *mysqlInfra.ProductDTO) ([]*mysqlInfra.ProductDTO, error) {
	visited := map[string]bool{product.PartNumber: true}
	queue := []string{product.PartNumber}
	chain := []*mysqlInfra.ProductDTO{product}

	for len(queue) > 0 && len(visited) < maxSuccessionChainNodes {
		current := queue[0]
		queue = queue[1:]

		var linkedPartNumbers []string

		forward, err := s.partNumberSupersessionsRepository.FindByOldPartNumber(ctx, product.SourceID, current)
		if err != nil && !errors.Is(err, mysqlInfra.ErrPartNumberSupersessionNotFound) {
			return nil, fmt.Errorf("error looking up successor for part number %s: %w", current, err)
		}
		if forward != nil {
			linkedPartNumbers = append(linkedPartNumbers, forward.NewPartNumber)
		}

		backward, err := s.partNumberSupersessionsRepository.FindByNewPartNumber(ctx, product.SourceID, current)
		if err != nil {
			return nil, fmt.Errorf("error looking up predecessors for part number %s: %w", current, err)
		}
		for _, row := range backward {
			linkedPartNumbers = append(linkedPartNumbers, row.OldPartNumber)
		}

		for _, partNumber := range linkedPartNumbers {
			if visited[partNumber] {
				continue
			}
			visited[partNumber] = true
			queue = append(queue, partNumber)

			// ecom_products no tiene unique key sobre (source_id, part_number) — más de un
			// producto puede compartir el mismo part_number dentro del source, y todos
			// cuentan como miembros de esta cadena de sucesión (a diferencia de
			// FindBySourceAndPartNumber, que solo devolvería uno arbitrario).
			members, err := s.productRepository.FindAllBySourceAndPartNumber(ctx, product.SourceID, partNumber)
			if err != nil {
				return nil, fmt.Errorf("error loading products for part number %s: %w", partNumber, err)
			}
			for i := range members {
				chain = append(chain, &members[i])
			}
		}
	}

	return chain, nil
}

// resolveImageSource decides which product's images member's listing should
// use: member's own, if it already has a cover image (ecom_product_images
// row with is_first=true); otherwise the first other product in chain (its
// succession chain) that has one, in chain order. hasImage is false only
// when neither member nor any of its chain siblings has a cover image, in
// which case imageSourceProductID is meaningless and must not be used.
func (s *Service) resolveImageSource(ctx context.Context, member *mysqlInfra.ProductDTO, chain []*mysqlInfra.ProductDTO) (imageSourceProductID int64, hasImage bool, err error) {
	ownCover, err := s.hasCoverImage(ctx, member.ID)
	if err != nil {
		return 0, false, err
	}
	if ownCover {
		return member.ID, true, nil
	}

	for _, sibling := range chain {
		if sibling.ID == member.ID {
			continue
		}
		siblingCover, err := s.hasCoverImage(ctx, sibling.ID)
		if err != nil {
			return 0, false, err
		}
		if siblingCover {
			return sibling.ID, true, nil
		}
	}

	return 0, false, nil
}

// processChainMember validates member (has stock, has a usable cover image —
// its own or borrowed from another product in chain, see resolveImageSource)
// and fans it out into one or more listing outcomes per the multi-listing/
// compatibility rules described on CreateListings. A product with no available
// stock, or with no usable cover image anywhere in chain, is not published —
// the outcome just carries the reason in Error. It is not enqueued for retry:
// ListingDiscoveryScheduler picks the product up on its own once it is ready.
func (s *Service) processChainMember(ctx context.Context, publisher Publisher, connection *mysqlInfra.ChannelConnectionDTO, member *mysqlInfra.ProductDTO, chain []*mysqlInfra.ProductDTO, officialStoreID *int64) []ListingOutcome {
	availableStock, err := s.sumAvailableStock(ctx, member.ID)
	if err != nil {
		return []ListingOutcome{{SKU: member.SKU, Error: fmt.Sprintf("error checking product stock: %v", err)}}
	}
	if availableStock <= 0 {
		return []ListingOutcome{{SKU: member.SKU, Error: reasonNoStock}}
	}

	imageSourceProductID, hasImage, err := s.resolveImageSource(ctx, member, chain)
	if err != nil {
		return []ListingOutcome{{SKU: member.SKU, Error: fmt.Sprintf("error checking product images: %v", err)}}
	}
	if !hasImage {
		return []ListingOutcome{{SKU: member.SKU, Error: reasonNoImage}}
	}

	return s.publishReady(ctx, publisher, connection, member, chain, imageSourceProductID, officialStoreID)
}

// publishReady builds and publishes every listing for product on
// connection, once stock and a usable cover image are already confirmed
// (product's own, or imageSourceProductID naming another product in chain —
// see resolveImageSource) — shared by processChainMember (first attempt,
// called right after its own gate checks pass) and PublishQueuedProduct
// (the marketplace consumer, once it has re-confirmed stock is still there).
// On a connection that allows multiple listings,
// the vehicle compatibilities considered are the union across every product
// in chain (see pooledCompatibilityFitmentIDs) — not just product's own — so
// a superseded/superseding sibling's compatibilities are picked up too: two
// vehicles on the old part plus one different vehicle on the new part means
// three listings for each of them, provided each individually clears the
// stock/image gate.
func (s *Service) publishReady(ctx context.Context, publisher Publisher, connection *mysqlInfra.ChannelConnectionDTO, product *mysqlInfra.ProductDTO, chain []*mysqlInfra.ProductDTO, imageSourceProductID int64, officialStoreID *int64) []ListingOutcome {
	if product.BrandID == nil {
		return []ListingOutcome{{SKU: product.SKU, Error: "product has no brand"}}
	}

	// Nissan is the one brand that never fans out into per-fitment listings:
	// even on an allows_multiple_listings connection it publishes as a single
	// general listing (see nissanBrandID).
	if !connection.AllowsMultipleListings || *product.BrandID == nissanBrandID {
		return []ListingOutcome{s.publishOne(ctx, publisher, product, connection.ID, generalListingTitle(product.Name), nil, imageSourceProductID, officialStoreID)}
	}

	fitmentIDs, err := s.pooledCompatibilityFitmentIDs(ctx, chain)
	if err != nil {
		return []ListingOutcome{{SKU: product.SKU, Error: fmt.Sprintf("error loading vehicle compatibilities: %v", err)}}
	}

	if len(fitmentIDs) == 0 {
		return []ListingOutcome{s.publishOne(ctx, publisher, product, connection.ID, generalListingTitle(product.Name), nil, imageSourceProductID, officialStoreID)}
	}

	outcomes := make([]ListingOutcome, 0, len(fitmentIDs))
	for _, fitmentID := range fitmentIDs {
		fitmentID := fitmentID

		fitment, err := s.vehicleFitmentsRepository.FindByID(ctx, fitmentID)
		if err != nil {
			outcomes = append(outcomes, ListingOutcome{SKU: product.SKU, VehicleFitmentID: &fitmentID, Error: fmt.Sprintf("error loading vehicle fitment %d: %v", fitmentID, err)})
			continue
		}

		fitmentBrand, err := s.brandsRepository.FindByID(ctx, fitment.BrandID)
		if err != nil {
			outcomes = append(outcomes, ListingOutcome{SKU: product.SKU, VehicleFitmentID: &fitmentID, Error: fmt.Sprintf("error loading vehicle fitment brand %d: %v", fitment.BrandID, err)})
			continue
		}

		title := fitmentListingTitle(product.Name, fitmentBrand.Name, fitment)
		outcomes = append(outcomes, s.publishOne(ctx, publisher, product, connection.ID, title, &fitmentID, imageSourceProductID, officialStoreID))
	}

	return outcomes
}

// pooledCompatibilityFitmentIDs returns the union of vehicle_fitment_id
// across every product in chain, deduplicated, in first-seen (chain) order.
func (s *Service) pooledCompatibilityFitmentIDs(ctx context.Context, chain []*mysqlInfra.ProductDTO) ([]int64, error) {
	seen := make(map[int64]bool)
	fitmentIDs := make([]int64, 0)

	for _, member := range chain {
		compatibilities, err := s.productVehicleCompatibilityRepository.FindByProductID(ctx, member.ID, 0, maxCompatibilitiesPerSKU)
		if err != nil {
			return nil, err
		}
		for _, compatibility := range compatibilities.Compatibilities {
			if seen[compatibility.VehicleFitmentID] {
				continue
			}
			seen[compatibility.VehicleFitmentID] = true
			fitmentIDs = append(fitmentIDs, compatibility.VehicleFitmentID)
		}
	}

	return fitmentIDs, nil
}

// publishOne skips (product, connection, vehicleFitmentID) if it already has
// a row in ecom_channel_product_map — this flow only ever creates — and
// otherwise delegates to publisher.Publish.
func (s *Service) publishOne(ctx context.Context, publisher Publisher, product *mysqlInfra.ProductDTO, connectionID int64, title string, vehicleFitmentID *int64, imageSourceProductID int64, officialStoreID *int64) ListingOutcome {
	outcome := ListingOutcome{SKU: product.SKU, VehicleFitmentID: vehicleFitmentID, Title: title}

	existing, err := s.channelProductMapRepository.FindByProductConnectionAndFitment(ctx, product.ID, connectionID, vehicleFitmentID)
	if err != nil && !errors.Is(err, mysqlInfra.ErrChannelProductMapNotFound) {
		outcome.Error = fmt.Sprintf("error checking existing listing: %v", err)
		return outcome
	}
	if existing != nil && existing.Status != channelProductMapStatusClosed {
		outcome.Skipped = true
		if existing.ExternalID != nil {
			outcome.ExternalID = *existing.ExternalID
		}
		return outcome
	}

	externalID, missingRequiredAttributes, missingOptionalAttributes, err := publisher.Publish(ctx, product, connectionID, title, vehicleFitmentID, imageSourceProductID, officialStoreID)
	if err != nil {
		outcome.Error = err.Error()
		return outcome
	}

	outcome.Success = true
	outcome.ExternalID = externalID
	outcome.MissingRequiredAttributes = missingRequiredAttributes
	outcome.MissingOptionalAttributes = missingOptionalAttributes
	return outcome
}

func (s *Service) sumAvailableStock(ctx context.Context, productID int64) (int, error) {
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

func (s *Service) hasCoverImage(ctx context.Context, productID int64) (bool, error) {
	images, err := s.productImagesRepository.FindAllByProductID(ctx, productID)
	if err != nil {
		return false, err
	}
	for _, image := range images {
		if image.IsFirst {
			return true, nil
		}
	}
	return false, nil
}

// cleanSKUs strips every whitespace rune (not just leading/trailing) from
// each entry and drops blanks and duplicates, preserving first-seen order.
func cleanSKUs(raw []string) []string {
	seen := make(map[string]bool, len(raw))
	cleaned := make([]string, 0, len(raw))
	for _, s := range raw {
		sku := strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, s)
		if sku == "" || seen[sku] {
			continue
		}
		seen[sku] = true
		cleaned = append(cleaned, sku)
	}
	return cleaned
}

// generalListingTitle is used both for connections that don't allow multiple
// listings and for products with no vehicle compatibility at all (apparel,
// accessories, ...). It deliberately doesn't append the product's own brand
// — sync_mercadolibre_products.go's createNewItem already does that exactly
// once when building family_name (query = listingName + " " + brand.Name);
// appending it here too used to duplicate it (e.g. "Taza negra Nissan
// Nissan").
func generalListingTitle(productName string) string {
	return strings.TrimSpace(productName)
}

// fitmentListingTitle is used per vehicle compatibility on a connection that
// allows multiple listings, where "brand" is the vehicle's brand (from
// ecom_vehicle_fitments), not the product's own brand.
func fitmentListingTitle(productName, vehicleBrandName string, fitment *mysqlInfra.VehicleFitmentDTO) string {
	yearRange := strconv.Itoa(fitment.YearStart)
	if fitment.YearEnd != nil {
		yearRange = fmt.Sprintf("%d-%d", fitment.YearStart, *fitment.YearEnd)
	}
	return strings.TrimSpace(fmt.Sprintf("%s %s %s %s", productName, vehicleBrandName, fitment.Model, yearRange))
}

func normalizeChannelCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

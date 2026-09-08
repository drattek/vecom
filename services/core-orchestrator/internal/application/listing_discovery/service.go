// Package listing_discovery holds the once-a-day scan that finds products
// ready to be published on a marketplace and enqueues them into
// ecom_channel_sync_queue for the marketplace consumer to pick up. It is the
// only producer of that table (see ADR 0003). It never calls a marketplace
// API itself — publishing stays in internal/application/channel_listings,
// driven by the consumer in internal/workers.
package listing_discovery

import (
	"context"
	"errors"
	"fmt"
	"log"

	pricingApp "core-orchestrator/internal/application/pricing"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// marketplacePriceCurrencyCode is the currency every marketplace channel
// receives prices in (mirrors sync's mercadoLibreItemCurrencyID /
// odooSyncCurrency). The price gate resolves each product's effective price
// into this currency and requires it to be > 0.
const marketplacePriceCurrencyCode = "MXN"

// maxListingQueueAttempts caps how many times a (product, connection) pair is
// retried through the queue. Once a failed row reaches this many attempts the
// scheduler stops reactivating it — it stays 'failed' until someone fixes the
// underlying data and resets the row by hand.
const maxListingQueueAttempts = 5

// discoveryPageSize is how many ready products are pulled per keyset page.
const discoveryPageSize = 200

// systemActorID is recorded as updated_by on every queue row this scheduler
// writes, since there is no authenticated user behind an automated scan
// (mirrors the other schedulers/workers).
const systemActorID int64 = 1

type Service struct {
	products           *mysqlInfra.ProductRepository
	channelCategoryMap *mysqlInfra.ChannelCategoryMapRepository
	channelProductMap  *mysqlInfra.ChannelProductMapRepository
	queue              *mysqlInfra.ChannelSyncQueueRepository
	currencies         *mysqlInfra.CurrenciesRepository
	effectivePrice     *pricingApp.EffectivePriceResolver
	formula            *pricingApp.PricingFormulaCalculator
}

func NewService(
	products *mysqlInfra.ProductRepository,
	channelCategoryMap *mysqlInfra.ChannelCategoryMapRepository,
	channelProductMap *mysqlInfra.ChannelProductMapRepository,
	queue *mysqlInfra.ChannelSyncQueueRepository,
	currencies *mysqlInfra.CurrenciesRepository,
	effectivePrice *pricingApp.EffectivePriceResolver,
	formula *pricingApp.PricingFormulaCalculator,
) *Service {
	return &Service{
		products:           products,
		channelCategoryMap: channelCategoryMap,
		channelProductMap:  channelProductMap,
		queue:              queue,
		currencies:         currencies,
		effectivePrice:     effectivePrice,
		formula:            formula,
	}
}

// Result is a run summary for logging.
type Result struct {
	ScannedProducts int
	Enqueued        int
	Reactivated     int
	Skipped         int
}

// DiscoverAndEnqueue walks every "ready" product (see
// ProductRepository.FindReadyForListingPage) and, for each connection its
// category is mapped to, enqueues a 'listing' row unless the product already
// has a publication there, the effective price in MXN isn't > 0, or a
// reusable/exhausted queue row already exists. A failure on one
// (product, connection) pair is logged and skipped; it never aborts the run.
func (s *Service) DiscoverAndEnqueue(ctx context.Context) (Result, error) {
	var result Result

	mxn, err := s.currencies.FindByCode(ctx, marketplacePriceCurrencyCode)
	if err != nil {
		return result, fmt.Errorf("error loading %s currency: %w", marketplacePriceCurrencyCode, err)
	}

	var afterID int64
	for {
		page, err := s.products.FindReadyForListingPage(ctx, afterID, discoveryPageSize)
		if err != nil {
			return result, fmt.Errorf("error listing products ready for listing (after id %d): %w", afterID, err)
		}
		if len(page) == 0 {
			break
		}

		for _, product := range page {
			afterID = product.ID
			result.ScannedProducts++
			s.processProduct(ctx, product, mxn.ID, &result)
		}

		if len(page) < discoveryPageSize {
			break
		}
	}

	return result, nil
}

func (s *Service) processProduct(ctx context.Context, product mysqlInfra.ReadyForListingProduct, mxnCurrencyID int64, result *Result) {
	connectionIDs, err := s.channelCategoryMap.FindActiveConnectionIDsByCategory(ctx, product.CategoryID)
	if err != nil {
		log.Printf("listing discovery: product %d — error resolving connections for category %d: %v", product.ID, product.CategoryID, err)
		return
	}

	for _, connectionID := range connectionIDs {
		if err := s.processPair(ctx, product, connectionID, mxnCurrencyID, result); err != nil {
			log.Printf("listing discovery: product %d, connection %d — %v", product.ID, connectionID, err)
		}
	}
}

func (s *Service) processPair(ctx context.Context, product mysqlInfra.ReadyForListingProduct, connectionID, mxnCurrencyID int64, result *Result) error {
	ok, err := s.hasSellablePrice(ctx, product, connectionID, mxnCurrencyID)
	if err != nil {
		return fmt.Errorf("error resolving effective price: %w", err)
	}
	if !ok {
		result.Skipped++
		return nil
	}

	activeListings, err := s.channelProductMap.CountActiveListings(ctx, product.ID, connectionID)
	if err != nil {
		return fmt.Errorf("error counting existing listings: %w", err)
	}
	if activeListings > 0 {
		result.Skipped++
		return nil
	}

	entry, err := s.queue.FindListingEntry(ctx, product.ID, connectionID)
	if err != nil && !errors.Is(err, mysqlInfra.ErrChannelSyncQueueEntryNotFound) {
		return fmt.Errorf("error loading existing queue entry: %w", err)
	}

	if entry == nil {
		if _, err := s.queue.Create(ctx, mysqlInfra.CreateChannelSyncQueueEntryInput{
			ProductID:    product.ID,
			ConnectionID: connectionID,
			SyncType:     mysqlInfra.ChannelSyncQueueListingSyncType,
			UpdatedBy:    systemActorID,
		}); err != nil {
			return fmt.Errorf("error enqueueing: %w", err)
		}
		result.Enqueued++
		return nil
	}

	if entry.Status == "failed" && entry.Attempts < maxListingQueueAttempts {
		if err := s.queue.ReactivateToPending(ctx, entry.ID, systemActorID); err != nil {
			return fmt.Errorf("error reactivating queue entry %d: %w", entry.ID, err)
		}
		result.Reactivated++
		return nil
	}

	// pending/processing/done, or failed with attempts exhausted → nothing to do.
	result.Skipped++
	return nil
}

// hasSellablePrice resolves product's effective price the same way publish
// does — highest-priority price list, converted into MXN, then run through the
// connection's pricing formula — and reports whether the result is a real,
// positive price. A product with no active price list entry at all is simply
// not ready (ok=false, no error).
func (s *Service) hasSellablePrice(ctx context.Context, product mysqlInfra.ReadyForListingProduct, connectionID, mxnCurrencyID int64) (bool, error) {
	basePrice, _, priceListID, err := s.effectivePrice.ResolveInCurrency(ctx, product.ID, mxnCurrencyID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductPriceNotFound) {
			return false, nil
		}
		return false, err
	}

	brandID := product.BrandID
	finalPrice, err := s.formula.CalculatePrice(ctx, &brandID, connectionID, priceListID, basePrice)
	if err != nil {
		return false, err
	}

	return finalPrice > 0, nil
}

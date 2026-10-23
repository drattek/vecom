package sync

import (
	"context"
	"errors"
	"fmt"
	"log"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// FoldMercadoLibreItemStatus exposes foldMercadoLibreStatus to
// meli_listing_import, which can't live in this package (it depends on
// product_category_selection, which already imports this one).
func FoldMercadoLibreItemStatus(rawStatus string, subStatus []string) string {
	return foldMercadoLibreStatus(rawStatus, subStatus)
}

// ImportedListingPushResult reports what PushImportedListing sent to
// MercadoLibre after an existing listing was linked to a local product. The
// *SkippedReason fields explain a deliberate no-op (nothing to send, or a rule
// said to leave the listing alone); the *Error fields are real failures.
type ImportedListingPushResult struct {
	PriceStockUpdated       bool    `json:"priceStockUpdated"`
	PriceStockSkippedReason string  `json:"priceStockSkippedReason,omitempty"`
	PriceStockError         string  `json:"priceStockError,omitempty"`
	Price                   float64 `json:"price,omitempty"`
	Quantity                int     `json:"quantity,omitempty"`
}

// PushImportedListing is the last phase of importing an already-existing
// MercadoLibre listing (see meli_listing_import.Service): once its data has
// been saved locally and listing recorded in ecom_channel_product_map, and only
// if the product currently has both stock and a price, it pushes the product's
// current price (through the same pricing formula, price list and pinned-SKU
// rules as every other price push — see resolvePriceStockAndToken) and total
// stock. The listing's title, family_name, listing type and official store are
// never touched.
//
// Unlike Refresh it is never gated on "did price/stock change since the last
// sync" — an import always pushes — and it does not skip a paused listing: a
// listing that already exists is very often paused precisely because it ran out
// of stock, and bringing it back in line with the local price/stock is the
// point of importing it (MercadoLibre reactivates an out-of-stock listing on
// its own once it has stock again). The live status is re-read afterwards and
// recorded as it really is. Closed and under_review listings are left alone,
// as in every other automatic flow. Only price
// and quantity are sent: the package attributes
// updateExistingListings adds would overwrite whatever the seller had on the
// listing with generic defaults when the product has no dimensions of its own.
func (s *MercadoLibreProductSyncService) PushImportedListing(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO, item *mercadoLibreInfra.ItemDetail) *ImportedListingPushResult {
	result := &ImportedListingPushResult{}
	externalID := derefString(listing.ExternalID)

	// closed can't be recovered and under_review is never updated automatically
	// (the user fixes the product's data and presses "Resincronizar"); a paused
	// listing is updated like any other.
	switch listing.Status {
	case mercadoLibreMapStatusClosed:
		reason := "la publicación está cerrada en MercadoLibre; no se modifica"
		result.PriceStockSkippedReason = reason
		return result
	case mercadoLibreMapStatusUnderReview:
		reason := "la publicación está en revisión en MercadoLibre; corrige los datos del producto y usa Resincronizar"
		result.PriceStockSkippedReason = reason
		return result
	}

	finalPrice, qtyAvailable, accessToken, err := s.resolvePriceStockAndToken(ctx, product.ID, product.SKU, product.BrandID, connectionID)
	switch {
	case errors.Is(err, mysqlInfra.ErrProductPriceNotFound):
		result.PriceStockSkippedReason = "el producto no tiene precio"
		return result
	case err != nil:
		result.PriceStockError = err.Error()
		return result
	}
	if finalPrice <= 0 {
		result.PriceStockSkippedReason = "el producto no tiene precio"
		return result
	}
	if qtyAvailable <= 0 {
		result.PriceStockSkippedReason = "el producto no tiene stock"
		return result
	}

	if err := s.itemsHandler.UpdateItem(ctx, mercadoLibreInfra.UpdateItemRequest{
		AccessToken: accessToken,
		ExternalID:  externalID,
		Vals: mercadoLibreInfra.UpdateItemVals{
			Price:             finalPrice,
			AvailableQuantity: qtyAvailable,
		},
	}); err != nil {
		result.PriceStockError = fmt.Sprintf("error actualizando precio y stock en MercadoLibre: %v", err)
	} else if _, err := s.channelProductMapRepository.Upsert(ctx, s.liveListingMapInput(ctx, product, connectionID, listing, accessToken)); err != nil {
		result.PriceStockError = fmt.Sprintf("precio y stock enviados, pero no se pudo registrar la sincronización: %v", err)
	} else {
		result.PriceStockUpdated = true
		result.Price = finalPrice
		result.Quantity = qtyAvailable
	}

	return result
}

// liveListingStatus returns externalID's current status on MercadoLibre folded
// into the ecom_channel_product_map vocabulary, or fallback if it can't be read.
func (s *MercadoLibreProductSyncService) liveListingStatus(ctx context.Context, accessToken, externalID, fallback string) string {
	live, err := s.itemsHandler.GetItem(ctx, accessToken, externalID)
	if err != nil {
		log.Printf("mercadolibre: listing %s — could not re-read its status after a push, keeping %q: %v", externalID, fallback, err)
		return fallback
	}
	return foldMercadoLibreStatus(live.Status, live.SubStatus)
}

// liveListingMapInput builds the ecom_channel_product_map upsert for listing
// right after a price/stock push, with the status MercadoLibre reports now (the
// push may have reactivated an out-of-stock listing). A listing that is still
// paused is recorded with paused_reason 'user' — this system didn't pause it, so
// no later refresh reactivates it on its own. If the status can't be read, the
// one recorded at import time is kept.
func (s *MercadoLibreProductSyncService) liveListingMapInput(ctx context.Context, product *mysqlInfra.ProductDTO, connectionID int64, listing *mysqlInfra.ChannelProductMapDTO, accessToken string) mysqlInfra.UpsertChannelProductMapInput {
	status := s.liveListingStatus(ctx, accessToken, derefString(listing.ExternalID), listing.Status)

	pausedReason := ""
	if status == mercadoLibreMapStatusPaused {
		pausedReason = "user"
	}

	return mysqlInfra.UpsertChannelProductMapInput{
		ProductID:          product.ID,
		ConnectionID:       connectionID,
		VehicleFitmentID:   listing.VehicleFitmentID,
		ListingTitle:       derefString(listing.ListingTitle),
		ExternalID:         derefString(listing.ExternalID),
		ExternalCategoryID: derefString(listing.ExternalCategoryID),
		Status:             status,
		PausedReason:       pausedReason,
		ActorID:            systemMercadoLibreSyncActorID,
	}
}

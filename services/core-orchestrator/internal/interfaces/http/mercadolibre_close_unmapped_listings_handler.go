package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	syncApp "core-orchestrator/internal/application/sync"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// closeUnmappedListingsFlaggedPrice/closeUnmappedListingsFlaggedStock mirror
// sync.mercadoLibreFlaggedListingPrice/mercadoLibreFlaggedListingStock: the
// sentinel price/stock MercadoLibreListingsAuditHandler's endpoint already
// downloads and processes on its own. Listings matching both, exactly, are
// excluded here so the two temporary endpoints never fight over the same
// listings.
const (
	closeUnmappedListingsFlaggedPrice = 999999
	closeUnmappedListingsFlaggedStock = 0
)

// MercadoLibreCloseUnmappedListingsHandler is a TEMPORARY, one-off endpoint:
// given a connectionId in the request body, it lists every listing currently
// published on MercadoLibre for that connection's seller account (any
// status), excludes the ones flagged with price=999999/stock=0 (those belong
// to MercadoLibreListingsAuditHandler's endpoint instead), and for every
// remaining external id that has no matching row in
// ecom_channel_product_map (a listing this system no longer tracks), sets
// its status to "closed" on MercadoLibre. A listing that IS mapped locally
// is left completely untouched (skipped). Remove this handler and its route
// once run.
type MercadoLibreCloseUnmappedListingsHandler struct {
	tokenService                *syncApp.MercadoLibreTokenService
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository
	channelRepository           *mysqlInfra.ChannelRepository
	channelProductMapRepo       *mysqlInfra.ChannelProductMapRepository
	itemsHandler                *mercadoLibreInfra.ItemsHandler
	usersHandler                *mercadoLibreInfra.UsersHandler
}

func NewMercadoLibreCloseUnmappedListingsHandler(
	tokenService *syncApp.MercadoLibreTokenService,
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	channelProductMapRepo *mysqlInfra.ChannelProductMapRepository,
	rateLimiter *mercadoLibreInfra.RateLimiter,
) *MercadoLibreCloseUnmappedListingsHandler {
	client := mercadoLibreInfra.NewClient(nil, "", rateLimiter)

	return &MercadoLibreCloseUnmappedListingsHandler{
		tokenService:                tokenService,
		channelConnectionRepository: channelConnectionRepository,
		channelRepository:           channelRepository,
		channelProductMapRepo:       channelProductMapRepo,
		itemsHandler:                mercadoLibreInfra.NewItemsHandler(client),
		usersHandler:                mercadoLibreInfra.NewUsersHandler(client),
	}
}

type closeUnmappedListingsRequest struct {
	ConnectionID int64 `json:"connectionId"`
}

type closeUnmappedListingsResultItem struct {
	ExternalID string `json:"externalId"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

// Run lists every MercadoLibre item for input.ConnectionID's seller,
// excludes the flagged price/stock sentinel listings, diffs the rest against
// ecom_channel_product_map, and closes every external id that isn't tracked
// there.
func (h *MercadoLibreCloseUnmappedListingsHandler) Run(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req closeUnmappedListingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ConnectionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "connectionId is required")
		return
	}

	connection, err := h.channelConnectionRepository.FindByID(r.Context(), req.ConnectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			writeJSONError(w, http.StatusNotFound, "connection not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error loading connection: %v", err))
		return
	}
	channel, err := h.channelRepository.FindByID(r.Context(), connection.ChannelID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error loading channel: %v", err))
		return
	}
	if !strings.EqualFold(strings.TrimSpace(channel.Code), "MERCADOLIBRE") {
		writeJSONError(w, http.StatusBadRequest, "connection is not a mercadolibre connection")
		return
	}

	accessToken, err := h.tokenService.EnsureValidAccessToken(ctx, req.ConnectionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error getting mercadolibre access token: %v", err))
		return
	}

	me, err := h.usersHandler.GetMe(ctx, accessToken)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("error resolving mercadolibre account id: %v", err))
		return
	}

	externalIDs, err := h.listAllSellerItemIDs(ctx, req.ConnectionID, me.ID)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("error listing mercadolibre items: %v", err))
		return
	}
	log.Printf("mercadolibre close unmapped listings: connection %d — seller %d has %d listing(s)", req.ConnectionID, me.ID, len(externalIDs))

	summaries, err := h.fetchAllItemSummaries(ctx, req.ConnectionID, externalIDs)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("error fetching mercadolibre item summaries: %v", err))
		return
	}

	candidateIDs := make([]string, 0, len(summaries))
	totalFlaggedExcluded := 0
	for _, summary := range summaries {
		if summary.Price == closeUnmappedListingsFlaggedPrice && summary.AvailableQuantity == closeUnmappedListingsFlaggedStock {
			totalFlaggedExcluded++
			continue
		}
		candidateIDs = append(candidateIDs, summary.ID)
	}

	mapped, err := h.channelProductMapRepo.FindAllByConnectionID(r.Context(), req.ConnectionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error reading ecom_channel_product_map: %v", err))
		return
	}
	mappedExternalIDs := make(map[string]bool, len(mapped))
	for _, m := range mapped {
		if m.ExternalID != nil {
			mappedExternalIDs[strings.TrimSpace(*m.ExternalID)] = true
		}
	}

	results := make([]closeUnmappedListingsResultItem, 0)
	totalSkippedMapped := 0
	for _, externalID := range candidateIDs {
		if mappedExternalIDs[externalID] {
			totalSkippedMapped++
			continue
		}

		result := closeUnmappedListingsResultItem{ExternalID: externalID}

		// Re-resolve the token per item rather than reusing the one fetched
		// above: this run is throttled by the shared MercadoLibre rate
		// limiter and can take long enough for the background token refresh
		// scheduler to rotate this same connection's token mid-run (mirrors
		// MercadoLibrePauseUnmappedListingsHandler.Run).
		itemAccessToken, tokenErr := h.tokenService.EnsureValidAccessToken(ctx, req.ConnectionID)
		if tokenErr != nil {
			result.Error = fmt.Sprintf("error getting mercadolibre access token: %v", tokenErr)
			results = append(results, result)
			continue
		}

		if err := h.itemsHandler.UpdateItemStatus(ctx, mercadoLibreInfra.UpdateItemStatusRequest{
			AccessToken: itemAccessToken,
			ExternalID:  externalID,
			Status:      "closed",
		}); err != nil {
			result.Error = err.Error()
		} else {
			result.Success = true
		}
		results = append(results, result)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"totalListedOnMercadoLibre": len(externalIDs),
		"totalFlaggedExcluded":      totalFlaggedExcluded,
		"totalMappedLocallySkipped": totalSkippedMapped,
		"totalClosed":               len(results),
		"results":                   results,
	})
}

// listAllSellerItemIDs pages through GET /users/{sellerID}/items/search
// (MercadoLibre's cursor-based "scan" mode — no 1000-result cap) until a
// page comes back with no results, re-validating the access token before
// every page (mirrors sync.MercadoLibreListingsAuditService.listAllSellerItemIDs).
func (h *MercadoLibreCloseUnmappedListingsHandler) listAllSellerItemIDs(ctx context.Context, connectionID, sellerID int64) ([]string, error) {
	all := make([]string, 0)
	scrollID := ""

	for {
		accessToken, err := h.tokenService.EnsureValidAccessToken(ctx, connectionID)
		if err != nil {
			return nil, fmt.Errorf("error getting mercadolibre access token: %w", err)
		}

		page, err := h.itemsHandler.ScanSellerItems(ctx, accessToken, sellerID, scrollID)
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
// batch, re-validating the access token before each one (mirrors
// sync.MercadoLibreListingsAuditService.fetchAllItemSummaries).
func (h *MercadoLibreCloseUnmappedListingsHandler) fetchAllItemSummaries(ctx context.Context, connectionID int64, externalIDs []string) ([]mercadoLibreInfra.ItemSummary, error) {
	summaries := make([]mercadoLibreInfra.ItemSummary, 0, len(externalIDs))

	for start := 0; start < len(externalIDs); start += mercadoLibreInfra.MaxItemsBatchSize {
		end := start + mercadoLibreInfra.MaxItemsBatchSize
		if end > len(externalIDs) {
			end = len(externalIDs)
		}

		accessToken, err := h.tokenService.EnsureValidAccessToken(ctx, connectionID)
		if err != nil {
			return nil, fmt.Errorf("error getting mercadolibre access token: %w", err)
		}

		batch, err := h.itemsHandler.GetItemsSummary(ctx, accessToken, externalIDs[start:end])
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, batch...)
	}

	return summaries, nil
}

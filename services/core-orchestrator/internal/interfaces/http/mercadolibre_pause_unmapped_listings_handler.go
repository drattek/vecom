package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	syncApp "core-orchestrator/internal/application/sync"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// pauseUnmappedListingsConnectionID is hardcoded to 2 per explicit
// instruction.
const pauseUnmappedListingsConnectionID int64 = 2

// MercadoLibrePauseUnmappedListingsHandler is a TEMPORARY, one-off endpoint:
// it lists every "active" item currently published on MercadoLibre for
// connection 2's seller account and, for every external id that has no
// matching row in ecom_channel_product_map (a listing this system no longer
// tracks), sets available_quantity to 0. It never closes or deletes anything
// — the intent
// is only to make MercadoLibre's own "long time out of stock" rule pause the
// listing automatically later. Read-only against ecom_channel_product_map
// (never writes to it) and shares the app-wide MercadoLibre rate limiter
// (passed in, same instance used everywhere else) so this bulk run doesn't
// blow through the account's real MercadoLibre rate limit. Remove this
// handler and its route once run.
type MercadoLibrePauseUnmappedListingsHandler struct {
	tokenService          *syncApp.MercadoLibreTokenService
	channelProductMapRepo *mysqlInfra.ChannelProductMapRepository
	client                *mercadoLibreInfra.Client
}

func NewMercadoLibrePauseUnmappedListingsHandler(
	tokenService *syncApp.MercadoLibreTokenService,
	channelProductMapRepo *mysqlInfra.ChannelProductMapRepository,
	rateLimiter *mercadoLibreInfra.RateLimiter,
) *MercadoLibrePauseUnmappedListingsHandler {
	return &MercadoLibrePauseUnmappedListingsHandler{
		tokenService:          tokenService,
		channelProductMapRepo: channelProductMapRepo,
		client:                mercadoLibreInfra.NewClient(nil, "", rateLimiter),
	}
}

type pauseUnmappedListingsResultItem struct {
	ExternalID string `json:"externalId"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

// Run lists every "active" MercadoLibre item for connection 2's seller,
// diffs it against ecom_channel_product_map, and zeroes the stock of every
// external id that isn't tracked there.
func (h *MercadoLibrePauseUnmappedListingsHandler) Run(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	accessToken, err := h.tokenService.EnsureValidAccessToken(ctx, pauseUnmappedListingsConnectionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error getting mercadolibre access token: %v", err))
		return
	}

	sellerID, err := h.fetchSellerID(ctx, accessToken)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("error getting mercadolibre seller id: %v", err))
		return
	}

	listedExternalIDs, err := h.fetchAllItemIDs(ctx, accessToken, sellerID)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("error listing mercadolibre items: %v", err))
		return
	}

	mapped, err := h.channelProductMapRepo.FindAllByConnectionID(pauseUnmappedListingsConnectionID)
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

	results := make([]pauseUnmappedListingsResultItem, 0)
	for _, externalID := range listedExternalIDs {
		if mappedExternalIDs[externalID] {
			continue
		}

		result := pauseUnmappedListingsResultItem{ExternalID: externalID}

		// Re-resolve the token per item instead of reusing the one fetched
		// above: EnsureValidAccessToken is a cheap no-op while it's still
		// valid, but this run is throttled by the shared MercadoLibre rate
		// limiter and can take long enough for the background
		// tokenRefreshScheduler to rotate this same connection's token
		// mid-run, invalidating whatever copy was grabbed once at the start
		// — which is what was surfacing as a 401 on the PUT.
		itemAccessToken, tokenErr := h.tokenService.EnsureValidAccessToken(ctx, pauseUnmappedListingsConnectionID)
		if tokenErr != nil {
			result.Error = fmt.Sprintf("error getting mercadolibre access token: %v", tokenErr)
			results = append(results, result)
			continue
		}

		if err := h.zeroStock(ctx, itemAccessToken, externalID); err != nil {
			result.Error = err.Error()
		} else {
			result.Success = true
		}
		results = append(results, result)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"totalListedOnMercadoLibre": len(listedExternalIDs),
		"totalMappedLocally":        len(mappedExternalIDs),
		"totalProcessed":            len(results),
		"results":                   results,
	})
}

type meliUserMeResponse struct {
	ID int64 `json:"id"`
}

// fetchSellerID calls GET /users/me to resolve the numeric seller id behind
// accessToken, needed for the items/search call below.
func (h *MercadoLibrePauseUnmappedListingsHandler) fetchSellerID(ctx context.Context, accessToken string) (int64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.mercadolibre.com/users/me", nil)
	if err != nil {
		return 0, fmt.Errorf("error creating users/me request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("error calling mercadolibre users/me: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading users/me response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("mercadolibre users/me failed (%d): %s", response.StatusCode, body)
	}

	var result meliUserMeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("error decoding users/me response: %w", err)
	}

	return result.ID, nil
}

type meliItemsSearchResponse struct {
	Results  []string `json:"results"`
	ScrollID string   `json:"scroll_id"`
}

// fetchAllItemIDs paginates GET /users/{sellerID}/items/search with
// search_type=scan (MercadoLibre's cursor-based scroll, unlike offset
// pagination it has no 1000-result cap) until a page comes back empty,
// collecting every "active" item id the seller has published — status=active
// is passed straight to MercadoLibre's own filter so paused/closed/
// under_review listings never come back and don't need filtering here.
func (h *MercadoLibrePauseUnmappedListingsHandler) fetchAllItemIDs(ctx context.Context, accessToken string, sellerID int64) ([]string, error) {
	ids := make([]string, 0)
	scrollID := ""

	for {
		requestURL := fmt.Sprintf("https://api.mercadolibre.com/users/%d/items/search?search_type=scan&status=active&limit=100", sellerID)
		if scrollID != "" {
			requestURL += "&scroll_id=" + url.QueryEscape(scrollID)
		}

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating items/search request: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+accessToken)

		response, err := h.client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("error calling mercadolibre items/search: %w", err)
		}

		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("error reading items/search response: %w", err)
		}

		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("mercadolibre items/search failed (%d): %s", response.StatusCode, body)
		}

		var page meliItemsSearchResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("error decoding items/search response: %w", err)
		}

		if len(page.Results) == 0 {
			break
		}
		ids = append(ids, page.Results...)
		scrollID = page.ScrollID
	}

	return ids, nil
}

// zeroStock calls PUT /items/{externalId} with only {"available_quantity": 0}
// — a partial update, same as UpdateItemShipping in the mercadolibre package
// — so it doesn't touch price, category or anything else already set on the
// listing.
func (h *MercadoLibrePauseUnmappedListingsHandler) zeroStock(ctx context.Context, accessToken, externalID string) error {
	payload, err := json.Marshal(map[string]int{"available_quantity": 0})
	if err != nil {
		return fmt.Errorf("error encoding stock update: %w", err)
	}

	requestURL := fmt.Sprintf("https://api.mercadolibre.com/items/%s", externalID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, requestURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("error creating stock update request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.client.Do(request)
	if err != nil {
		return fmt.Errorf("error calling mercadolibre: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("error reading mercadolibre response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("mercadolibre stock update failed (%d): %s", response.StatusCode, body)
	}

	return nil
}

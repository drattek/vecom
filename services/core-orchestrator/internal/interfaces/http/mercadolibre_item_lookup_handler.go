package http

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	syncApp "core-orchestrator/internal/application/sync"
)

// MercadoLibreItemLookupHandler is a TEMPORARY, throwaway endpoint for
// manually inspecting a MercadoLibre item's raw API response by
// connectionId + externalId. It does not belong to any long-lived
// integration, bypasses the internal mercadolibre.Client/rate limiter on
// purpose to stay fully isolated from the existing sync flows, and should be
// removed (along with its route) once no longer needed.
type MercadoLibreItemLookupHandler struct {
	tokenService *syncApp.MercadoLibreTokenService
	httpClient   *http.Client
}

func NewMercadoLibreItemLookupHandler(tokenService *syncApp.MercadoLibreTokenService) *MercadoLibreItemLookupHandler {
	return &MercadoLibreItemLookupHandler{
		tokenService: tokenService,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

// GetItem calls GET https://api.mercadolibre.com/items/{externalId} using
// connectionId's current access token and relays the raw MercadoLibre
// response back as-is (status code and body).
func (h *MercadoLibreItemLookupHandler) GetItem(w http.ResponseWriter, r *http.Request) {
	connectionIDStr := strings.TrimSpace(r.URL.Query().Get("connectionId"))
	externalID := strings.TrimSpace(r.URL.Query().Get("externalId"))

	if connectionIDStr == "" || externalID == "" {
		writeJSONError(w, http.StatusBadRequest, "connectionId and externalId are required")
		return
	}

	connectionID, err := strconv.ParseInt(connectionIDStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
		return
	}

	accessToken, err := h.tokenService.EnsureValidAccessToken(r.Context(), connectionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error getting mercadolibre access token: %v", err))
		return
	}

	requestURL := fmt.Sprintf("https://api.mercadolibre.com/items/%s", externalID)
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, requestURL, nil)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "error creating mercadolibre request")
		return
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.httpClient.Do(request)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("error calling mercadolibre: %v", err))
		return
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "error reading mercadolibre response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	w.Write(body)
}

// GetSellerBrands calls GET https://api.mercadolibre.com/users/{sellerId}/brands
// using connectionId's current access token and relays the raw MercadoLibre
// response back as-is — which brand(s) this seller is registered as an
// Official Store for, and each one's official_store_id. Added to diagnose
// why some items on a connection get auto-linked to an Official Store
// (official_store_id set on the published item, even though this app never
// sends that field itself) while others on the very same seller account
// don't — see the request body of a raw GET /items/{id} via GetItem above
// for confirmation on any specific item.
func (h *MercadoLibreItemLookupHandler) GetSellerBrands(w http.ResponseWriter, r *http.Request) {
	connectionIDStr := strings.TrimSpace(r.URL.Query().Get("connectionId"))
	sellerID := strings.TrimSpace(r.URL.Query().Get("sellerId"))

	if connectionIDStr == "" || sellerID == "" {
		writeJSONError(w, http.StatusBadRequest, "connectionId and sellerId are required")
		return
	}

	connectionID, err := strconv.ParseInt(connectionIDStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
		return
	}

	accessToken, err := h.tokenService.EnsureValidAccessToken(r.Context(), connectionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error getting mercadolibre access token: %v", err))
		return
	}

	requestURL := fmt.Sprintf("https://api.mercadolibre.com/users/%s/brands", sellerID)
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, requestURL, nil)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "error creating mercadolibre request")
		return
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := h.httpClient.Do(request)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("error calling mercadolibre: %v", err))
		return
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "error reading mercadolibre response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	w.Write(body)
}

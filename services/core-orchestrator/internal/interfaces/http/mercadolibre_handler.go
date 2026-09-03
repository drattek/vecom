package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	syncApp "core-orchestrator/internal/application/sync"
)

// defaultMercadoLibreOAuthConnectionID is used by OAuthCallback when the
// redirect doesn't carry a connectionId: MercadoLibre's redirect_uri has no
// way to round-trip our own query parameters unless we control the
// authorization request that started the flow, so callers of
// GetAuthorizationURL that don't pass one through get routed back here.
const defaultMercadoLibreOAuthConnectionID = 2

type MercadoLibreHandler struct {
	predictorService   *syncApp.MercadoLibreCategoryPredictorService
	productSyncService *syncApp.MercadoLibreProductSyncService
	tokenService       *syncApp.MercadoLibreTokenService
}

func NewMercadoLibreHandler(
	predictorService *syncApp.MercadoLibreCategoryPredictorService,
	productSyncService *syncApp.MercadoLibreProductSyncService,
	tokenService *syncApp.MercadoLibreTokenService,
) *MercadoLibreHandler {
	return &MercadoLibreHandler{
		predictorService:   predictorService,
		productSyncService: productSyncService,
		tokenService:       tokenService,
	}
}

// GetAuthorizationURL sends the browser straight to MercadoLibre's
// authorization (login/consent) page for connectionId, using the
// app_id/redirect_uri already configured in ecom_connection_settings for
// that connection — so opening this URL directly (e.g. a link or
// window.location) starts the OAuth flow immediately.
func (h *MercadoLibreHandler) GetAuthorizationURL(w http.ResponseWriter, r *http.Request) {
	connectionIDStr := strings.TrimSpace(r.URL.Query().Get("connectionId"))
	if connectionIDStr == "" {
		writeJSONError(w, http.StatusBadRequest, "connectionId is required")
		return
	}

	connectionID, err := strconv.ParseInt(connectionIDStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
		return
	}

	authURL, err := h.tokenService.GetAuthorizationURL(r.Context(), connectionID)
	if err != nil {
		if errors.Is(err, syncApp.ErrInvalidMercadoLibreConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		if errors.Is(err, syncApp.ErrMissingMercadoLibreSettings) {
			writeJSONError(w, http.StatusBadRequest, "missing required Mercado Libre connection settings (app_id/redirect_uri)")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// OAuthCallback is the redirect_uri target registered in the MercadoLibre
// developer app (.../meli_callback): MercadoLibre sends the user's browser
// here with a one-time `code` after they approve access. That redirect
// carries no app-level authentication and, unless a connectionId query
// parameter is also present, no way to say which connection the code
// belongs to — so this endpoint is intentionally public (see router.go) and
// defaults to defaultMercadoLibreOAuthConnectionID.
func (h *MercadoLibreHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		writeJSONError(w, http.StatusBadRequest, "code is required")
		return
	}

	connectionID := int64(defaultMercadoLibreOAuthConnectionID)
	if connectionIDStr := strings.TrimSpace(r.URL.Query().Get("connectionId")); connectionIDStr != "" {
		parsed, err := strconv.ParseInt(connectionIDStr, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		connectionID = parsed
	}

	if err := h.tokenService.ExchangeAuthorizationCode(r.Context(), connectionID, code); err != nil {
		if errors.Is(err, syncApp.ErrMissingMercadoLibreSettings) {
			writeJSONError(w, http.StatusBadRequest, "missing required Mercado Libre connection settings (app_id/client_secret/redirect_uri)")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"success": true, "connectionId": connectionID})
}

type uploadMercadoLibreProductRequest struct {
	SKU              string `json:"sku"`
	Name             string `json:"name"`
	VehicleFitmentID *int64 `json:"vehicleFitmentId,omitempty"`
}

type uploadMercadoLibreProductsRequest struct {
	ConnectionID int64                              `json:"connectionId"`
	Products     []uploadMercadoLibreProductRequest `json:"products"`
}

// UploadProducts accepts a list of {sku, name, vehicleFitmentId} entries and
// lists each one as a new item on MercadoLibre, using name instead of the
// product's own ecom_products.name — so the same product can be listed more
// than once under different names, one per vehicle compatibility, on
// connections where that's allowed (see
// syncApp.MercadoLibreProductSyncService.Upload).
func (h *MercadoLibreHandler) UploadProducts(w http.ResponseWriter, r *http.Request) {
	var req uploadMercadoLibreProductsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	products := make([]syncApp.UploadItemInput, 0, len(req.Products))
	for _, p := range req.Products {
		products = append(products, syncApp.UploadItemInput{
			SKU:              p.SKU,
			Name:             p.Name,
			VehicleFitmentID: p.VehicleFitmentID,
		})
	}

	result, err := h.productSyncService.Upload(r.Context(), syncApp.UploadInput{
		ConnectionID: req.ConnectionID,
		Products:     products,
	})
	if err != nil {
		if errors.Is(err, syncApp.ErrEmptyMercadoLibreUpload) {
			writeJSONError(w, http.StatusBadRequest, "connectionId and at least one product are required")
			return
		}
		if errors.Is(err, syncApp.ErrInvalidMercadoLibreConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

type updateMercadoLibrePricesStockRequest struct {
	ConnectionID int64    `json:"connectionId"`
	SKUs         []string `json:"skus"`
}

// UpdatePricesAndStock accepts a list of skus and refreshes price/stock on
// every MercadoLibre listing already recorded for each one in
// ecom_channel_product_map for the given connection — it never creates a
// new listing (use UploadProducts for that).
func (h *MercadoLibreHandler) UpdatePricesAndStock(w http.ResponseWriter, r *http.Request) {
	var req updateMercadoLibrePricesStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.productSyncService.UpdatePricesAndStock(r.Context(), syncApp.UpdatePricesStockInput{
		ConnectionID: req.ConnectionID,
		SKUs:         req.SKUs,
	})
	if err != nil {
		if errors.Is(err, syncApp.ErrEmptyMercadoLibreUpdate) {
			writeJSONError(w, http.StatusBadRequest, "connectionId and at least one sku are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type refreshChannelProductMapStatusRequest struct {
	ConnectionID int64 `json:"connectionId"`
}

// RefreshChannelProductMapStatus queries MercadoLibre for the current status
// of every ecom_channel_product_map row recorded for the given connection
// (every product's listings, general and per-vehicle-fitment alike) and
// persists any change back into that table's status column.
func (h *MercadoLibreHandler) RefreshChannelProductMapStatus(w http.ResponseWriter, r *http.Request) {
	var req refreshChannelProductMapStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.productSyncService.RefreshChannelProductMapStatus(r.Context(), req.ConnectionID)
	if err != nil {
		if errors.Is(err, syncApp.ErrInvalidMercadoLibreConnection) {
			writeJSONError(w, http.StatusBadRequest, "connectionId is required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type refreshChannelProductMapStatusAndCategoryRequest struct {
	ConnectionID int64 `json:"connectionId"`
}

// RefreshChannelProductMapStatusAndCategory queries MercadoLibre for the
// current status AND category of every ecom_channel_product_map row recorded
// for the given connection. A changed status is persisted the same way
// RefreshChannelProductMapStatus does; a changed category additionally
// triggers the same category-provisioning procedure item creation uses
// (replicating whatever's missing of MercadoLibre's category tree into
// ecom_categories and upserting ecom_channel_category_map for it) before the
// map row's external_category_id is updated — see
// syncApp.MercadoLibreProductSyncService.RefreshChannelProductMapStatusAndCategory.
func (h *MercadoLibreHandler) RefreshChannelProductMapStatusAndCategory(w http.ResponseWriter, r *http.Request) {
	var req refreshChannelProductMapStatusAndCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.productSyncService.RefreshChannelProductMapStatusAndCategory(r.Context(), req.ConnectionID)
	if err != nil {
		if errors.Is(err, syncApp.ErrInvalidMercadoLibreConnection) {
			writeJSONError(w, http.StatusBadRequest, "connectionId is required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *MercadoLibreHandler) PredictCategory(w http.ResponseWriter, r *http.Request) {
	connectionIDStr := strings.TrimSpace(r.URL.Query().Get("connectionId"))
	title := strings.TrimSpace(r.URL.Query().Get("title"))
	siteID := strings.TrimSpace(r.URL.Query().Get("siteId"))
	limitStr := strings.TrimSpace(r.URL.Query().Get("limit"))

	if connectionIDStr == "" {
		writeJSONError(w, http.StatusBadRequest, "connectionId is required")
		return
	}

	connectionID, err := strconv.ParseInt(connectionIDStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
		return
	}

	limit := 1
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = parsedLimit
	}

	result, err := h.predictorService.PredictCategories(r.Context(), connectionID, title, siteID, limit)
	if err != nil {
		if errors.Is(err, syncApp.ErrInvalidMercadoLibreConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		if errors.Is(err, syncApp.ErrInvalidMercadoLibreTitle) {
			writeJSONError(w, http.StatusBadRequest, "title is required")
			return
		}
		if errors.Is(err, syncApp.ErrMissingMercadoLibreSiteID) {
			writeJSONError(w, http.StatusBadRequest, "siteId is required or must exist in connection settings as site_id")
			return
		}
		if errors.Is(err, syncApp.ErrMissingMercadoLibreSettings) {
			writeJSONError(w, http.StatusBadRequest, "missing required Mercado Libre connection settings")
			return
		}
		if errors.Is(err, syncApp.ErrInvalidExpirationTimeFormat) {
			writeJSONError(w, http.StatusBadRequest, "invalid expiration_time format in connection settings")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

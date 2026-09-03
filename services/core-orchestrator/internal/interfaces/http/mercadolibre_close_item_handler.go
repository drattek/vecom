package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	syncApp "core-orchestrator/internal/application/sync"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
)

// MercadoLibreCloseItemHandler is a TEMPORARY, throwaway endpoint: given a
// connectionId and an itemId in the request body, it sets that single
// listing's status to "closed" on MercadoLibre. Unlike
// MercadoLibreCloseUnmappedListingsHandler this doesn't scan or filter
// anything — it's a manual, one-item close for ad-hoc use. Remove this
// handler and its route once no longer needed.
type MercadoLibreCloseItemHandler struct {
	tokenService *syncApp.MercadoLibreTokenService
	itemsHandler *mercadoLibreInfra.ItemsHandler
}

func NewMercadoLibreCloseItemHandler(
	tokenService *syncApp.MercadoLibreTokenService,
	rateLimiter *mercadoLibreInfra.RateLimiter,
) *MercadoLibreCloseItemHandler {
	client := mercadoLibreInfra.NewClient(nil, "", rateLimiter)

	return &MercadoLibreCloseItemHandler{
		tokenService: tokenService,
		itemsHandler: mercadoLibreInfra.NewItemsHandler(client),
	}
}

type closeItemRequest struct {
	ConnectionID int64  `json:"connectionId"`
	ItemID       string `json:"itemId"`
}

// Run sets input.ItemID's status to "closed" on MercadoLibre, using
// input.ConnectionID's current access token.
func (h *MercadoLibreCloseItemHandler) Run(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req closeItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ConnectionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "connectionId is required")
		return
	}
	if req.ItemID == "" {
		writeJSONError(w, http.StatusBadRequest, "itemId is required")
		return
	}

	accessToken, err := h.tokenService.EnsureValidAccessToken(ctx, req.ConnectionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error getting mercadolibre access token: %v", err))
		return
	}

	if err := h.itemsHandler.UpdateItemStatus(ctx, mercadoLibreInfra.UpdateItemStatusRequest{
		AccessToken: accessToken,
		ExternalID:  req.ItemID,
		Status:      "closed",
	}); err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("error closing mercadolibre item: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"itemId": req.ItemID,
		"status": "closed",
	})
}

package http

import (
	"encoding/json"
	"errors"
	"net/http"

	syncApp "core-orchestrator/internal/application/sync"
)

// MercadoLibreListingsAuditHandler scans every listing in a MercadoLibre
// account for ones matching the flagged price/stock sentinel and, for each
// match, resolves the local ecom_products row (creating it from the listing
// when absent) and copies the listing's vehicle compatibilities into MySQL —
// see sync.MercadoLibreListingsAuditService.
type MercadoLibreListingsAuditHandler struct {
	service *syncApp.MercadoLibreListingsAuditService
}

func NewMercadoLibreListingsAuditHandler(service *syncApp.MercadoLibreListingsAuditService) *MercadoLibreListingsAuditHandler {
	return &MercadoLibreListingsAuditHandler{service: service}
}

type syncFlaggedMercadoLibreListingsRequest struct {
	ConnectionID int64 `json:"connectionId"`
}

func (h *MercadoLibreListingsAuditHandler) SyncFlaggedListings(w http.ResponseWriter, r *http.Request) {
	var req syncFlaggedMercadoLibreListingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.SyncFlaggedListings(r.Context(), syncApp.SyncFlaggedListingsInput{
		ConnectionID: req.ConnectionID,
	})
	if err != nil {
		if errors.Is(err, syncApp.ErrInvalidMercadoLibreConnection) {
			writeJSONError(w, http.StatusBadRequest, "connectionId is required and must be a valid connection")
			return
		}
		if errors.Is(err, syncApp.ErrNotMercadoLibreConnection) {
			writeJSONError(w, http.StatusBadRequest, "connectionId is not a MERCADOLIBRE connection")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

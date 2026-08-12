package http

import (
	"encoding/json"
	"errors"
	"net/http"

	syncApp "core-orchestrator/internal/application/sync"
)

type MercadoLibreCompatibilitiesHandler struct {
	service *syncApp.MercadoLibreCompatibilityService
}

func NewMercadoLibreCompatibilitiesHandler(service *syncApp.MercadoLibreCompatibilityService) *MercadoLibreCompatibilitiesHandler {
	return &MercadoLibreCompatibilitiesHandler{service: service}
}

type fixMercadoLibreCompatibilitiesRequest struct {
	ConnectionID int64 `json:"connectionId"`
	// Limit is optional — see syncApp.MercadoLibreCompatibilityService for
	// the default/max applied when omitted.
	Limit int `json:"limit,omitempty"`
}

// FixUnderReview finds ecom_channel_product_map rows for connectionId that
// are status=under_review with a vehicle fitment attached, and reports each
// one's compatibility data (brand/model/year, plus motor/position/side when
// available) to MercadoLibre so the listing can clear moderation. Processes
// up to `limit` rows per call — call again to continue a larger backlog.
func (h *MercadoLibreCompatibilitiesHandler) FixUnderReview(w http.ResponseWriter, r *http.Request) {
	var req fixMercadoLibreCompatibilitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.FixUnderReviewListings(r.Context(), syncApp.FixCompatibilitiesInput{
		ConnectionID: req.ConnectionID,
		Limit:        req.Limit,
	})
	if err != nil {
		if errors.Is(err, syncApp.ErrInvalidMercadoLibreConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		if errors.Is(err, syncApp.ErrNotMercadoLibreConnection) {
			writeJSONError(w, http.StatusBadRequest, "connectionId is not a MERCADOLIBRE connection")
			return
		}
		if errors.Is(err, syncApp.ErrMissingMercadoLibreSettings) {
			writeJSONError(w, http.StatusBadRequest, "missing required Mercado Libre connection settings")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

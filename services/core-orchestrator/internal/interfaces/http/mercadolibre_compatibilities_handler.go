package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	syncApp "core-orchestrator/internal/application/sync"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type MercadoLibreCompatibilitiesHandler struct {
	service *syncApp.MercadoLibreCompatibilityService
}

func NewMercadoLibreCompatibilitiesHandler(service *syncApp.MercadoLibreCompatibilityService) *MercadoLibreCompatibilitiesHandler {
	return &MercadoLibreCompatibilitiesHandler{service: service}
}

type fixMercadoLibreCompatibilitiesRequest struct {
	ConnectionID int64 `json:"connectionId"`
}

// FixUnderReview finds every ecom_channel_product_map row for connectionId
// that is not status=closed and has a vehicle fitment attached, and reports
// each one's compatibility data (brand/model/year, plus motor/position/side
// when available) to MercadoLibre so the listing can clear moderation. All
// eligible rows are processed in this single call.
func (h *MercadoLibreCompatibilitiesHandler) FixUnderReview(w http.ResponseWriter, r *http.Request) {
	var req fixMercadoLibreCompatibilitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.FixUnderReviewListings(r.Context(), syncApp.FixCompatibilitiesInput{
		ConnectionID: req.ConnectionID,
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
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error fixing under-review compatibilities: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type diagnoseMercadoLibreCompatibilitiesRequest struct {
	ConnectionID int64  `json:"connectionId"`
	ItemID       string `json:"itemId"`
}

// Diagnose is a read-only troubleshooting endpoint: for one MercadoLibre item
// it reports the catalog/user-product linkage, whether the category supports
// vehicle compatibilities, and the raw compatibility payloads from both the
// item and (when linked) the user-product endpoint — so it's visible whether
// CopyCompatibilities found nothing because the data is catalog-managed
// (brand totals only, since 2026-07-15) or lives on the user product. Nothing
// is written to MercadoLibre or the local database.
func (h *MercadoLibreCompatibilitiesHandler) Diagnose(w http.ResponseWriter, r *http.Request) {
	var req diagnoseMercadoLibreCompatibilitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.DiagnoseItem(r.Context(), req.ConnectionID, req.ItemID)
	if err != nil {
		if errors.Is(err, syncApp.ErrInvalidMercadoLibreConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		if errors.Is(err, syncApp.ErrInvalidCompatibilityCopyItems) {
			writeJSONError(w, http.StatusBadRequest, "itemId is required")
			return
		}
		if errors.Is(err, syncApp.ErrMissingMercadoLibreSettings) {
			writeJSONError(w, http.StatusBadRequest, "missing required Mercado Libre connection settings")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error diagnosing compatibilities: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type copyMercadoLibreCompatibilitiesRequest struct {
	ConnectionID int64 `json:"connectionId"`
	// SourceItemID is the MercadoLibre item id whose compatibilities are
	// downloaded — nothing is written back to MercadoLibre.
	SourceItemID string `json:"sourceItemId"`
	// SKU resolves the local ecom_products row the downloaded
	// compatibilities are linked to.
	SKU string `json:"sku"`
}

// CopyCompatibilities downloads every compatibility MercadoLibre reports for
// sourceItemId and links each one to the local product sku resolves to,
// using connectionId to resolve which MercadoLibre account's token to call
// with. New local rows are stamped with the authenticated user as
// created_by.
func (h *MercadoLibreCompatibilitiesHandler) CopyCompatibilities(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req copyMercadoLibreCompatibilitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.CopyCompatibilities(r.Context(), syncApp.CopyCompatibilitiesInput{
		ConnectionID: req.ConnectionID,
		SourceItemID: req.SourceItemID,
		SKU:          req.SKU,
		ActorID:      user.ID,
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
		if errors.Is(err, syncApp.ErrInvalidCompatibilityCopyItems) {
			writeJSONError(w, http.StatusBadRequest, "sourceItemId and sku are required")
			return
		}
		if errors.Is(err, syncApp.ErrInvalidCompatibilityCopyActor) {
			writeJSONError(w, http.StatusBadRequest, "could not resolve the authenticated user to stamp local compatibilities with")
			return
		}
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("product not found: %v", err))
			return
		}
		if errors.Is(err, syncApp.ErrMissingMercadoLibreSettings) {
			writeJSONError(w, http.StatusBadRequest, "missing required Mercado Libre connection settings")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error copying compatibilities: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

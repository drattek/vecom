package http

import (
	"encoding/json"
	"net/http"
	"strings"

	syncApp "core-orchestrator/internal/application/sync"
)

// MercadoLibreCategoryAttributesDebugHandler is a TEMPORARY diagnostic
// endpoint: given a MercadoLibre categoryId, returns exactly what
// GET /categories/{id}/attributes reports for it (tags.required/
// tags.hidden/tags.read_only, value_type, values) — used to check why a
// specific attribute isn't coming back as a settable custom_attribute from
// channel_attribute_values.Service.ProvisionCategoryAttributes (see
// sync.MercadoLibreListingsAuditService). Public MercadoLibre endpoint
// underneath, no connection/access token needed. Remove once the
// investigation it's for is done.
type MercadoLibreCategoryAttributesDebugHandler struct {
	categoryPredictorService *syncApp.MercadoLibreCategoryPredictorService
}

func NewMercadoLibreCategoryAttributesDebugHandler(categoryPredictorService *syncApp.MercadoLibreCategoryPredictorService) *MercadoLibreCategoryAttributesDebugHandler {
	return &MercadoLibreCategoryAttributesDebugHandler{categoryPredictorService: categoryPredictorService}
}

// GetCategoryAttributes handles GET .../debug/category-attributes?categoryId=MLM163957.
func (h *MercadoLibreCategoryAttributesDebugHandler) GetCategoryAttributes(w http.ResponseWriter, r *http.Request) {
	categoryID := strings.TrimSpace(r.URL.Query().Get("categoryId"))
	if categoryID == "" {
		writeJSONError(w, http.StatusBadRequest, "categoryId query param is required")
		return
	}

	attributes, err := h.categoryPredictorService.GetCategoryAttributes(r.Context(), categoryID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(attributes)
}

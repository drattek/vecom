package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	categoryImportApp "core-orchestrator/internal/application/category_import"
	productCategorySelectionApp "core-orchestrator/internal/application/product_category_selection"
	syncApp "core-orchestrator/internal/application/sync"
	"github.com/go-chi/chi/v5"
)

// ProductCategorySelectionHandler serves the "Sincronización" tab's category
// picker: POST /api/products/{productId}/details/sync/{connectionId}/category
// records which external category the user picked for a product on a
// connection that has no listing yet (see ADR 0005).
type ProductCategorySelectionHandler struct {
	service *productCategorySelectionApp.Service
}

func NewProductCategorySelectionHandler(service *productCategorySelectionApp.Service) *ProductCategorySelectionHandler {
	return &ProductCategorySelectionHandler{service: service}
}

type selectProductCategoryRequest struct {
	ExternalCategoryID string `json:"externalCategoryId"`
}

func (h *ProductCategorySelectionHandler) Select(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	connectionID, err := strconv.ParseInt(chi.URLParam(r, "connectionId"), 10, 64)
	if err != nil || connectionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid connection id")
		return
	}

	var req selectProductCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.Select(r.Context(), productID, connectionID, req.ExternalCategoryID, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, productCategorySelectionApp.ErrInvalidInput):
			writeJSONError(w, http.StatusBadRequest, "productId, connectionId and externalCategoryId are required")
		case errors.Is(err, productCategorySelectionApp.ErrProductNotFound):
			writeJSONError(w, http.StatusNotFound, "product not found")
		case errors.Is(err, productCategorySelectionApp.ErrConnectionNotFound):
			writeJSONError(w, http.StatusNotFound, "channel connection not found")
		case errors.Is(err, categoryImportApp.ErrChannelNotSupported):
			writeJSONError(w, http.StatusConflict, "this channel does not support category selection")
		case errors.Is(err, categoryImportApp.ErrExternalCategoryNotFound):
			writeJSONError(w, http.StatusNotFound, "external category not found")
		case errors.Is(err, categoryImportApp.ErrNotLeaf):
			writeJSONError(w, http.StatusUnprocessableEntity, "only leaf categories can be selected")
		case errors.Is(err, syncApp.ErrMissingMercadoLibreSiteID):
			writeJSONError(w, http.StatusBadRequest, "the MercadoLibre connection has no site_id configured")
		case errors.Is(err, syncApp.ErrMissingMercadoLibreSettings),
			errors.Is(err, syncApp.ErrInvalidMercadoLibreConnection),
			errors.Is(err, syncApp.ErrInvalidExpirationTimeFormat):
			writeJSONError(w, http.StatusBadRequest, "the MercadoLibre connection is not fully configured or authenticated")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSONResponse(w, result)
}

package http

import (
	"errors"
	"net/http"

	productSyncApp "core-orchestrator/internal/application/product_sync"
)

// ProductSyncHandler serves the product detail page's read-only
// "Sincronización" view (GET .../details/sync): one box per active channel
// connection with whatever the product already has synced there.
type ProductSyncHandler struct {
	service *productSyncApp.Service
}

func NewProductSyncHandler(service *productSyncApp.Service) *ProductSyncHandler {
	return &ProductSyncHandler{service: service}
}

func (h *ProductSyncHandler) GetSync(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	sync, err := h.service.GetSync(r.Context(), productID)
	if err != nil {
		switch {
		case errors.Is(err, productSyncApp.ErrInvalidInput):
			writeJSONError(w, http.StatusBadRequest, "invalid product id")
		case errors.Is(err, productSyncApp.ErrProductNotFound):
			writeJSONError(w, http.StatusNotFound, "product not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSONResponse(w, sync)
}

package http

import (
	"errors"
	"net/http"
	"strconv"

	checklistApp "core-orchestrator/internal/application/product_attributes_checklist"
)

// ProductAttributeChecklistHandler serves the per-connection attribute checklist
// for a product (GET .../details/attributes/checklist?connectionId=N).
type ProductAttributeChecklistHandler struct {
	service *checklistApp.Service
}

func NewProductAttributeChecklistHandler(service *checklistApp.Service) *ProductAttributeChecklistHandler {
	return &ProductAttributeChecklistHandler{service: service}
}

func (h *ProductAttributeChecklistHandler) GetChecklist(w http.ResponseWriter, r *http.Request) {
	productID, ok := parseDetailProductID(w, r)
	if !ok {
		return
	}

	connectionID, err := strconv.ParseInt(r.URL.Query().Get("connectionId"), 10, 64)
	if err != nil || connectionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "connectionId is required")
		return
	}

	checklist, err := h.service.GetChecklist(r.Context(), productID, connectionID)
	if err != nil {
		switch {
		case errors.Is(err, checklistApp.ErrInvalidInput):
			writeJSONError(w, http.StatusBadRequest, "invalid checklist input")
		case errors.Is(err, checklistApp.ErrProductNotFound):
			writeJSONError(w, http.StatusNotFound, "product not found")
		case errors.Is(err, checklistApp.ErrConnectionNotFound):
			writeJSONError(w, http.StatusNotFound, "connection not found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSONResponse(w, checklist)
}

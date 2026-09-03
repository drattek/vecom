package http

import (
	"encoding/json"
	"errors"
	"net/http"

	importApp "core-orchestrator/internal/application/product_image_import"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type importProductImagesProductRequest struct {
	SKU  string   `json:"sku"`
	URLs []string `json:"urls"`
}

type importProductImagesRequest struct {
	StorageDiskID int64                               `json:"storageDiskId"`
	Products      []importProductImagesProductRequest `json:"products"`
}

type ProductImageImportHandler struct {
	service *importApp.Service
}

func NewProductImageImportHandler(service *importApp.Service) *ProductImageImportHandler {
	return &ProductImageImportHandler{service: service}
}

func (h *ProductImageImportHandler) Import(w http.ResponseWriter, r *http.Request) {
	_, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req importProductImagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	products := make([]importApp.ProductImageURLsInput, 0, len(req.Products))
	for _, p := range req.Products {
		products = append(products, importApp.ProductImageURLsInput{
			SKU:  p.SKU,
			URLs: p.URLs,
		})
	}

	result, err := h.service.Import(r.Context(), importApp.ImportProductImagesInput{
		StorageDiskID: req.StorageDiskID,
		Products:      products,
	})
	if err != nil {
		if errors.Is(err, importApp.ErrInvalidImportPayload) {
			writeJSONError(w, http.StatusBadRequest, "storageDiskId and products are required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"core-orchestrator/internal/application/products"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type ProductHandler struct {
	service *products.ProductService
}

type upsertProductRequest struct {
	SKU              string  `json:"sku"`
	PartNumber       string  `json:"partNumber"`
	Name             string  `json:"name"`
	Description      *string `json:"description,omitempty"`
	ShortDescription *string `json:"shortDescription,omitempty"`
	BrandID          *int64  `json:"brandId,omitempty"`
	CategoryID       *int64  `json:"categoryId,omitempty"`
	ProductType      string  `json:"productType"`
	Status           *string `json:"status,omitempty"`
	IsSellable       bool    `json:"isSellable"`
	IsStockable      bool    `json:"isStockable"`
	SourceID         int64   `json:"sourceId"`
}

func NewProductHandler(service *products.ProductService) *ProductHandler {
	return &ProductHandler{service: service}
}

func (h *ProductHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	// Default values
	offset := 0
	pageSize := 10

	// Parse offset
	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsedOffset
		}
	}

	// Parse pageSize
	if pageSizeStr != "" {
		if parsedPageSize, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsedPageSize
		}
	}

	// sortBy/sortDir are optional; an unrecognized sortBy falls back to the
	// default order (see ProductRepository.FindPaginated). search filters by
	// sku / part_number / name; empty means no filter.
	sortBy := r.URL.Query().Get("sortBy")
	sortDir := r.URL.Query().Get("sortDir")
	search := r.URL.Query().Get("search")

	// Get products from service
	result, err := h.service.GetPaginatedProducts(r.Context(), offset, pageSize, sortBy, sortDir, search)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *ProductHandler) GetProductByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseProductID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	product, err := h.service.GetProductByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			writeJSONError(w, http.StatusNotFound, "product not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateProductInput{
		SKU:              req.SKU,
		PartNumber:       req.PartNumber,
		Name:             req.Name,
		Description:      req.Description,
		ShortDescription: req.ShortDescription,
		BrandID:          req.BrandID,
		CategoryID:       req.CategoryID,
		ProductType:      req.ProductType,
		Status:           req.Status,
		IsSellable:       req.IsSellable,
		IsStockable:      req.IsStockable,
		SourceID:         req.SourceID,
		CreatedBy:        user.ID,
	}

	product, err := h.service.CreateProduct(r.Context(), input)
	if err != nil {
		if errors.Is(err, products.ErrInvalidProductPayload) {
			writeJSONError(w, http.StatusBadRequest, "sku, partNumber, name and sourceId are required; productType/status must be valid")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}

func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseProductID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req upsertProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateProductInput{
		SKU:              req.SKU,
		PartNumber:       req.PartNumber,
		Name:             req.Name,
		Description:      req.Description,
		ShortDescription: req.ShortDescription,
		BrandID:          req.BrandID,
		CategoryID:       req.CategoryID,
		ProductType:      req.ProductType,
		Status:           req.Status,
		IsSellable:       req.IsSellable,
		IsStockable:      req.IsStockable,
		SourceID:         req.SourceID,
		UpdatedBy:        user.ID,
	}

	product, err := h.service.UpdateProduct(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, products.ErrInvalidProductPayload) {
			writeJSONError(w, http.StatusBadRequest, "sku, partNumber, name and sourceId are required; productType/status must be valid")
			return
		}
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			writeJSONError(w, http.StatusNotFound, "product not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parseProductID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	err = h.service.DeleteProduct(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			writeJSONError(w, http.StatusNotFound, "product not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseProductID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

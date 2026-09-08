package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	brandsApp "core-orchestrator/internal/application/brands"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type BrandHandler struct {
	service *brandsApp.BrandService
}

type upsertBrandRequest struct {
	Name string `json:"name"`
}

func NewBrandHandler(service *brandsApp.BrandService) *BrandHandler {
	return &BrandHandler{service: service}
}

func (h *BrandHandler) GetBrands(w http.ResponseWriter, r *http.Request) {
	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset := 0
	pageSize := 10

	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsedOffset
		}
	}

	if pageSizeStr != "" {
		if parsedPageSize, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsedPageSize
		}
	}

	result, err := h.service.GetPaginatedBrands(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *BrandHandler) GetBrandOptions(w http.ResponseWriter, r *http.Request) {
	options, err := h.service.GetBrandOptions(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"brands": options})
}

func (h *BrandHandler) GetBrandByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseBrandID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid brand id")
		return
	}

	brand, err := h.service.GetBrandByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrBrandNotFound) {
			writeJSONError(w, http.StatusNotFound, "brand not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(brand)
}

func (h *BrandHandler) CreateBrand(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertBrandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateBrandInput{
		Name:      req.Name,
		CreatedBy: user.ID,
	}

	brand, err := h.service.CreateBrand(r.Context(), input)
	if err != nil {
		if errors.Is(err, brandsApp.ErrInvalidBrandPayload) {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(brand)
}

func (h *BrandHandler) UpdateBrand(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseBrandID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid brand id")
		return
	}

	var req upsertBrandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateBrandInput{
		Name:      req.Name,
		UpdatedBy: user.ID,
	}

	brand, err := h.service.UpdateBrand(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, brandsApp.ErrInvalidBrandPayload) {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrBrandNotFound) {
			writeJSONError(w, http.StatusNotFound, "brand not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(brand)
}

type bulkAssignBrandsItem struct {
	SKU   string `json:"sku"`
	Brand string `json:"brand"`
}

func (h *BrandHandler) BulkAssignBrands(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req []bulkAssignBrandsItem
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req) == 0 {
		writeJSONError(w, http.StatusBadRequest, "request body must be a non-empty array")
		return
	}

	items := make([]brandsApp.BulkBrandAssignmentItem, len(req))
	for i, item := range req {
		items[i] = brandsApp.BulkBrandAssignmentItem{
			SKU:   item.SKU,
			Brand: item.Brand,
		}
	}

	results, err := h.service.BulkAssignBrands(r.Context(), items, user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func (h *BrandHandler) DeleteBrand(w http.ResponseWriter, r *http.Request) {
	id, err := parseBrandID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid brand id")
		return
	}

	err = h.service.DeleteBrand(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrBrandNotFound) {
			writeJSONError(w, http.StatusNotFound, "brand not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseBrandID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	inventoryApp "core-orchestrator/internal/application/inventory"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type BranchHandler struct {
	service *inventoryApp.BranchService
}

type upsertBranchRequest struct {
	Name string `json:"name"`
}

func NewBranchHandler(service *inventoryApp.BranchService) *BranchHandler {
	return &BranchHandler{service: service}
}

func (h *BranchHandler) GetBranches(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedBranches(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *BranchHandler) GetBranchByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseBranchID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid branch id")
		return
	}

	branch, err := h.service.GetBranchByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrBranchNotFound) {
			writeJSONError(w, http.StatusNotFound, "branch not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(branch)
}

func (h *BranchHandler) CreateBranch(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateBranchInput{
		Name:      req.Name,
		CreatedBy: user.ID,
	}

	branch, err := h.service.CreateBranch(input)
	if err != nil {
		if errors.Is(err, inventoryApp.ErrInvalidInventoryPayload) {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(branch)
}

func (h *BranchHandler) UpdateBranch(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseBranchID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid branch id")
		return
	}

	var req upsertBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateBranchInput{
		Name:      req.Name,
		UpdatedBy: user.ID,
	}

	branch, err := h.service.UpdateBranch(id, input)
	if err != nil {
		if errors.Is(err, inventoryApp.ErrInvalidInventoryPayload) {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrBranchNotFound) {
			writeJSONError(w, http.StatusNotFound, "branch not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(branch)
}

func (h *BranchHandler) DeleteBranch(w http.ResponseWriter, r *http.Request) {
	id, err := parseBranchID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid branch id")
		return
	}

	err = h.service.DeleteBranch(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrBranchNotFound) {
			writeJSONError(w, http.StatusNotFound, "branch not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseBranchID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

// WarehouseHandler
type WarehouseHandler struct {
	service *inventoryApp.WarehouseService
}

type upsertWarehouseRequest struct {
	Name     string `json:"name"`
	BranchID int64  `json:"branchId"`
}

func NewWarehouseHandler(service *inventoryApp.WarehouseService) *WarehouseHandler {
	return &WarehouseHandler{service: service}
}

func (h *WarehouseHandler) GetWarehouses(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedWarehouses(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *WarehouseHandler) GetWarehouseByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseWarehouseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid warehouse id")
		return
	}

	warehouse, err := h.service.GetWarehouseByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrWarehouseNotFound) {
			writeJSONError(w, http.StatusNotFound, "warehouse not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(warehouse)
}

func (h *WarehouseHandler) CreateWarehouse(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertWarehouseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateWarehouseInput{
		Name:      req.Name,
		BranchID:  req.BranchID,
		CreatedBy: user.ID,
	}

	warehouse, err := h.service.CreateWarehouse(input)
	if err != nil {
		if errors.Is(err, inventoryApp.ErrInvalidInventoryPayload) {
			writeJSONError(w, http.StatusBadRequest, "name and branchId are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(warehouse)
}

func (h *WarehouseHandler) UpdateWarehouse(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseWarehouseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid warehouse id")
		return
	}

	var req upsertWarehouseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateWarehouseInput{
		Name:      req.Name,
		BranchID:  req.BranchID,
		UpdatedBy: user.ID,
	}

	warehouse, err := h.service.UpdateWarehouse(id, input)
	if err != nil {
		if errors.Is(err, inventoryApp.ErrInvalidInventoryPayload) {
			writeJSONError(w, http.StatusBadRequest, "name and branchId are required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrWarehouseNotFound) {
			writeJSONError(w, http.StatusNotFound, "warehouse not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(warehouse)
}

func (h *WarehouseHandler) DeleteWarehouse(w http.ResponseWriter, r *http.Request) {
	id, err := parseWarehouseID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid warehouse id")
		return
	}

	err = h.service.DeleteWarehouse(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrWarehouseNotFound) {
			writeJSONError(w, http.StatusNotFound, "warehouse not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseWarehouseID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

// ProductStockHandler
type ProductStockHandler struct {
	service *inventoryApp.ProductStockService
}

type upsertProductStockRequest struct {
	ProductID    int64 `json:"productId"`
	BranchID     int64 `json:"branchId"`
	WarehouseID  int64 `json:"warehouseId"`
	AvailableQty int   `json:"availableQty"`
}

func NewProductStockHandler(service *inventoryApp.ProductStockService) *ProductStockHandler {
	return &ProductStockHandler{service: service}
}

func (h *ProductStockHandler) GetProductStock(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedProductStock(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *ProductStockHandler) GetProductStockByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseProductStockID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid product stock id")
		return
	}

	stock, err := h.service.GetProductStockByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductStockNotFound) {
			writeJSONError(w, http.StatusNotFound, "product stock not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stock)
}

func (h *ProductStockHandler) CreateProductStock(w http.ResponseWriter, r *http.Request) {
	_, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertProductStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateProductStockInput{
		ProductID:    req.ProductID,
		BranchID:     req.BranchID,
		WarehouseID:  req.WarehouseID,
		AvailableQty: req.AvailableQty,
	}

	stock, err := h.service.CreateProductStock(input)
	if err != nil {
		if errors.Is(err, inventoryApp.ErrInvalidInventoryPayload) {
			writeJSONError(w, http.StatusBadRequest, "productId, branchId, and warehouseId are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(stock)
}

func (h *ProductStockHandler) UpdateProductStock(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseProductStockID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid product stock id")
		return
	}

	var req struct {
		AvailableQty int `json:"availableQty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateProductStockInput{
		AvailableQty: req.AvailableQty,
		UpdatedBy:    user.ID,
	}

	stock, err := h.service.UpdateProductStock(id, input)
	if err != nil {
		if errors.Is(err, inventoryApp.ErrInvalidInventoryPayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid payload")
			return
		}
		if errors.Is(err, mysqlInfra.ErrProductStockNotFound) {
			writeJSONError(w, http.StatusNotFound, "product stock not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stock)
}

func parseProductStockID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

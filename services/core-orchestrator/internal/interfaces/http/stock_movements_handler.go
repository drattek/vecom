package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	stockMovementsApp "core-orchestrator/internal/application/stock_movements"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type upsertStockMovementRequest struct {
	ProductID      int64  `json:"productId"`
	BranchID       int64  `json:"branchId"`
	WarehouseID    int64  `json:"warehouseId"`
	MovementType   string `json:"movementType"`
	QuantityBefore int    `json:"quantityBefore"`
	QuantityChange int    `json:"quantityChange"`
	QuantityAfter  int    `json:"quantityAfter"`
}

type StockMovementsHandler struct {
	service *stockMovementsApp.StockMovementsService
}

func NewStockMovementsHandler(service *stockMovementsApp.StockMovementsService) *StockMovementsHandler {
	return &StockMovementsHandler{service: service}
}

func (h *StockMovementsHandler) GetStockMovements(w http.ResponseWriter, r *http.Request) {
	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset, pageSize := 0, 10
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}
	if pageSizeStr != "" {
		if parsed, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsed
		}
	}

	result, err := h.service.GetPaginatedMovements(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *StockMovementsHandler) GetMovementByID(w http.ResponseWriter, r *http.Request) {
	id := parseStockMovementID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid movement id")
		return
	}

	movement, err := h.service.GetMovementByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrStockMovementNotFound) {
			writeJSONError(w, http.StatusNotFound, "movement not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(movement)
}

func (h *StockMovementsHandler) GetMovementsByProduct(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "productId")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	offsetStr := r.URL.Query().Get("offset")
	pageSizeStr := r.URL.Query().Get("pageSize")

	offset, pageSize := 0, 10
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}
	if pageSizeStr != "" {
		if parsed, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsed
		}
	}

	result, err := h.service.GetMovementsByProduct(r.Context(), productID, offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *StockMovementsHandler) CreateMovement(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertStockMovementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateStockMovementInput{
		ProductID:      req.ProductID,
		BranchID:       req.BranchID,
		WarehouseID:    req.WarehouseID,
		MovementType:   req.MovementType,
		QuantityBefore: req.QuantityBefore,
		QuantityChange: req.QuantityChange,
		QuantityAfter:  req.QuantityAfter,
		UpdatedBy:      user.ID,
	}

	movement, err := h.service.CreateMovement(r.Context(), input)
	if err != nil {
		if errors.Is(err, stockMovementsApp.ErrInvalidStockMovement) {
			writeJSONError(w, http.StatusBadRequest, "productId, branchId and warehouseId are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(movement)
}

func (h *StockMovementsHandler) DeleteMovement(w http.ResponseWriter, r *http.Request) {
	id := parseStockMovementID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid movement id")
		return
	}

	err := h.service.DeleteMovement(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrStockMovementNotFound) {
			writeJSONError(w, http.StatusNotFound, "movement not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseStockMovementID(r *http.Request) int64 {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}

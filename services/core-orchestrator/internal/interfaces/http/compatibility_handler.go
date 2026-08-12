package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	compatibilityApp "core-orchestrator/internal/application/compatibility"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

func parsePaginationParams(r *http.Request) (offset, pageSize int) {
	offset, pageSize = 0, 10

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}
	if pageSizeStr := r.URL.Query().Get("pageSize"); pageSizeStr != "" {
		if parsed, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = parsed
		}
	}

	return offset, pageSize
}

func parseIDParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, name), 10, 64)
}

// EquipmentTypeHandler

type EquipmentTypeHandler struct {
	service *compatibilityApp.EquipmentTypeService
}

type upsertEquipmentTypeRequest struct {
	Name string `json:"name"`
}

func NewEquipmentTypeHandler(service *compatibilityApp.EquipmentTypeService) *EquipmentTypeHandler {
	return &EquipmentTypeHandler{service: service}
}

func (h *EquipmentTypeHandler) GetEquipmentTypes(w http.ResponseWriter, r *http.Request) {
	offset, pageSize := parsePaginationParams(r)

	result, err := h.service.GetPaginatedEquipmentTypes(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *EquipmentTypeHandler) GetEquipmentTypeByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid equipment type id")
		return
	}

	equipmentType, err := h.service.GetEquipmentTypeByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrEquipmentTypeNotFound) {
			writeJSONError(w, http.StatusNotFound, "equipment type not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(equipmentType)
}

func (h *EquipmentTypeHandler) CreateEquipmentType(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertEquipmentTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	equipmentType, err := h.service.CreateEquipmentType(mysqlInfra.CreateEquipmentTypeInput{
		Name:      req.Name,
		CreatedBy: user.ID,
	})
	if err != nil {
		if errors.Is(err, compatibilityApp.ErrInvalidEquipmentType) {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(equipmentType)
}

func (h *EquipmentTypeHandler) UpdateEquipmentType(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid equipment type id")
		return
	}

	var req upsertEquipmentTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	equipmentType, err := h.service.UpdateEquipmentType(id, mysqlInfra.UpdateEquipmentTypeInput{
		Name:      req.Name,
		UpdatedBy: user.ID,
	})
	if err != nil {
		if errors.Is(err, compatibilityApp.ErrInvalidEquipmentType) {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrEquipmentTypeNotFound) {
			writeJSONError(w, http.StatusNotFound, "equipment type not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(equipmentType)
}

func (h *EquipmentTypeHandler) DeleteEquipmentType(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid equipment type id")
		return
	}

	if err := h.service.DeleteEquipmentType(id); err != nil {
		if errors.Is(err, mysqlInfra.ErrEquipmentTypeNotFound) {
			writeJSONError(w, http.StatusNotFound, "equipment type not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// VehicleFitmentHandler

type VehicleFitmentHandler struct {
	service *compatibilityApp.VehicleFitmentService
}

type upsertVehicleFitmentRequest struct {
	BrandID   int64  `json:"brandId"`
	Model     string `json:"model"`
	YearStart int    `json:"yearStart"`
	YearEnd   *int   `json:"yearEnd"`
}

func NewVehicleFitmentHandler(service *compatibilityApp.VehicleFitmentService) *VehicleFitmentHandler {
	return &VehicleFitmentHandler{service: service}
}

func (h *VehicleFitmentHandler) GetVehicleFitments(w http.ResponseWriter, r *http.Request) {
	offset, pageSize := parsePaginationParams(r)

	result, err := h.service.GetPaginatedFitments(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *VehicleFitmentHandler) GetVehicleFitmentByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid vehicle fitment id")
		return
	}

	fitment, err := h.service.GetFitmentByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrVehicleFitmentNotFound) {
			writeJSONError(w, http.StatusNotFound, "vehicle fitment not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(fitment)
}

func (h *VehicleFitmentHandler) CreateVehicleFitment(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertVehicleFitmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fitment, err := h.service.CreateFitment(mysqlInfra.CreateVehicleFitmentInput{
		BrandID:   req.BrandID,
		Model:     req.Model,
		YearStart: req.YearStart,
		YearEnd:   req.YearEnd,
		CreatedBy: user.ID,
	})
	if err != nil {
		if errors.Is(err, compatibilityApp.ErrInvalidVehicleFitment) {
			writeJSONError(w, http.StatusBadRequest, "brandId, model and yearStart are required, and yearEnd cannot be before yearStart")
			return
		}
		if errors.Is(err, mysqlInfra.ErrVehicleFitmentAlreadyExists) {
			writeJSONError(w, http.StatusConflict, "vehicle fitment already exists")
			return
		}
		if errors.Is(err, mysqlInfra.ErrVehicleFitmentInvalidReference) {
			writeJSONError(w, http.StatusBadRequest, "brand does not exist")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fitment)
}

func (h *VehicleFitmentHandler) UpdateVehicleFitment(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid vehicle fitment id")
		return
	}

	var req upsertVehicleFitmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fitment, err := h.service.UpdateFitment(id, mysqlInfra.UpdateVehicleFitmentInput{
		BrandID:   req.BrandID,
		Model:     req.Model,
		YearStart: req.YearStart,
		YearEnd:   req.YearEnd,
		UpdatedBy: user.ID,
	})
	if err != nil {
		if errors.Is(err, compatibilityApp.ErrInvalidVehicleFitment) {
			writeJSONError(w, http.StatusBadRequest, "brandId, model and yearStart are required, and yearEnd cannot be before yearStart")
			return
		}
		if errors.Is(err, mysqlInfra.ErrVehicleFitmentAlreadyExists) {
			writeJSONError(w, http.StatusConflict, "vehicle fitment already exists")
			return
		}
		if errors.Is(err, mysqlInfra.ErrVehicleFitmentInvalidReference) {
			writeJSONError(w, http.StatusBadRequest, "brand does not exist")
			return
		}
		if errors.Is(err, mysqlInfra.ErrVehicleFitmentNotFound) {
			writeJSONError(w, http.StatusNotFound, "vehicle fitment not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(fitment)
}

type bulkImportVehicleFitmentItem struct {
	Brand     string `json:"brand"`
	Model     string `json:"model"`
	YearStart int    `json:"yearStart"`
	YearEnd   *int   `json:"yearEnd"`
	SKU       string `json:"sku"`
	Motor     string `json:"motor"`
	Position  string `json:"position"`
	Side      string `json:"side"`
}

func (h *VehicleFitmentHandler) BulkImportVehicleFitments(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req []bulkImportVehicleFitmentItem
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req) == 0 {
		writeJSONError(w, http.StatusBadRequest, "request body must be a non-empty array")
		return
	}

	items := make([]compatibilityApp.BulkVehicleFitmentItem, len(req))
	for i, item := range req {
		items[i] = compatibilityApp.BulkVehicleFitmentItem{
			Brand:     item.Brand,
			Model:     item.Model,
			YearStart: item.YearStart,
			YearEnd:   item.YearEnd,
			SKU:       item.SKU,
			Motor:     item.Motor,
			Position:  item.Position,
			Side:      item.Side,
		}
	}

	results, err := h.service.BulkImportFitments(items, user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func (h *VehicleFitmentHandler) ResolvePendingFitments(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	results, err := h.service.ResolvePendingFitments(user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func (h *VehicleFitmentHandler) DeleteVehicleFitment(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid vehicle fitment id")
		return
	}

	if err := h.service.DeleteFitment(id); err != nil {
		if errors.Is(err, mysqlInfra.ErrVehicleFitmentNotFound) {
			writeJSONError(w, http.StatusNotFound, "vehicle fitment not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// EquipmentFitmentHandler

type EquipmentFitmentHandler struct {
	service *compatibilityApp.EquipmentFitmentService
}

type upsertEquipmentFitmentRequest struct {
	BrandID         int64   `json:"brandId"`
	EquipmentTypeID int64   `json:"equipmentTypeId"`
	Model           *string `json:"model"`
	Serie           *string `json:"serie"`
}

func NewEquipmentFitmentHandler(service *compatibilityApp.EquipmentFitmentService) *EquipmentFitmentHandler {
	return &EquipmentFitmentHandler{service: service}
}

func (h *EquipmentFitmentHandler) GetEquipmentFitments(w http.ResponseWriter, r *http.Request) {
	offset, pageSize := parsePaginationParams(r)

	result, err := h.service.GetPaginatedFitments(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *EquipmentFitmentHandler) GetEquipmentFitmentByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid equipment fitment id")
		return
	}

	fitment, err := h.service.GetFitmentByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrEquipmentFitmentNotFound) {
			writeJSONError(w, http.StatusNotFound, "equipment fitment not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(fitment)
}

func (h *EquipmentFitmentHandler) CreateEquipmentFitment(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertEquipmentFitmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fitment, err := h.service.CreateFitment(mysqlInfra.CreateEquipmentFitmentInput{
		BrandID:         req.BrandID,
		EquipmentTypeID: req.EquipmentTypeID,
		Model:           req.Model,
		Serie:           req.Serie,
		CreatedBy:       user.ID,
	})
	if err != nil {
		if errors.Is(err, compatibilityApp.ErrInvalidEquipmentFitment) {
			writeJSONError(w, http.StatusBadRequest, "brandId and equipmentTypeId are required, and either model or serie must be provided")
			return
		}
		if errors.Is(err, mysqlInfra.ErrEquipmentFitmentInvalidReference) {
			writeJSONError(w, http.StatusBadRequest, "brand or equipment type does not exist")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fitment)
}

func (h *EquipmentFitmentHandler) UpdateEquipmentFitment(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid equipment fitment id")
		return
	}

	var req upsertEquipmentFitmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fitment, err := h.service.UpdateFitment(id, mysqlInfra.UpdateEquipmentFitmentInput{
		BrandID:         req.BrandID,
		EquipmentTypeID: req.EquipmentTypeID,
		Model:           req.Model,
		Serie:           req.Serie,
		UpdatedBy:       user.ID,
	})
	if err != nil {
		if errors.Is(err, compatibilityApp.ErrInvalidEquipmentFitment) {
			writeJSONError(w, http.StatusBadRequest, "brandId and equipmentTypeId are required, and either model or serie must be provided")
			return
		}
		if errors.Is(err, mysqlInfra.ErrEquipmentFitmentInvalidReference) {
			writeJSONError(w, http.StatusBadRequest, "brand or equipment type does not exist")
			return
		}
		if errors.Is(err, mysqlInfra.ErrEquipmentFitmentNotFound) {
			writeJSONError(w, http.StatusNotFound, "equipment fitment not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(fitment)
}

func (h *EquipmentFitmentHandler) DeleteEquipmentFitment(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid equipment fitment id")
		return
	}

	if err := h.service.DeleteFitment(id); err != nil {
		if errors.Is(err, mysqlInfra.ErrEquipmentFitmentNotFound) {
			writeJSONError(w, http.StatusNotFound, "equipment fitment not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ProductVehicleCompatibilityHandler

type ProductVehicleCompatibilityHandler struct {
	service *compatibilityApp.ProductVehicleCompatibilityService
}

type createProductVehicleCompatibilityRequest struct {
	ProductID        int64  `json:"productId"`
	VehicleFitmentID int64  `json:"vehicleFitmentId"`
	Motor            string `json:"motor"`
	Position         string `json:"position"`
	Side             string `json:"side"`
}

func NewProductVehicleCompatibilityHandler(service *compatibilityApp.ProductVehicleCompatibilityService) *ProductVehicleCompatibilityHandler {
	return &ProductVehicleCompatibilityHandler{service: service}
}

func (h *ProductVehicleCompatibilityHandler) GetByProduct(w http.ResponseWriter, r *http.Request) {
	productID, err := parseIDParam(r, "productId")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	offset, pageSize := parsePaginationParams(r)

	result, err := h.service.GetByProduct(productID, offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *ProductVehicleCompatibilityHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createProductVehicleCompatibilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	compat, err := h.service.Create(mysqlInfra.CreateProductVehicleCompatibilityInput{
		ProductID:        req.ProductID,
		VehicleFitmentID: req.VehicleFitmentID,
		Motor:            strings.TrimSpace(req.Motor),
		Position:         strings.TrimSpace(req.Position),
		Side:             strings.TrimSpace(req.Side),
		CreatedBy:        user.ID,
	})
	if err != nil {
		if errors.Is(err, compatibilityApp.ErrInvalidProductCompatibility) {
			writeJSONError(w, http.StatusBadRequest, "productId and vehicleFitmentId are required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrProductVehicleCompatibilityAlreadyExists) {
			writeJSONError(w, http.StatusConflict, "product is already linked to this vehicle fitment")
			return
		}
		if errors.Is(err, mysqlInfra.ErrProductVehicleCompatibilityInvalidReference) {
			writeJSONError(w, http.StatusBadRequest, "product or vehicle fitment does not exist")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(compat)
}

func (h *ProductVehicleCompatibilityHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid compatibility id")
		return
	}

	if err := h.service.Delete(id); err != nil {
		if errors.Is(err, mysqlInfra.ErrProductVehicleCompatibilityNotFound) {
			writeJSONError(w, http.StatusNotFound, "product vehicle compatibility not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ProductEquipmentCompatibilityHandler

type ProductEquipmentCompatibilityHandler struct {
	service *compatibilityApp.ProductEquipmentCompatibilityService
}

type createProductEquipmentCompatibilityRequest struct {
	ProductID          int64 `json:"productId"`
	EquipmentFitmentID int64 `json:"equipmentFitmentId"`
}

func NewProductEquipmentCompatibilityHandler(service *compatibilityApp.ProductEquipmentCompatibilityService) *ProductEquipmentCompatibilityHandler {
	return &ProductEquipmentCompatibilityHandler{service: service}
}

func (h *ProductEquipmentCompatibilityHandler) GetByProduct(w http.ResponseWriter, r *http.Request) {
	productID, err := parseIDParam(r, "productId")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	offset, pageSize := parsePaginationParams(r)

	result, err := h.service.GetByProduct(productID, offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *ProductEquipmentCompatibilityHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createProductEquipmentCompatibilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	compat, err := h.service.Create(mysqlInfra.CreateProductEquipmentCompatibilityInput{
		ProductID:          req.ProductID,
		EquipmentFitmentID: req.EquipmentFitmentID,
		CreatedBy:          user.ID,
	})
	if err != nil {
		if errors.Is(err, compatibilityApp.ErrInvalidProductCompatibility) {
			writeJSONError(w, http.StatusBadRequest, "productId and equipmentFitmentId are required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrProductEquipmentCompatibilityAlreadyExists) {
			writeJSONError(w, http.StatusConflict, "product is already linked to this equipment fitment")
			return
		}
		if errors.Is(err, mysqlInfra.ErrProductEquipmentCompatibilityInvalidReference) {
			writeJSONError(w, http.StatusBadRequest, "product or equipment fitment does not exist")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(compat)
}

func (h *ProductEquipmentCompatibilityHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid compatibility id")
		return
	}

	if err := h.service.Delete(id); err != nil {
		if errors.Is(err, mysqlInfra.ErrProductEquipmentCompatibilityNotFound) {
			writeJSONError(w, http.StatusNotFound, "product equipment compatibility not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

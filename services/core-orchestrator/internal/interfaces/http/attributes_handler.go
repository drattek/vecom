package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	attributesApp "core-orchestrator/internal/application/attributes"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// AttributeHandler — ecom_attributes (catalog of product attributes).

type upsertAttributeRequest struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	DataType string  `json:"dataType"`
	Unit     *string `json:"unit,omitempty"`
}

type AttributeHandler struct {
	service *attributesApp.AttributeService
}

func NewAttributeHandler(service *attributesApp.AttributeService) *AttributeHandler {
	return &AttributeHandler{service: service}
}

func (h *AttributeHandler) GetAttributes(w http.ResponseWriter, r *http.Request) {
	offset, pageSize := parsePaginationParams(r)

	result, err := h.service.GetPaginatedAttributes(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *AttributeHandler) GetAttributeByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid attribute id")
		return
	}

	attribute, err := h.service.GetAttributeByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrAttributeNotFound) {
			writeJSONError(w, http.StatusNotFound, "attribute not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attribute)
}

func (h *AttributeHandler) CreateAttribute(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertAttributeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateAttributeInput{
		Code:      req.Code,
		Name:      req.Name,
		DataType:  req.DataType,
		Unit:      req.Unit,
		CreatedBy: user.ID,
	}

	attribute, err := h.service.CreateAttribute(r.Context(), input)
	if err != nil {
		if errors.Is(err, attributesApp.ErrInvalidAttributePayload) {
			writeJSONError(w, http.StatusBadRequest, "code, name and a valid dataType are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(attribute)
}

func (h *AttributeHandler) UpdateAttribute(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid attribute id")
		return
	}

	var req upsertAttributeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateAttributeInput{
		Name:      req.Name,
		DataType:  req.DataType,
		Unit:      req.Unit,
		UpdatedBy: user.ID,
	}

	attribute, err := h.service.UpdateAttribute(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, attributesApp.ErrInvalidAttributePayload) {
			writeJSONError(w, http.StatusBadRequest, "name and a valid dataType are required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrAttributeNotFound) {
			writeJSONError(w, http.StatusNotFound, "attribute not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attribute)
}

func (h *AttributeHandler) DeleteAttribute(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid attribute id")
		return
	}

	if err := h.service.DeleteAttribute(r.Context(), id); err != nil {
		if errors.Is(err, mysqlInfra.ErrAttributeNotFound) {
			writeJSONError(w, http.StatusNotFound, "attribute not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AttributeOptionHandler — ecom_attribute_options (predefined values for
// 'enum' attributes).

type upsertAttributeOptionRequest struct {
	AttributeID int64  `json:"attributeId"`
	Value       string `json:"value"`
}

type AttributeOptionHandler struct {
	service *attributesApp.AttributeOptionService
}

func NewAttributeOptionHandler(service *attributesApp.AttributeOptionService) *AttributeOptionHandler {
	return &AttributeOptionHandler{service: service}
}

func (h *AttributeOptionHandler) GetOptionsByAttribute(w http.ResponseWriter, r *http.Request) {
	attributeID, err := parseIDParam(r, "attributeId")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid attribute id")
		return
	}

	options, err := h.service.GetOptionsByAttribute(r.Context(), attributeID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

func (h *AttributeOptionHandler) CreateOption(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertAttributeOptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateAttributeOptionInput{
		AttributeID: req.AttributeID,
		Value:       req.Value,
		CreatedBy:   user.ID,
	}

	option, err := h.service.CreateOption(r.Context(), input)
	if err != nil {
		if errors.Is(err, attributesApp.ErrInvalidAttributeOptionPayload) {
			writeJSONError(w, http.StatusBadRequest, "attributeId and value are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(option)
}

func (h *AttributeOptionHandler) UpdateOption(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid option id")
		return
	}

	var req struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateAttributeOptionInput{
		Value:     req.Value,
		UpdatedBy: user.ID,
	}

	option, err := h.service.UpdateOption(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, attributesApp.ErrInvalidAttributeOptionPayload) {
			writeJSONError(w, http.StatusBadRequest, "value is required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrAttributeOptionNotFound) {
			writeJSONError(w, http.StatusNotFound, "option not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(option)
}

func (h *AttributeOptionHandler) DeleteOption(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid option id")
		return
	}

	if err := h.service.DeleteOption(r.Context(), id); err != nil {
		if errors.Is(err, mysqlInfra.ErrAttributeOptionNotFound) {
			writeJSONError(w, http.StatusNotFound, "option not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ProductAttributeHandler — ecom_product_attributes (the actual value of a
// catalog attribute on a specific product). Read/write only — not consumed
// by any marketplace sync flow yet.

type setProductAttributeRequest struct {
	AttributeID int64      `json:"attributeId"`
	ValueText   *string    `json:"valueText,omitempty"`
	ValueNumber *float64   `json:"valueNumber,omitempty"`
	ValueBool   *bool      `json:"valueBoolean,omitempty"`
	ValueDate   *time.Time `json:"valueDate,omitempty"`
	OptionID    *int64     `json:"optionId,omitempty"`
}

type ProductAttributeHandler struct {
	service *attributesApp.ProductAttributeService
}

func NewProductAttributeHandler(service *attributesApp.ProductAttributeService) *ProductAttributeHandler {
	return &ProductAttributeHandler{service: service}
}

func (h *ProductAttributeHandler) GetAttributesByProduct(w http.ResponseWriter, r *http.Request) {
	productID, err := parseIDParam(r, "productId")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	attributes, err := h.service.GetAttributesByProduct(r.Context(), productID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attributes)
}

func (h *ProductAttributeHandler) SetAttribute(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	productID, err := parseIDParam(r, "productId")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req setProductAttributeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpsertProductAttributeInput{
		ProductID:   productID,
		AttributeID: req.AttributeID,
		ValueText:   req.ValueText,
		ValueNumber: req.ValueNumber,
		ValueBool:   req.ValueBool,
		ValueDate:   req.ValueDate,
		OptionID:    req.OptionID,
		ActorID:     user.ID,
	}

	attribute, err := h.service.SetValue(r.Context(), input)
	if err != nil {
		if errors.Is(err, attributesApp.ErrInvalidProductAttributePayload) {
			writeJSONError(w, http.StatusBadRequest, "attributeId is required")
			return
		}
		if errors.Is(err, attributesApp.ErrProductAttributeValueMismatch) {
			writeJSONError(w, http.StatusBadRequest, "value does not match the attribute's data type")
			return
		}
		if errors.Is(err, mysqlInfra.ErrAttributeNotFound) {
			writeJSONError(w, http.StatusNotFound, "attribute not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(attribute)
}

func (h *ProductAttributeHandler) DeleteAttribute(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid product attribute id")
		return
	}

	if err := h.service.DeleteValue(r.Context(), id); err != nil {
		if errors.Is(err, mysqlInfra.ErrProductAttributeNotFound) {
			writeJSONError(w, http.StatusNotFound, "product attribute not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

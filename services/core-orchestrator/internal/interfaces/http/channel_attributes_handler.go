package http

import (
	"encoding/json"
	"errors"
	"net/http"

	channelAttributesApp "core-orchestrator/internal/application/channel_attributes"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// ChannelAttributeHandler — ecom_channel_attributes (the attribute "slots" a
// channel exposes: MercadoLibre's fixed keys like BRAND/PART_NUMBER, or
// Odoo's dynamic-field targets).

type upsertChannelAttributeRequest struct {
	ChannelID      int64   `json:"channelId"`
	TargetStrategy string  `json:"targetStrategy"`
	ExternalKey    *string `json:"externalKey,omitempty"`
	TargetField    *string `json:"targetField,omitempty"`
	ExternalLabel  *string `json:"externalLabel,omitempty"`
	ValueMode      string  `json:"valueMode"`
	CategoryID     *int64  `json:"categoryId,omitempty"`
	IsRequired     bool    `json:"isRequired"`
}

type ChannelAttributeHandler struct {
	service *channelAttributesApp.ChannelAttributeService
}

func NewChannelAttributeHandler(service *channelAttributesApp.ChannelAttributeService) *ChannelAttributeHandler {
	return &ChannelAttributeHandler{service: service}
}

func (h *ChannelAttributeHandler) GetAttributes(w http.ResponseWriter, r *http.Request) {
	offset, pageSize := parsePaginationParams(r)

	result, err := h.service.GetPaginatedAttributes(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ChannelAttributeHandler) GetAttributeByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel attribute id")
		return
	}

	attribute, err := h.service.GetAttributeByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelAttributeNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel attribute not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attribute)
}

func (h *ChannelAttributeHandler) GetAttributesByChannel(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseIDParam(r, "channelId")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	attributes, err := h.service.GetAttributesByChannel(channelID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attributes)
}

func (h *ChannelAttributeHandler) CreateAttribute(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertChannelAttributeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateChannelAttributeInput{
		ChannelID:      req.ChannelID,
		TargetStrategy: req.TargetStrategy,
		ExternalKey:    req.ExternalKey,
		TargetField:    req.TargetField,
		ExternalLabel:  req.ExternalLabel,
		ValueMode:      req.ValueMode,
		CategoryID:     req.CategoryID,
		IsRequired:     req.IsRequired,
		CreatedBy:      user.ID,
	}

	attribute, err := h.service.CreateAttribute(input)
	if err != nil {
		if errors.Is(err, channelAttributesApp.ErrInvalidChannelAttributePayload) {
			writeJSONError(w, http.StatusBadRequest, "channelId, a valid targetStrategy/valueMode and externalKey (fixed_key) or targetField (dynamic_field) are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(attribute)
}

func (h *ChannelAttributeHandler) UpdateAttribute(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel attribute id")
		return
	}

	var req upsertChannelAttributeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateChannelAttributeInput{
		TargetStrategy: req.TargetStrategy,
		ExternalKey:    req.ExternalKey,
		TargetField:    req.TargetField,
		ExternalLabel:  req.ExternalLabel,
		ValueMode:      req.ValueMode,
		CategoryID:     req.CategoryID,
		IsRequired:     req.IsRequired,
		UpdatedBy:      user.ID,
	}

	attribute, err := h.service.UpdateAttribute(id, input)
	if err != nil {
		if errors.Is(err, channelAttributesApp.ErrInvalidChannelAttributePayload) {
			writeJSONError(w, http.StatusBadRequest, "a valid targetStrategy/valueMode and externalKey (fixed_key) or targetField (dynamic_field) are required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrChannelAttributeNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel attribute not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attribute)
}

func (h *ChannelAttributeHandler) DeleteAttribute(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel attribute id")
		return
	}

	if err := h.service.DeleteAttribute(id); err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelAttributeNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel attribute not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ChannelAttributeMapHandler — ecom_channel_attribute_map (which internal
// source — a custom attribute, a domain field, or a fixed value — fills a
// given channel attribute slot).

type upsertChannelAttributeMapRequest struct {
	ChannelAttributeID int64   `json:"channelAttributeId"`
	ConnectionID       *int64  `json:"connectionId,omitempty"`
	SourceType         string  `json:"sourceType"`
	AttributeID        *int64  `json:"attributeId,omitempty"`
	SystemField        *string `json:"systemField,omitempty"`
	StaticValue        *string `json:"staticValue,omitempty"`
}

type ChannelAttributeMapHandler struct {
	service *channelAttributesApp.ChannelAttributeMapService
}

func NewChannelAttributeMapHandler(service *channelAttributesApp.ChannelAttributeMapService) *ChannelAttributeMapHandler {
	return &ChannelAttributeMapHandler{service: service}
}

func (h *ChannelAttributeMapHandler) GetMapsByChannelAttribute(w http.ResponseWriter, r *http.Request) {
	channelAttributeID, err := parseIDParam(r, "channelAttributeId")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel attribute id")
		return
	}

	maps, err := h.service.GetMapsByChannelAttribute(channelAttributeID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(maps)
}

func (h *ChannelAttributeMapHandler) CreateMap(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertChannelAttributeMapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateChannelAttributeMapInput{
		ChannelAttributeID: req.ChannelAttributeID,
		ConnectionID:       req.ConnectionID,
		SourceType:         req.SourceType,
		AttributeID:        req.AttributeID,
		SystemField:        req.SystemField,
		StaticValue:        req.StaticValue,
		CreatedBy:          user.ID,
	}

	m, err := h.service.CreateMap(input)
	if err != nil {
		if errors.Is(err, channelAttributesApp.ErrInvalidChannelAttributeMapPayload) {
			writeJSONError(w, http.StatusBadRequest, "channelAttributeId, a valid sourceType and its matching attributeId/systemField/staticValue are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(m)
}

func (h *ChannelAttributeMapHandler) UpdateMap(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel attribute map id")
		return
	}

	var req upsertChannelAttributeMapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateChannelAttributeMapInput{
		SourceType:  req.SourceType,
		AttributeID: req.AttributeID,
		SystemField: req.SystemField,
		StaticValue: req.StaticValue,
		UpdatedBy:   user.ID,
	}

	m, err := h.service.UpdateMap(id, input)
	if err != nil {
		if errors.Is(err, channelAttributesApp.ErrInvalidChannelAttributeMapPayload) {
			writeJSONError(w, http.StatusBadRequest, "a valid sourceType and its matching attributeId/systemField/staticValue are required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrChannelAttributeMapNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel attribute map not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

func (h *ChannelAttributeMapHandler) DeleteMap(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel attribute map id")
		return
	}

	if err := h.service.DeleteMap(id); err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelAttributeMapNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel attribute map not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

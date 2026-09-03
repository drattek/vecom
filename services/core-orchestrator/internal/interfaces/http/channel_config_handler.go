package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	channelConfigApp "core-orchestrator/internal/application/channel_config"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// ConnectionCredentialsHandler
type upsertConnectionCredentialRequest struct {
	ConnectionID int64  `json:"connectionId"`
	KeyName      string `json:"keyName"`
	Value        string `json:"value"`
	IsEncrypted  bool   `json:"isEncrypted"`
}

type ConnectionCredentialsHandler struct {
	service *channelConfigApp.ConnectionCredentialsService
}

func NewConnectionCredentialsHandler(service *channelConfigApp.ConnectionCredentialsService) *ConnectionCredentialsHandler {
	return &ConnectionCredentialsHandler{service: service}
}

func (h *ConnectionCredentialsHandler) GetCredentials(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedCredentials(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ConnectionCredentialsHandler) GetCredentialByID(w http.ResponseWriter, r *http.Request) {
	id := parseConnectionCredentialID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid credential id")
		return
	}

	credential, err := h.service.GetCredentialByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrConnectionCredentialNotFound) {
			writeJSONError(w, http.StatusNotFound, "credential not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(credential)
}

func (h *ConnectionCredentialsHandler) CreateCredential(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertConnectionCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateConnectionCredentialInput{
		ConnectionID: req.ConnectionID,
		KeyName:      req.KeyName,
		Value:        req.Value,
		IsEncrypted:  req.IsEncrypted,
		CreatedBy:    user.ID,
	}

	credential, err := h.service.CreateCredential(r.Context(), input)
	if err != nil {
		if errors.Is(err, channelConfigApp.ErrInvalidChannelConfig) {
			writeJSONError(w, http.StatusBadRequest, "connectionId and keyName are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(credential)
}

func (h *ConnectionCredentialsHandler) UpdateCredential(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := parseConnectionCredentialID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid credential id")
		return
	}

	var req struct {
		Value       string `json:"value"`
		IsEncrypted bool   `json:"isEncrypted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateConnectionCredentialInput{
		Value:       req.Value,
		IsEncrypted: req.IsEncrypted,
		UpdatedBy:   user.ID,
	}

	credential, err := h.service.UpdateCredential(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, channelConfigApp.ErrInvalidChannelConfig) {
			writeJSONError(w, http.StatusBadRequest, "value is required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrConnectionCredentialNotFound) {
			writeJSONError(w, http.StatusNotFound, "credential not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(credential)
}

func (h *ConnectionCredentialsHandler) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	id := parseConnectionCredentialID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid credential id")
		return
	}

	err := h.service.DeleteCredential(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrConnectionCredentialNotFound) {
			writeJSONError(w, http.StatusNotFound, "credential not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseConnectionCredentialID(r *http.Request) int64 {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}

// ConnectionSettingsHandler
type upsertConnectionSettingRequest struct {
	ConnectionID int64  `json:"connectionId"`
	KeyName      string `json:"keyName"`
	Value        string `json:"value"`
	IsEncrypted  bool   `json:"isEncrypted"`
}

type ConnectionSettingsHandler struct {
	service *channelConfigApp.ConnectionSettingsService
}

func NewConnectionSettingsHandler(service *channelConfigApp.ConnectionSettingsService) *ConnectionSettingsHandler {
	return &ConnectionSettingsHandler{service: service}
}

func (h *ConnectionSettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedSettings(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ConnectionSettingsHandler) GetSettingByID(w http.ResponseWriter, r *http.Request) {
	id := parseConnectionSettingID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid setting id")
		return
	}

	setting, err := h.service.GetSettingByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrConnectionSettingNotFound) {
			writeJSONError(w, http.StatusNotFound, "setting not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(setting)
}

func (h *ConnectionSettingsHandler) CreateSetting(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertConnectionSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateConnectionSettingInput{
		ConnectionID: req.ConnectionID,
		KeyName:      req.KeyName,
		Value:        req.Value,
		IsEncrypted:  req.IsEncrypted,
		CreatedBy:    user.ID,
	}

	setting, err := h.service.CreateSetting(r.Context(), input)
	if err != nil {
		if errors.Is(err, channelConfigApp.ErrInvalidChannelConfig) {
			writeJSONError(w, http.StatusBadRequest, "connectionId and keyName are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(setting)
}

func (h *ConnectionSettingsHandler) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := parseConnectionSettingID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid setting id")
		return
	}

	var req struct {
		Value       string `json:"value"`
		IsEncrypted bool   `json:"isEncrypted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateConnectionSettingInput{
		Value:       req.Value,
		IsEncrypted: req.IsEncrypted,
		UpdatedBy:   user.ID,
	}

	setting, err := h.service.UpdateSetting(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, channelConfigApp.ErrInvalidChannelConfig) {
			writeJSONError(w, http.StatusBadRequest, "value is required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrConnectionSettingNotFound) {
			writeJSONError(w, http.StatusNotFound, "setting not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(setting)
}

func (h *ConnectionSettingsHandler) DeleteSetting(w http.ResponseWriter, r *http.Request) {
	id := parseConnectionSettingID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid setting id")
		return
	}

	err := h.service.DeleteSetting(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrConnectionSettingNotFound) {
			writeJSONError(w, http.StatusNotFound, "setting not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseConnectionSettingID(r *http.Request) int64 {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}

// ConnectionStatusHandler
type ConnectionStatusHandler struct {
	service *channelConfigApp.ConnectionStatusService
}

func NewConnectionStatusHandler(service *channelConfigApp.ConnectionStatusService) *ConnectionStatusHandler {
	return &ConnectionStatusHandler{service: service}
}

func (h *ConnectionStatusHandler) GetStatusByConnection(w http.ResponseWriter, r *http.Request) {
	connectionIDStr := chi.URLParam(r, "connectionId")
	connectionID, err := strconv.ParseInt(connectionIDStr, 10, 64)
	if err != nil || connectionID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid connection id")
		return
	}

	status, err := h.service.GetStatusByConnection(r.Context(), connectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrConnectionStatusNotFound) {
			writeJSONError(w, http.StatusNotFound, "connection status not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// ChannelParametersHandler
type upsertChannelParameterRequest struct {
	ChannelID     int64   `json:"channelId"`
	ParameterName string  `json:"parameterName"`
	DisplayName   string  `json:"displayName"`
	ParameterType string  `json:"parameterType"`
	Required      bool    `json:"required"`
	DefaultValue  *string `json:"defaultValue"`
	IsEncrypted   bool    `json:"isEncrypted"`
}

type ChannelParametersHandler struct {
	service *channelConfigApp.ChannelParametersService
}

func NewChannelParametersHandler(service *channelConfigApp.ChannelParametersService) *ChannelParametersHandler {
	return &ChannelParametersHandler{service: service}
}

func (h *ChannelParametersHandler) GetParameters(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedParameters(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ChannelParametersHandler) GetParameterByID(w http.ResponseWriter, r *http.Request) {
	id := parseChannelParameterID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid parameter id")
		return
	}

	parameter, err := h.service.GetParameterByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelParameterNotFound) {
			writeJSONError(w, http.StatusNotFound, "parameter not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parameter)
}

func (h *ChannelParametersHandler) GetParametersByChannel(w http.ResponseWriter, r *http.Request) {
	channelIDStr := chi.URLParam(r, "channelId")
	channelID, err := strconv.ParseInt(channelIDStr, 10, 64)
	if err != nil || channelID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid channel id")
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

	result, err := h.service.GetParametersByChannel(r.Context(), channelID, offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ChannelParametersHandler) CreateParameter(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertChannelParameterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateChannelParameterInput{
		ChannelID:     req.ChannelID,
		ParameterName: req.ParameterName,
		DisplayName:   req.DisplayName,
		ParameterType: req.ParameterType,
		Required:      req.Required,
		DefaultValue:  req.DefaultValue,
		IsEncrypted:   req.IsEncrypted,
		CreatedBy:     user.ID,
	}

	parameter, err := h.service.CreateParameter(r.Context(), input)
	if err != nil {
		if errors.Is(err, channelConfigApp.ErrInvalidChannelConfig) {
			writeJSONError(w, http.StatusBadRequest, "channelId and parameterName are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(parameter)
}

func (h *ChannelParametersHandler) UpdateParameter(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := parseChannelParameterID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid parameter id")
		return
	}

	var req struct {
		DisplayName   string  `json:"displayName"`
		ParameterType string  `json:"parameterType"`
		Required      bool    `json:"required"`
		DefaultValue  *string `json:"defaultValue"`
		IsEncrypted   bool    `json:"isEncrypted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateChannelParameterInput{
		DisplayName:   req.DisplayName,
		ParameterType: req.ParameterType,
		Required:      req.Required,
		DefaultValue:  req.DefaultValue,
		IsEncrypted:   req.IsEncrypted,
		UpdatedBy:     user.ID,
	}

	parameter, err := h.service.UpdateParameter(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, channelConfigApp.ErrInvalidChannelConfig) {
			writeJSONError(w, http.StatusBadRequest, "displayName is required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrChannelParameterNotFound) {
			writeJSONError(w, http.StatusNotFound, "parameter not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parameter)
}

func (h *ChannelParametersHandler) DeleteParameter(w http.ResponseWriter, r *http.Request) {
	id := parseChannelParameterID(r)
	if id == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid parameter id")
		return
	}

	err := h.service.DeleteParameter(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelParameterNotFound) {
			writeJSONError(w, http.StatusNotFound, "parameter not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseChannelParameterID(r *http.Request) int64 {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}

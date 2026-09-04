package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	channelsApp "core-orchestrator/internal/application/channels"
)

type ChannelHandler struct {
	service *channelsApp.ChannelService
}

type upsertChannelRequest struct {
	Name        string  `json:"name"`
	Code        string  `json:"code"`
	Status      string  `json:"status"`
	IconID      *int64  `json:"iconId"`
	Description *string `json:"description"`
}

func NewChannelHandler(service *channelsApp.ChannelService) *ChannelHandler {
	return &ChannelHandler{service: service}
}

func (h *ChannelHandler) GetChannels(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedChannels(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *ChannelHandler) GetChannelByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseChannelID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	channel, err := h.service.GetChannelByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, channelsApp.ErrInvalidChannelPayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid channel id")
			return
		}
		if errors.Is(err, channelsApp.ErrChannelNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(channel)
}

func (h *ChannelHandler) GetChannelSyncSummary(w http.ResponseWriter, r *http.Request) {
	id, err := parseChannelID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	summary, err := h.service.GetChannelSyncSummary(r.Context(), id)
	if err != nil {
		if errors.Is(err, channelsApp.ErrInvalidChannelPayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid channel id")
			return
		}
		if errors.Is(err, channelsApp.ErrChannelNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(summary)
}

func (h *ChannelHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	input, err := decodeChannelRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	channel, err := h.service.CreateChannel(r.Context(), input, user.ID)
	if err != nil {
		if errors.Is(err, channelsApp.ErrInvalidChannelPayload) {
			writeJSONError(w, http.StatusBadRequest, "name and code are required; status must be valid")
			return
		}
		if errors.Is(err, channelsApp.ErrChannelCodeAlreadyExists) {
			writeJSONError(w, http.StatusConflict, "channel code already exists")
			return
		}
		if errors.Is(err, channelsApp.ErrInvalidChannelReference) {
			writeJSONError(w, http.StatusBadRequest, "invalid iconId")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(channel)
}

func (h *ChannelHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseChannelID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	input, err := decodeChannelRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	channel, err := h.service.UpdateChannel(r.Context(), id, input, user.ID)
	if err != nil {
		if errors.Is(err, channelsApp.ErrInvalidChannelPayload) {
			writeJSONError(w, http.StatusBadRequest, "name and code are required; status must be valid")
			return
		}
		if errors.Is(err, channelsApp.ErrChannelCodeAlreadyExists) {
			writeJSONError(w, http.StatusConflict, "channel code already exists")
			return
		}
		if errors.Is(err, channelsApp.ErrInvalidChannelReference) {
			writeJSONError(w, http.StatusBadRequest, "invalid iconId")
			return
		}
		if errors.Is(err, channelsApp.ErrChannelNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(channel)
}

func (h *ChannelHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseChannelID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	err = h.service.SoftDeleteChannel(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, channelsApp.ErrInvalidChannelPayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid channel id")
			return
		}
		if errors.Is(err, channelsApp.ErrChannelNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseChannelID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

func decodeChannelRequest(r *http.Request) (channelsApp.UpsertChannelInput, error) {
	var body upsertChannelRequest
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return channelsApp.UpsertChannelInput{}, err
	}

	return channelsApp.UpsertChannelInput{
		Name:        body.Name,
		Code:        body.Code,
		Status:      body.Status,
		IconID:      body.IconID,
		Description: body.Description,
	}, nil
}

package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	channelConnectionsApp "core-orchestrator/internal/application/channel_connections"
)

type ChannelConnectionHandler struct {
	service *channelConnectionsApp.ChannelConnectionService
}

type upsertChannelConnectionRequest struct {
	ChannelID              int64  `json:"channelId"`
	Name                   string `json:"name"`
	Status                 string `json:"status"`
	Environment            string `json:"environment"`
	CurrencyID             int64  `json:"currencyId"`
	AllowsMultipleListings bool   `json:"allowsMultipleListings"`
}

func NewChannelConnectionHandler(service *channelConnectionsApp.ChannelConnectionService) *ChannelConnectionHandler {
	return &ChannelConnectionHandler{service: service}
}

func (h *ChannelConnectionHandler) GetChannelConnections(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedChannelConnections(r.Context(), offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *ChannelConnectionHandler) GetChannelConnectionByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseChannelConnectionID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel connection id")
		return
	}

	channelConnection, err := h.service.GetChannelConnectionByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, channelConnectionsApp.ErrInvalidChannelConnectionPayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid channel connection id")
			return
		}
		if errors.Is(err, channelConnectionsApp.ErrChannelConnectionNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel connection not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(channelConnection)
}

func (h *ChannelConnectionHandler) CreateChannelConnection(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	input, err := decodeChannelConnectionRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	channelConnection, err := h.service.CreateChannelConnection(r.Context(), input, user.ID)
	if err != nil {
		if errors.Is(err, channelConnectionsApp.ErrInvalidChannelConnectionPayload) {
			writeJSONError(w, http.StatusBadRequest, "channelId, name and currencyId are required; status and environment must be valid")
			return
		}
		if errors.Is(err, channelConnectionsApp.ErrInvalidChannelConnectionReference) {
			writeJSONError(w, http.StatusBadRequest, "invalid channelId or currencyId")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(channelConnection)
}

func (h *ChannelConnectionHandler) UpdateChannelConnection(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseChannelConnectionID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel connection id")
		return
	}

	input, err := decodeChannelConnectionRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	channelConnection, err := h.service.UpdateChannelConnection(r.Context(), id, input, user.ID)
	if err != nil {
		if errors.Is(err, channelConnectionsApp.ErrInvalidChannelConnectionPayload) {
			writeJSONError(w, http.StatusBadRequest, "channelId, name and currencyId are required; status and environment must be valid")
			return
		}
		if errors.Is(err, channelConnectionsApp.ErrInvalidChannelConnectionReference) {
			writeJSONError(w, http.StatusBadRequest, "invalid channelId or currencyId")
			return
		}
		if errors.Is(err, channelConnectionsApp.ErrChannelConnectionNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel connection not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(channelConnection)
}

func (h *ChannelConnectionHandler) DeleteChannelConnection(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseChannelConnectionID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid channel connection id")
		return
	}

	err = h.service.SoftDeleteChannelConnection(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, channelConnectionsApp.ErrInvalidChannelConnectionPayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid channel connection id")
			return
		}
		if errors.Is(err, channelConnectionsApp.ErrChannelConnectionNotFound) {
			writeJSONError(w, http.StatusNotFound, "channel connection not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseChannelConnectionID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

func decodeChannelConnectionRequest(r *http.Request) (channelConnectionsApp.UpsertChannelConnectionInput, error) {
	var body upsertChannelConnectionRequest
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return channelConnectionsApp.UpsertChannelConnectionInput{}, err
	}

	return channelConnectionsApp.UpsertChannelConnectionInput{
		ChannelID:              body.ChannelID,
		Name:                   body.Name,
		Status:                 body.Status,
		Environment:            body.Environment,
		CurrencyID:             body.CurrencyID,
		AllowsMultipleListings: body.AllowsMultipleListings,
	}, nil
}

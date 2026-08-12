package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	sourcesApp "core-orchestrator/internal/application/sources"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type SourceHandler struct {
	service *sourcesApp.SourceService
}

type upsertSourceRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func NewSourceHandler(service *sourcesApp.SourceService) *SourceHandler {
	return &SourceHandler{service: service}
}

func (h *SourceHandler) GetSources(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedSources(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *SourceHandler) GetSourceByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseSourceID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid source id")
		return
	}

	source, err := h.service.GetSourceByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrSourceNotFound) {
			writeJSONError(w, http.StatusNotFound, "source not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(source)
}

func (h *SourceHandler) CreateSource(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateSourceInput{
		Code:      req.Code,
		Name:      req.Name,
		CreatedBy: user.ID,
	}

	source, err := h.service.CreateSource(input)
	if err != nil {
		if errors.Is(err, sourcesApp.ErrInvalidSourcePayload) {
			writeJSONError(w, http.StatusBadRequest, "code and name are required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(source)
}

func (h *SourceHandler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseSourceID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid source id")
		return
	}

	var req upsertSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateSourceInput{
		Code:      req.Code,
		Name:      req.Name,
		UpdatedBy: user.ID,
	}

	source, err := h.service.UpdateSource(id, input)
	if err != nil {
		if errors.Is(err, sourcesApp.ErrInvalidSourcePayload) {
			writeJSONError(w, http.StatusBadRequest, "code and name are required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrSourceNotFound) {
			writeJSONError(w, http.StatusNotFound, "source not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(source)
}

func (h *SourceHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseSourceID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid source id")
		return
	}

	err = h.service.DeleteSource(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrSourceNotFound) {
			writeJSONError(w, http.StatusNotFound, "source not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseSourceID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

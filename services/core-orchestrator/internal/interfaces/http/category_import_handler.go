package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	categoryImportApp "core-orchestrator/internal/application/category_import"
	syncApp "core-orchestrator/internal/application/sync"
)

// CategoryImportHandler serves the Settings → Categorías "Importar categoría"
// flow: GET /api/category-import/sources (channels with active connections),
// GET /api/category-import/tree (one level of a channel's external category
// tree) and POST /api/category-import (replicate a chosen leaf locally).
type CategoryImportHandler struct {
	service *categoryImportApp.Service
}

func NewCategoryImportHandler(service *categoryImportApp.Service) *CategoryImportHandler {
	return &CategoryImportHandler{service: service}
}

func (h *CategoryImportHandler) GetSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.service.ListSources(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSONResponse(w, map[string]any{"sources": sources})
}

func (h *CategoryImportHandler) GetTree(w http.ResponseWriter, r *http.Request) {
	connectionID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("connectionId")), 10, 64)
	if err != nil || connectionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "connectionId query param is required")
		return
	}

	externalCategoryID := strings.TrimSpace(r.URL.Query().Get("externalCategoryId"))

	nodes, err := h.service.BrowseTree(r.Context(), connectionID, externalCategoryID)
	if err != nil {
		h.writeImportError(w, err)
		return
	}

	writeJSONResponse(w, map[string]any{"nodes": nodes})
}

type importCategoryRequest struct {
	ConnectionID           int64  `json:"connectionId"`
	ExternalCategoryID     string `json:"externalCategoryId"`
	OnlySelectedConnection bool   `json:"onlySelectedConnection"`
}

func (h *CategoryImportHandler) Import(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req importCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.Import(r.Context(), req.ConnectionID, req.ExternalCategoryID, user.ID, req.OnlySelectedConnection)
	if err != nil {
		h.writeImportError(w, err)
		return
	}

	writeJSONResponse(w, result)
}

type setCategoryMappingRequest struct {
	CategoryID         int64  `json:"categoryId"`
	ConnectionID       int64  `json:"connectionId"`
	ExternalCategoryID string `json:"externalCategoryId"`
}

// SetMapping links an existing local category to an external leaf category on
// one connection (or replaces its current mapping) — the "Vincular / Reemplazar"
// action in the category detail view. POST /api/category-import/mapping.
func (h *CategoryImportHandler) SetMapping(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req setCategoryMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.SetMapping(r.Context(), req.CategoryID, req.ConnectionID, req.ExternalCategoryID, user.ID)
	if err != nil {
		h.writeImportError(w, err)
		return
	}

	writeJSONResponse(w, result)
}

func (h *CategoryImportHandler) writeImportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, categoryImportApp.ErrInvalidInput):
		writeJSONError(w, http.StatusBadRequest, "invalid request")
	case errors.Is(err, categoryImportApp.ErrLocalCategoryNotFound):
		writeJSONError(w, http.StatusNotFound, "category not found")
	case errors.Is(err, categoryImportApp.ErrConnectionNotFound):
		writeJSONError(w, http.StatusNotFound, "channel connection not found")
	case errors.Is(err, categoryImportApp.ErrChannelNotSupported):
		writeJSONError(w, http.StatusConflict, "this channel does not support category import")
	case errors.Is(err, categoryImportApp.ErrExternalCategoryNotFound):
		writeJSONError(w, http.StatusNotFound, "external category not found")
	case errors.Is(err, categoryImportApp.ErrNotLeaf):
		writeJSONError(w, http.StatusUnprocessableEntity, "only leaf categories can be imported")
	case errors.Is(err, syncApp.ErrMissingMercadoLibreSiteID):
		writeJSONError(w, http.StatusBadRequest, "the MercadoLibre connection has no site_id configured")
	case errors.Is(err, syncApp.ErrMissingMercadoLibreSettings),
		errors.Is(err, syncApp.ErrInvalidMercadoLibreConnection),
		errors.Is(err, syncApp.ErrInvalidExpirationTimeFormat):
		writeJSONError(w, http.StatusBadRequest, "the MercadoLibre connection is not fully configured or authenticated")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal error")
	}
}

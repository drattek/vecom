package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	categoriesApp "core-orchestrator/internal/application/categories"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type CategoryHandler struct {
	service *categoriesApp.CategoryService
}

type upsertCategoryRequest struct {
	Name     string `json:"name"`
	ParentID *int64 `json:"parentId,omitempty"`
}

func NewCategoryHandler(service *categoriesApp.CategoryService) *CategoryHandler {
	return &CategoryHandler{service: service}
}

func (h *CategoryHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.service.GetCategories(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"categories": categories})
}

func (h *CategoryHandler) GetCategoryByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseCategoryID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid category id")
		return
	}

	category, err := h.service.GetCategoryByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrCategoryNotFound) {
			writeJSONError(w, http.StatusNotFound, "category not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(category)
}

func (h *CategoryHandler) GetCategoryChildren(w http.ResponseWriter, r *http.Request) {
	id, err := parseCategoryID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid category id")
		return
	}

	children, err := h.service.GetCategoryChildren(r.Context(), id)
	if err != nil {
		if errors.Is(err, categoriesApp.ErrInvalidCategoryPayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid category id")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"children": children})
}

func (h *CategoryHandler) GetCategoryChannelMappings(w http.ResponseWriter, r *http.Request) {
	id, err := parseCategoryID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid category id")
		return
	}

	mappings, err := h.service.GetChannelMappings(r.Context(), id)
	if err != nil {
		if errors.Is(err, categoriesApp.ErrInvalidCategoryPayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid category id")
			return
		}
		if errors.Is(err, mysqlInfra.ErrCategoryNotFound) {
			writeJSONError(w, http.StatusNotFound, "category not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(mappings)
}

func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.CreateCategoryInput{
		Name:      req.Name,
		ParentID:  req.ParentID,
		CreatedBy: user.ID,
	}

	category, err := h.service.CreateCategory(r.Context(), input)
	if err != nil {
		if errors.Is(err, categoriesApp.ErrInvalidCategoryPayload) {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(category)
}

func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseCategoryID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid category id")
		return
	}

	var req upsertCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := mysqlInfra.UpdateCategoryInput{
		Name:      req.Name,
		ParentID:  req.ParentID,
		UpdatedBy: user.ID,
	}

	category, err := h.service.UpdateCategory(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, categoriesApp.ErrInvalidCategoryPayload) {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}
		if errors.Is(err, mysqlInfra.ErrCategoryNotFound) {
			writeJSONError(w, http.StatusNotFound, "category not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(category)
}

func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := parseCategoryID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid category id")
		return
	}

	err = h.service.DeleteCategory(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrCategoryNotFound) {
			writeJSONError(w, http.StatusNotFound, "category not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseCategoryID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

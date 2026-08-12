package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	storageDisksApp "core-orchestrator/internal/application/storage_disks"
)

type StorageDiskHandler struct {
	service *storageDisksApp.StorageDiskService
}

type upsertStorageDiskRequest struct {
	Name     string  `json:"name"`
	Code     string  `json:"code"`
	BaseURL  string  `json:"baseUrl"`
	Bucket   *string `json:"bucket"`
	Endpoint *string `json:"endpoint"`
	IsPublic *bool   `json:"isPublic"`
}

func NewStorageDiskHandler(service *storageDisksApp.StorageDiskService) *StorageDiskHandler {
	return &StorageDiskHandler{service: service}
}

func (h *StorageDiskHandler) GetStorageDisks(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.GetPaginatedStorageDisks(offset, pageSize)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *StorageDiskHandler) GetStorageDiskByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseStorageDiskID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid storage disk id")
		return
	}

	storageDisk, err := h.service.GetStorageDiskByID(id)
	if err != nil {
		if errors.Is(err, storageDisksApp.ErrInvalidStorageDiskPayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid storage disk id")
			return
		}
		if errors.Is(err, storageDisksApp.ErrStorageDiskNotFound) {
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(storageDisk)
}

func (h *StorageDiskHandler) CreateStorageDisk(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	input, err := decodeStorageDiskRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	storageDisk, err := h.service.CreateStorageDisk(input, user.ID)
	if err != nil {
		if errors.Is(err, storageDisksApp.ErrInvalidStorageDiskPayload) {
			writeJSONError(w, http.StatusBadRequest, "name, code and baseUrl are required")
			return
		}
		if errors.Is(err, storageDisksApp.ErrStorageDiskCodeAlreadyExists) {
			writeJSONError(w, http.StatusConflict, "storage disk code already exists")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(storageDisk)
}

func (h *StorageDiskHandler) UpdateStorageDisk(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseStorageDiskID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid storage disk id")
		return
	}

	input, err := decodeStorageDiskRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	storageDisk, err := h.service.UpdateStorageDisk(id, input, user.ID)
	if err != nil {
		if errors.Is(err, storageDisksApp.ErrInvalidStorageDiskPayload) {
			writeJSONError(w, http.StatusBadRequest, "name, code and baseUrl are required")
			return
		}
		if errors.Is(err, storageDisksApp.ErrStorageDiskCodeAlreadyExists) {
			writeJSONError(w, http.StatusConflict, "storage disk code already exists")
			return
		}
		if errors.Is(err, storageDisksApp.ErrStorageDiskNotFound) {
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(storageDisk)
}

func (h *StorageDiskHandler) DeleteStorageDisk(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := parseStorageDiskID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid storage disk id")
		return
	}

	err = h.service.SoftDeleteStorageDisk(id, user.ID)
	if err != nil {
		if errors.Is(err, storageDisksApp.ErrInvalidStorageDiskPayload) {
			writeJSONError(w, http.StatusBadRequest, "invalid storage disk id")
			return
		}
		if errors.Is(err, storageDisksApp.ErrStorageDiskNotFound) {
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseStorageDiskID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

func decodeStorageDiskRequest(r *http.Request) (storageDisksApp.UpsertStorageDiskInput, error) {
	var body upsertStorageDiskRequest
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return storageDisksApp.UpsertStorageDiskInput{}, err
	}

	isPublic := true
	if body.IsPublic != nil {
		isPublic = *body.IsPublic
	}

	return storageDisksApp.UpsertStorageDiskInput{
		Name:     body.Name,
		Code:     body.Code,
		BaseURL:  body.BaseURL,
		Bucket:   body.Bucket,
		Endpoint: body.Endpoint,
		IsPublic: isPublic,
	}, nil
}

package http

import (
	"encoding/json"
	"errors"
	"net/http"

	storageDisksApp "core-orchestrator/internal/application/storage_disks"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type setAzureCredentialsRequest struct {
	AccountName   string `json:"accountName"`
	AccountKey    string `json:"accountKey"`
	ContainerName string `json:"containerName"`
}

// SetAzureCredentials stores (encrypted) the Azure Blob Storage account name,
// account key and container name a storage disk uploads to. The values are
// never returned, by this or any other endpoint.
func (h *StorageDiskHandler) SetAzureCredentials(w http.ResponseWriter, r *http.Request) {
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

	var req setAzureCredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.SetAzureCredentials(r.Context(), id, req.AccountName, req.AccountKey, req.ContainerName, user.ID); err != nil {
		switch {
		case errors.Is(err, storageDisksApp.ErrInvalidAzureCredentials):
			writeJSONError(w, http.StatusBadRequest, "accountName, accountKey and containerName are required")
		case errors.Is(err, storageDisksApp.ErrStorageDiskNotFound):
			writeJSONError(w, http.StatusNotFound, "storage disk not found")
		case errors.Is(err, mysqlInfra.ErrInvalidCredentialsEncryptionKey):
			writeJSONError(w, http.StatusInternalServerError, "the credentials encryption key is not configured")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSONResponse(w, map[string]bool{"updated": true})
}

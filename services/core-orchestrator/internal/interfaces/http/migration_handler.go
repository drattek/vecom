package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	migrationApp "core-orchestrator/internal/application/migration"
)

// MigrationHandler exposes one-off data-completion endpoints under
// /api/migration. Unlike /api/marketplaces, these are not long-lived
// integration endpoints and are expected to be removed once the underlying
// data migration is complete.
type MigrationHandler struct {
	odooCategoryMigrationService *migrationApp.OdooCategoryMigrationService
}

func NewMigrationHandler(odooCategoryMigrationService *migrationApp.OdooCategoryMigrationService) *MigrationHandler {
	return &MigrationHandler{odooCategoryMigrationService: odooCategoryMigrationService}
}

func (h *MigrationHandler) SyncOdooCategories(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	connectionIDStr := strings.TrimSpace(r.URL.Query().Get("connectionId"))
	if connectionIDStr == "" {
		writeJSONError(w, http.StatusBadRequest, "connectionId is required")
		return
	}

	connectionID, err := strconv.ParseInt(connectionIDStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
		return
	}

	result, err := h.odooCategoryMigrationService.MigrateCategories(r.Context(), connectionID, user.ID)
	if err != nil {
		if errors.Is(err, migrationApp.ErrInvalidOdooCategoryConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		if errors.Is(err, migrationApp.ErrMissingOdooCategorySettings) || errors.Is(err, migrationApp.ErrMissingOdooCategoryCredentials) {
			writeJSONError(w, http.StatusBadRequest, "missing required Odoo connection settings or credentials")
			return
		}

		log.Printf("odoo category migration failed for connectionId=%d: %v", connectionID, err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

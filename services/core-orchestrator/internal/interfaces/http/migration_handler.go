package http

import (
	"encoding/json"
	"errors"
	"io"
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
	odooCategoryMigrationService     *migrationApp.OdooCategoryMigrationService
	vecomSyncProductMigrationService *migrationApp.VecomSyncProductMigrationService
}

func NewMigrationHandler(
	odooCategoryMigrationService *migrationApp.OdooCategoryMigrationService,
	vecomSyncProductMigrationService *migrationApp.VecomSyncProductMigrationService,
) *MigrationHandler {
	return &MigrationHandler{
		odooCategoryMigrationService:     odooCategoryMigrationService,
		vecomSyncProductMigrationService: vecomSyncProductMigrationService,
	}
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

// migrateVecomRequest is the (optional) JSON body for MigrateVecomSyncProducts.
// Every field is optional: an empty body — or no body at all — migrates every
// vecom_sync_product row in one call. integrationId narrows the run to Odoo (4)
// or MercadoLibre (5); limit/offset page through the source rows so a large
// migration can be done in batches (each request is one batch — call again
// with offset += limit for the next).
type migrateVecomRequest struct {
	IntegrationID *int64 `json:"integrationId"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
}

// MigrateVecomSyncProducts is a TEMPORARY one-off endpoint: it migrates the
// previous system's vecom_sync_product / vecom_products tables into
// ecom_products / ecom_channel_product_map, completing category/status/
// description/dimensions/attributes/brand from the real Odoo and MercadoLibre
// connections. Remove this handler and its route once the migration is done.
func (h *MigrationHandler) MigrateVecomSyncProducts(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body migrateVecomRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.IntegrationID != nil && *body.IntegrationID != 4 && *body.IntegrationID != 5 {
		writeJSONError(w, http.StatusBadRequest, "integrationId must be 4 (Odoo) or 5 (MercadoLibre)")
		return
	}
	if body.Limit < 0 || body.Offset < 0 {
		writeJSONError(w, http.StatusBadRequest, "limit and offset must be >= 0")
		return
	}

	input := migrationApp.MigrateVecomInput{
		IntegrationID: body.IntegrationID,
		Limit:         body.Limit,
		Offset:        body.Offset,
	}

	result, err := h.vecomSyncProductMigrationService.Migrate(r.Context(), input, user.ID)
	if err != nil {
		log.Printf("vecom sync product migration failed: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

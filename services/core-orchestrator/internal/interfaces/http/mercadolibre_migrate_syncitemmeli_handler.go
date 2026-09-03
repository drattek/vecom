package http

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	syncApp "core-orchestrator/internal/application/sync"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// migrateSyncItemMeliActorID is recorded as the actor for ecom_products /
// ecom_channel_product_map writes this migration makes, mirroring
// systemMercadoLibreSyncActorID in sync_mercadolibre_products.go — this run
// isn't triggered by an authenticated user either.
const migrateSyncItemMeliActorID int64 = 1

// MercadoLibreMigrateSyncItemMeliHandler is a TEMPORARY, one-off migration
// endpoint: it reads the previous application version's syncitemmeli table
// (RecId, ResponseId, InternalCode, Code, Name, Description, CategoryId,
// Status) and, for each row, resolves the local product by
// InternalCode=ecom_products.sku.
//
//   - If no local product matches: the row's listing is orphaned from this
//     system's point of view, so its MercadoLibre stock (ResponseId is the
//     MercadoLibre item id) is set to 0 and nothing is written to MySQL.
//   - If a local product matches: the listing's live status is pulled from
//     MercadoLibre and mirrored into ecom_channel_product_map (created if
//     missing, refreshed otherwise), and the row's Description backfills
//     ecom_products.description.
//
// Remove this handler and its route once the migration has been run.
type MercadoLibreMigrateSyncItemMeliHandler struct {
	db                    *sql.DB
	tokenService          *syncApp.MercadoLibreTokenService
	productRepository     *mysqlInfra.ProductRepository
	channelProductMapRepo *mysqlInfra.ChannelProductMapRepository
	itemsHandler          *mercadoLibreInfra.ItemsHandler
}

func NewMercadoLibreMigrateSyncItemMeliHandler(
	db *sql.DB,
	tokenService *syncApp.MercadoLibreTokenService,
	productRepository *mysqlInfra.ProductRepository,
	channelProductMapRepo *mysqlInfra.ChannelProductMapRepository,
	rateLimiter *mercadoLibreInfra.RateLimiter,
) *MercadoLibreMigrateSyncItemMeliHandler {
	return &MercadoLibreMigrateSyncItemMeliHandler{
		db:                    db,
		tokenService:          tokenService,
		productRepository:     productRepository,
		channelProductMapRepo: channelProductMapRepo,
		itemsHandler:          mercadoLibreInfra.NewItemsHandler(mercadoLibreInfra.NewClient(nil, "", rateLimiter)),
	}
}

type syncItemMeliRow struct {
	RecID        int64
	ResponseID   string
	InternalCode string
	Description  sql.NullString
}

type migrateSyncItemMeliEntry struct {
	SKU        string `json:"sku"`
	ExternalID string `json:"externalId"`
}

type migrateSyncItemMeliErrorEntry struct {
	SKU        string `json:"sku"`
	ExternalID string `json:"externalId"`
	Error      string `json:"error"`
}

// Run migrates every syncitemmeli row against connectionId's MercadoLibre
// connection. connectionId is required as a query parameter.
func (h *MercadoLibreMigrateSyncItemMeliHandler) Run(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	connectionIDStr := strings.TrimSpace(r.URL.Query().Get("connectionId"))
	connectionID, err := strconv.ParseInt(connectionIDStr, 10, 64)
	if connectionIDStr == "" || err != nil || connectionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "connectionId is required")
		return
	}

	rows, err := h.db.QueryContext(ctx, `SELECT RecId, ResponseId, InternalCode, Description FROM syncitemmeli`)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error querying syncitemmeli: %v", err))
		return
	}

	items := make([]syncItemMeliRow, 0)
	for rows.Next() {
		var it syncItemMeliRow
		if err := rows.Scan(&it.RecID, &it.ResponseID, &it.InternalCode, &it.Description); err != nil {
			rows.Close()
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error scanning syncitemmeli row: %v", err))
			return
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("error reading syncitemmeli rows: %v", err))
		return
	}

	added := make([]migrateSyncItemMeliEntry, 0)
	updated := make([]migrateSyncItemMeliEntry, 0)
	stockZeroed := make([]migrateSyncItemMeliEntry, 0)
	errorsOut := make([]migrateSyncItemMeliErrorEntry, 0)

	for _, item := range items {
		sku := strings.TrimSpace(item.InternalCode)
		externalID := strings.TrimSpace(item.ResponseID)
		if sku == "" || externalID == "" {
			errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: "missing InternalCode or ResponseId"})
			continue
		}

		accessToken, err := h.tokenService.EnsureValidAccessToken(ctx, connectionID)
		if err != nil {
			errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: fmt.Sprintf("error getting mercadolibre access token: %v", err)})
			continue
		}

		product, err := h.productRepository.FindBySKU(r.Context(), sku)
		if err != nil && !errors.Is(err, mysqlInfra.ErrProductNotFound) {
			errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: fmt.Sprintf("error looking up product: %v", err)})
			continue
		}

		if product == nil {
			// No local product for this sku: zero the listing's MercadoLibre
			// stock and touch nothing in MySQL. UpdateItem requires price and
			// stock together, so the listing's current price is fetched and
			// resent unchanged.
			detail, err := h.itemsHandler.GetItem(ctx, accessToken, externalID)
			if err != nil {
				errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: fmt.Sprintf("error fetching mercadolibre item: %v", err)})
				continue
			}

			if err := h.itemsHandler.UpdateItem(ctx, mercadoLibreInfra.UpdateItemRequest{
				AccessToken: accessToken,
				ExternalID:  externalID,
				Vals: mercadoLibreInfra.UpdateItemVals{
					Price:             detail.Price,
					AvailableQuantity: 0,
				},
			}); err != nil {
				errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: fmt.Sprintf("error zeroing mercadolibre stock: %v", err)})
				continue
			}

			stockZeroed = append(stockZeroed, migrateSyncItemMeliEntry{SKU: sku, ExternalID: externalID})
			continue
		}

		// Local product found: backfill the description only when
		// ecom_products doesn't already have one — this migration never
		// overwrites an existing description.
		if product.Description == nil || strings.TrimSpace(*product.Description) == "" {
			if description := strings.TrimSpace(item.Description.String); description != "" {
				if err := h.productRepository.UpdateDescription(r.Context(), product.ID, description, migrateSyncItemMeliActorID); err != nil {
					errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: fmt.Sprintf("error updating product description: %v", err)})
					continue
				}
			}
		}

		existing, err := h.channelProductMapRepo.FindByProductConnectionAndFitment(r.Context(), product.ID, connectionID, nil)
		if err != nil && !errors.Is(err, mysqlInfra.ErrChannelProductMapNotFound) {
			errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: fmt.Sprintf("error loading channel product map: %v", err)})
			continue
		}

		existingExternalID := ""
		if existing != nil && existing.ExternalID != nil {
			existingExternalID = strings.TrimSpace(*existing.ExternalID)
		}

		// A mapping already recorded under a different external_id is treated
		// as the canonical listing for this product — it's left untouched,
		// and syncitemmeli's own listing (now a duplicate) has its
		// MercadoLibre stock zeroed instead, same treatment as an unmatched
		// sku.
		if existingExternalID != "" && existingExternalID != externalID {
			detail, err := h.itemsHandler.GetItem(ctx, accessToken, externalID)
			if err != nil {
				errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: fmt.Sprintf("error fetching mercadolibre item: %v", err)})
				continue
			}

			if err := h.itemsHandler.UpdateItem(ctx, mercadoLibreInfra.UpdateItemRequest{
				AccessToken: accessToken,
				ExternalID:  externalID,
				Vals: mercadoLibreInfra.UpdateItemVals{
					Price:             detail.Price,
					AvailableQuantity: 0,
				},
			}); err != nil {
				errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: fmt.Sprintf("error zeroing mercadolibre stock: %v", err)})
				continue
			}

			stockZeroed = append(stockZeroed, migrateSyncItemMeliEntry{SKU: sku, ExternalID: externalID})
			continue
		}

		// No mapping yet, or it already points at this same external_id:
		// pull the listing's live status from MercadoLibre.
		statuses, err := h.itemsHandler.GetItemsStatus(ctx, accessToken, []string{externalID})
		if err != nil {
			errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: fmt.Sprintf("error fetching mercadolibre item status: %v", err)})
			continue
		}
		if len(statuses) == 0 {
			errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: "mercadolibre item not found"})
			continue
		}
		status := statuses[0]
		foldedStatus := foldSyncItemMeliMigrationStatus(status.Status, status.SubStatus)

		if existing != nil && existingExternalID == externalID {
			// Same listing already mapped: only the status is refreshed,
			// external_id/title/category are left exactly as they are.
			if err := h.channelProductMapRepo.UpdateStatus(r.Context(), existing.ID, foldedStatus); err != nil {
				errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: fmt.Sprintf("error updating channel product map status: %v", err)})
				continue
			}
			updated = append(updated, migrateSyncItemMeliEntry{SKU: sku, ExternalID: externalID})
			continue
		}

		// No mapping existed yet (or the existing row had no external_id
		// recorded): create/populate it fully.
		if _, err := h.channelProductMapRepo.Upsert(r.Context(), mysqlInfra.UpsertChannelProductMapInput{
			ProductID:          product.ID,
			ConnectionID:       connectionID,
			ListingTitle:       status.Title,
			ExternalID:         externalID,
			ExternalCategoryID: status.CategoryID,
			Status:             foldedStatus,
			ActorID:            migrateSyncItemMeliActorID,
		}); err != nil {
			errorsOut = append(errorsOut, migrateSyncItemMeliErrorEntry{SKU: sku, ExternalID: externalID, Error: fmt.Sprintf("error upserting channel product map: %v", err)})
			continue
		}

		if existing == nil {
			added = append(added, migrateSyncItemMeliEntry{SKU: sku, ExternalID: externalID})
		} else {
			updated = append(updated, migrateSyncItemMeliEntry{SKU: sku, ExternalID: externalID})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"totalRows":                 len(items),
		"addedToChannelProductMap":  added,
		"updatedChannelProductMap":  updated,
		"stockZeroedInMercadoLibre": stockZeroed,
		"errors":                    errorsOut,
	})
}

// foldSyncItemMeliMigrationStatus mirrors foldMercadoLibreStatus in
// sync_mercadolibre_products.go (unexported there, so duplicated here rather
// than reaching into that package for a handler meant to be deleted whole).
func foldSyncItemMeliMigrationStatus(rawStatus string, subStatus []string) string {
	for _, s := range subStatus {
		if strings.EqualFold(strings.TrimSpace(s), "forbidden") {
			return "closed"
		}
	}

	switch strings.ToLower(strings.TrimSpace(rawStatus)) {
	case "under_review":
		return "under_review"
	case "paused":
		return "paused"
	default:
		return "synced"
	}
}

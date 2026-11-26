package http

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	salesApp "core-orchestrator/internal/application/sales"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// SalesHandler expone la sección "Ventas" (ADR 0010).
type SalesHandler struct {
	queries *salesApp.QueryService
	sync    *salesApp.SyncService
}

func NewSalesHandler(queries *salesApp.QueryService, syncService *salesApp.SyncService) *SalesHandler {
	return &SalesHandler{queries: queries, sync: syncService}
}

// List: GET /api/sales
func (h *SalesHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	connectionID := int64(0)
	if raw := query.Get("connectionId"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			writeJSONError(w, http.StatusBadRequest, "connectionId inválido")
			return
		}
		connectionID = parsed
	}
	offset, _ := strconv.Atoi(query.Get("offset"))
	pageSize, _ := strconv.Atoi(query.Get("pageSize"))

	result, err := h.queries.List(r.Context(), salesApp.ListInput{
		ChannelCode:    query.Get("channelCode"),
		ConnectionID:   connectionID,
		OrderStatus:    query.Get("orderStatus"),
		PaymentStatus:  query.Get("paymentStatus"),
		ShippingStatus: query.Get("shippingStatus"),
		From:           query.Get("from"),
		To:             query.Get("to"),
		Search:         query.Get("search"),
		Offset:         offset,
		PageSize:       pageSize,
	})
	if err != nil {
		if errors.Is(err, salesApp.ErrInvalidSalesFilter) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("error listing sales: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "no se pudieron consultar las ventas")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Get: GET /api/sales/{id}
func (h *SalesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "id de venta inválido")
		return
	}
	detail, err := h.queries.Detail(r.Context(), id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrSaleNotFound) {
			writeJSONError(w, http.StatusNotFound, "venta no encontrada")
			return
		}
		log.Printf("error reading sale %d: %v", id, err)
		writeJSONError(w, http.StatusInternalServerError, "no se pudo consultar la venta")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// SyncStatus: GET /api/sales/sync-status
func (h *SalesHandler) SyncStatus(w http.ResponseWriter, r *http.Request) {
	connections, err := h.sync.Status(r.Context())
	if err != nil {
		log.Printf("error reading sales sync status: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "no se pudo consultar el estado de la sincronización")
		return
	}
	enabled, interval := h.sync.Settings()
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":         enabled,
		"intervalMinutes": int(interval.Minutes()),
		"connections":     connections,
	})
}

// Sync: POST /api/sales/sync con {"connectionId": N}; sin connectionId
// sincroniza todas las conexiones activas.
func (h *SalesHandler) Sync(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ConnectionID int64 `json:"connectionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, "cuerpo de la petición inválido")
		return
	}
	if request.ConnectionID < 0 {
		writeJSONError(w, http.StatusBadRequest, "connectionId inválido")
		return
	}

	var actorID *int64
	if user, ok := UserFromContext(r.Context()); ok {
		id := int64(user.ID)
		actorID = &id
	}

	queued, err := h.sync.Request(r.Context(), request.ConnectionID, actorID)
	switch {
	case errors.Is(err, salesApp.ErrConnectionNotSyncable):
		writeJSONError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, salesApp.ErrSyncAlreadyRunning):
		writeJSONError(w, http.StatusConflict, err.Error())
	case errors.Is(err, salesApp.ErrSyncQueueFull):
		writeJSONError(w, http.StatusServiceUnavailable, err.Error())
	case err != nil:
		log.Printf("error requesting sales sync: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "no se pudo iniciar la sincronización")
	default:
		writeJSON(w, http.StatusAccepted, map[string]any{"connectionIds": queued})
	}
}

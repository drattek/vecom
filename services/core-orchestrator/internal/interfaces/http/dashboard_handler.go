package http

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	dashboardApp "core-orchestrator/internal/application/dashboard"
)

type DashboardHandler struct {
	service *dashboardApp.Service
}

func NewDashboardHandler(service *dashboardApp.Service) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// Listings: ?state=&offset=&pageSize= — publicaciones que requieren atención.
func (h *DashboardHandler) Listings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))

	page, err := h.service.Attention(r.Context(), q.Get("state"), offset, pageSize)
	if err != nil {
		writeDashboardError(w, "listings", err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// CatalogCount: ?filter=&connectionId= — total de un filtro del catálogo.
func (h *DashboardHandler) CatalogCount(w http.ResponseWriter, r *http.Request) {
	connectionID, ok := parseDashboardConnectionID(w, r)
	if !ok {
		return
	}

	total, err := h.service.CatalogCount(r.Context(), r.URL.Query().Get("filter"), connectionID)
	if err != nil {
		writeDashboardError(w, "catalog count", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"total": total})
}

// CatalogProducts: ?filter=&connectionId=&offset=&pageSize=
func (h *DashboardHandler) CatalogProducts(w http.ResponseWriter, r *http.Request) {
	connectionID, ok := parseDashboardConnectionID(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))

	page, err := h.service.CatalogProducts(r.Context(), q.Get("filter"), connectionID, offset, pageSize)
	if err != nil {
		writeDashboardError(w, "catalog products", err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *DashboardHandler) ConnectionsHealth(w http.ResponseWriter, r *http.Request) {
	connections, err := h.service.ConnectionsHealth(r.Context())
	if err != nil {
		writeDashboardError(w, "connections health", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connections": connections})
}

func (h *DashboardHandler) InventoryHealth(w http.ResponseWriter, r *http.Request) {
	sources, err := h.service.InventoryHealth(r.Context())
	if err != nil {
		writeDashboardError(w, "inventory health", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

func (h *DashboardHandler) QueueHealth(w http.ResponseWriter, r *http.Request) {
	queues, err := h.service.QueueHealth(r.Context())
	if err != nil {
		writeDashboardError(w, "queue health", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"queues": queues})
}

// parseDashboardConnectionID lee connectionId (opcional; 0 = todas). Un valor
// mal formado es un 400: tratarlo como "todas" mostraría cifras equivocadas.
func parseDashboardConnectionID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.URL.Query().Get("connectionId")
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
		return 0, false
	}
	return id, true
}

func writeDashboardError(w http.ResponseWriter, operation string, err error) {
	if errors.Is(err, dashboardApp.ErrInvalidState) ||
		errors.Is(err, dashboardApp.ErrInvalidFilter) ||
		errors.Is(err, dashboardApp.ErrConnectionRequired) {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Printf("dashboard %s: %v", operation, err)
	writeJSONError(w, http.StatusInternalServerError, "internal error")
}

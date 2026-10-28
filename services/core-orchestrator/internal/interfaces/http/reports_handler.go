package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	reportsApp "core-orchestrator/internal/application/reports"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type ReportsHandler struct {
	service *reportsApp.Service
}

func NewReportsHandler(service *reportsApp.Service) *ReportsHandler {
	return &ReportsHandler{service: service}
}

// PublicationsReport backs Reportes → Publicaciones.
// Query: sourceId, channelCode, connectionId (publicado en), missingChannelCode,
// missingConnectionId (no publicado en), withStockAndPrice, search, offset, pageSize.
func (h *ReportsHandler) PublicationsReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sourceID, _ := strconv.ParseInt(q.Get("sourceId"), 10, 64)
	connectionID, _ := strconv.ParseInt(q.Get("connectionId"), 10, 64)
	offset, _ := strconv.Atoi(q.Get("offset"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	missingConnectionID, _ := strconv.ParseInt(q.Get("missingConnectionId"), 10, 64)

	result, err := h.service.PublicationsReport(r.Context(), mysqlInfra.PublicationsReportFilter{
		SourceID:            sourceID,
		ChannelCode:         strings.TrimSpace(q.Get("channelCode")),
		ConnectionID:        connectionID,
		Search:              q.Get("search"),
		MissingChannelCode:  strings.TrimSpace(q.Get("missingChannelCode")),
		MissingConnectionID: missingConnectionID,
		WithStockAndPrice:   q.Get("withStockAndPrice") == "true" || q.Get("withStockAndPrice") == "1",
		Offset:              offset,
		PageSize:            pageSize,
	})
	if err != nil {
		log.Printf("publications report: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// maxAttributePairs acota cuántos pares producto:conexión acepta un request.
const maxAttributePairs = 400

// PublicationsAttributes backs la columna "Atributos MercadoLibre" del reporte.
// Query: pairs=productId:connectionId,productId:connectionId,...
func (h *ReportsHandler) PublicationsAttributes(w http.ResponseWriter, r *http.Request) {
	raw := strings.Split(r.URL.Query().Get("pairs"), ",")
	pairs := make([]reportsApp.AttributePair, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 {
			writeJSONError(w, http.StatusBadRequest, "invalid pairs")
			return
		}
		productID, err1 := strconv.ParseInt(parts[0], 10, 64)
		connectionID, err2 := strconv.ParseInt(parts[1], 10, 64)
		if err1 != nil || err2 != nil || productID <= 0 || connectionID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid pairs")
			return
		}
		pairs = append(pairs, reportsApp.AttributePair{ProductID: productID, ConnectionID: connectionID})
	}
	if len(pairs) > maxAttributePairs {
		writeJSONError(w, http.StatusBadRequest, "too many pairs")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"statuses": h.service.AttributeStatuses(r.Context(), pairs)})
}

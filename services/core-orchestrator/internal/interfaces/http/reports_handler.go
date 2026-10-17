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

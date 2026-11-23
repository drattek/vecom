package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	channelConfigApp "core-orchestrator/internal/application/channel_config"
)

// ConnectionConfigViewHandler sirve la vista de solo lectura de lo que una
// conexión tiene configurado (credenciales y ajustes ya enmascarados).
type ConnectionConfigViewHandler struct {
	service *channelConfigApp.ConnectionConfigViewService
}

func NewConnectionConfigViewHandler(service *channelConfigApp.ConnectionConfigViewService) *ConnectionConfigViewHandler {
	return &ConnectionConfigViewHandler{service: service}
}

// GetConfig: GET /api/channel-connections/{id}/config
func (h *ConnectionConfigViewHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid connection id")
		return
	}

	view, err := h.service.GetView(r.Context(), id)
	if err != nil {
		log.Printf("connection config view %d: %v", id, err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(view)
}

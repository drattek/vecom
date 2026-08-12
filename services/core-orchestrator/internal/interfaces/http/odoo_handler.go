package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	syncApp "core-orchestrator/internal/application/sync"
)

type OdooHandler struct {
	connectionService *syncApp.OdooConnectionService
}

func NewOdooHandler(connectionService *syncApp.OdooConnectionService) *OdooHandler {
	return &OdooHandler{connectionService: connectionService}
}

func (h *OdooHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
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

	var productID int64
	if productIDStr := strings.TrimSpace(r.URL.Query().Get("productId")); productIDStr != "" {
		productID, err = strconv.ParseInt(productIDStr, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid productId")
			return
		}
	}

	result, err := h.connectionService.TestConnection(r.Context(), connectionID, productID)
	if err != nil {
		if errors.Is(err, syncApp.ErrInvalidOdooConnection) {
			writeJSONError(w, http.StatusBadRequest, "invalid connectionId")
			return
		}
		if errors.Is(err, syncApp.ErrMissingOdooSettings) || errors.Is(err, syncApp.ErrMissingOdooCredentials) {
			writeJSONError(w, http.StatusBadRequest, "missing required Odoo connection settings or credentials")
			return
		}

		log.Printf("odoo test-connection failed for connectionId=%d: %v", connectionID, err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

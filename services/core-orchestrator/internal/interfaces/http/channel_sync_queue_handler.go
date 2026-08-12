package http

import (
	"encoding/json"
	"errors"
	"net/http"

	syncQueueApp "core-orchestrator/internal/application/channel_sync_queue"
)

// enqueueSyncQueueItemRequest mirrors one entry of the request body: a sku
// and every marketplace connection it should be queued for.
type enqueueSyncQueueItemRequest struct {
	SKU           string  `json:"sku"`
	ConnectionIDs []int64 `json:"connectionIds"`
}

type ChannelSyncQueueHandler struct {
	service *syncQueueApp.Service
}

func NewChannelSyncQueueHandler(service *syncQueueApp.Service) *ChannelSyncQueueHandler {
	return &ChannelSyncQueueHandler{service: service}
}

// Enqueue accepts a JSON array of {sku, connectionIds} and writes one
// ecom_channel_sync_queue row per (sku, connectionId) pair.
func (h *ChannelSyncQueueHandler) Enqueue(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req []enqueueSyncQueueItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	items := make([]syncQueueApp.EnqueueItemInput, 0, len(req))
	for _, item := range req {
		items = append(items, syncQueueApp.EnqueueItemInput{
			SKU:           item.SKU,
			ConnectionIDs: item.ConnectionIDs,
		})
	}

	result, err := h.service.Enqueue(syncQueueApp.EnqueueInput{
		Items:     items,
		UpdatedBy: user.ID,
	})
	if err != nil {
		if errors.Is(err, syncQueueApp.ErrEmptyEnqueueRequest) {
			writeJSONError(w, http.StatusBadRequest, "at least one item is required")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

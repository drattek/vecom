package http

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	meliNotificationsApp "core-orchestrator/internal/application/meli_notifications"
)

type MeliNotificationHandler struct {
	service *meliNotificationsApp.MeliNotificationService
}

func NewMeliNotificationHandler(service *meliNotificationsApp.MeliNotificationService) *MeliNotificationHandler {
	return &MeliNotificationHandler{service: service}
}

type meliNotificationRequest struct {
	ID            string     `json:"_id"`
	Resource      string     `json:"resource"`
	UserID        int64      `json:"user_id"`
	Topic         string     `json:"topic"`
	ApplicationID int64      `json:"application_id"`
	Attempts      int        `json:"attempts"`
	Sent          *time.Time `json:"sent"`
	Received      *time.Time `json:"received"`
}

// Receive is the callback registered as this app's notification URL in the
// MercadoLibre developer console (see ecom_meli_notifications in
// infrastructure/mysql/eco-architecture.sql for the table this persists
// into). MercadoLibre POSTs here directly with no way to attach our own JWT,
// so this route is public (see router.go) — same reasoning as
// /meli_callback.
//
// MercadoLibre requires a 2xx response within a short window or it
// redelivers the same notification with a growing `attempts` count and,
// after enough failed attempts, stops sending notifications to the app
// entirely. So this handler only stores the raw payload (status 'pending')
// and always acknowledges with 200 — even on a malformed body or a storage
// error, which are logged instead of surfaced, since rejecting the request
// would just trigger MercadoLibre retries without fixing anything. It never
// fetches `resource` itself: the notification body is only a pointer saying
// something changed, not the changed data — MercadoLibre's own guidance is
// to distrust the payload's business fields and re-fetch resource with the
// owning connection's access token. That fetch, and resolving which
// ecom_channel_connections row owns user_id, is left to whatever consumes
// ecom_meli_notifications next.
func (h *MeliNotificationHandler) Receive(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("meli notification: failed to read body: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	var req meliNotificationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("meli notification: invalid payload: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if _, err := h.service.Receive(meliNotificationsApp.ReceiveNotificationInput{
		NotificationID: req.ID,
		Resource:       req.Resource,
		Topic:          req.Topic,
		MeliUserID:     req.UserID,
		ApplicationID:  req.ApplicationID,
		Attempts:       req.Attempts,
		Sent:           req.Sent,
		Received:       req.Received,
		RawPayload:     string(body),
	}); err != nil {
		log.Printf("meli notification: failed to persist: %v", err)
	}

	w.WriteHeader(http.StatusOK)
}

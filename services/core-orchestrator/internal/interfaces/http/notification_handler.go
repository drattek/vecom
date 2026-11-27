package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	notificationsApp "core-orchestrator/internal/application/notifications"
	"core-orchestrator/internal/domain"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// NotificationHandler expone la configuración del bot (solo admin) y lo que
// cada usuario administra de sus propios avisos (ADR 0012). Las rutas de
// usuario actúan siempre sobre quien tiene la sesión, nunca sobre un id
// recibido.
type NotificationHandler struct {
	service *notificationsApp.Service
}

func NewNotificationHandler(service *notificationsApp.Service) *NotificationHandler {
	return &NotificationHandler{service: service}
}

type updateNotificationSettingsRequest struct {
	BotToken               string `json:"botToken"`
	IsEnabled              bool   `json:"isEnabled"`
	DailySummaryTime       string `json:"dailySummaryTime"`
	ErpSilenceHours        int    `json:"erpSilenceHours"`
	ConnectionCheckMinutes int    `json:"connectionCheckMinutes"`
}

// GetSettings: GET /api/notification-settings.
func (h *NotificationHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetSettings(r.Context())
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSONResponse(w, settings)
}

// UpdateSettings: PUT /api/notification-settings.
func (h *NotificationHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := notificationUser(w, r)
	if !ok {
		return
	}
	var req updateNotificationSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	settings, err := h.service.UpdateSettings(r.Context(), notificationsApp.UpdateSettingsInput{
		BotToken:               req.BotToken,
		IsEnabled:              req.IsEnabled,
		DailySummaryTime:       req.DailySummaryTime,
		ErpSilenceHours:        req.ErpSilenceHours,
		ConnectionCheckMinutes: req.ConnectionCheckMinutes,
		ActorID:                user.ID,
	})
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSONResponse(w, settings)
}

// GetMine: GET /api/me/notifications.
func (h *NotificationHandler) GetMine(w http.ResponseWriter, r *http.Request) {
	user, ok := notificationUser(w, r)
	if !ok {
		return
	}
	view, err := h.service.GetUserView(r.Context(), user)
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSONResponse(w, view)
}

// SetEnabled: PATCH /api/me/notifications.
func (h *NotificationHandler) SetEnabled(w http.ResponseWriter, r *http.Request) {
	user, ok := notificationUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.SetNotificationsEnabled(r.Context(), user.ID, *req.Enabled); err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSONResponse(w, map[string]bool{"updated": true})
}

// SavePreferences: PUT /api/me/notifications/preferences.
func (h *NotificationHandler) SavePreferences(w http.ResponseWriter, r *http.Request) {
	user, ok := notificationUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Preferences map[string]bool `json:"preferences"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Preferences) == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.SavePreferences(r.Context(), user, req.Preferences); err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSONResponse(w, map[string]bool{"updated": true})
}

// StartLink: POST /api/me/notifications/telegram/link.
func (h *NotificationHandler) StartLink(w http.ResponseWriter, r *http.Request) {
	user, ok := notificationUser(w, r)
	if !ok {
		return
	}
	link, err := h.service.StartLink(r.Context(), user.ID)
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSONResponse(w, link)
}

// Unlink: DELETE /api/me/notifications/telegram/link.
func (h *NotificationHandler) Unlink(w http.ResponseWriter, r *http.Request) {
	user, ok := notificationUser(w, r)
	if !ok {
		return
	}
	if err := h.service.Unlink(r.Context(), user.ID); err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSONResponse(w, map[string]bool{"updated": true})
}

// SendTest: POST /api/me/notifications/telegram/test.
func (h *NotificationHandler) SendTest(w http.ResponseWriter, r *http.Request) {
	user, ok := notificationUser(w, r)
	if !ok {
		return
	}
	if err := h.service.SendTest(r.Context(), user); err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSONResponse(w, map[string]bool{"sent": true})
}

func notificationUser(w http.ResponseWriter, r *http.Request) (*domain.AuthUser, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	return user, true
}

func writeNotificationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, notificationsApp.ErrInvalidSettings):
		writeJSONError(w, http.StatusBadRequest, "Revisa la hora del resumen y los intervalos.")
	case errors.Is(err, notificationsApp.ErrInvalidBotToken):
		writeJSONError(w, http.StatusBadRequest, "Telegram rechazó el token del bot.")
	case errors.Is(err, notificationsApp.ErrTokenRequired):
		writeJSONError(w, http.StatusBadRequest, "Captura el token del bot antes de activar las notificaciones.")
	case errors.Is(err, notificationsApp.ErrUnknownType):
		writeJSONError(w, http.StatusBadRequest, "El tipo de aviso no existe o no tienes permiso para recibirlo.")
	case errors.Is(err, notificationsApp.ErrUnavailable):
		writeJSONError(w, http.StatusConflict, "Un administrador aún no ha activado las notificaciones de Telegram.")
	case errors.Is(err, notificationsApp.ErrNotLinked):
		writeJSONError(w, http.StatusConflict, "Primero vincula tu cuenta de Telegram.")
	case errors.Is(err, notificationsApp.ErrBotBlocked):
		writeJSONError(w, http.StatusConflict, "Bloqueaste al bot en Telegram. Desbloquéalo e inténtalo de nuevo.")
	case errors.Is(err, notificationsApp.ErrTelegramUnreachable):
		log.Printf("notifications: %v", err)
		writeJSONError(w, http.StatusBadGateway, "No se pudo comunicar con Telegram. Inténtalo de nuevo.")
	case errors.Is(err, mysqlInfra.ErrInvalidCredentialsEncryptionKey):
		writeJSONError(w, http.StatusInternalServerError, "La llave de cifrado de credenciales no está configurada.")
	default:
		log.Printf("notifications: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
	}
}

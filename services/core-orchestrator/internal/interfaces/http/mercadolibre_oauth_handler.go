package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	syncApp "core-orchestrator/internal/application/sync"
)

// connectionsPagePath es la pantalla del dashboard a la que vuelve el usuario.
// Es relativa: el redirect_url de MercadoLibre apunta al dominio del
// dashboard, cuyo nginx reenvía /meli_callback a este servicio.
const connectionsPagePath = "/settings/marketplace-connections"

type mercadoLibreOAuthFlow interface {
	Start(ctx context.Context, connectionID, actorID int64, allowAccountChange bool) (string, error)
	Complete(ctx context.Context, state, code string) (syncApp.OAuthResult, error)
	Cancel(ctx context.Context, state string) (syncApp.OAuthResult, error)
	AppSettings(ctx context.Context, connectionID int64) (*syncApp.MercadoLibreAppSettings, error)
	UpdateAppSettings(ctx context.Context, connectionID, actorID int64, input syncApp.UpdateMercadoLibreAppInput) (*syncApp.UpdateMercadoLibreAppResult, error)
}

type MercadoLibreOAuthHandler struct {
	service mercadoLibreOAuthFlow
}

func NewMercadoLibreOAuthHandler(service mercadoLibreOAuthFlow) *MercadoLibreOAuthHandler {
	return &MercadoLibreOAuthHandler{service: service}
}

// Start: POST /api/channel-connections/{id}/mercadolibre/authorize, con
// {"allowAccountChange": bool} opcional. Devuelve la URL de MercadoLibre.
func (h *MercadoLibreOAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	connectionID, ok := parseOAuthConnectionID(w, r)
	if !ok {
		return
	}
	var request struct {
		AllowAccountChange bool `json:"allowAccountChange"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, "cuerpo de la petición inválido")
		return
	}
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "sesión no válida")
		return
	}

	authorizationURL, err := h.service.Start(r.Context(), connectionID, int64(user.ID), request.AllowAccountChange)
	switch {
	case errors.Is(err, syncApp.ErrOAuthConnection):
		writeJSONError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, syncApp.ErrMissingMercadoLibreSettings):
		writeJSONError(w, http.StatusBadRequest, "faltan ajustes de la conexión (app_id, client_secret o redirect_url): "+err.Error())
	case err != nil:
		log.Printf("mercadolibre oauth: connection %d — could not start: %v", connectionID, err)
		writeJSONError(w, http.StatusInternalServerError, "no se pudo iniciar la vinculación")
	default:
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]string{"authorizationUrl": authorizationURL})
	}
}

// GetApp: GET /api/channel-connections/{id}/mercadolibre/app. No devuelve el
// client_secret.
func (h *MercadoLibreOAuthHandler) GetApp(w http.ResponseWriter, r *http.Request) {
	connectionID, ok := parseOAuthConnectionID(w, r)
	if !ok {
		return
	}
	settings, err := h.service.AppSettings(r.Context(), connectionID)
	if err != nil {
		writeAppSettingsError(w, connectionID, "no se pudieron consultar los datos de la aplicación", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, settings)
}

// UpdateApp: PUT /api/channel-connections/{id}/mercadolibre/app con
// {"appId", "clientSecret" (vacío = conservar), "redirectUrl"}.
func (h *MercadoLibreOAuthHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	connectionID, ok := parseOAuthConnectionID(w, r)
	if !ok {
		return
	}
	var request struct {
		AppID        string `json:"appId"`
		ClientSecret string `json:"clientSecret"`
		RedirectURL  string `json:"redirectUrl"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "cuerpo de la petición inválido")
		return
	}
	user, found := UserFromContext(r.Context())
	if !found {
		writeJSONError(w, http.StatusUnauthorized, "sesión no válida")
		return
	}

	result, err := h.service.UpdateAppSettings(r.Context(), connectionID, int64(user.ID), syncApp.UpdateMercadoLibreAppInput{
		AppID:        request.AppID,
		ClientSecret: request.ClientSecret,
		RedirectURL:  request.RedirectURL,
	})
	if err != nil {
		writeAppSettingsError(w, connectionID, "no se pudieron guardar los datos de la aplicación", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func parseOAuthConnectionID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	connectionID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || connectionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "id de conexión inválido")
		return 0, false
	}
	return connectionID, true
}

func writeAppSettingsError(w http.ResponseWriter, connectionID int64, fallback string, err error) {
	switch {
	case errors.Is(err, syncApp.ErrOAuthConnection):
		writeJSONError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, syncApp.ErrInvalidMercadoLibreApp):
		writeJSONError(w, http.StatusBadRequest, err.Error())
	default:
		log.Printf("mercadolibre app settings: connection %d: %v", connectionID, err)
		writeJSONError(w, http.StatusInternalServerError, fallback)
	}
}

// Callback: GET /meli_callback. Es público; solo actúa con un state válido.
// Siempre regresa al dashboard con un código de resultado, nunca con el texto
// del error.
func (h *MercadoLibreOAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	state, code := query.Get("state"), query.Get("code")

	var result syncApp.OAuthResult
	var err error
	reason := ""
	if query.Get("error") != "" || code == "" {
		result, err = h.service.Cancel(r.Context(), state)
		reason = "denied"
	} else {
		result, err = h.service.Complete(r.Context(), state, code)
	}
	if err != nil {
		reason = oauthFailureReason(err)
		log.Printf("mercadolibre oauth: connection %d — callback rejected (%s): %v", result.ConnectionID, reason, err)
	}

	target := url.Values{}
	if result.ConnectionID > 0 {
		target.Set("connectionId", strconv.FormatInt(result.ConnectionID, 10))
	}
	if reason == "" {
		target.Set("meliAuth", "success")
		if result.Account != "" {
			target.Set("account", result.Account)
		}
	} else {
		target.Set("meliAuth", "error")
		target.Set("reason", reason)
	}

	// La URL de entrada lleva el code: que no se cachee ni viaje como referer.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, connectionsPagePath+"?"+target.Encode(), http.StatusSeeOther)
}

func oauthFailureReason(err error) string {
	switch {
	case errors.Is(err, syncApp.ErrOAuthStateInvalid):
		return "invalid_state"
	case errors.Is(err, syncApp.ErrOAuthAccountMismatch):
		return "account_mismatch"
	case errors.Is(err, syncApp.ErrOAuthAccountInUse):
		return "account_in_use"
	case errors.Is(err, syncApp.ErrMissingMercadoLibreSettings):
		return "missing_settings"
	case errors.Is(err, syncApp.ErrOAuthConnection):
		return "connection"
	case errors.Is(err, syncApp.ErrOAuthExchange):
		return "exchange"
	}
	return "internal"
}

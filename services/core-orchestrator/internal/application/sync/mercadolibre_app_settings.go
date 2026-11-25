package sync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// Datos de la aplicación de MercadoLibre de una conexión (ADR 0011). Cambiarlos
// no toca la conexión ni sus publicaciones: solo con qué aplicación se
// autentica. El client_secret nunca se devuelve.

const mercadoLibreCallbackPath = "/meli_callback"

// ErrInvalidMercadoLibreApp describe un dato de la aplicación no admitido. Su
// mensaje no incluye el valor recibido.
var ErrInvalidMercadoLibreApp = errors.New("datos de la aplicación no válidos")

var mercadoLibreAppIDPattern = regexp.MustCompile(`^[0-9]{5,20}$`)

type MercadoLibreAppSettings struct {
	AppID           string `json:"appId"`
	RedirectURL     string `json:"redirectUrl"`
	HasClientSecret bool   `json:"hasClientSecret"`
	// LinkedAccountID es la cuenta registrada en la conexión; nil si todavía
	// no se conoce.
	LinkedAccountID *int64 `json:"linkedAccountId"`
}

// UpdateMercadoLibreAppInput: ClientSecret vacío conserva el guardado.
type UpdateMercadoLibreAppInput struct {
	AppID        string
	ClientSecret string
	RedirectURL  string
}

type UpdateMercadoLibreAppResult struct {
	MercadoLibreAppSettings
	// AppChanged: los tokens actuales son de otra aplicación y hay que volver a
	// vincular la conexión.
	AppChanged bool `json:"appChanged"`
}

func describeAppSettings(settingsByKey map[string]mysqlInfra.ConnectionSettingDTO) MercadoLibreAppSettings {
	value := func(key string) string { return strings.TrimSpace(settingsByKey[key].Value) }
	result := MercadoLibreAppSettings{
		AppID:           value("app_id"),
		RedirectURL:     value("redirect_url"),
		HasClientSecret: value("client_secret") != "",
	}
	if account := storedAccount(settingsByKey); account > 0 {
		result.LinkedAccountID = &account
	}
	return result
}

// validateAppUpdate normaliza la entrada contra lo que la conexión tiene hoy y
// dice si cambia la aplicación.
func validateAppUpdate(input UpdateMercadoLibreAppInput, current MercadoLibreAppSettings) (UpdateMercadoLibreAppInput, bool, error) {
	input.AppID = strings.TrimSpace(input.AppID)
	input.ClientSecret = strings.TrimSpace(input.ClientSecret)
	input.RedirectURL = strings.TrimSpace(input.RedirectURL)
	invalid := func(message string) (UpdateMercadoLibreAppInput, bool, error) {
		return input, false, fmt.Errorf("%w: %s", ErrInvalidMercadoLibreApp, message)
	}

	if !mercadoLibreAppIDPattern.MatchString(input.AppID) {
		return invalid("el app_id debe ser el número de la aplicación")
	}
	redirect, err := url.Parse(input.RedirectURL)
	if err != nil || redirect.Scheme != "https" || redirect.Host == "" || redirect.User != nil ||
		redirect.Path != mercadoLibreCallbackPath || redirect.RawQuery != "" || redirect.Fragment != "" {
		return invalid("el redirect_url debe ser https://<dominio del dashboard>" + mercadoLibreCallbackPath + ", sin parámetros")
	}

	appChanged := input.AppID != current.AppID
	if input.ClientSecret == "" && (appChanged || !current.HasClientSecret) {
		return invalid("captura el client_secret de la aplicación")
	}
	if len(input.ClientSecret) > 256 || strings.ContainsAny(input.ClientSecret, " \t\r\n") {
		return invalid("el client_secret no tiene un formato válido")
	}
	return input, appChanged, nil
}

// AppSettings devuelve los datos de la aplicación, sin el client_secret.
func (s *MercadoLibreOAuthService) AppSettings(ctx context.Context, connectionID int64) (*MercadoLibreAppSettings, error) {
	if err := s.requireConnection(ctx, connectionID); err != nil {
		return nil, err
	}
	settings, err := s.tokens.settingsRepository.FindByConnectionID(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("error loading connection settings: %w", err)
	}
	result := describeAppSettings(mercadoLibreSettingsByKey(settings))
	return &result, nil
}

// UpdateAppSettings guarda app_id, client_secret (cifrado) y redirect_url. No
// modifica ni renueva los tokens: un respaldo previo de ellos sigue sirviendo.
func (s *MercadoLibreOAuthService) UpdateAppSettings(ctx context.Context, connectionID, actorID int64, input UpdateMercadoLibreAppInput) (*UpdateMercadoLibreAppResult, error) {
	current, err := s.AppSettings(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	release, err := s.tokens.locks.acquire(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	defer release()
	// Se relee ya con el candado tomado.
	if current, err = s.AppSettings(ctx, connectionID); err != nil {
		return nil, err
	}

	input, appChanged, err := validateAppUpdate(input, *current)
	if err != nil {
		return nil, err
	}

	// Al cambiar de aplicación, los tokens dejarán de renovarse: se registra
	// antes la cuenta actual para poder verificarla al volver a vincular.
	var account int64
	if appChanged && current.LinkedAccountID == nil {
		if account, err = s.currentAccount(ctx, connectionID); err != nil {
			return nil, err
		}
	}

	var saved MercadoLibreAppSettings
	err = s.mutateWithBackup(ctx, connectionID, actorID, backupReasonAppUpdate,
		func(repository *mysqlInfra.ConnectionSettingsRepository, settingsByKey map[string]mysqlInfra.ConnectionSettingDTO) error {
			// app_id y redirect_url conservan su cifrado actual.
			values := []struct {
				key, value string
				encrypted  bool
			}{
				{"app_id", input.AppID, settingsByKey["app_id"].IsEncrypted},
				{"redirect_url", input.RedirectURL, settingsByKey["redirect_url"].IsEncrypted},
			}
			if input.ClientSecret != "" {
				values = append(values, struct {
					key, value string
					encrypted  bool
				}{"client_secret", input.ClientSecret, true})
			}
			if account > 0 {
				values = append(values, struct {
					key, value string
					encrypted  bool
				}{mercadoLibreUserIDSetting, strconv.FormatInt(account, 10), true})
			}
			for _, v := range values {
				if err := upsertSettingValue(ctx, repository, settingsByKey, connectionID, v.key, v.value, v.encrypted, actorID); err != nil {
					return fmt.Errorf("error saving %s: %w", v.key, err)
				}
			}

			updated, err := repository.FindByConnectionID(ctx, connectionID)
			if err != nil {
				return fmt.Errorf("error reloading connection settings: %w", err)
			}
			saved = describeAppSettings(mercadoLibreSettingsByKey(updated))
			return nil
		})
	if err != nil {
		return nil, err
	}

	log.Printf("mercadolibre app settings: connection %d updated by user %d (app changed: %t)", connectionID, actorID, appChanged)
	return &UpdateMercadoLibreAppResult{MercadoLibreAppSettings: saved, AppChanged: appChanged}, nil
}

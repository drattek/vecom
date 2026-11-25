package sync

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// Vinculación de una conexión con una cuenta de MercadoLibre (OAuth).
//
// MercadoLibre devuelve al usuario a una URL pública, sin la sesión del
// dashboard. Lo único que liga ese regreso con quien inició la vinculación es
// el parámetro state: aleatorio, de un solo uso, con caducidad y guardado del
// lado del servidor. Sin un state válido no se toca ninguna conexión.
//
// Además usa PKCE (S256): en la URL viaja solo la huella de un verificador
// aleatorio que se queda en el servidor, junto al state, y se envía a
// MercadoLibre al canjear el code. Quien intercepte el code no puede usarlo.

const (
	mercadoLibreOAuthChannelCode = "MERCADOLIBRE"
	mercadoLibreOAuthStateTTL    = 10 * time.Minute
	// mercadoLibreUserIDSetting es la cuenta vinculada a la conexión. Se
	// registra al vincular y en cada renovación del token.
	mercadoLibreUserIDSetting = "meli_user_id"
)

var (
	ErrOAuthStateInvalid    = errors.New("el enlace de vinculación no es válido, caducó o ya se usó")
	ErrOAuthConnection      = errors.New("la conexión no existe o no es de MercadoLibre")
	ErrOAuthAccountMismatch = errors.New("la cuenta autorizada no es la que la conexión tiene vinculada")
	ErrOAuthAccountInUse    = errors.New("la cuenta autorizada ya está vinculada a otra conexión")
	ErrOAuthExchange        = errors.New("MercadoLibre no entregó los tokens")
)

// OAuthStateStore guarda los states pendientes. Take debe leer y eliminar de
// forma atómica.
type OAuthStateStore interface {
	Save(ctx context.Context, state string, payload []byte, ttl time.Duration) error
	Take(ctx context.Context, state string) ([]byte, bool, error)
}

type oauthPendingState struct {
	ConnectionID       int64 `json:"connectionId"`
	ActorID            int64 `json:"actorId"`
	AllowAccountChange bool  `json:"allowAccountChange"`
	// CodeVerifier es el verificador PKCE; no sale del servidor salvo hacia
	// el canje del code.
	CodeVerifier string `json:"codeVerifier"`
}

// OAuthResult describe una vinculación. ConnectionID viene informado en cuanto
// se identifica la conexión, también cuando hay error.
type OAuthResult struct {
	ConnectionID int64
	Account      string
}

type MercadoLibreOAuthService struct {
	db          *sql.DB
	tokens      *MercadoLibreTokenService
	connections *mysqlInfra.ChannelConnectionRepository
	users       *mercadoLibreInfra.UsersHandler
	states      OAuthStateStore
}

func NewMercadoLibreOAuthService(
	db *sql.DB,
	tokens *MercadoLibreTokenService,
	connections *mysqlInfra.ChannelConnectionRepository,
	users *mercadoLibreInfra.UsersHandler,
	states OAuthStateStore,
) *MercadoLibreOAuthService {
	return &MercadoLibreOAuthService{db: db, tokens: tokens, connections: connections, users: users, states: states}
}

type oauthAppSettings struct {
	appID, clientSecret, redirectURL string
}

func requiredOAuthSettings(settingsByKey map[string]mysqlInfra.ConnectionSettingDTO) (oauthAppSettings, error) {
	var app oauthAppSettings
	for _, field := range []struct {
		key    string
		target *string
	}{
		{"app_id", &app.appID},
		{"client_secret", &app.clientSecret},
		{"redirect_url", &app.redirectURL},
	} {
		*field.target = strings.TrimSpace(settingsByKey[field.key].Value)
		if *field.target == "" {
			return app, fmt.Errorf("%w: %s", ErrMissingMercadoLibreSettings, field.key)
		}
	}
	return app, nil
}

func (s *MercadoLibreOAuthService) requireConnection(ctx context.Context, connectionID int64) error {
	connection, err := s.connections.FindByID(ctx, connectionID)
	if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
		return ErrOAuthConnection
	}
	if err != nil {
		return fmt.Errorf("error loading connection %d: %w", connectionID, err)
	}
	if !strings.EqualFold(strings.TrimSpace(connection.ChannelCode), mercadoLibreOAuthChannelCode) {
		return ErrOAuthConnection
	}
	return nil
}

// newOAuthSecret genera 32 bytes aleatorios en base64url (43 caracteres).
// Sirve para el state y para el verificador PKCE, que pide entre 43 y 128.
func newOAuthSecret() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("error generating oauth secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

// pkceChallenge es el code_challenge S256 de un verificador (RFC 7636).
func pkceChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

// Start devuelve la URL de MercadoLibre para vincular la conexión.
// allowAccountChange permite quedar vinculado a una cuenta distinta.
func (s *MercadoLibreOAuthService) Start(ctx context.Context, connectionID, actorID int64, allowAccountChange bool) (string, error) {
	if err := s.requireConnection(ctx, connectionID); err != nil {
		return "", err
	}
	settings, err := s.tokens.settingsRepository.FindByConnectionID(ctx, connectionID)
	if err != nil {
		return "", fmt.Errorf("error loading connection settings: %w", err)
	}
	app, err := requiredOAuthSettings(mercadoLibreSettingsByKey(settings))
	if err != nil {
		return "", err
	}

	state, err := newOAuthSecret()
	if err != nil {
		return "", err
	}
	verifier, err := newOAuthSecret()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(oauthPendingState{
		ConnectionID:       connectionID,
		ActorID:            actorID,
		AllowAccountChange: allowAccountChange,
		CodeVerifier:       verifier,
	})
	if err != nil {
		return "", fmt.Errorf("error encoding oauth state: %w", err)
	}
	if err := s.states.Save(ctx, state, payload, mercadoLibreOAuthStateTTL); err != nil {
		return "", err
	}
	return s.tokens.authHandler.AuthorizationURL(app.appID, app.redirectURL, state, pkceChallenge(verifier)), nil
}

// Cancel descarta un state, p. ej. cuando el usuario rechaza la autorización.
func (s *MercadoLibreOAuthService) Cancel(ctx context.Context, state string) (OAuthResult, error) {
	pending, err := s.take(ctx, state)
	if err != nil {
		return OAuthResult{}, err
	}
	return OAuthResult{ConnectionID: pending.ConnectionID}, nil
}

func (s *MercadoLibreOAuthService) take(ctx context.Context, state string) (oauthPendingState, error) {
	var pending oauthPendingState
	if strings.TrimSpace(state) == "" {
		return pending, ErrOAuthStateInvalid
	}
	payload, found, err := s.states.Take(ctx, state)
	if err != nil {
		return pending, err
	}
	if !found {
		return pending, ErrOAuthStateInvalid
	}
	if err := json.Unmarshal(payload, &pending); err != nil || pending.ConnectionID <= 0 || pending.CodeVerifier == "" {
		return pending, ErrOAuthStateInvalid
	}
	return pending, nil
}

// Complete canjea el code y guarda los tokens en la conexión del state. Un
// fallo no modifica la conexión: sus tokens anteriores siguen en uso.
func (s *MercadoLibreOAuthService) Complete(ctx context.Context, state, code string) (OAuthResult, error) {
	pending, err := s.take(ctx, state)
	if err != nil {
		return OAuthResult{}, err
	}
	result := OAuthResult{ConnectionID: pending.ConnectionID}
	if strings.TrimSpace(code) == "" {
		return result, ErrOAuthExchange
	}
	if err := s.requireConnection(ctx, pending.ConnectionID); err != nil {
		return result, err
	}

	settings, err := s.tokens.settingsRepository.FindByConnectionID(ctx, pending.ConnectionID)
	if err != nil {
		return result, fmt.Errorf("error loading connection settings: %w", err)
	}
	app, err := requiredOAuthSettings(mercadoLibreSettingsByKey(settings))
	if err != nil {
		return result, err
	}

	tokens, err := s.tokens.authHandler.ExchangeAuthorizationCode(ctx, mercadoLibreInfra.ExchangeCodeRequest{
		ClientID:     app.appID,
		ClientSecret: app.clientSecret,
		Code:         strings.TrimSpace(code),
		RedirectURI:  app.redirectURL,
		CodeVerifier: pending.CodeVerifier,
	})
	if err != nil {
		return result, fmt.Errorf("%w: %w", ErrOAuthExchange, err)
	}
	if strings.TrimSpace(tokens.AccessToken) == "" || strings.TrimSpace(tokens.RefreshToken) == "" {
		return result, fmt.Errorf("%w: respuesta sin tokens", ErrOAuthExchange)
	}

	// El nickname es informativo; el id sí es obligatorio para verificar.
	userID := tokens.UserID
	me, meErr := s.users.GetMe(ctx, tokens.AccessToken)
	if meErr == nil {
		result.Account = me.Nickname
		if userID <= 0 {
			userID = me.ID
		}
	}
	if userID <= 0 {
		return result, fmt.Errorf("%w: no se pudo identificar la cuenta autorizada", ErrOAuthExchange)
	}

	// Desde aquí hasta guardar, nadie más reescribe los ajustes.
	release, err := s.tokens.locks.acquire(ctx, pending.ConnectionID)
	if err != nil {
		return result, err
	}
	defer release()

	if !pending.AllowAccountChange {
		if err := s.verifyAccount(ctx, pending.ConnectionID, userID); err != nil {
			return result, err
		}
	}

	if err := s.saveTokens(ctx, pending, tokens, userID); err != nil {
		return result, err
	}
	log.Printf("mercadolibre oauth: connection %d — account linked", pending.ConnectionID)
	return result, nil
}

// verifyAccount evita vincular por error otra cuenta (p. ej. con la sesión que
// el navegador ya tenía abierta).
func (s *MercadoLibreOAuthService) verifyAccount(ctx context.Context, connectionID, userID int64) error {
	current, err := s.currentAccount(ctx, connectionID)
	if err != nil {
		return err
	}
	var others []int64
	if current <= 0 {
		if others, err = s.otherConnectionAccounts(ctx, connectionID); err != nil {
			return err
		}
	}
	return checkLinkedAccount(current, userID, others)
}

// checkLinkedAccount: si la conexión ya tiene cuenta, debe ser la misma. Sin
// cuenta previa conocida, al menos no debe ser la de otra conexión.
func checkLinkedAccount(current, authorized int64, otherConnections []int64) error {
	if current > 0 {
		if current != authorized {
			return ErrOAuthAccountMismatch
		}
		return nil
	}
	if slices.Contains(otherConnections, authorized) {
		return ErrOAuthAccountInUse
	}
	return nil
}

func (s *MercadoLibreOAuthService) otherConnectionAccounts(ctx context.Context, connectionID int64) ([]int64, error) {
	others, err := s.connections.FindActiveByChannelCode(ctx, mercadoLibreOAuthChannelCode)
	if err != nil {
		return nil, err
	}
	accounts := make([]int64, 0, len(others))
	for _, other := range others {
		if other.ID == connectionID {
			continue
		}
		settings, err := s.tokens.settingsRepository.FindByConnectionID(ctx, other.ID)
		if err != nil {
			return nil, fmt.Errorf("error loading connection settings: %w", err)
		}
		if account := storedAccount(mercadoLibreSettingsByKey(settings)); account > 0 {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func storedAccount(settingsByKey map[string]mysqlInfra.ConnectionSettingDTO) int64 {
	id, err := strconv.ParseInt(strings.TrimSpace(settingsByKey[mercadoLibreUserIDSetting].Value), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// currentAccount devuelve 0 cuando no hay forma de saber qué cuenta tiene hoy
// la conexión (nunca se vinculó, o sus tokens ya no sirven).
func (s *MercadoLibreOAuthService) currentAccount(ctx context.Context, connectionID int64) (int64, error) {
	settings, err := s.tokens.settingsRepository.FindByConnectionID(ctx, connectionID)
	if err != nil {
		return 0, fmt.Errorf("error loading connection settings: %w", err)
	}
	settingsByKey := mercadoLibreSettingsByKey(settings)
	if id := storedAccount(settingsByKey); id > 0 {
		return id, nil
	}
	// El token guardado puede seguir vigente aunque ya no se pueda renovar,
	// como después de cambiar de aplicación.
	if token := strings.TrimSpace(settingsByKey["access_token"].Value); token != "" {
		if me, err := s.users.GetMe(ctx, token); err == nil && me.ID > 0 {
			return me.ID, nil
		}
	}
	return 0, nil
}

// saveTokens guarda todo o nada: tokens de una cuenta con el id de otra
// dejarían la conexión inconsistente.
func (s *MercadoLibreOAuthService) saveTokens(ctx context.Context, pending oauthPendingState, tokens *mercadoLibreInfra.RefreshTokenResponse, userID int64) error {
	ttl := mercadoLibreTokenTTL
	if tokens.ExpiresIn > 0 {
		ttl = time.Duration(tokens.ExpiresIn) * time.Second
	}
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	err := s.mutateWithBackup(ctx, pending.ConnectionID, pending.ActorID, backupReasonAccountLink,
		func(repository *mysqlInfra.ConnectionSettingsRepository, settingsByKey map[string]mysqlInfra.ConnectionSettingDTO) error {
			for _, value := range []struct{ key, value string }{
				{"access_token", tokens.AccessToken},
				{"refresh_token", tokens.RefreshToken},
				{"expiration_time", expiresAt.Format(time.RFC3339)},
				{mercadoLibreUserIDSetting, strconv.FormatInt(userID, 10)},
			} {
				if err := upsertSetting(ctx, repository, settingsByKey, pending.ConnectionID, value.key, value.value, pending.ActorID); err != nil {
					return fmt.Errorf("error saving %s: %w", value.key, err)
				}
			}
			return nil
		})
	if err != nil {
		return err
	}

	if s.tokens.statusRepository != nil {
		if _, err := s.tokens.statusRepository.Upsert(ctx, mysqlInfra.UpsertConnectionStatusInput{
			ConnectionID:  pending.ConnectionID,
			Authenticated: true,
			LastAuth:      &now,
			ExpiresAt:     &expiresAt,
		}); err != nil {
			log.Printf("Error recording connection status for connection %d: %v", pending.ConnectionID, err)
		}
	}
	return nil
}

package notifications

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"core-orchestrator/internal/domain"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/infrastructure/telegram"
)

const (
	// linkCodeTTL es la vigencia del enlace de vinculación.
	linkCodeTTL = 10 * time.Minute
	// linkCodeBytes produce 32 caracteres; Telegram admite hasta 64 en ?start=.
	linkCodeBytes = 24

	minErpSilenceHours        = 1
	maxErpSilenceHours        = 240
	minConnectionCheckMinutes = 5
	maxConnectionCheckMinutes = 1440
)

var (
	ErrInvalidSettings     = errors.New("invalid notification settings")
	ErrInvalidBotToken     = errors.New("telegram rejected the bot token")
	ErrTokenRequired       = errors.New("a bot token is required to enable notifications")
	ErrTelegramUnreachable = errors.New("could not reach telegram")
	ErrUnavailable         = errors.New("telegram notifications are not enabled")
	ErrNotLinked           = errors.New("telegram account is not linked")
	ErrBotBlocked          = errors.New("the user blocked the bot")
	ErrUnknownType         = errors.New("unknown or not allowed notification type")
)

type settingsStore interface {
	Get(ctx context.Context) (*mysqlInfra.NotificationSettingsDTO, error)
	Update(ctx context.Context, input mysqlInfra.UpdateNotificationSettingsInput) error
	SaveUpdateOffset(ctx context.Context, botUsername string, offset int64) error
}

type linkStore interface {
	FindByUserID(ctx context.Context, userID int64) (*mysqlInfra.TelegramLinkDTO, error)
	FindByChatID(ctx context.Context, chatID int64) (*mysqlInfra.TelegramLinkDTO, error)
	FindByCodeHash(ctx context.Context, codeHash string) (*mysqlInfra.TelegramLinkDTO, error)
	SaveLinkCode(ctx context.Context, userID int64, codeHash string, expiresAt time.Time) error
	Complete(ctx context.Context, input mysqlInfra.CompleteTelegramLinkInput) (bool, error)
	Unlink(ctx context.Context, userID int64) error
	SetNotificationsEnabled(ctx context.Context, userID int64, enabled bool) error
	SetBlockedByChatID(ctx context.Context, chatID int64, blocked bool) error
}

type erpStateStore interface {
	RecordCompleted(ctx context.Context, feedCode string, total, failed int, at time.Time) error
	RecordFailed(ctx context.Context, feedCode, message string, at time.Time) error
	List(ctx context.Context) ([]mysqlInfra.ErpSyncStateDTO, error)
}

type secretCipher interface {
	Encrypt(value string) (string, error)
	Decrypt(value string) (string, error)
}

// BotAPI lo implementa telegram.Client.
type BotAPI interface {
	GetMe(ctx context.Context, token string) (*telegram.User, error)
	SendMessage(ctx context.Context, token string, chatID int64, text string) error
	SendDocument(ctx context.Context, token string, chatID int64, fileName string, content []byte, caption string) error
	GetUpdates(ctx context.Context, token string, offset int64, wait time.Duration) ([]telegram.Update, error)
}

type Service struct {
	db          *sql.DB
	settings    settingsStore
	links       linkStore
	preferences *mysqlInfra.NotificationPreferenceRepository
	cipher      secretCipher
	bot         BotAPI
	now         func() time.Time

	// inUnit agrupa un incidente y sus envíos en una transacción.
	inUnit        unitRunner
	erpState      erpStateStore
	batches       batchSource
	listingStates listingStateSource
	stockPauses   stockPauseSource
	jobs          jobStore
	closures      closureStore
	closeListing  listingCloser
	// closureSource lo registra el arranque.
	closureSource ClosureSource
	incidents     incidentLister
	deliveries    deliveryStore
	claimDelivery deliveryClaimer

	connections      connectionLister
	connectionChecks map[string]ConnectionCheck
	// connectionStates solo lo usa Monitor.
	connectionStates map[int64]*connectionState
}

func NewService(
	db *sql.DB,
	settings *mysqlInfra.NotificationSettingsRepository,
	links *mysqlInfra.TelegramLinkRepository,
	preferences *mysqlInfra.NotificationPreferenceRepository,
	cipher *mysqlInfra.CredentialCipher,
	bot BotAPI,
	connections *mysqlInfra.ChannelConnectionRepository,
) *Service {
	return &Service{
		db:          db,
		settings:    settings,
		links:       links,
		preferences: preferences,
		cipher:      cipher,
		bot:         bot,
		now:         time.Now,

		inUnit:        newSQLUnitRunner(db),
		erpState:      mysqlInfra.NewErpSyncStateRepository(db),
		batches:       mysqlInfra.NewNotificationQueryRepository(db),
		listingStates: mysqlInfra.NewNotificationQueryRepository(db),
		stockPauses:   mysqlInfra.NewNotificationQueryRepository(db),
		jobs:          mysqlInfra.NewScheduledJobRunRepository(db),
		closures:      mysqlInfra.NewListingClosureRepository(db),
		closeListing:  newSQLListingCloser(db),
		incidents:     mysqlInfra.NewNotificationIncidentRepository(db),
		deliveries:    mysqlInfra.NewNotificationDeliveryRepository(db),
		claimDelivery: newSQLDeliveryClaimer(db),

		connections:      connections,
		connectionChecks: make(map[string]ConnectionCheck),
		connectionStates: make(map[int64]*connectionState),
	}
}

// ---------------------------------------------------------------------------
// Configuración (solo admin)
// ---------------------------------------------------------------------------

// Settings es la vista de la configuración. El token no se incluye.
type Settings struct {
	HasToken               bool   `json:"hasToken"`
	BotUsername            string `json:"botUsername"`
	IsEnabled              bool   `json:"isEnabled"`
	DailySummaryTime       string `json:"dailySummaryTime"`
	Timezone               string `json:"timezone"`
	ErpSilenceHours        int    `json:"erpSilenceHours"`
	ConnectionCheckMinutes int    `json:"connectionCheckMinutes"`
}

type UpdateSettingsInput struct {
	// BotToken vacío conserva el token guardado.
	BotToken               string
	IsEnabled              bool
	DailySummaryTime       string // "HH:MM"
	ErpSilenceHours        int
	ConnectionCheckMinutes int
	ActorID                int64
}

func (s *Service) GetSettings(ctx context.Context) (*Settings, error) {
	current, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	return &Settings{
		HasToken:               current.BotTokenEncrypted != "",
		BotUsername:            current.BotUsername,
		IsEnabled:              current.IsEnabled,
		DailySummaryTime:       shortTime(current.DailySummaryTime),
		Timezone:               current.Timezone,
		ErpSilenceHours:        current.ErpSilenceHours,
		ConnectionCheckMinutes: current.ConnectionCheckMinutes,
	}, nil
}

func (s *Service) UpdateSettings(ctx context.Context, input UpdateSettingsInput) (*Settings, error) {
	summaryTime, err := time.Parse("15:04", strings.TrimSpace(input.DailySummaryTime))
	if err != nil {
		return nil, fmt.Errorf("%w: dailySummaryTime must be HH:MM", ErrInvalidSettings)
	}
	if input.ErpSilenceHours < minErpSilenceHours || input.ErpSilenceHours > maxErpSilenceHours {
		return nil, fmt.Errorf("%w: erpSilenceHours must be between %d and %d", ErrInvalidSettings, minErpSilenceHours, maxErpSilenceHours)
	}
	if input.ConnectionCheckMinutes < minConnectionCheckMinutes || input.ConnectionCheckMinutes > maxConnectionCheckMinutes {
		return nil, fmt.Errorf("%w: connectionCheckMinutes must be between %d and %d", ErrInvalidSettings, minConnectionCheckMinutes, maxConnectionCheckMinutes)
	}

	current, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}

	update := mysqlInfra.UpdateNotificationSettingsInput{
		BotTokenEncrypted:      current.BotTokenEncrypted,
		BotUsername:            current.BotUsername,
		IsEnabled:              input.IsEnabled,
		DailySummaryTime:       summaryTime.Format("15:04:05"),
		ErpSilenceHours:        input.ErpSilenceHours,
		ConnectionCheckMinutes: input.ConnectionCheckMinutes,
		TelegramUpdateOffset:   current.TelegramUpdateOffset,
		ActorID:                input.ActorID,
	}

	if token := strings.TrimSpace(input.BotToken); token != "" {
		me, err := s.bot.GetMe(ctx, token)
		if err != nil {
			return nil, classifyTokenError(err)
		}
		encrypted, err := s.cipher.Encrypt(token)
		if err != nil {
			return nil, fmt.Errorf("encrypting bot token: %w", err)
		}
		update.BotTokenEncrypted = encrypted
		// Cada bot numera sus updates por separado.
		if me.Username != current.BotUsername {
			update.TelegramUpdateOffset = 0
		}
		update.BotUsername = me.Username
	}

	if update.IsEnabled && update.BotTokenEncrypted == "" {
		return nil, ErrTokenRequired
	}

	if err := s.settings.Update(ctx, update); err != nil {
		return nil, err
	}
	return s.GetSettings(ctx)
}

func classifyTokenError(err error) error {
	if errors.Is(err, telegram.ErrMalformedToken) {
		return ErrInvalidBotToken
	}
	var apiErr *telegram.APIError
	if errors.As(err, &apiErr) && (apiErr.Code == 401 || apiErr.Code == 404) {
		return ErrInvalidBotToken
	}
	return fmt.Errorf("%w: %v", ErrTelegramUnreachable, err)
}

// activeBot es el bot en uso; ok=false si está apagado o sin token.
type activeBot struct {
	token        string
	username     string
	updateOffset int64
}

func (s *Service) activeBot(ctx context.Context) (activeBot, bool, error) {
	current, err := s.settings.Get(ctx)
	if err != nil {
		return activeBot{}, false, err
	}
	if !current.IsEnabled || current.BotTokenEncrypted == "" || current.BotUsername == "" {
		return activeBot{}, false, nil
	}
	token, err := s.cipher.Decrypt(current.BotTokenEncrypted)
	if err != nil {
		return activeBot{}, false, fmt.Errorf("decrypting bot token: %w", err)
	}
	return activeBot{token: token, username: current.BotUsername, updateOffset: current.TelegramUpdateOffset}, true, nil
}

// ---------------------------------------------------------------------------
// Cada usuario: su vínculo y sus avisos
// ---------------------------------------------------------------------------

// Estados del vínculo que ve el usuario.
const (
	LinkStatusNone    = "none"
	LinkStatusLinked  = "linked"
	LinkStatusBlocked = "blocked"
)

type LinkView struct {
	Status string `json:"status"`
	// NeedsRelink: el vínculo es de un bot que ya no se usa.
	NeedsRelink      bool       `json:"needsRelink"`
	TelegramUsername string     `json:"telegramUsername"`
	TelegramName     string     `json:"telegramName"`
	LinkedAt         *time.Time `json:"linkedAt"`
}

type PreferenceView struct {
	Type        string `json:"type"`
	Group       string `json:"group"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Delivery    string `json:"delivery"`
	// Active: el aviso ya se envía.
	Active  bool `json:"active"`
	Enabled bool `json:"enabled"`
}

type UserView struct {
	// Available: el administrador activó las notificaciones.
	Available            bool             `json:"available"`
	Link                 LinkView         `json:"link"`
	NotificationsEnabled bool             `json:"notificationsEnabled"`
	DailySummaryTime     string           `json:"dailySummaryTime"`
	Timezone             string           `json:"timezone"`
	Preferences          []PreferenceView `json:"preferences"`
}

func (s *Service) GetUserView(ctx context.Context, user *domain.AuthUser) (*UserView, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	link, err := s.links.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	saved, err := s.preferences.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	view := &UserView{
		Available:            settings.IsEnabled && settings.BotTokenEncrypted != "",
		Link:                 linkView(link, settings.BotUsername),
		NotificationsEnabled: link == nil || link.NotificationsEnabled,
		DailySummaryTime:     shortTime(settings.DailySummaryTime),
		Timezone:             settings.Timezone,
		Preferences:          []PreferenceView{},
	}
	for _, definition := range AllowedDefinitions(user) {
		view.Preferences = append(view.Preferences, PreferenceView{
			Type:        definition.Type,
			Group:       definition.Group,
			Label:       definition.Label,
			Description: definition.Description,
			Delivery:    definition.Delivery,
			Active:      definition.Active,
			Enabled:     saved[definition.Type],
		})
	}
	return view, nil
}

func linkView(link *mysqlInfra.TelegramLinkDTO, currentBot string) LinkView {
	if link == nil || (link.Status != mysqlInfra.TelegramLinkLinked && link.Status != mysqlInfra.TelegramLinkBlocked) {
		return LinkView{Status: LinkStatusNone}
	}
	status := LinkStatusLinked
	if link.Status == mysqlInfra.TelegramLinkBlocked {
		status = LinkStatusBlocked
	}
	return LinkView{
		Status:           status,
		NeedsRelink:      link.BotUsername != currentBot,
		TelegramUsername: link.TelegramUsername,
		TelegramName:     link.TelegramName,
		LinkedAt:         link.LinkedAt,
	}
}

type LinkRequest struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// StartLink genera el enlace de un solo uso para vincular Telegram.
func (s *Service) StartLink(ctx context.Context, userID int64) (*LinkRequest, error) {
	bot, ok, err := s.activeBot(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrUnavailable
	}

	raw := make([]byte, linkCodeBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generating link code: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(raw)
	expiresAt := s.now().Add(linkCodeTTL)

	if err := s.links.SaveLinkCode(ctx, userID, hashLinkCode(code), expiresAt); err != nil {
		return nil, err
	}
	return &LinkRequest{URL: "https://t.me/" + bot.username + "?start=" + code, ExpiresAt: expiresAt}, nil
}

func hashLinkCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Unlink(ctx context.Context, userID int64) error {
	return s.links.Unlink(ctx, userID)
}

func (s *Service) SetNotificationsEnabled(ctx context.Context, userID int64, enabled bool) error {
	link, err := s.links.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if link == nil {
		return ErrNotLinked
	}
	return s.links.SetNotificationsEnabled(ctx, userID, enabled)
}

// SavePreferences guarda tipo → activado. Rechaza todo el cambio si incluye un
// tipo que no existe o que el usuario no puede recibir.
func (s *Service) SavePreferences(ctx context.Context, user *domain.AuthUser, preferences map[string]bool) error {
	allowed := make(map[string]bool)
	for _, definition := range AllowedDefinitions(user) {
		allowed[definition.Type] = true
	}
	for notificationType := range preferences {
		if !allowed[notificationType] {
			return fmt.Errorf("%w: %s", ErrUnknownType, notificationType)
		}
	}

	return mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		repository := mysqlInfra.NewNotificationPreferenceRepository(tx)
		for notificationType, enabled := range preferences {
			if err := repository.Upsert(ctx, user.ID, notificationType, enabled); err != nil {
				return err
			}
		}
		return nil
	})
}

// SendTest envía un mensaje de prueba al Telegram del usuario.
func (s *Service) SendTest(ctx context.Context, user *domain.AuthUser) error {
	bot, ok, err := s.activeBot(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUnavailable
	}
	link, err := s.links.FindByUserID(ctx, user.ID)
	if err != nil {
		return err
	}
	if link == nil || link.ChatID == nil || link.BotUsername != bot.username ||
		(link.Status != mysqlInfra.TelegramLinkLinked && link.Status != mysqlInfra.TelegramLinkBlocked) {
		return ErrNotLinked
	}

	err = s.bot.SendMessage(ctx, bot.token, *link.ChatID,
		"✅ Mensaje de prueba de Vegusa para el usuario "+user.Username+".")
	if err != nil {
		var apiErr *telegram.APIError
		if errors.As(err, &apiErr) && apiErr.Code == 403 {
			if markErr := s.links.SetBlockedByChatID(ctx, *link.ChatID, true); markErr != nil {
				return markErr
			}
			return ErrBotBlocked
		}
		return fmt.Errorf("%w: %v", ErrTelegramUnreachable, err)
	}

	// Un envío correcto confirma que ya no está bloqueado.
	if link.Status == mysqlInfra.TelegramLinkBlocked {
		return s.links.SetBlockedByChatID(ctx, *link.ChatID, false)
	}
	return nil
}

// shortTime convierte "HH:MM:SS" en "HH:MM".
func shortTime(value string) string {
	if len(value) >= 5 {
		return value[:5]
	}
	return value
}

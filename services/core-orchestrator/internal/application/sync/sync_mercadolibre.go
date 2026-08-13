package sync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

const (
	defaultTokenRefreshSkew = 5 * time.Minute
	// mercadoLibreTokenTTL is only a fallback for the rare case where the
	// refresh response omits expires_in; the real TTL is read from the
	// response on every refresh.
	mercadoLibreTokenTTL = 6 * time.Hour
	defaultSystemActorID = int64(1)
)

// MercadoLibreRateLimit and MercadoLibreRateLimitWindow bound how many
// MercadoLibre API calls (of any kind — auth, categories, items, etc.) the
// whole application may make in a given window. A single shared
// *mercadoLibreInfra.RateLimiter built from these values in main.go must be
// injected into every service that talks to MercadoLibre, so the budget is
// enforced globally rather than per connection.
//
// 90/min leaves headroom under the 100 RPM per APP_ID documented for the
// compatibilities endpoints (the tightest documented per-endpoint limit this
// app hits) — confirmed well within this app's actual quota via
// GET /applications/{app_id} (max_requests_per_hour: 18000, i.e. 300/min).
const (
	MercadoLibreRateLimit       = 90
	MercadoLibreRateLimitWindow = time.Minute
)

var (
	ErrInvalidMercadoLibreConnection = errors.New("invalid mercadolibre connection")
	ErrMissingMercadoLibreSettings   = errors.New("missing required mercadolibre settings")
	ErrInvalidExpirationTimeFormat   = errors.New("invalid expiration_time format")
	ErrInvalidMercadoLibreTitle      = errors.New("invalid mercadolibre title")
	ErrMissingMercadoLibreSiteID     = errors.New("missing required mercadolibre site_id")
)

type MercadoLibreTokenService struct {
	settingsRepository *mysqlInfra.ConnectionSettingsRepository
	statusRepository   *mysqlInfra.ConnectionStatusRepository
	authHandler        *mercadoLibreInfra.AuthHandler
	refreshSkew        time.Duration
}

type MercadoLibreCategoryPredictorService struct {
	tokenService                 *MercadoLibreTokenService
	settingsRepository           *mysqlInfra.ConnectionSettingsRepository
	categoriesHandler            *mercadoLibreInfra.CategoriesHandler
	categoriesRepository         *mysqlInfra.CategoriesRepository
	channelCategoryMapRepository *mysqlInfra.ChannelCategoryMapRepository
}

type MercadoLibreCategoryAttribute = mercadoLibreInfra.CategoryAttribute
type MercadoLibreCategoryPrediction = mercadoLibreInfra.CategoryPrediction

type MercadoLibreCategoryPredictorResult struct {
	ConnectionID int64                            `json:"connectionId"`
	SiteID       string                           `json:"siteId"`
	Query        string                           `json:"query"`
	Predictions  []MercadoLibreCategoryPrediction `json:"predictions"`
}

func NewMercadoLibreTokenService(
	settingsRepository *mysqlInfra.ConnectionSettingsRepository,
	statusRepository *mysqlInfra.ConnectionStatusRepository,
	rateLimiter *mercadoLibreInfra.RateLimiter,
) *MercadoLibreTokenService {
	client := mercadoLibreInfra.NewClient(nil, "", rateLimiter)

	return &MercadoLibreTokenService{
		settingsRepository: settingsRepository,
		statusRepository:   statusRepository,
		authHandler:        mercadoLibreInfra.NewAuthHandler(client),
		refreshSkew:        defaultTokenRefreshSkew,
	}
}

func NewMercadoLibreCategoryPredictorService(
	tokenService *MercadoLibreTokenService,
	settingsRepository *mysqlInfra.ConnectionSettingsRepository,
	categoriesRepository *mysqlInfra.CategoriesRepository,
	channelCategoryMapRepository *mysqlInfra.ChannelCategoryMapRepository,
	rateLimiter *mercadoLibreInfra.RateLimiter,
) *MercadoLibreCategoryPredictorService {
	client := mercadoLibreInfra.NewClient(nil, "", rateLimiter)

	return &MercadoLibreCategoryPredictorService{
		tokenService:                 tokenService,
		settingsRepository:           settingsRepository,
		categoriesHandler:            mercadoLibreInfra.NewCategoriesHandler(client),
		categoriesRepository:         categoriesRepository,
		channelCategoryMapRepository: channelCategoryMapRepository,
	}
}

// EnsureValidAccessToken returns a valid access token for the connection.
// If the current token is expired (or about to expire), it refreshes it and stores
// access_token, refresh_token and expiration_time in encrypted settings.
func (s *MercadoLibreTokenService) EnsureValidAccessToken(ctx context.Context, connectionID int64) (accessToken string, err error) {
	if connectionID <= 0 {
		return "", ErrInvalidMercadoLibreConnection
	}

	var refreshedExpiration time.Time
	refreshed := false

	defer func() {
		s.recordConnectionStatus(connectionID, err, refreshed, refreshedExpiration)
	}()

	log.Printf("mercadolibre token: connection %d — loading settings", connectionID)
	settings, err := s.settingsRepository.FindByConnectionID(connectionID)
	if err != nil {
		return "", fmt.Errorf("error loading connection settings: %w", err)
	}

	settingsByKey := mercadoLibreSettingsByKey(settings)

	accessTokenSetting, ok := settingsByKey["access_token"]
	if !ok {
		return "", fmt.Errorf("%w: access_token", ErrMissingMercadoLibreSettings)
	}

	refreshTokenSetting, ok := settingsByKey["refresh_token"]
	if !ok {
		return "", fmt.Errorf("%w: refresh_token", ErrMissingMercadoLibreSettings)
	}

	expirationTimeSetting, ok := settingsByKey["expiration_time"]
	if !ok {
		return "", fmt.Errorf("%w: expiration_time", ErrMissingMercadoLibreSettings)
	}

	appIDSetting, ok := settingsByKey["app_id"]
	if !ok {
		return "", fmt.Errorf("%w: app_id", ErrMissingMercadoLibreSettings)
	}

	clientSecretSetting, ok := settingsByKey["client_secret"]
	if !ok {
		return "", fmt.Errorf("%w: client_secret", ErrMissingMercadoLibreSettings)
	}

	if strings.TrimSpace(accessTokenSetting.Value) == "" {
		return "", fmt.Errorf("%w: access_token empty", ErrMissingMercadoLibreSettings)
	}

	if strings.TrimSpace(refreshTokenSetting.Value) == "" {
		return "", fmt.Errorf("%w: refresh_token empty", ErrMissingMercadoLibreSettings)
	}

	expirationTime, err := parseMercadoLibreExpirationTime(expirationTimeSetting.Value)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	refreshDeadline := expirationTime.Add(-s.refreshSkew)
	if now.Before(refreshDeadline) {
		log.Printf("mercadolibre token: connection %d — current access_token still valid, reusing it", connectionID)
		return accessTokenSetting.Value, nil
	}

	log.Printf("mercadolibre token: connection %d — access_token expired/expiring, refreshing", connectionID)
	refreshResponse, err := s.refreshAccessToken(ctx, strings.TrimSpace(appIDSetting.Value), strings.TrimSpace(clientSecretSetting.Value), strings.TrimSpace(refreshTokenSetting.Value))
	if err != nil {
		return "", err
	}
	log.Printf("mercadolibre token: connection %d — refresh call returned, persisting new tokens", connectionID)

	if strings.TrimSpace(refreshResponse.AccessToken) == "" || strings.TrimSpace(refreshResponse.RefreshToken) == "" {
		return "", errors.New("mercadolibre refresh response missing token values")
	}

	tokenTTL := mercadoLibreTokenTTL
	if refreshResponse.ExpiresIn > 0 {
		tokenTTL = time.Duration(refreshResponse.ExpiresIn) * time.Second
	}
	newExpirationTime := now.Add(tokenTTL)
	updatedBy := resolveSettingsActorID(settings)

	refreshed = true
	refreshedExpiration = newExpirationTime

	_, err = s.settingsRepository.Update(accessTokenSetting.ID, mysqlInfra.UpdateConnectionSettingInput{
		Value:       refreshResponse.AccessToken,
		IsEncrypted: true,
		UpdatedBy:   updatedBy,
	})
	if err != nil {
		return "", fmt.Errorf("error updating access_token: %w", err)
	}

	_, err = s.settingsRepository.Update(refreshTokenSetting.ID, mysqlInfra.UpdateConnectionSettingInput{
		Value:       refreshResponse.RefreshToken,
		IsEncrypted: true,
		UpdatedBy:   updatedBy,
	})
	if err != nil {
		return "", fmt.Errorf("error updating refresh_token: %w", err)
	}

	_, err = s.settingsRepository.Update(expirationTimeSetting.ID, mysqlInfra.UpdateConnectionSettingInput{
		Value:       newExpirationTime.UTC().Format(time.RFC3339),
		IsEncrypted: true,
		UpdatedBy:   updatedBy,
	})
	if err != nil {
		return "", fmt.Errorf("error updating expiration_time: %w", err)
	}

	return refreshResponse.AccessToken, nil
}

// recordConnectionStatus persists the outcome of an EnsureValidAccessToken call
// into ecom_connection_status: whether the connection is currently authenticated,
// its last authentication error (if any), and, when a token refresh actually
// happened, when it happened and when the new token expires.
func (s *MercadoLibreTokenService) recordConnectionStatus(connectionID int64, opErr error, refreshed bool, expiresAt time.Time) {
	if s.statusRepository == nil {
		return
	}

	input := mysqlInfra.UpsertConnectionStatusInput{
		ConnectionID:  connectionID,
		Authenticated: opErr == nil,
	}

	if opErr != nil {
		errMsg := opErr.Error()
		input.LastError = &errMsg
	}

	if refreshed {
		now := time.Now().UTC()
		input.LastAuth = &now
		input.ExpiresAt = &expiresAt
	}

	if _, err := s.statusRepository.Upsert(input); err != nil {
		log.Printf("Error recording connection status for connection %d: %v", connectionID, err)
	}
}

func (s *MercadoLibreTokenService) refreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (*mercadoLibreInfra.RefreshTokenResponse, error) {
	if clientID == "" || clientSecret == "" || refreshToken == "" {
		return nil, fmt.Errorf("%w: client_id/client_secret/refresh_token", ErrMissingMercadoLibreSettings)
	}

	return s.authHandler.RefreshToken(ctx, mercadoLibreInfra.RefreshTokenRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
	})
}

// GetAuthorizationURL builds the URL an admin should be sent to in order to
// grant this app access to a MercadoLibre account, using the app_id and
// redirect_url already stored in ecom_connection_settings for connectionID
// (the redirect_url must match exactly what's registered in the MercadoLibre
// developer app).
func (s *MercadoLibreTokenService) GetAuthorizationURL(connectionID int64) (string, error) {
	if connectionID <= 0 {
		return "", ErrInvalidMercadoLibreConnection
	}

	settings, err := s.settingsRepository.FindByConnectionID(connectionID)
	if err != nil {
		return "", fmt.Errorf("error loading connection settings: %w", err)
	}
	settingsByKey := mercadoLibreSettingsByKey(settings)

	appIDSetting, ok := settingsByKey["app_id"]
	if !ok || strings.TrimSpace(appIDSetting.Value) == "" {
		return "", fmt.Errorf("%w: app_id", ErrMissingMercadoLibreSettings)
	}

	redirectURISetting, ok := settingsByKey["redirect_url"]
	if !ok || strings.TrimSpace(redirectURISetting.Value) == "" {
		return "", fmt.Errorf("%w: redirect_url", ErrMissingMercadoLibreSettings)
	}

	return s.authHandler.AuthorizationURL(strings.TrimSpace(appIDSetting.Value), strings.TrimSpace(redirectURISetting.Value)), nil
}

// ExchangeAuthorizationCode trades the one-time code MercadoLibre's OAuth
// redirect handed back (after GetAuthorizationURL) for connectionID's first
// access_token/refresh_token pair, and persists them the same way
// EnsureValidAccessToken does after a routine refresh. Unlike a routine
// refresh, the access_token/refresh_token/expiration_time settings may not
// exist yet — this can be the very first authorization for the connection —
// so each one is created if missing rather than assumed to already exist.
func (s *MercadoLibreTokenService) ExchangeAuthorizationCode(ctx context.Context, connectionID int64, code string) (err error) {
	if connectionID <= 0 {
		return ErrInvalidMercadoLibreConnection
	}
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("%w: code", ErrMissingMercadoLibreSettings)
	}

	var refreshedExpiration time.Time
	refreshed := false

	defer func() {
		s.recordConnectionStatus(connectionID, err, refreshed, refreshedExpiration)
	}()

	settings, loadErr := s.settingsRepository.FindByConnectionID(connectionID)
	if loadErr != nil {
		err = fmt.Errorf("error loading connection settings: %w", loadErr)
		return err
	}
	settingsByKey := mercadoLibreSettingsByKey(settings)

	appIDSetting, ok := settingsByKey["app_id"]
	if !ok || strings.TrimSpace(appIDSetting.Value) == "" {
		err = fmt.Errorf("%w: app_id", ErrMissingMercadoLibreSettings)
		return err
	}
	clientSecretSetting, ok := settingsByKey["client_secret"]
	if !ok || strings.TrimSpace(clientSecretSetting.Value) == "" {
		err = fmt.Errorf("%w: client_secret", ErrMissingMercadoLibreSettings)
		return err
	}
	redirectURISetting, ok := settingsByKey["redirect_url"]
	if !ok || strings.TrimSpace(redirectURISetting.Value) == "" {
		err = fmt.Errorf("%w: redirect_url", ErrMissingMercadoLibreSettings)
		return err
	}

	log.Printf("mercadolibre oauth: connection %d — exchanging authorization code", connectionID)
	tokenResponse, exchangeErr := s.authHandler.ExchangeAuthorizationCode(ctx, mercadoLibreInfra.ExchangeCodeRequest{
		ClientID:     strings.TrimSpace(appIDSetting.Value),
		ClientSecret: strings.TrimSpace(clientSecretSetting.Value),
		Code:         strings.TrimSpace(code),
		RedirectURI:  strings.TrimSpace(redirectURISetting.Value),
	})
	if exchangeErr != nil {
		err = fmt.Errorf("error exchanging mercadolibre authorization code: %w", exchangeErr)
		return err
	}
	log.Printf("mercadolibre oauth: connection %d — code exchange returned, persisting tokens", connectionID)

	if strings.TrimSpace(tokenResponse.AccessToken) == "" || strings.TrimSpace(tokenResponse.RefreshToken) == "" {
		err = errors.New("mercadolibre authorization response missing token values")
		return err
	}

	tokenTTL := mercadoLibreTokenTTL
	if tokenResponse.ExpiresIn > 0 {
		tokenTTL = time.Duration(tokenResponse.ExpiresIn) * time.Second
	}
	newExpirationTime := time.Now().UTC().Add(tokenTTL)
	actorID := resolveSettingsActorID(settings)

	refreshed = true
	refreshedExpiration = newExpirationTime

	if upsertErr := s.upsertSetting(settingsByKey, connectionID, "access_token", tokenResponse.AccessToken, actorID); upsertErr != nil {
		err = fmt.Errorf("error saving access_token: %w", upsertErr)
		return err
	}
	if upsertErr := s.upsertSetting(settingsByKey, connectionID, "refresh_token", tokenResponse.RefreshToken, actorID); upsertErr != nil {
		err = fmt.Errorf("error saving refresh_token: %w", upsertErr)
		return err
	}
	if upsertErr := s.upsertSetting(settingsByKey, connectionID, "expiration_time", newExpirationTime.UTC().Format(time.RFC3339), actorID); upsertErr != nil {
		err = fmt.Errorf("error saving expiration_time: %w", upsertErr)
		return err
	}

	log.Printf("mercadolibre oauth: connection %d — first access_token stored, expires %s", connectionID, newExpirationTime.UTC().Format(time.RFC3339))

	return nil
}

// upsertSetting creates keyName for connectionID if it isn't already present
// in settingsByKey (loaded fresh from FindByConnectionID), or updates it in
// place otherwise. Values are always stored encrypted, matching how
// EnsureValidAccessToken stores access_token/refresh_token/expiration_time.
func (s *MercadoLibreTokenService) upsertSetting(settingsByKey map[string]mysqlInfra.ConnectionSettingDTO, connectionID int64, keyName, value string, actorID int64) error {
	if existing, ok := settingsByKey[keyName]; ok {
		_, err := s.settingsRepository.Update(existing.ID, mysqlInfra.UpdateConnectionSettingInput{
			Value:       value,
			IsEncrypted: true,
			UpdatedBy:   actorID,
		})
		return err
	}

	_, err := s.settingsRepository.Create(mysqlInfra.CreateConnectionSettingInput{
		ConnectionID: connectionID,
		KeyName:      keyName,
		Value:        value,
		IsEncrypted:  true,
		CreatedBy:    actorID,
	})
	return err
}

// mercadoLibreSettingsByKey indexes a connection's settings by
// lower-cased, trimmed key_name, so lookups like settingsByKey["app_id"]
// don't depend on how the key was originally cased.
func mercadoLibreSettingsByKey(settings []mysqlInfra.ConnectionSettingDTO) map[string]mysqlInfra.ConnectionSettingDTO {
	settingsByKey := make(map[string]mysqlInfra.ConnectionSettingDTO, len(settings))
	for _, setting := range settings {
		normalizedKey := strings.ToLower(strings.TrimSpace(setting.KeyName))
		if normalizedKey == "" {
			continue
		}
		settingsByKey[normalizedKey] = setting
	}

	return settingsByKey
}

func parseMercadoLibreExpirationTime(rawValue string) (time.Time, error) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return time.Time{}, fmt.Errorf("%w: empty value", ErrInvalidExpirationTimeFormat)
	}

	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}

	if unixSeconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(unixSeconds, 0).UTC(), nil
	}

	return time.Time{}, fmt.Errorf("%w: %s", ErrInvalidExpirationTimeFormat, value)
}

func resolveSettingsActorID(settings []mysqlInfra.ConnectionSettingDTO) int64 {
	for _, setting := range settings {
		if setting.UpdatedBy != nil && *setting.UpdatedBy > 0 {
			return *setting.UpdatedBy
		}
	}

	for _, setting := range settings {
		if setting.CreatedBy > 0 {
			return setting.CreatedBy
		}
	}

	return defaultSystemActorID
}

func (s *MercadoLibreCategoryPredictorService) PredictCategories(
	ctx context.Context,
	connectionID int64,
	title string,
	siteID string,
	limit int,
) (*MercadoLibreCategoryPredictorResult, error) {
	if connectionID <= 0 {
		return nil, ErrInvalidMercadoLibreConnection
	}

	queryTitle := strings.TrimSpace(title)
	if queryTitle == "" {
		return nil, ErrInvalidMercadoLibreTitle
	}

	log.Printf("mercadolibre predictor: connection %d — resolving site id", connectionID)
	resolvedSiteID, err := s.resolveSiteID(connectionID, siteID)
	if err != nil {
		return nil, err
	}
	log.Printf("mercadolibre predictor: connection %d — site id=%s, ensuring access token", connectionID, resolvedSiteID)

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 1
	}
	if limit > 8 {
		limit = 8
	}

	log.Printf("mercadolibre predictor: connection %d — calling domain_discovery (query=%q, limit=%d)", connectionID, queryTitle, limit)
	predictions, err := s.categoriesHandler.PredictCategory(ctx, mercadoLibreInfra.CategoryPredictorRequest{
		AccessToken: accessToken,
		SiteID:      resolvedSiteID,
		Query:       queryTitle,
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("mercadolibre predictor: connection %d — got %d prediction(s)", connectionID, len(predictions))

	return &MercadoLibreCategoryPredictorResult{
		ConnectionID: connectionID,
		SiteID:       resolvedSiteID,
		Query:        queryTitle,
		Predictions:  predictions,
	}, nil
}

// GetCategoryAttributes is a thin passthrough to the underlying (public, no
// access token needed) GET /categories/{id}/attributes call — exposed here
// so a caller can inspect exactly how MercadoLibre tags a category's
// attributes (Tags.Hidden/Tags.ReadOnly/Tags.Required, ValueType, Values)
// without needing product/connection context, e.g. to debug why
// channel_attribute_values.Service.ProvisionCategoryAttributes silently
// skipped a given attribute for that category.
func (s *MercadoLibreCategoryPredictorService) GetCategoryAttributes(ctx context.Context, categoryID string) ([]mercadoLibreInfra.CategoryRequiredAttribute, error) {
	return s.categoriesHandler.GetCategoryAttributes(ctx, categoryID)
}

// EnsureLocalCategory guarantees a local ecom_categories hierarchy exists
// for externalCategoryID (a MercadoLibre category id, as predicted by
// PredictCategories or returned by an item creation call) on connectionID,
// and returns the leaf category's local ecom_categories.id. The first time
// this external category is seen for connectionID, its full path_from_root
// is fetched from MercadoLibre and replicated root to leaf (matching
// existing categories by name+parent, creating whatever is missing), and the
// leaf gets its own ecom_channel_category_map row. Every later call for the
// same (externalCategoryID, connectionID) pair reuses that row and never
// calls MercadoLibre or writes to ecom_categories again.
func (s *MercadoLibreCategoryPredictorService) EnsureLocalCategory(ctx context.Context, connectionID int64, externalCategoryID string, actorID int64) (int64, error) {
	existing, err := s.channelCategoryMapRepository.FindByExternalCategoryAndConnection(externalCategoryID, connectionID)
	if err != nil && !errors.Is(err, mysqlInfra.ErrChannelCategoryMapNotFound) {
		return 0, fmt.Errorf("error loading channel category map for external category %q: %w", externalCategoryID, err)
	}
	if existing != nil {
		return existing.CategoryID, nil
	}

	log.Printf("mercadolibre category resolver: external category %s not yet mapped for connection %d, fetching path from mercadolibre", externalCategoryID, connectionID)
	details, err := s.categoriesHandler.GetCategory(ctx, externalCategoryID)
	if err != nil {
		return 0, fmt.Errorf("error fetching mercadolibre category %q: %w", externalCategoryID, err)
	}

	path := details.PathFromRoot
	if len(path) == 0 {
		path = []mercadoLibreInfra.CategoryPathNode{{ID: details.ID, Name: details.Name}}
	}

	var parentLocalID *int64
	var leafLocalID int64
	for _, node := range path {
		name := strings.TrimSpace(node.Name)
		if name == "" {
			return 0, fmt.Errorf("mercadolibre category %q has an empty name in its path", node.ID)
		}

		existingCategory, err := s.categoriesRepository.FindByNameAndParentID(name, parentLocalID)
		if err != nil && !errors.Is(err, mysqlInfra.ErrCategoryNotFound) {
			return 0, fmt.Errorf("error looking up category %q: %w", name, err)
		}

		var categoryID int64
		if existingCategory != nil {
			categoryID = existingCategory.ID
		} else {
			created, err := s.categoriesRepository.Create(mysqlInfra.CreateCategoryInput{
				Name:      name,
				ParentID:  parentLocalID,
				CreatedBy: actorID,
			})
			if err != nil {
				return 0, fmt.Errorf("error creating category %q: %w", name, err)
			}
			categoryID = created.ID
			log.Printf("mercadolibre category resolver: created local category %q (id=%d)", name, categoryID)
		}

		parentLocalID = &categoryID
		leafLocalID = categoryID
	}

	leafName := strings.TrimSpace(path[len(path)-1].Name)
	if _, err := s.channelCategoryMapRepository.Upsert(mysqlInfra.UpsertChannelCategoryMapInput{
		CategoryID:           leafLocalID,
		ConnectionID:         connectionID,
		ExternalCategoryID:   externalCategoryID,
		ExternalCategoryName: &leafName,
		ActorID:              actorID,
	}); err != nil {
		return 0, fmt.Errorf("error recording channel category map for external category %q: %w", externalCategoryID, err)
	}

	return leafLocalID, nil
}

func (s *MercadoLibreCategoryPredictorService) resolveSiteID(connectionID int64, siteID string) (string, error) {
	trimmedSiteID := strings.TrimSpace(strings.ToUpper(siteID))
	if trimmedSiteID != "" {
		return trimmedSiteID, nil
	}

	settings, err := s.settingsRepository.FindByConnectionID(connectionID)
	if err != nil {
		return "", fmt.Errorf("error loading connection settings: %w", err)
	}

	for _, setting := range settings {
		if strings.EqualFold(strings.TrimSpace(setting.KeyName), "site_id") {
			resolved := strings.TrimSpace(strings.ToUpper(setting.Value))
			if resolved != "" {
				return resolved, nil
			}
		}
	}

	return "", ErrMissingMercadoLibreSiteID
}

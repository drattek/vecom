package sync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	odooInfra "core-orchestrator/internal/infrastructure/marketplace/odoo"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

const defaultOdooTestConnectionLimit = 20

// OdooRateLimit and OdooRateLimitWindow bound how many Odoo API calls (of
// any kind — products, categories, etc.) the whole application may make in
// a given window. A single shared *odooInfra.RateLimiter built from these
// values in main.go must be injected into every service that talks to
// Odoo, so the budget is enforced globally rather than per connection.
const (
	OdooRateLimit       = 1
	OdooRateLimitWindow = time.Second
)

var (
	ErrInvalidOdooConnection  = errors.New("invalid odoo connection")
	ErrMissingOdooSettings    = errors.New("missing required odoo connection settings")
	ErrMissingOdooCredentials = errors.New("missing required odoo connection credentials")
)

type OdooConnectionService struct {
	credentialsRepository *mysqlInfra.ConnectionCredentialsRepository
	settingsRepository    *mysqlInfra.ConnectionSettingsRepository
	statusRepository      *mysqlInfra.ConnectionStatusRepository
	rateLimiter           *odooInfra.RateLimiter
}

type OdooTestConnectionResult struct {
	ConnectionID int64               `json:"connectionId"`
	Products     []odooInfra.Product `json:"products"`
}

func NewOdooConnectionService(
	credentialsRepository *mysqlInfra.ConnectionCredentialsRepository,
	settingsRepository *mysqlInfra.ConnectionSettingsRepository,
	statusRepository *mysqlInfra.ConnectionStatusRepository,
	rateLimiter *odooInfra.RateLimiter,
) *OdooConnectionService {
	return &OdooConnectionService{
		credentialsRepository: credentialsRepository,
		settingsRepository:    settingsRepository,
		statusRepository:      statusRepository,
		rateLimiter:           rateLimiter,
	}
}

// TestConnection reads product.template records from Odoo using the
// connection's odoo_url/ApiKey/X-Odoo-Database values, to prove the
// connection is configured correctly end to end. Odoo's External JSON-2 API
// is key-based with no session/token to refresh, so unlike MercadoLibre there
// is no EnsureValidAccessToken step here. When productID is > 0, the search
// is restricted to that single product.
func (s *OdooConnectionService) TestConnection(ctx context.Context, connectionID int64, productID int64) (result *OdooTestConnectionResult, err error) {
	if connectionID <= 0 {
		return nil, ErrInvalidOdooConnection
	}

	defer func() {
		s.recordConnectionStatus(connectionID, err)
	}()

	values, err := s.loadConnectionValues(connectionID)
	if err != nil {
		return nil, err
	}

	odooURL, ok := values["odoo_url"]
	if !ok || odooURL == "" {
		return nil, fmt.Errorf("%w: odoo_url", ErrMissingOdooSettings)
	}

	apiKey, ok := values["apikey"]
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("%w: ApiKey", ErrMissingOdooCredentials)
	}

	database := values["x-odoo-database"]

	var ids []int64
	if productID > 0 {
		ids = []int64{productID}
	}

	client := odooInfra.NewClient(nil, odooURL, s.rateLimiter)
	productsHandler := odooInfra.NewProductsHandler(client)

	products, err := productsHandler.SearchReadProducts(ctx, odooInfra.SearchReadProductsRequest{
		Credentials: odooInfra.Credentials{APIKey: apiKey, Database: database},
		IDs:         ids,
		Limit:       defaultOdooTestConnectionLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("error testing odoo connection: %w", err)
	}

	return &OdooTestConnectionResult{ConnectionID: connectionID, Products: products}, nil
}

func (s *OdooConnectionService) loadConnectionValues(connectionID int64) (map[string]string, error) {
	return LoadOdooConnectionValues(s.credentialsRepository, s.settingsRepository, connectionID)
}

// LoadOdooConnectionValues merges ecom_connection_settings and
// ecom_connection_credentials for a connection into a single map keyed by
// lowercased, trimmed key_name. Odoo's three required values (odoo_url,
// ApiKey, X-Odoo-Database) may live in either table depending on how the
// connection was configured, so both sources are consulted rather than
// assuming one location for a given key. Exported so other Odoo callers
// (e.g. the category migration service) can resolve the same connection
// values without duplicating this lookup.
func LoadOdooConnectionValues(
	credentialsRepository *mysqlInfra.ConnectionCredentialsRepository,
	settingsRepository *mysqlInfra.ConnectionSettingsRepository,
	connectionID int64,
) (map[string]string, error) {
	settings, err := settingsRepository.FindByConnectionID(connectionID)
	if err != nil {
		return nil, fmt.Errorf("error loading connection settings: %w", err)
	}

	credentials, err := credentialsRepository.FindByConnectionID(connectionID)
	if err != nil {
		return nil, fmt.Errorf("error loading connection credentials: %w", err)
	}

	values := make(map[string]string, len(settings)+len(credentials))
	for _, setting := range settings {
		key := strings.ToLower(strings.TrimSpace(setting.KeyName))
		if key == "" {
			continue
		}
		values[key] = strings.TrimSpace(setting.Value)
	}
	for _, credential := range credentials {
		key := strings.ToLower(strings.TrimSpace(credential.KeyName))
		if key == "" {
			continue
		}
		values[key] = strings.TrimSpace(credential.Value)
	}

	return values, nil
}

// recordConnectionStatus persists the outcome of a TestConnection call into
// ecom_connection_status. Odoo API keys don't expire, so ExpiresAt is never
// set here (see ConnectionStatusRepository.FindDueForRefresh).
func (s *OdooConnectionService) recordConnectionStatus(connectionID int64, opErr error) {
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
	} else {
		now := time.Now().UTC()
		input.LastAuth = &now
	}

	if _, err := s.statusRepository.Upsert(input); err != nil {
		log.Printf("Error recording connection status for connection %d: %v", connectionID, err)
	}
}

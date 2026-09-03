package channel_config

import (
	"context"
	"database/sql"
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidChannelConfig = errors.New("invalid channel config")
)

type ConnectionCredentialsService struct {
	db         *sql.DB
	repository *mysqlInfra.ConnectionCredentialsRepository
}

func NewConnectionCredentialsService(db *sql.DB, repository *mysqlInfra.ConnectionCredentialsRepository) *ConnectionCredentialsService {
	return &ConnectionCredentialsService{db: db, repository: repository}
}

func (s *ConnectionCredentialsService) GetPaginatedCredentials(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedConnectionCredentials, error) {
	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *ConnectionCredentialsService) GetCredentialByID(ctx context.Context, id int64) (*mysqlInfra.ConnectionCredentialDTO, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *ConnectionCredentialsService) GetCredentialsByConnection(ctx context.Context, connectionID int64) ([]mysqlInfra.ConnectionCredentialDTO, error) {
	return s.repository.FindByConnectionID(ctx, connectionID)
}

func (s *ConnectionCredentialsService) CreateCredential(ctx context.Context, input mysqlInfra.CreateConnectionCredentialInput) (*mysqlInfra.ConnectionCredentialDTO, error) {
	if input.ConnectionID == 0 || input.KeyName == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Create(ctx, input)
}

func (s *ConnectionCredentialsService) UpdateCredential(ctx context.Context, id int64, input mysqlInfra.UpdateConnectionCredentialInput) (*mysqlInfra.ConnectionCredentialDTO, error) {
	if input.Value == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Update(ctx, id, input)
}

func (s *ConnectionCredentialsService) DeleteCredential(ctx context.Context, id int64) error {
	return s.repository.SoftDelete(ctx, id)
}

type ConnectionSettingsService struct {
	db         *sql.DB
	repository *mysqlInfra.ConnectionSettingsRepository
}

func NewConnectionSettingsService(db *sql.DB, repository *mysqlInfra.ConnectionSettingsRepository) *ConnectionSettingsService {
	return &ConnectionSettingsService{db: db, repository: repository}
}

func (s *ConnectionSettingsService) GetPaginatedSettings(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedConnectionSettings, error) {
	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *ConnectionSettingsService) GetSettingByID(ctx context.Context, id int64) (*mysqlInfra.ConnectionSettingDTO, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *ConnectionSettingsService) GetSettingsByConnection(ctx context.Context, connectionID int64) ([]mysqlInfra.ConnectionSettingDTO, error) {
	return s.repository.FindByConnectionID(ctx, connectionID)
}

func (s *ConnectionSettingsService) CreateSetting(ctx context.Context, input mysqlInfra.CreateConnectionSettingInput) (*mysqlInfra.ConnectionSettingDTO, error) {
	if input.ConnectionID == 0 || input.KeyName == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Create(ctx, input)
}

func (s *ConnectionSettingsService) UpdateSetting(ctx context.Context, id int64, input mysqlInfra.UpdateConnectionSettingInput) (*mysqlInfra.ConnectionSettingDTO, error) {
	if input.Value == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Update(ctx, id, input)
}

func (s *ConnectionSettingsService) DeleteSetting(ctx context.Context, id int64) error {
	return s.repository.SoftDelete(ctx, id)
}

type ConnectionStatusService struct {
	db         *sql.DB
	repository *mysqlInfra.ConnectionStatusRepository
}

func NewConnectionStatusService(db *sql.DB, repository *mysqlInfra.ConnectionStatusRepository) *ConnectionStatusService {
	return &ConnectionStatusService{db: db, repository: repository}
}

func (s *ConnectionStatusService) GetStatusByConnection(ctx context.Context, connectionID int64) (*mysqlInfra.ConnectionStatusDTO, error) {
	return s.repository.FindByConnectionID(ctx, connectionID)
}

type ChannelParametersService struct {
	db         *sql.DB
	repository *mysqlInfra.ChannelParametersRepository
}

func NewChannelParametersService(db *sql.DB, repository *mysqlInfra.ChannelParametersRepository) *ChannelParametersService {
	return &ChannelParametersService{db: db, repository: repository}
}

func (s *ChannelParametersService) GetPaginatedParameters(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedChannelParameters, error) {
	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *ChannelParametersService) GetParameterByID(ctx context.Context, id int64) (*mysqlInfra.ChannelParameterDTO, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *ChannelParametersService) GetParametersByChannel(ctx context.Context, channelID int64, offset, pageSize int) (*mysqlInfra.PaginatedChannelParameters, error) {
	return s.repository.FindByChannelID(ctx, channelID, offset, pageSize)
}

func (s *ChannelParametersService) CreateParameter(ctx context.Context, input mysqlInfra.CreateChannelParameterInput) (*mysqlInfra.ChannelParameterDTO, error) {
	if input.ChannelID == 0 || input.ParameterName == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Create(ctx, input)
}

func (s *ChannelParametersService) UpdateParameter(ctx context.Context, id int64, input mysqlInfra.UpdateChannelParameterInput) (*mysqlInfra.ChannelParameterDTO, error) {
	if input.DisplayName == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Update(ctx, id, input)
}

func (s *ChannelParametersService) DeleteParameter(ctx context.Context, id int64) error {
	return s.repository.SoftDelete(ctx, id)
}

package channel_config

import (
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidChannelConfig = errors.New("invalid channel config")
)

type ConnectionCredentialsService struct {
	repository *mysqlInfra.ConnectionCredentialsRepository
}

func NewConnectionCredentialsService(repository *mysqlInfra.ConnectionCredentialsRepository) *ConnectionCredentialsService {
	return &ConnectionCredentialsService{repository: repository}
}

func (s *ConnectionCredentialsService) GetPaginatedCredentials(offset, pageSize int) (*mysqlInfra.PaginatedConnectionCredentials, error) {
	return s.repository.FindPaginated(offset, pageSize)
}

func (s *ConnectionCredentialsService) GetCredentialByID(id int64) (*mysqlInfra.ConnectionCredentialDTO, error) {
	return s.repository.FindByID(id)
}

func (s *ConnectionCredentialsService) GetCredentialsByConnection(connectionID int64) ([]mysqlInfra.ConnectionCredentialDTO, error) {
	return s.repository.FindByConnectionID(connectionID)
}

func (s *ConnectionCredentialsService) CreateCredential(input mysqlInfra.CreateConnectionCredentialInput) (*mysqlInfra.ConnectionCredentialDTO, error) {
	if input.ConnectionID == 0 || input.KeyName == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Create(input)
}

func (s *ConnectionCredentialsService) UpdateCredential(id int64, input mysqlInfra.UpdateConnectionCredentialInput) (*mysqlInfra.ConnectionCredentialDTO, error) {
	if input.Value == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Update(id, input)
}

func (s *ConnectionCredentialsService) DeleteCredential(id int64) error {
	return s.repository.SoftDelete(id)
}

type ConnectionSettingsService struct {
	repository *mysqlInfra.ConnectionSettingsRepository
}

func NewConnectionSettingsService(repository *mysqlInfra.ConnectionSettingsRepository) *ConnectionSettingsService {
	return &ConnectionSettingsService{repository: repository}
}

func (s *ConnectionSettingsService) GetPaginatedSettings(offset, pageSize int) (*mysqlInfra.PaginatedConnectionSettings, error) {
	return s.repository.FindPaginated(offset, pageSize)
}

func (s *ConnectionSettingsService) GetSettingByID(id int64) (*mysqlInfra.ConnectionSettingDTO, error) {
	return s.repository.FindByID(id)
}

func (s *ConnectionSettingsService) GetSettingsByConnection(connectionID int64) ([]mysqlInfra.ConnectionSettingDTO, error) {
	return s.repository.FindByConnectionID(connectionID)
}

func (s *ConnectionSettingsService) CreateSetting(input mysqlInfra.CreateConnectionSettingInput) (*mysqlInfra.ConnectionSettingDTO, error) {
	if input.ConnectionID == 0 || input.KeyName == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Create(input)
}

func (s *ConnectionSettingsService) UpdateSetting(id int64, input mysqlInfra.UpdateConnectionSettingInput) (*mysqlInfra.ConnectionSettingDTO, error) {
	if input.Value == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Update(id, input)
}

func (s *ConnectionSettingsService) DeleteSetting(id int64) error {
	return s.repository.SoftDelete(id)
}

type ConnectionStatusService struct {
	repository *mysqlInfra.ConnectionStatusRepository
}

func NewConnectionStatusService(repository *mysqlInfra.ConnectionStatusRepository) *ConnectionStatusService {
	return &ConnectionStatusService{repository: repository}
}

func (s *ConnectionStatusService) GetStatusByConnection(connectionID int64) (*mysqlInfra.ConnectionStatusDTO, error) {
	return s.repository.FindByConnectionID(connectionID)
}

type ChannelParametersService struct {
	repository *mysqlInfra.ChannelParametersRepository
}

func NewChannelParametersService(repository *mysqlInfra.ChannelParametersRepository) *ChannelParametersService {
	return &ChannelParametersService{repository: repository}
}

func (s *ChannelParametersService) GetPaginatedParameters(offset, pageSize int) (*mysqlInfra.PaginatedChannelParameters, error) {
	return s.repository.FindPaginated(offset, pageSize)
}

func (s *ChannelParametersService) GetParameterByID(id int64) (*mysqlInfra.ChannelParameterDTO, error) {
	return s.repository.FindByID(id)
}

func (s *ChannelParametersService) GetParametersByChannel(channelID int64, offset, pageSize int) (*mysqlInfra.PaginatedChannelParameters, error) {
	return s.repository.FindByChannelID(channelID, offset, pageSize)
}

func (s *ChannelParametersService) CreateParameter(input mysqlInfra.CreateChannelParameterInput) (*mysqlInfra.ChannelParameterDTO, error) {
	if input.ChannelID == 0 || input.ParameterName == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Create(input)
}

func (s *ChannelParametersService) UpdateParameter(id int64, input mysqlInfra.UpdateChannelParameterInput) (*mysqlInfra.ChannelParameterDTO, error) {
	if input.DisplayName == "" {
		return nil, ErrInvalidChannelConfig
	}

	return s.repository.Update(id, input)
}

func (s *ChannelParametersService) DeleteParameter(id int64) error {
	return s.repository.SoftDelete(id)
}

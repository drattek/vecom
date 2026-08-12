package sources

import (
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidSourcePayload = errors.New("invalid source payload")

type SourceService struct {
	repository *mysqlInfra.SourcesRepository
}

func NewSourceService(repository *mysqlInfra.SourcesRepository) *SourceService {
	return &SourceService{repository: repository}
}

func (s *SourceService) GetPaginatedSources(offset, pageSize int) (*mysqlInfra.PaginatedSources, error) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindPaginated(offset, pageSize)
}

func (s *SourceService) GetSourceByID(id int64) (*mysqlInfra.SourceDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidSourcePayload
	}
	return s.repository.FindByID(id)
}

func (s *SourceService) GetSourceByCode(code string) (*mysqlInfra.SourceDTO, error) {
	if code == "" {
		return nil, ErrInvalidSourcePayload
	}
	return s.repository.FindByCode(code)
}

func (s *SourceService) CreateSource(input mysqlInfra.CreateSourceInput) (*mysqlInfra.SourceDTO, error) {
	if input.Code == "" || input.Name == "" {
		return nil, ErrInvalidSourcePayload
	}
	if input.CreatedBy <= 0 {
		return nil, ErrInvalidSourcePayload
	}

	return s.repository.Create(input)
}

func (s *SourceService) UpdateSource(id int64, input mysqlInfra.UpdateSourceInput) (*mysqlInfra.SourceDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidSourcePayload
	}
	if input.Code == "" || input.Name == "" {
		return nil, ErrInvalidSourcePayload
	}
	if input.UpdatedBy <= 0 {
		return nil, ErrInvalidSourcePayload
	}

	return s.repository.Update(id, input)
}

func (s *SourceService) DeleteSource(id int64) error {
	if id <= 0 {
		return ErrInvalidSourcePayload
	}
	return s.repository.SoftDelete(id)
}

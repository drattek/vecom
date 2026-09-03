package sources

import (
	"context"
	"database/sql"
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidSourcePayload = errors.New("invalid source payload")

type SourceService struct {
	db         *sql.DB
	repository *mysqlInfra.SourcesRepository
}

func NewSourceService(db *sql.DB, repository *mysqlInfra.SourcesRepository) *SourceService {
	return &SourceService{db: db, repository: repository}
}

func (s *SourceService) GetPaginatedSources(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedSources, error) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *SourceService) GetSourceByID(ctx context.Context, id int64) (*mysqlInfra.SourceDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidSourcePayload
	}
	return s.repository.FindByID(ctx, id)
}

func (s *SourceService) GetSourceByCode(ctx context.Context, code string) (*mysqlInfra.SourceDTO, error) {
	if code == "" {
		return nil, ErrInvalidSourcePayload
	}
	return s.repository.FindByCode(ctx, code)
}

func (s *SourceService) CreateSource(ctx context.Context, input mysqlInfra.CreateSourceInput) (*mysqlInfra.SourceDTO, error) {
	if input.Code == "" || input.Name == "" {
		return nil, ErrInvalidSourcePayload
	}
	if input.CreatedBy <= 0 {
		return nil, ErrInvalidSourcePayload
	}

	return s.repository.Create(ctx, input)
}

func (s *SourceService) UpdateSource(ctx context.Context, id int64, input mysqlInfra.UpdateSourceInput) (*mysqlInfra.SourceDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidSourcePayload
	}
	if input.Code == "" || input.Name == "" {
		return nil, ErrInvalidSourcePayload
	}
	if input.UpdatedBy <= 0 {
		return nil, ErrInvalidSourcePayload
	}

	return s.repository.Update(ctx, id, input)
}

func (s *SourceService) DeleteSource(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidSourcePayload
	}
	return s.repository.SoftDelete(ctx, id)
}

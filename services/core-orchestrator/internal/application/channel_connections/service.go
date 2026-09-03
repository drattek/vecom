package channel_connections

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrChannelConnectionNotFound = errors.New("channel connection not found")
var ErrInvalidChannelConnectionPayload = errors.New("invalid channel connection payload")
var ErrInvalidChannelConnectionReference = errors.New("invalid channel connection reference")

type ChannelConnectionService struct {
	db         *sql.DB
	repository *mysqlInfra.ChannelConnectionRepository
}

type UpsertChannelConnectionInput struct {
	ChannelID              int64
	Name                   string
	Status                 string
	Environment            string
	CurrencyID             int64
	AllowsMultipleListings bool
}

func NewChannelConnectionService(db *sql.DB, repository *mysqlInfra.ChannelConnectionRepository) *ChannelConnectionService {
	return &ChannelConnectionService{db: db, repository: repository}
}

func (s *ChannelConnectionService) GetPaginatedChannelConnections(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedChannelConnections, error) {
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

func (s *ChannelConnectionService) GetChannelConnectionByID(ctx context.Context, id int64) (*mysqlInfra.ChannelConnectionDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidChannelConnectionPayload
	}

	channelConnection, err := s.repository.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return nil, ErrChannelConnectionNotFound
		}
		return nil, err
	}

	return channelConnection, nil
}

func (s *ChannelConnectionService) CreateChannelConnection(ctx context.Context, input UpsertChannelConnectionInput, actorID int64) (*mysqlInfra.ChannelConnectionDTO, error) {
	normalized, err := normalizeAndValidate(input)
	if err != nil {
		return nil, err
	}

	channelConnection, err := s.repository.Create(ctx, mysqlInfra.CreateChannelConnectionInput{
		ChannelID:              normalized.ChannelID,
		Name:                   normalized.Name,
		Status:                 normalized.Status,
		Environment:            normalized.Environment,
		CurrencyID:             normalized.CurrencyID,
		AllowsMultipleListings: normalized.AllowsMultipleListings,
		CreatedBy:              actorID,
	})
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionInvalidReference) {
			return nil, ErrInvalidChannelConnectionReference
		}
		return nil, err
	}

	return channelConnection, nil
}

func (s *ChannelConnectionService) UpdateChannelConnection(ctx context.Context, id int64, input UpsertChannelConnectionInput, actorID int64) (*mysqlInfra.ChannelConnectionDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidChannelConnectionPayload
	}

	normalized, err := normalizeAndValidate(input)
	if err != nil {
		return nil, err
	}

	channelConnection, err := s.repository.Update(ctx, id, mysqlInfra.UpdateChannelConnectionInput{
		ChannelID:              normalized.ChannelID,
		Name:                   normalized.Name,
		Status:                 normalized.Status,
		Environment:            normalized.Environment,
		CurrencyID:             normalized.CurrencyID,
		AllowsMultipleListings: normalized.AllowsMultipleListings,
		UpdatedBy:              actorID,
	})
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return nil, ErrChannelConnectionNotFound
		}
		if errors.Is(err, mysqlInfra.ErrChannelConnectionInvalidReference) {
			return nil, ErrInvalidChannelConnectionReference
		}
		return nil, err
	}

	return channelConnection, nil
}

func (s *ChannelConnectionService) SoftDeleteChannelConnection(ctx context.Context, id int64, actorID int64) error {
	if id <= 0 {
		return ErrInvalidChannelConnectionPayload
	}

	err := s.repository.SoftDelete(ctx, id, actorID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return ErrChannelConnectionNotFound
		}
		return err
	}

	return nil
}

func normalizeAndValidate(input UpsertChannelConnectionInput) (UpsertChannelConnectionInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.TrimSpace(strings.ToLower(input.Status))
	input.Environment = strings.TrimSpace(strings.ToLower(input.Environment))

	if input.Name == "" || input.ChannelID <= 0 || input.CurrencyID <= 0 {
		return UpsertChannelConnectionInput{}, ErrInvalidChannelConnectionPayload
	}

	if input.Status == "" {
		input.Status = "active"
	}

	if input.Environment == "" {
		input.Environment = "development"
	}

	if input.Status != "active" && input.Status != "discontinued" && input.Status != "hidden" {
		return UpsertChannelConnectionInput{}, ErrInvalidChannelConnectionPayload
	}

	if input.Environment != "development" && input.Environment != "production" {
		return UpsertChannelConnectionInput{}, ErrInvalidChannelConnectionPayload
	}

	return input, nil
}

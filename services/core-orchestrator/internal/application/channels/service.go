package channels

import (
	"errors"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrChannelNotFound = errors.New("channel not found")
var ErrChannelCodeAlreadyExists = errors.New("channel code already exists")
var ErrInvalidChannelPayload = errors.New("invalid channel payload")
var ErrInvalidChannelReference = errors.New("invalid channel reference")

type ChannelService struct {
	repository *mysqlInfra.ChannelRepository
}

type UpsertChannelInput struct {
	Name        string
	Code        string
	Status      string
	IconID      *int64
	Description *string
}

func NewChannelService(repository *mysqlInfra.ChannelRepository) *ChannelService {
	return &ChannelService{repository: repository}
}

func (s *ChannelService) GetPaginatedChannels(offset, pageSize int) (*mysqlInfra.PaginatedChannels, error) {
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

func (s *ChannelService) GetChannelByID(id int64) (*mysqlInfra.ChannelDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidChannelPayload
	}

	channel, err := s.repository.FindByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}

	return channel, nil
}

func (s *ChannelService) CreateChannel(input UpsertChannelInput, actorID int64) (*mysqlInfra.ChannelDTO, error) {
	normalized, err := normalizeAndValidate(input)
	if err != nil {
		return nil, err
	}

	channel, err := s.repository.Create(mysqlInfra.CreateChannelInput{
		Name:        normalized.Name,
		Code:        normalized.Code,
		Status:      normalized.Status,
		IconID:      normalized.IconID,
		Description: normalized.Description,
		CreatedBy:   actorID,
	})
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelCodeAlreadyExists) {
			return nil, ErrChannelCodeAlreadyExists
		}
		if errors.Is(err, mysqlInfra.ErrChannelInvalidReference) {
			return nil, ErrInvalidChannelReference
		}
		return nil, err
	}

	return channel, nil
}

func (s *ChannelService) UpdateChannel(id int64, input UpsertChannelInput, actorID int64) (*mysqlInfra.ChannelDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidChannelPayload
	}

	normalized, err := normalizeAndValidate(input)
	if err != nil {
		return nil, err
	}

	channel, err := s.repository.Update(id, mysqlInfra.UpdateChannelInput{
		Name:        normalized.Name,
		Code:        normalized.Code,
		Status:      normalized.Status,
		IconID:      normalized.IconID,
		Description: normalized.Description,
		UpdatedBy:   actorID,
	})
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelNotFound) {
			return nil, ErrChannelNotFound
		}
		if errors.Is(err, mysqlInfra.ErrChannelCodeAlreadyExists) {
			return nil, ErrChannelCodeAlreadyExists
		}
		if errors.Is(err, mysqlInfra.ErrChannelInvalidReference) {
			return nil, ErrInvalidChannelReference
		}
		return nil, err
	}

	return channel, nil
}

func (s *ChannelService) SoftDeleteChannel(id int64, actorID int64) error {
	if id <= 0 {
		return ErrInvalidChannelPayload
	}

	err := s.repository.SoftDelete(id, actorID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelNotFound) {
			return ErrChannelNotFound
		}
		return err
	}

	return nil
}

func normalizeAndValidate(input UpsertChannelInput) (UpsertChannelInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Code = strings.TrimSpace(strings.ToLower(input.Code))
	input.Status = strings.TrimSpace(strings.ToLower(input.Status))

	if input.Name == "" || input.Code == "" {
		return UpsertChannelInput{}, ErrInvalidChannelPayload
	}

	if input.Status == "" {
		input.Status = "active"
	}

	if input.Status != "active" && input.Status != "discontinued" && input.Status != "hidden" {
		return UpsertChannelInput{}, ErrInvalidChannelPayload
	}

	if input.IconID != nil && *input.IconID <= 0 {
		input.IconID = nil
	}

	if input.Description != nil {
		trimmedDescription := strings.TrimSpace(*input.Description)
		if trimmedDescription == "" {
			input.Description = nil
		} else {
			input.Description = &trimmedDescription
		}
	}

	return input, nil
}

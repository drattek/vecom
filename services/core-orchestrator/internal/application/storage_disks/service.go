package storage_disks

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrStorageDiskNotFound = errors.New("storage disk not found")
var ErrStorageDiskCodeAlreadyExists = errors.New("storage disk code already exists")
var ErrInvalidStorageDiskPayload = errors.New("invalid storage disk payload")

type StorageDiskService struct {
	db         *sql.DB
	repository *mysqlInfra.StorageDiskRepository
}

type UpsertStorageDiskInput struct {
	Name     string
	Code     string
	BaseURL  string
	Bucket   *string
	Endpoint *string
	IsPublic bool
}

func NewStorageDiskService(db *sql.DB, repository *mysqlInfra.StorageDiskRepository) *StorageDiskService {
	return &StorageDiskService{db: db, repository: repository}
}

func (s *StorageDiskService) GetPaginatedStorageDisks(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedStorageDisks, error) {
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

func (s *StorageDiskService) GetStorageDiskByID(ctx context.Context, id int64) (*mysqlInfra.StorageDiskDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidStorageDiskPayload
	}

	storageDisk, err := s.repository.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return nil, ErrStorageDiskNotFound
		}
		return nil, err
	}

	return storageDisk, nil
}

func (s *StorageDiskService) CreateStorageDisk(ctx context.Context, input UpsertStorageDiskInput, actorID int64) (*mysqlInfra.StorageDiskDTO, error) {
	normalized, err := normalizeAndValidate(input)
	if err != nil {
		return nil, err
	}

	storageDisk, err := s.repository.Create(ctx, mysqlInfra.CreateStorageDiskInput{
		Name:      normalized.Name,
		Code:      normalized.Code,
		BaseURL:   normalized.BaseURL,
		Bucket:    normalized.Bucket,
		Endpoint:  normalized.Endpoint,
		IsPublic:  normalized.IsPublic,
		CreatedBy: actorID,
	})
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskCodeAlreadyExists) {
			return nil, ErrStorageDiskCodeAlreadyExists
		}
		return nil, err
	}

	return storageDisk, nil
}

func (s *StorageDiskService) UpdateStorageDisk(ctx context.Context, id int64, input UpsertStorageDiskInput, actorID int64) (*mysqlInfra.StorageDiskDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidStorageDiskPayload
	}

	normalized, err := normalizeAndValidate(input)
	if err != nil {
		return nil, err
	}

	storageDisk, err := s.repository.Update(ctx, id, mysqlInfra.UpdateStorageDiskInput{
		Name:      normalized.Name,
		Code:      normalized.Code,
		BaseURL:   normalized.BaseURL,
		Bucket:    normalized.Bucket,
		Endpoint:  normalized.Endpoint,
		IsPublic:  normalized.IsPublic,
		UpdatedBy: actorID,
	})
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return nil, ErrStorageDiskNotFound
		}
		if errors.Is(err, mysqlInfra.ErrStorageDiskCodeAlreadyExists) {
			return nil, ErrStorageDiskCodeAlreadyExists
		}
		return nil, err
	}

	return storageDisk, nil
}

func (s *StorageDiskService) SoftDeleteStorageDisk(ctx context.Context, id int64, actorID int64) error {
	if id <= 0 {
		return ErrInvalidStorageDiskPayload
	}

	err := s.repository.SoftDelete(ctx, id, actorID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return ErrStorageDiskNotFound
		}
		return err
	}

	return nil
}

func normalizeAndValidate(input UpsertStorageDiskInput) (UpsertStorageDiskInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Code = strings.TrimSpace(input.Code)
	input.BaseURL = strings.TrimSpace(input.BaseURL)

	if input.Name == "" || input.Code == "" || input.BaseURL == "" {
		return UpsertStorageDiskInput{}, ErrInvalidStorageDiskPayload
	}

	if input.Bucket != nil {
		trimmed := strings.TrimSpace(*input.Bucket)
		if trimmed == "" {
			input.Bucket = nil
		} else {
			input.Bucket = &trimmed
		}
	}

	if input.Endpoint != nil {
		trimmed := strings.TrimSpace(*input.Endpoint)
		if trimmed == "" {
			input.Endpoint = nil
		} else {
			input.Endpoint = &trimmed
		}
	}

	return input, nil
}

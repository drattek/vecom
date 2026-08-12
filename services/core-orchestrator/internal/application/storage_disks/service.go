package storage_disks

import (
	"errors"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrStorageDiskNotFound = errors.New("storage disk not found")
var ErrStorageDiskCodeAlreadyExists = errors.New("storage disk code already exists")
var ErrInvalidStorageDiskPayload = errors.New("invalid storage disk payload")

type StorageDiskService struct {
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

func NewStorageDiskService(repository *mysqlInfra.StorageDiskRepository) *StorageDiskService {
	return &StorageDiskService{repository: repository}
}

func (s *StorageDiskService) GetPaginatedStorageDisks(offset, pageSize int) (*mysqlInfra.PaginatedStorageDisks, error) {
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

func (s *StorageDiskService) GetStorageDiskByID(id int64) (*mysqlInfra.StorageDiskDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidStorageDiskPayload
	}

	storageDisk, err := s.repository.FindByID(id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrStorageDiskNotFound) {
			return nil, ErrStorageDiskNotFound
		}
		return nil, err
	}

	return storageDisk, nil
}

func (s *StorageDiskService) CreateStorageDisk(input UpsertStorageDiskInput, actorID int64) (*mysqlInfra.StorageDiskDTO, error) {
	normalized, err := normalizeAndValidate(input)
	if err != nil {
		return nil, err
	}

	storageDisk, err := s.repository.Create(mysqlInfra.CreateStorageDiskInput{
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

func (s *StorageDiskService) UpdateStorageDisk(id int64, input UpsertStorageDiskInput, actorID int64) (*mysqlInfra.StorageDiskDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidStorageDiskPayload
	}

	normalized, err := normalizeAndValidate(input)
	if err != nil {
		return nil, err
	}

	storageDisk, err := s.repository.Update(id, mysqlInfra.UpdateStorageDiskInput{
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

func (s *StorageDiskService) SoftDeleteStorageDisk(id int64, actorID int64) error {
	if id <= 0 {
		return ErrInvalidStorageDiskPayload
	}

	err := s.repository.SoftDelete(id, actorID)
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

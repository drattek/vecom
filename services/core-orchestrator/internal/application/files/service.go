package files

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrFileNotFound = errors.New("file not found")
var ErrInvalidFilePayload = errors.New("invalid file payload")

type FileService struct {
	db         *sql.DB
	repository *mysqlInfra.FilesRepository
}

type UpsertFileInput struct {
	DiskID           int64
	Path             string
	Filename         string
	OriginalFilename *string
	MimeType         string
	FileType         string
	Extension        string
	Size             int64
	Checksum         *string
	Width            *int64
	Height           *int64
	IsPublic         bool
}

func NewFileService(db *sql.DB, repository *mysqlInfra.FilesRepository) *FileService {
	return &FileService{db: db, repository: repository}
}

func (s *FileService) GetPaginatedFiles(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedFiles, error) {
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

func (s *FileService) GetFileByID(ctx context.Context, id int64) (*mysqlInfra.FileDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidFilePayload
	}

	file, err := s.repository.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrFileNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	return file, nil
}

func (s *FileService) CreateFile(ctx context.Context, input UpsertFileInput, actorID int64) (*mysqlInfra.FileDTO, error) {
	normalized, err := normalizeAndValidate(input)
	if err != nil {
		return nil, err
	}

	file, err := s.repository.Create(ctx, mysqlInfra.CreateFileInput{
		DiskID:           normalized.DiskID,
		Path:             normalized.Path,
		Filename:         normalized.Filename,
		OriginalFilename: normalized.OriginalFilename,
		MimeType:         normalized.MimeType,
		FileType:         normalized.FileType,
		Extension:        normalized.Extension,
		Size:             normalized.Size,
		Checksum:         normalized.Checksum,
		Width:            normalized.Width,
		Height:           normalized.Height,
		IsPublic:         normalized.IsPublic,
		CreatedBy:        actorID,
	})
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (s *FileService) UpdateFile(ctx context.Context, id int64, input UpsertFileInput, actorID int64) (*mysqlInfra.FileDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidFilePayload
	}

	normalized, err := normalizeAndValidate(input)
	if err != nil {
		return nil, err
	}

	file, err := s.repository.Update(ctx, id, mysqlInfra.UpdateFileInput{
		DiskID:           normalized.DiskID,
		Path:             normalized.Path,
		Filename:         normalized.Filename,
		OriginalFilename: normalized.OriginalFilename,
		MimeType:         normalized.MimeType,
		FileType:         normalized.FileType,
		Extension:        normalized.Extension,
		Size:             normalized.Size,
		Checksum:         normalized.Checksum,
		Width:            normalized.Width,
		Height:           normalized.Height,
		IsPublic:         normalized.IsPublic,
		UpdatedBy:        actorID,
	})
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrFileNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	return file, nil
}

func (s *FileService) SoftDeleteFile(ctx context.Context, id int64, actorID int64) error {
	if id <= 0 {
		return ErrInvalidFilePayload
	}

	err := s.repository.SoftDelete(ctx, id, actorID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrFileNotFound) {
			return ErrFileNotFound
		}
		return err
	}

	return nil
}

func normalizeAndValidate(input UpsertFileInput) (UpsertFileInput, error) {
	input.Path = strings.TrimSpace(input.Path)
	input.Filename = strings.TrimSpace(input.Filename)
	input.MimeType = strings.TrimSpace(input.MimeType)
	input.FileType = strings.TrimSpace(strings.ToLower(input.FileType))
	input.Extension = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(input.Extension), "."))

	if input.DiskID <= 0 {
		return UpsertFileInput{}, ErrInvalidFilePayload
	}

	if input.Path == "" {
		return UpsertFileInput{}, ErrInvalidFilePayload
	}

	if input.Filename == "" {
		return UpsertFileInput{}, ErrInvalidFilePayload
	}

	if input.MimeType == "" {
		return UpsertFileInput{}, ErrInvalidFilePayload
	}

	if input.FileType == "" {
		return UpsertFileInput{}, ErrInvalidFilePayload
	}

	if input.FileType != "image" && input.FileType != "document" && input.FileType != "video" && input.FileType != "icon" && input.FileType != "archive" && input.FileType != "other" {
		return UpsertFileInput{}, ErrInvalidFilePayload
	}

	if input.Extension == "" {
		return UpsertFileInput{}, ErrInvalidFilePayload
	}

	if input.Size <= 0 {
		return UpsertFileInput{}, ErrInvalidFilePayload
	}

	if input.Width != nil && *input.Width <= 0 {
		return UpsertFileInput{}, ErrInvalidFilePayload
	}

	if input.Height != nil && *input.Height <= 0 {
		return UpsertFileInput{}, ErrInvalidFilePayload
	}

	if input.OriginalFilename != nil {
		trimmed := strings.TrimSpace(*input.OriginalFilename)
		if trimmed == "" {
			input.OriginalFilename = nil
		} else {
			input.OriginalFilename = &trimmed
		}
	}

	if input.Checksum != nil {
		trimmed := strings.TrimSpace(strings.ToLower(*input.Checksum))
		if trimmed == "" {
			input.Checksum = nil
		} else {
			input.Checksum = &trimmed
		}
	}

	return input, nil
}

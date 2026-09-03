package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrFileNotFound = errors.New("file not found")

type FileDTO struct {
	ID               int64      `json:"id"`
	DiskID           int64      `json:"diskId"`
	Path             string     `json:"path"`
	Filename         string     `json:"filename"`
	OriginalFilename *string    `json:"originalFilename"`
	MimeType         string     `json:"mimeType"`
	FileType         string     `json:"fileType"`
	Extension        string     `json:"extension"`
	Size             int64      `json:"size"`
	Checksum         *string    `json:"checksum"`
	Width            *int64     `json:"width,omitempty"`
	Height           *int64     `json:"height,omitempty"`
	IsPublic         bool       `json:"isPublic"`
	CreatedBy        int64      `json:"createdBy"`
	UpdatedBy        *int64     `json:"updatedBy,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedFiles struct {
	Total    int64     `json:"total"`
	Offset   int       `json:"offset"`
	PageSize int       `json:"pageSize"`
	Files    []FileDTO `json:"files"`
}

type CreateFileInput struct {
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
	CreatedBy        int64
}

type UpdateFileInput struct {
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
	UpdatedBy        int64
}

type FilesRepository struct {
	db Querier
}

func NewFilesRepository(db Querier) *FilesRepository {
	return &FilesRepository{db: db}
}

func (r *FilesRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedFiles, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_files WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting files: %w", err)
	}

	query := `
		SELECT
			c.id,
			c.disk_id,
			c.path,
			c.filename,
			c.original_filename,
			c.mime_type,
			c.file_type,
			c.extension,
			c.size,
			c.checksum,
			c.width,
			c.height,
			c.is_public,
			c.created_by,
			c.updated_by,
			c.created_at,
			c.updated_at,
			c.deleted_at
		FROM ecom_files c
		WHERE c.deleted_at IS NULL
		ORDER BY c.id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying files: %w", err)
	}
	defer rows.Close()

	files := make([]FileDTO, 0)
	for rows.Next() {
		file, scanErr := scanFile(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		files = append(files, file)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating files: %w", err)
	}

	return &PaginatedFiles{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Files:    files,
	}, nil
}

func (r *FilesRepository) FindByID(ctx context.Context, id int64) (*FileDTO, error) {
	query := `
		SELECT
			c.id,
			c.disk_id,
			c.path,
			c.filename,
			c.original_filename,
			c.mime_type,
			c.file_type,
			c.extension,
			c.size,
			c.checksum,
			c.width,
			c.height,
			c.is_public,
			c.created_by,
			c.updated_by,
			c.created_at,
			c.updated_at,
			c.deleted_at
		FROM ecom_files c
		WHERE c.id = ? AND c.deleted_at IS NULL
	`
	row := r.db.QueryRowContext(ctx, query, id)
	file, err := scanFile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("error querying file by id: %w", err)
	}
	return &file, nil
}

// FindByPath looks up an existing file by its stored path/URL, so callers
// that receive the same URL twice can reuse the existing ecom_files row
// instead of re-downloading/re-inserting it.
func (r *FilesRepository) FindByPath(ctx context.Context, path string) (*FileDTO, error) {
	query := `
		SELECT
			c.id,
			c.disk_id,
			c.path,
			c.filename,
			c.original_filename,
			c.mime_type,
			c.file_type,
			c.extension,
			c.size,
			c.checksum,
			c.width,
			c.height,
			c.is_public,
			c.created_by,
			c.updated_by,
			c.created_at,
			c.updated_at,
			c.deleted_at
		FROM ecom_files c
		WHERE c.path = ? AND c.deleted_at IS NULL
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, path)
	file, err := scanFile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("error querying file by path: %w", err)
	}
	return &file, nil
}

func (r *FilesRepository) Create(ctx context.Context, input CreateFileInput) (*FileDTO, error) {
	query := `
		INSERT INTO ecom_files (disk_id, path, filename, original_filename, mime_type, file_type, extension, size, checksum, width, height, is_public, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx,
		query,
		input.DiskID,
		input.Path,
		input.Filename,
		normalizedStringOrNil(input.OriginalFilename),
		input.MimeType,
		input.FileType,
		input.Extension,
		input.Size,
		normalizedStringOrNil(input.Checksum),
		input.Width,
		input.Height,
		input.IsPublic,
		input.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *FilesRepository) Update(ctx context.Context, id int64, input UpdateFileInput) (*FileDTO, error) {
	query := `
		UPDATE ecom_files
		SET disk_id = ?, path = ?, filename = ?, original_filename = ?, mime_type = ?, file_type = ?, extension = ?, size = ?, checksum = ?, width = ?, height = ?, is_public = ?, updated_by = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx,
		query,
		input.DiskID,
		input.Path,
		input.Filename,
		normalizedStringOrNil(input.OriginalFilename),
		input.MimeType,
		input.FileType,
		input.Extension,
		input.Size,
		normalizedStringOrNil(input.Checksum),
		input.Width,
		input.Height,
		input.IsPublic,
		input.UpdatedBy,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("error updating file: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error checking updated rows: %w", err)
	}

	if affected == 0 {
		return nil, ErrFileNotFound
	}

	file, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (r *FilesRepository) SoftDelete(ctx context.Context, id, updatedBy int64) error {
	query := `
		UPDATE ecom_files
		SET deleted_at = CURRENT_TIMESTAMP, updated_by = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, updatedBy, id)
	if err != nil {
		return fmt.Errorf("error soft deleting file: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking deleted rows: %w", err)
	}

	if affected == 0 {
		return ErrFileNotFound
	}

	return nil
}

func scanFile(scanner interface{ Scan(dest ...any) error }) (FileDTO, error) {
	var file FileDTO
	var originalFilename sql.NullString
	var checksum sql.NullString
	var width sql.NullInt64
	var height sql.NullInt64
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := scanner.Scan(
		&file.ID,
		&file.DiskID,
		&file.Path,
		&file.Filename,
		&originalFilename,
		&file.MimeType,
		&file.FileType,
		&file.Extension,
		&file.Size,
		&checksum,
		&width,
		&height,
		&file.IsPublic,
		&file.CreatedBy,
		&updatedBy,
		&file.CreatedAt,
		&file.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return FileDTO{}, fmt.Errorf("error scanning file: %w", err)
	}

	if originalFilename.Valid {
		trimmedOriginalFilename := strings.TrimSpace(originalFilename.String)
		if trimmedOriginalFilename != "" {
			file.OriginalFilename = &trimmedOriginalFilename
		}
	}

	if checksum.Valid {
		trimmedChecksum := strings.TrimSpace(checksum.String)
		if trimmedChecksum != "" {
			file.Checksum = &trimmedChecksum
		}
	}

	if width.Valid {
		file.Width = &width.Int64
	}

	if height.Valid {
		file.Height = &height.Int64
	}

	if updatedBy.Valid {
		file.UpdatedBy = &updatedBy.Int64
	}

	if deletedAt.Valid {
		file.DeletedAt = &deletedAt.Time
	}

	return file, nil
}

package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

var ErrStorageDiskNotFound = errors.New("storage disk not found")
var ErrStorageDiskCodeAlreadyExists = errors.New("storage disk code already exists")

type StorageDiskDTO struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Code      string     `json:"code"`
	BaseURL   string     `json:"baseUrl"`
	Bucket    *string    `json:"bucket,omitempty"`
	Endpoint  *string    `json:"endpoint,omitempty"`
	IsPublic  bool       `json:"isPublic"`
	CreatedBy int64      `json:"createdBy"`
	UpdatedBy *int64     `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedStorageDisks struct {
	Total        int64            `json:"total"`
	Offset       int              `json:"offset"`
	PageSize     int              `json:"pageSize"`
	StorageDisks []StorageDiskDTO `json:"storageDisks"`
}

type CreateStorageDiskInput struct {
	Name      string
	Code      string
	BaseURL   string
	Bucket    *string
	Endpoint  *string
	IsPublic  bool
	CreatedBy int64
}

type UpdateStorageDiskInput struct {
	Name      string
	Code      string
	BaseURL   string
	Bucket    *string
	Endpoint  *string
	IsPublic  bool
	UpdatedBy int64
}

type StorageDiskRepository struct {
	db *sql.DB
}

func NewStorageDiskRepository(db *sql.DB) *StorageDiskRepository {
	return &StorageDiskRepository{db: db}
}

func (r *StorageDiskRepository) FindPaginated(offset, pageSize int) (*PaginatedStorageDisks, error) {
	var total int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM ecom_storage_disks WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting storage disks: %w", err)
	}

	query := `
		SELECT id, name, code, base_url, bucket, endpoint, is_public, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_storage_disks
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying storage disks: %w", err)
	}
	defer rows.Close()

	storageDisks := make([]StorageDiskDTO, 0)
	for rows.Next() {
		storageDisk, scanErr := scanStorageDisk(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		storageDisks = append(storageDisks, storageDisk)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating storage disks: %w", err)
	}

	return &PaginatedStorageDisks{
		Total:        total,
		Offset:       offset,
		PageSize:     pageSize,
		StorageDisks: storageDisks,
	}, nil
}

func (r *StorageDiskRepository) FindByID(id int64) (*StorageDiskDTO, error) {
	query := `
		SELECT id, name, code, base_url, bucket, endpoint, is_public, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_storage_disks
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, id)
	storageDisk, err := scanStorageDisk(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStorageDiskNotFound
		}
		return nil, err
	}

	return &storageDisk, nil
}

func (r *StorageDiskRepository) Create(input CreateStorageDiskInput) (*StorageDiskDTO, error) {
	query := `
		INSERT INTO ecom_storage_disks (name, code, base_url, bucket, endpoint, is_public, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
	`

	result, err := r.db.Exec(
		query,
		input.Name,
		input.Code,
		input.BaseURL,
		normalizedStringOrNil(input.Bucket),
		normalizedStringOrNil(input.Endpoint),
		input.IsPublic,
		input.CreatedBy,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrStorageDiskCodeAlreadyExists
		}
		return nil, fmt.Errorf("error creating storage disk: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting storage disk id: %w", err)
	}

	return r.FindByID(id)
}

func (r *StorageDiskRepository) Update(id int64, input UpdateStorageDiskInput) (*StorageDiskDTO, error) {
	query := `
		UPDATE ecom_storage_disks
		SET name = ?, code = ?, base_url = ?, bucket = ?, endpoint = ?, is_public = ?, updated_by = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(
		query,
		input.Name,
		input.Code,
		input.BaseURL,
		normalizedStringOrNil(input.Bucket),
		normalizedStringOrNil(input.Endpoint),
		input.IsPublic,
		input.UpdatedBy,
		id,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrStorageDiskCodeAlreadyExists
		}
		return nil, fmt.Errorf("error updating storage disk: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error checking updated rows: %w", err)
	}

	if affected == 0 {
		return nil, ErrStorageDiskNotFound
	}

	return r.FindByID(id)
}

func (r *StorageDiskRepository) SoftDelete(id, updatedBy int64) error {
	query := `
		UPDATE ecom_storage_disks
		SET deleted_at = CURRENT_TIMESTAMP, updated_by = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, updatedBy, id)
	if err != nil {
		return fmt.Errorf("error soft deleting storage disk: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking deleted rows: %w", err)
	}

	if affected == 0 {
		return ErrStorageDiskNotFound
	}

	return nil
}

func scanStorageDisk(scanner interface{ Scan(dest ...any) error }) (StorageDiskDTO, error) {
	var storageDisk StorageDiskDTO
	var bucket sql.NullString
	var endpoint sql.NullString
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := scanner.Scan(
		&storageDisk.ID,
		&storageDisk.Name,
		&storageDisk.Code,
		&storageDisk.BaseURL,
		&bucket,
		&endpoint,
		&storageDisk.IsPublic,
		&storageDisk.CreatedBy,
		&updatedBy,
		&storageDisk.CreatedAt,
		&storageDisk.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return StorageDiskDTO{}, fmt.Errorf("error scanning storage disk: %w", err)
	}

	if bucket.Valid {
		storageDisk.Bucket = &bucket.String
	}

	if endpoint.Valid {
		storageDisk.Endpoint = &endpoint.String
	}

	if updatedBy.Valid {
		storageDisk.UpdatedBy = &updatedBy.Int64
	}

	if deletedAt.Valid {
		storageDisk.DeletedAt = &deletedAt.Time
	}

	return storageDisk, nil
}

func normalizedStringOrNil(value *string) interface{} {
	if value == nil {
		return nil
	}

	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil
	}

	return normalized
}

func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlErr) {
		return false
	}

	return mysqlErr.Number == 1062
}

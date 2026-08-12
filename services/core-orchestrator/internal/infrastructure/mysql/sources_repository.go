package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrSourceNotFound = errors.New("source not found")

type SourceDTO struct {
	ID        int64      `json:"id"`
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	CreatedBy int64      `json:"createdBy"`
	UpdatedBy *int64     `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedSources struct {
	Total    int64       `json:"total"`
	Offset   int         `json:"offset"`
	PageSize int         `json:"pageSize"`
	Sources  []SourceDTO `json:"sources"`
}

type CreateSourceInput struct {
	Code      string
	Name      string
	CreatedBy int64
}

type UpdateSourceInput struct {
	Code      string
	Name      string
	UpdatedBy int64
}

type SourcesRepository struct {
	db Querier
}

func NewSourcesRepository(db Querier) *SourcesRepository {
	return &SourcesRepository{db: db}
}

func (r *SourcesRepository) FindPaginated(offset, pageSize int) (*PaginatedSources, error) {
	var total int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM ecom_sources WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting sources: %w", err)
	}

	query := `
		SELECT id, code, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_sources
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying sources: %w", err)
	}
	defer rows.Close()

	sources := make([]SourceDTO, 0)
	for rows.Next() {
		source, scanErr := scanSource(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		sources = append(sources, source)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating sources: %w", err)
	}

	return &PaginatedSources{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Sources:  sources,
	}, nil
}

func (r *SourcesRepository) FindByID(id int64) (*SourceDTO, error) {
	query := `
		SELECT id, code, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_sources
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, id)
	return scanSourceRow(row)
}

func (r *SourcesRepository) FindByCode(code string) (*SourceDTO, error) {
	query := `
		SELECT id, code, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_sources
		WHERE code = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, code)
	return scanSourceRow(row)
}

func (r *SourcesRepository) Create(input CreateSourceInput) (*SourceDTO, error) {
	query := `
		INSERT INTO ecom_sources (code, name, created_by, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.Exec(query, input.Code, input.Name, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("error creating source: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(id)
}

func (r *SourcesRepository) Update(id int64, input UpdateSourceInput) (*SourceDTO, error) {
	query := `
		UPDATE ecom_sources
		SET code = ?, name = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, input.Code, input.Name, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating source: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrSourceNotFound
	}

	return r.FindByID(id)
}

func (r *SourcesRepository) SoftDelete(id int64) error {
	query := `
		UPDATE ecom_sources
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error deleting source: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrSourceNotFound
	}

	return nil
}

func scanSource(rows *sql.Rows) (SourceDTO, error) {
	var source SourceDTO
	err := rows.Scan(
		&source.ID,
		&source.Code,
		&source.Name,
		&source.CreatedBy,
		&source.UpdatedBy,
		&source.CreatedAt,
		&source.UpdatedAt,
		&source.DeletedAt,
	)
	if err != nil {
		return SourceDTO{}, fmt.Errorf("error scanning source: %w", err)
	}
	return source, nil
}

func scanSourceRow(row *sql.Row) (*SourceDTO, error) {
	var source SourceDTO
	err := row.Scan(
		&source.ID,
		&source.Code,
		&source.Name,
		&source.CreatedBy,
		&source.UpdatedBy,
		&source.CreatedAt,
		&source.UpdatedAt,
		&source.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSourceNotFound
		}
		return nil, fmt.Errorf("error scanning source: %w", err)
	}
	return &source, nil
}

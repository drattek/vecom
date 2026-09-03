package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrBranchNotFound = errors.New("branch not found")

type BranchDTO struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	CreatedBy int64      `json:"createdBy"`
	UpdatedBy *int64     `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedBranches struct {
	Total    int64       `json:"total"`
	Offset   int         `json:"offset"`
	PageSize int         `json:"pageSize"`
	Branches []BranchDTO `json:"branches"`
}

type CreateBranchInput struct {
	Name      string
	CreatedBy int64
}

type UpdateBranchInput struct {
	Name      string
	UpdatedBy int64
}

type BranchesRepository struct {
	db Querier
}

func NewBranchesRepository(db Querier) *BranchesRepository {
	return &BranchesRepository{db: db}
}

func (r *BranchesRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedBranches, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_branches WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting branches: %w", err)
	}

	query := `
		SELECT id, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_branches
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying branches: %w", err)
	}
	defer rows.Close()

	branches := make([]BranchDTO, 0)
	for rows.Next() {
		branch, scanErr := scanBranch(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		branches = append(branches, branch)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating branches: %w", err)
	}

	return &PaginatedBranches{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Branches: branches,
	}, nil
}

func (r *BranchesRepository) FindByID(ctx context.Context, id int64) (*BranchDTO, error) {
	query := `
		SELECT id, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_branches
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanBranchRow(row)
}

func (r *BranchesRepository) FindByName(ctx context.Context, name string) (*BranchDTO, error) {
	query := `
		SELECT id, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_branches
		WHERE name = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, name)
	return scanBranchRow(row)
}

func (r *BranchesRepository) Create(ctx context.Context, input CreateBranchInput) (*BranchDTO, error) {
	query := `
		INSERT INTO ecom_branches (name, created_by, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, input.Name, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("error creating branch: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *BranchesRepository) Update(ctx context.Context, id int64, input UpdateBranchInput) (*BranchDTO, error) {
	query := `
		UPDATE ecom_branches
		SET name = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.Name, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating branch: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrBranchNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *BranchesRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `
		UPDATE ecom_branches
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting branch: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrBranchNotFound
	}

	return nil
}

func scanBranch(rows *sql.Rows) (BranchDTO, error) {
	var branch BranchDTO
	err := rows.Scan(
		&branch.ID,
		&branch.Name,
		&branch.CreatedBy,
		&branch.UpdatedBy,
		&branch.CreatedAt,
		&branch.UpdatedAt,
		&branch.DeletedAt,
	)
	if err != nil {
		return BranchDTO{}, fmt.Errorf("error scanning branch: %w", err)
	}
	return branch, nil
}

func scanBranchRow(row *sql.Row) (*BranchDTO, error) {
	var branch BranchDTO
	err := row.Scan(
		&branch.ID,
		&branch.Name,
		&branch.CreatedBy,
		&branch.UpdatedBy,
		&branch.CreatedAt,
		&branch.UpdatedAt,
		&branch.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBranchNotFound
		}
		return nil, fmt.Errorf("error scanning branch: %w", err)
	}
	return &branch, nil
}

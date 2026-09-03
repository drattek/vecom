package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrBrandNotFound = errors.New("brand not found")
var ErrBrandNameAlreadyExists = errors.New("brand name already exists")

type BrandDTO struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	CreatedBy int64      `json:"createdBy"`
	UpdatedBy *int64     `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedBrands struct {
	Total    int64      `json:"total"`
	Offset   int        `json:"offset"`
	PageSize int        `json:"pageSize"`
	Brands   []BrandDTO `json:"brands"`
}

type CreateBrandInput struct {
	Name      string
	CreatedBy int64
}

type UpdateBrandInput struct {
	Name      string
	UpdatedBy int64
}

type BrandsRepository struct {
	db Querier
}

func NewBrandsRepository(db Querier) *BrandsRepository {
	return &BrandsRepository{db: db}
}

func (r *BrandsRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedBrands, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_brands WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting brands: %w", err)
	}

	query := `
		SELECT id, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_brands
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying brands: %w", err)
	}
	defer rows.Close()

	brands := make([]BrandDTO, 0)
	for rows.Next() {
		brand, scanErr := scanBrand(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		brands = append(brands, brand)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating brands: %w", err)
	}

	return &PaginatedBrands{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Brands:   brands,
	}, nil
}

func (r *BrandsRepository) FindByID(ctx context.Context, id int64) (*BrandDTO, error) {
	query := `
		SELECT id, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_brands
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanBrandRow(row)
}

func (r *BrandsRepository) FindByName(ctx context.Context, name string) (*BrandDTO, error) {
	query := `
		SELECT id, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_brands
		WHERE LOWER(name) = LOWER(?) AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, name)
	return scanBrandRow(row)
}

func (r *BrandsRepository) Create(ctx context.Context, input CreateBrandInput) (*BrandDTO, error) {
	query := `
		INSERT INTO ecom_brands (name, created_by, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, input.Name, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("error creating brand: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *BrandsRepository) Update(ctx context.Context, id int64, input UpdateBrandInput) (*BrandDTO, error) {
	query := `
		UPDATE ecom_brands
		SET name = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.Name, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating brand: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrBrandNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *BrandsRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `
		UPDATE ecom_brands
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting brand: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrBrandNotFound
	}

	return nil
}

func scanBrand(rows *sql.Rows) (BrandDTO, error) {
	var brand BrandDTO
	err := rows.Scan(
		&brand.ID,
		&brand.Name,
		&brand.CreatedBy,
		&brand.UpdatedBy,
		&brand.CreatedAt,
		&brand.UpdatedAt,
		&brand.DeletedAt,
	)
	if err != nil {
		return BrandDTO{}, fmt.Errorf("error scanning brand: %w", err)
	}
	return brand, nil
}

func scanBrandRow(row *sql.Row) (*BrandDTO, error) {
	var brand BrandDTO
	err := row.Scan(
		&brand.ID,
		&brand.Name,
		&brand.CreatedBy,
		&brand.UpdatedBy,
		&brand.CreatedAt,
		&brand.UpdatedAt,
		&brand.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBrandNotFound
		}
		return nil, fmt.Errorf("error scanning brand: %w", err)
	}
	return &brand, nil
}

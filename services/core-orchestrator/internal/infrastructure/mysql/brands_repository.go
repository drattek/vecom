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
	// ProductCount / VehicleFitmentCount sólo se llenan en FindPaginated (listado
	// de Settings → Marcas); en el resto de lecturas quedan en 0.
	ProductCount        int64 `json:"productCount"`
	VehicleFitmentCount int64 `json:"vehicleFitmentCount"`
}

// BrandUsage cuenta los registros vivos que referencian una marca; se usa para
// bloquear su baja mientras alguien la use.
type BrandUsage struct {
	Products          int64
	VehicleFitments   int64
	EquipmentFitments int64
	PartNumbers       int64
	PricingFormulas   int64
}

func (u BrandUsage) Total() int64 {
	return u.Products + u.VehicleFitments + u.EquipmentFitments + u.PartNumbers + u.PricingFormulas
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
		SELECT b.id, b.name, b.created_by, b.updated_by, b.created_at, b.updated_at, b.deleted_at,
			(SELECT COUNT(*) FROM ecom_products p WHERE p.brand_id = b.id AND p.deleted_at IS NULL),
			(SELECT COUNT(*) FROM ecom_vehicle_fitments vf WHERE vf.brand_id = b.id AND vf.deleted_at IS NULL)
		FROM ecom_brands b
		WHERE b.deleted_at IS NULL
		ORDER BY b.name ASC, b.id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying brands: %w", err)
	}
	defer rows.Close()

	brands := make([]BrandDTO, 0)
	for rows.Next() {
		var brand BrandDTO
		scanErr := rows.Scan(
			&brand.ID,
			&brand.Name,
			&brand.CreatedBy,
			&brand.UpdatedBy,
			&brand.CreatedAt,
			&brand.UpdatedAt,
			&brand.DeletedAt,
			&brand.ProductCount,
			&brand.VehicleFitmentCount,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("error scanning brand: %w", scanErr)
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

// BrandOption is the slim {id, name} shape the admin UI needs to populate a
// brand picker (no audit columns, no pagination).
type BrandOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// FindAllOptions returns every non-deleted brand as id/name, ordered by name —
// para los selectores de marca del panel (edición de producto, etc.).
func (r *BrandsRepository) FindAllOptions(ctx context.Context) ([]BrandOption, error) {
	query := `
		SELECT id, name
		FROM ecom_brands
		WHERE deleted_at IS NULL
		ORDER BY name ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying brand options: %w", err)
	}
	defer rows.Close()

	options := make([]BrandOption, 0)
	for rows.Next() {
		var option BrandOption
		if err := rows.Scan(&option.ID, &option.Name); err != nil {
			return nil, fmt.Errorf("error scanning brand option: %w", err)
		}
		options = append(options, option)
	}

	return options, rows.Err()
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

// FindByNameExcluding busca una marca viva con ese nombre (sin distinguir
// mayúsculas) distinta de excludeID; ErrBrandNotFound si no hay ninguna.
func (r *BrandsRepository) FindByNameExcluding(ctx context.Context, name string, excludeID int64) (*BrandDTO, error) {
	query := `
		SELECT id, name, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_brands
		WHERE LOWER(name) = LOWER(?) AND id <> ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, name, excludeID)
	return scanBrandRow(row)
}

// CountUsage cuenta los registros vivos que usan la marca.
func (r *BrandsRepository) CountUsage(ctx context.Context, id int64) (BrandUsage, error) {
	var usage BrandUsage
	query := `
		SELECT
			(SELECT COUNT(*) FROM ecom_products WHERE brand_id = ? AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM ecom_vehicle_fitments WHERE brand_id = ? AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM ecom_equipment_fitment WHERE brand_id = ? AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM ecom_product_part_numbers WHERE brand_id = ? AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM ecom_pricing_formulas WHERE brand_id = ? AND deleted_at IS NULL)
	`
	err := r.db.QueryRowContext(ctx, query, id, id, id, id, id).Scan(
		&usage.Products, &usage.VehicleFitments, &usage.EquipmentFitments, &usage.PartNumbers, &usage.PricingFormulas,
	)
	if err != nil {
		return BrandUsage{}, fmt.Errorf("error counting brand usage: %w", err)
	}
	return usage, nil
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

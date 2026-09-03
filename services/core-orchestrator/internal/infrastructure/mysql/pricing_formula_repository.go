package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrPricingFormulaNotFound = errors.New("pricing formula not found")
)

// PricingFormulaDTO is a stored expression that converts a product's base
// MXN price into the price pushed to a marketplace (MercadoLibre, Odoo) —
// see application/pricing.PricingFormulaCalculator, the only reader that
// compiles/evaluates Expression. BrandID/ConnectionID/PriceListID nil act
// as wildcards ("any brand" / "any connection" / "any price list"); the row
// with all three nil is the universal default. PriceListID is the price
// list that won when resolving the product's effective price (see
// ProductPricesRepository.FindEffectivePrice), not a list picked by hand.
// See Resolve for the specificity order used to pick a row.
type PricingFormulaDTO struct {
	ID           int64     `json:"id"`
	BrandID      *int64    `json:"brandId"`
	ConnectionID *int64    `json:"connectionId"`
	PriceListID  *int64    `json:"priceListId"`
	Expression   string    `json:"expression"`
	Description  *string   `json:"description"`
	CreatedBy    int64     `json:"createdBy"`
	UpdatedBy    *int64    `json:"updatedBy"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type PaginatedPricingFormulas struct {
	Data  []PricingFormulaDTO `json:"data"`
	Total int                 `json:"total"`
}

type CreatePricingFormulaInput struct {
	BrandID      *int64
	ConnectionID *int64
	PriceListID  *int64
	Expression   string
	Description  *string
	CreatedBy    int64
}

type UpdatePricingFormulaInput struct {
	Expression  string
	Description *string
	UpdatedBy   int64
}

type PricingFormulaRepository struct {
	db Querier
}

func NewPricingFormulaRepository(db Querier) *PricingFormulaRepository {
	return &PricingFormulaRepository{db: db}
}

func scanPricingFormula(s interface{ Scan(dest ...any) error }) (PricingFormulaDTO, error) {
	var p PricingFormulaDTO

	err := s.Scan(&p.ID, &p.BrandID, &p.ConnectionID, &p.PriceListID, &p.Expression, &p.Description,
		&p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return PricingFormulaDTO{}, err
	}

	return p, nil
}

func (r *PricingFormulaRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedPricingFormulas, error) {
	query := `
		SELECT id, brand_id, connection_id, price_list_id, expression, description,
		       created_by, updated_by, created_at, updated_at
		FROM ecom_pricing_formulas
		WHERE deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var formulas []PricingFormulaDTO
	for rows.Next() {
		f, err := scanPricingFormula(rows)
		if err != nil {
			return nil, err
		}
		formulas = append(formulas, f)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_pricing_formulas WHERE deleted_at IS NULL"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedPricingFormulas{Data: formulas, Total: total}, nil
}

func (r *PricingFormulaRepository) FindByID(ctx context.Context, id int64) (*PricingFormulaDTO, error) {
	query := `
		SELECT id, brand_id, connection_id, price_list_id, expression, description,
		       created_by, updated_by, created_at, updated_at
		FROM ecom_pricing_formulas
		WHERE id = ? AND deleted_at IS NULL
	`

	f, err := scanPricingFormula(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPricingFormulaNotFound
		}
		return nil, err
	}

	return &f, nil
}

// Resolve picks the applicable formula for a product's brand, the channel
// connection being synced to, and the price list that won when resolving
// the product's effective price. Each dimension has a fixed priority —
// connection_id beats brand_id beats price_list_id — and within that,
// matching a dimension beats leaving it as a wildcard (NULL) on that same
// dimension. In order of specificity (X/Y/Z = the actual ids, NULL = the
// wildcard slot for that dimension):
//  1. connection=Y brand=X price_list=Z (exact match on all three)
//  2. connection=Y brand=X price_list=NULL
//  3. connection=Y brand=NULL price_list=Z
//  4. connection=Y brand=NULL price_list=NULL (channel-wide)
//  5. connection=NULL brand=X price_list=Z
//  6. connection=NULL brand=X price_list=NULL (brand-wide)
//  7. connection=NULL brand=NULL price_list=Z
//  8. connection=NULL brand=NULL price_list=NULL (universal default)
//
// brandID/connectionID/priceListID nil match only rows whose column is
// NULL (a product with no brand, or a lookup with no price list, can only
// hit the wildcard row for that column, never a row scoped to a specific
// value).
func (r *PricingFormulaRepository) Resolve(ctx context.Context, brandID *int64, connectionID *int64, priceListID *int64) (*PricingFormulaDTO, error) {
	query := `
		SELECT id, brand_id, connection_id, price_list_id, expression, description,
		       created_by, updated_by, created_at, updated_at
		FROM ecom_pricing_formulas
		WHERE deleted_at IS NULL
		  AND (brand_id = ? OR brand_id IS NULL)
		  AND (connection_id = ? OR connection_id IS NULL)
		  AND (price_list_id = ? OR price_list_id IS NULL)
		ORDER BY (connection_id IS NOT NULL) DESC, (brand_id IS NOT NULL) DESC, (price_list_id IS NOT NULL) DESC
		LIMIT 1
	`

	f, err := scanPricingFormula(r.db.QueryRowContext(ctx, query, brandID, connectionID, priceListID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPricingFormulaNotFound
		}
		return nil, err
	}

	return &f, nil
}

// FindBySlot returns the formula for the exact (brandID, connectionID,
// priceListID) slot — no fallback to wildcard rows — used by
// PricingFormulaService to check whether a slot is already taken before
// creating a new formula in it.
func (r *PricingFormulaRepository) FindBySlot(ctx context.Context, brandID *int64, connectionID *int64, priceListID *int64) (*PricingFormulaDTO, error) {
	query := `
		SELECT id, brand_id, connection_id, price_list_id, expression, description,
		       created_by, updated_by, created_at, updated_at
		FROM ecom_pricing_formulas
		WHERE brand_id <=> ? AND connection_id <=> ? AND price_list_id <=> ? AND deleted_at IS NULL
		LIMIT 1
	`

	f, err := scanPricingFormula(r.db.QueryRowContext(ctx, query, brandID, connectionID, priceListID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPricingFormulaNotFound
		}
		return nil, err
	}

	return &f, nil
}

func (r *PricingFormulaRepository) Create(ctx context.Context, input CreatePricingFormulaInput) (*PricingFormulaDTO, error) {
	query := `
		INSERT INTO ecom_pricing_formulas (brand_id, connection_id, price_list_id, expression, description, created_by)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, input.BrandID, input.ConnectionID, input.PriceListID, input.Expression, input.Description, input.CreatedBy)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, id)
}

func (r *PricingFormulaRepository) Update(ctx context.Context, id int64, input UpdatePricingFormulaInput) (*PricingFormulaDTO, error) {
	query := `
		UPDATE ecom_pricing_formulas
		SET expression = ?, description = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.Expression, input.Description, input.UpdatedBy, id)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrPricingFormulaNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *PricingFormulaRepository) SoftDelete(ctx context.Context, id int64) error {
	query := "UPDATE ecom_pricing_formulas SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrPricingFormulaNotFound
	}

	return nil
}

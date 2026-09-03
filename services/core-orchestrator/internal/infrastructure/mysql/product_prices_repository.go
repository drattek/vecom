package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrProductPriceNotFound = errors.New("product price not found")
)

type ProductPriceDTO struct {
	ID          int64     `json:"id"`
	ProductID   int64     `json:"productId"`
	PriceListID int64     `json:"priceListId"`
	Price       string    `json:"price"`
	Currency    int64     `json:"currency"`
	Margin      string    `json:"margin"`
	TaxIncluded bool      `json:"taxIncluded"`
	UpdatedBy   int64     `json:"updatedBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type PaginatedProductPrices struct {
	Data  []ProductPriceDTO `json:"data"`
	Total int               `json:"total"`
}

type CreateProductPriceInput struct {
	ProductID   int64
	PriceListID int64
	Price       string
	Currency    int64
	Margin      string
	TaxIncluded bool
	UpdatedBy   int64
}

type UpdateProductPriceInput struct {
	Price       string
	Margin      string
	TaxIncluded bool
	UpdatedBy   int64
}

type ProductPricesRepository struct {
	db Querier
}

func NewProductPricesRepository(db Querier) *ProductPricesRepository {
	return &ProductPricesRepository{db: db}
}

func (r *ProductPricesRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedProductPrices, error) {
	query := `
		SELECT id, product_id, price_list_id, price, currency, margin, 
		       tax_included, updated_by, created_at, updated_at
		FROM ecom_product_prices
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prices []ProductPriceDTO
	for rows.Next() {
		var p ProductPriceDTO
		if err := rows.Scan(&p.ID, &p.ProductID, &p.PriceListID, &p.Price, &p.Currency,
			&p.Margin, &p.TaxIncluded, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		prices = append(prices, p)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_product_prices"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedProductPrices{Data: prices, Total: total}, nil
}

func (r *ProductPricesRepository) FindByID(ctx context.Context, id int64) (*ProductPriceDTO, error) {
	query := `
		SELECT id, product_id, price_list_id, price, currency, margin, 
		       tax_included, updated_by, created_at, updated_at
		FROM ecom_product_prices
		WHERE id = ?
	`

	var p ProductPriceDTO
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.ProductID, &p.PriceListID, &p.Price,
		&p.Currency, &p.Margin, &p.TaxIncluded, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProductPriceNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *ProductPricesRepository) FindByProductID(ctx context.Context, productID int64) ([]ProductPriceDTO, error) {
	query := `
		SELECT id, product_id, price_list_id, price, currency, margin, 
		       tax_included, updated_by, created_at, updated_at
		FROM ecom_product_prices
		WHERE product_id = ?
	`

	rows, err := r.db.QueryContext(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prices []ProductPriceDTO
	for rows.Next() {
		var p ProductPriceDTO
		if err := rows.Scan(&p.ID, &p.ProductID, &p.PriceListID, &p.Price, &p.Currency,
			&p.Margin, &p.TaxIncluded, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		prices = append(prices, p)
	}

	return prices, nil
}

func (r *ProductPricesRepository) FindByProductAndPriceList(ctx context.Context, productID, priceListID int64) (*ProductPriceDTO, error) {
	query := `
		SELECT id, product_id, price_list_id, price, currency, margin,
		       tax_included, updated_by, created_at, updated_at
		FROM ecom_product_prices
		WHERE product_id = ? AND price_list_id = ?
		LIMIT 1
	`

	var p ProductPriceDTO
	if err := r.db.QueryRowContext(ctx, query, productID, priceListID).Scan(&p.ID, &p.ProductID, &p.PriceListID, &p.Price,
		&p.Currency, &p.Margin, &p.TaxIncluded, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProductPriceNotFound
		}
		return nil, err
	}

	return &p, nil
}

// FindEffectivePrice resolves the price a product should be sold at, across
// every currency: among every active, currently-valid price list carrying a
// price for the product (regardless of that list's currency), it picks the
// one from the highest-priority price list (ecom_price_list.priority). The
// returned row's Currency may not be the caller's target currency — callers
// needing a specific currency (e.g. MXN for marketplace sync) must convert
// via ExchangeRatesRepository themselves; see
// application/pricing.EffectivePriceResolver, the only intended caller for
// that use case.
func (r *ProductPricesRepository) FindEffectivePrice(ctx context.Context, productID int64) (*ProductPriceDTO, error) {
	query := `
		SELECT pp.id, pp.product_id, pp.price_list_id, pp.price, pp.currency, pp.margin,
		       pp.tax_included, pp.updated_by, pp.created_at, pp.updated_at
		FROM ecom_product_prices pp
		INNER JOIN ecom_price_list pl ON pl.id = pp.price_list_id AND pl.deleted_at IS NULL
		WHERE pp.product_id = ?
		  AND pl.status = 'active' AND CURDATE() BETWEEN pl.valid_from AND pl.valid_to
		ORDER BY pl.priority DESC, pp.updated_at DESC
		LIMIT 1
	`

	var p ProductPriceDTO
	if err := r.db.QueryRowContext(ctx, query, productID).Scan(&p.ID, &p.ProductID, &p.PriceListID, &p.Price,
		&p.Currency, &p.Margin, &p.TaxIncluded, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProductPriceNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *ProductPricesRepository) Create(ctx context.Context, input CreateProductPriceInput) (*ProductPriceDTO, error) {
	query := `
		INSERT INTO ecom_product_prices (product_id, price_list_id, price, currency, margin, tax_included, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, input.ProductID, input.PriceListID, input.Price,
		input.Currency, input.Margin, input.TaxIncluded, input.UpdatedBy)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, id)
}

func (r *ProductPricesRepository) Update(ctx context.Context, id int64, input UpdateProductPriceInput) (*ProductPriceDTO, error) {
	query := `
		UPDATE ecom_product_prices
		SET price = ?, margin = ?, tax_included = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, input.Price, input.Margin, input.TaxIncluded, input.UpdatedBy, id)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrProductPriceNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *ProductPricesRepository) Delete(ctx context.Context, id int64) error {
	query := "DELETE FROM ecom_product_prices WHERE id = ?"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrProductPriceNotFound
	}

	return nil
}

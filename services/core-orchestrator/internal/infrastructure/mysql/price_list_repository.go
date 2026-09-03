package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrPriceListNotFound = errors.New("price list not found")
)

type PriceListDTO struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Currency  int64     `json:"currency"`
	Priority  int       `json:"priority"`
	Status    string    `json:"status"`
	ValidFrom string    `json:"validFrom"`
	ValidTo   string    `json:"validTo"`
	CreatedBy int64     `json:"createdBy"`
	UpdatedBy *int64    `json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PaginatedPriceLists struct {
	Data  []PriceListDTO `json:"data"`
	Total int            `json:"total"`
}

type CreatePriceListInput struct {
	Name      string
	Currency  int64
	Priority  int
	Status    string
	ValidFrom string
	ValidTo   string
	CreatedBy int64
}

type UpdatePriceListInput struct {
	Name      string
	Currency  int64
	Priority  int
	Status    string
	ValidFrom string
	ValidTo   string
	UpdatedBy int64
}

type PriceListRepository struct {
	db Querier
}

func NewPriceListRepository(db Querier) *PriceListRepository {
	return &PriceListRepository{db: db}
}

const priceListDateFormat = "2006-01-02"

// scanPriceList reads valid_from/valid_to into time.Time first: with parseTime=true in
// the DSN, the driver returns DATE columns as time.Time, and database/sql's default
// conversion from time.Time into a *string destination formats it as RFC3339
// ("2026-07-30T00:00:00Z"), not "2026-07-30" — which MySQL then rejects if that string
// is ever written back into a DATE column. Scanning into time.Time and formatting
// explicitly avoids that trap.
func scanPriceList(s interface{ Scan(dest ...any) error }) (PriceListDTO, error) {
	var p PriceListDTO
	var validFrom, validTo time.Time

	err := s.Scan(&p.ID, &p.Name, &p.Currency, &p.Priority, &p.Status,
		&validFrom, &validTo, &p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return PriceListDTO{}, err
	}

	p.ValidFrom = validFrom.Format(priceListDateFormat)
	p.ValidTo = validTo.Format(priceListDateFormat)

	return p, nil
}

func (r *PriceListRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedPriceLists, error) {
	query := `
		SELECT id, name, currency, priority, status, valid_from, valid_to, 
		       created_by, updated_by, created_at, updated_at
		FROM ecom_price_list
		WHERE deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var priceLists []PriceListDTO
	for rows.Next() {
		p, err := scanPriceList(rows)
		if err != nil {
			return nil, err
		}
		priceLists = append(priceLists, p)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_price_list WHERE deleted_at IS NULL"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedPriceLists{Data: priceLists, Total: total}, nil
}

func (r *PriceListRepository) FindByID(ctx context.Context, id int64) (*PriceListDTO, error) {
	query := `
		SELECT id, name, currency, priority, status, valid_from, valid_to, 
		       created_by, updated_by, created_at, updated_at
		FROM ecom_price_list
		WHERE id = ? AND deleted_at IS NULL
	`

	p, err := scanPriceList(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPriceListNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *PriceListRepository) FindByName(ctx context.Context, name string) (*PriceListDTO, error) {
	query := `
		SELECT id, name, currency, priority, status, valid_from, valid_to,
		       created_by, updated_by, created_at, updated_at
		FROM ecom_price_list
		WHERE name = ? AND deleted_at IS NULL
		LIMIT 1
	`

	p, err := scanPriceList(r.db.QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPriceListNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *PriceListRepository) Create(ctx context.Context, input CreatePriceListInput) (*PriceListDTO, error) {
	query := `
		INSERT INTO ecom_price_list (name, currency, priority, status, valid_from, valid_to, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, input.Name, input.Currency, input.Priority, input.Status,
		input.ValidFrom, input.ValidTo, input.CreatedBy)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, id)
}

func (r *PriceListRepository) Update(ctx context.Context, id int64, input UpdatePriceListInput) (*PriceListDTO, error) {
	query := `
		UPDATE ecom_price_list
		SET name = ?, currency = ?, priority = ?, status = ?, 
		    valid_from = ?, valid_to = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.Name, input.Currency, input.Priority, input.Status,
		input.ValidFrom, input.ValidTo, input.UpdatedBy, id)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrPriceListNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *PriceListRepository) SoftDelete(ctx context.Context, id int64) error {
	query := "UPDATE ecom_price_list SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrPriceListNotFound
	}

	return nil
}

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrExchangeRateNotFound = errors.New("exchange rate not found")
)

type ExchangeRateDTO struct {
	ID             int64     `json:"id"`
	FromCurrencyID int64     `json:"fromCurrencyId"`
	ToCurrencyID   int64     `json:"toCurrencyId"`
	Rate           string    `json:"rate"`
	UpdatedBy      int64     `json:"updatedBy"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type PaginatedExchangeRates struct {
	Data  []ExchangeRateDTO `json:"data"`
	Total int               `json:"total"`
}

type CreateExchangeRateInput struct {
	FromCurrencyID int64
	ToCurrencyID   int64
	Rate           string
	UpdatedBy      int64
}

type UpdateExchangeRateInput struct {
	Rate      string
	UpdatedBy int64
}

type ExchangeRatesRepository struct {
	db Querier
}

func NewExchangeRatesRepository(db Querier) *ExchangeRatesRepository {
	return &ExchangeRatesRepository{db: db}
}

func (r *ExchangeRatesRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedExchangeRates, error) {
	query := `
		SELECT id, from_currency_id, to_currency_id, rate, updated_by, created_at, updated_at
		FROM ecom_exchange_rates
		WHERE deleted_at IS NULL
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rates []ExchangeRateDTO
	for rows.Next() {
		var e ExchangeRateDTO
		if err := rows.Scan(&e.ID, &e.FromCurrencyID, &e.ToCurrencyID, &e.Rate, &e.UpdatedBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		rates = append(rates, e)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_exchange_rates WHERE deleted_at IS NULL"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedExchangeRates{Data: rates, Total: total}, nil
}

func (r *ExchangeRatesRepository) FindByID(ctx context.Context, id int64) (*ExchangeRateDTO, error) {
	query := `
		SELECT id, from_currency_id, to_currency_id, rate, updated_by, created_at, updated_at
		FROM ecom_exchange_rates
		WHERE id = ? AND deleted_at IS NULL
	`

	var e ExchangeRateDTO
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&e.ID, &e.FromCurrencyID, &e.ToCurrencyID, &e.Rate, &e.UpdatedBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrExchangeRateNotFound
		}
		return nil, err
	}

	return &e, nil
}

// FindByCurrencies returns the active rate to convert an amount in
// fromCurrencyID into toCurrencyID (multiply by Rate) — used by
// application/pricing.EffectivePriceResolver when a product's effective
// price list isn't already in the target currency.
func (r *ExchangeRatesRepository) FindByCurrencies(ctx context.Context, fromCurrencyID, toCurrencyID int64) (*ExchangeRateDTO, error) {
	query := `
		SELECT id, from_currency_id, to_currency_id, rate, updated_by, created_at, updated_at
		FROM ecom_exchange_rates
		WHERE from_currency_id = ? AND to_currency_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	var e ExchangeRateDTO
	if err := r.db.QueryRowContext(ctx, query, fromCurrencyID, toCurrencyID).Scan(&e.ID, &e.FromCurrencyID, &e.ToCurrencyID, &e.Rate, &e.UpdatedBy, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrExchangeRateNotFound
		}
		return nil, err
	}

	return &e, nil
}

func (r *ExchangeRatesRepository) Create(ctx context.Context, input CreateExchangeRateInput) (*ExchangeRateDTO, error) {
	query := `
		INSERT INTO ecom_exchange_rates (from_currency_id, to_currency_id, rate, updated_by)
		VALUES (?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, input.FromCurrencyID, input.ToCurrencyID, input.Rate, input.UpdatedBy)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, id)
}

func (r *ExchangeRatesRepository) Update(ctx context.Context, id int64, input UpdateExchangeRateInput) (*ExchangeRateDTO, error) {
	query := `
		UPDATE ecom_exchange_rates
		SET rate = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.Rate, input.UpdatedBy, id)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrExchangeRateNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *ExchangeRatesRepository) SoftDelete(ctx context.Context, id int64) error {
	query := "UPDATE ecom_exchange_rates SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrExchangeRateNotFound
	}

	return nil
}

package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrCurrencyNotFound = errors.New("currency not found")
var ErrCurrencyCodeAlreadyExists = errors.New("currency code already exists")

type CurrencyDTO struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Code          string     `json:"code"`
	Symbol        string     `json:"symbol"`
	DecimalPlaces int        `json:"decimalPlaces"`
	CreatedBy     int64      `json:"createdBy"`
	UpdatedBy     *int64     `json:"updatedBy,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedCurrencies struct {
	Total      int64         `json:"total"`
	Offset     int           `json:"offset"`
	PageSize   int           `json:"pageSize"`
	Currencies []CurrencyDTO `json:"currencies"`
}

type CreateCurrencyInput struct {
	Name          string
	Code          string
	Symbol        string
	DecimalPlaces int
	CreatedBy     int64
}

type UpdateCurrencyInput struct {
	Name          string
	Code          string
	Symbol        string
	DecimalPlaces int
	UpdatedBy     int64
}

type CurrenciesRepository struct {
	db Querier
}

func NewCurrenciesRepository(db Querier) *CurrenciesRepository {
	return &CurrenciesRepository{db: db}
}

func (r *CurrenciesRepository) FindPaginated(offset, pageSize int) (*PaginatedCurrencies, error) {
	var total int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM ecom_currencies WHERE deleted_at IS NULL").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting currencies: %w", err)
	}

	query := `
		SELECT id, name, code, symbol, decimal_places, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_currencies
		WHERE deleted_at IS NULL
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying currencies: %w", err)
	}
	defer rows.Close()

	currencies := make([]CurrencyDTO, 0)
	for rows.Next() {
		curr, scanErr := scanCurrency(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		currencies = append(currencies, curr)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating currencies: %w", err)
	}

	return &PaginatedCurrencies{
		Total:      total,
		Offset:     offset,
		PageSize:   pageSize,
		Currencies: currencies,
	}, nil
}

func (r *CurrenciesRepository) FindByID(id int64) (*CurrencyDTO, error) {
	query := `
		SELECT id, name, code, symbol, decimal_places, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_currencies
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, id)
	return scanCurrencyRow(row)
}

func (r *CurrenciesRepository) FindByCode(code string) (*CurrencyDTO, error) {
	query := `
		SELECT id, name, code, symbol, decimal_places, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_currencies
		WHERE code = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, code)
	return scanCurrencyRow(row)
}

func (r *CurrenciesRepository) Create(input CreateCurrencyInput) (*CurrencyDTO, error) {
	query := `
		INSERT INTO ecom_currencies (name, code, symbol, decimal_places, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.Exec(query, input.Name, input.Code, input.Symbol, input.DecimalPlaces, input.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("error creating currency: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(id)
}

func (r *CurrenciesRepository) Update(id int64, input UpdateCurrencyInput) (*CurrencyDTO, error) {
	query := `
		UPDATE ecom_currencies
		SET name = ?, code = ?, symbol = ?, decimal_places = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, input.Name, input.Code, input.Symbol, input.DecimalPlaces, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating currency: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return nil, ErrCurrencyNotFound
	}

	return r.FindByID(id)
}

func (r *CurrenciesRepository) SoftDelete(id int64) error {
	query := `
		UPDATE ecom_currencies
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error deleting currency: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrCurrencyNotFound
	}

	return nil
}

func scanCurrency(rows *sql.Rows) (CurrencyDTO, error) {
	var curr CurrencyDTO
	err := rows.Scan(
		&curr.ID,
		&curr.Name,
		&curr.Code,
		&curr.Symbol,
		&curr.DecimalPlaces,
		&curr.CreatedBy,
		&curr.UpdatedBy,
		&curr.CreatedAt,
		&curr.UpdatedAt,
		&curr.DeletedAt,
	)
	if err != nil {
		return CurrencyDTO{}, fmt.Errorf("error scanning currency: %w", err)
	}
	return curr, nil
}

func scanCurrencyRow(row *sql.Row) (*CurrencyDTO, error) {
	var curr CurrencyDTO
	err := row.Scan(
		&curr.ID,
		&curr.Name,
		&curr.Code,
		&curr.Symbol,
		&curr.DecimalPlaces,
		&curr.CreatedBy,
		&curr.UpdatedBy,
		&curr.CreatedAt,
		&curr.UpdatedAt,
		&curr.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCurrencyNotFound
		}
		return nil, fmt.Errorf("error scanning currency: %w", err)
	}
	return &curr, nil
}

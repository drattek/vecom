package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrPriceHistoryNotFound = errors.New("price history not found")
)

type PriceHistoryDTO struct {
	ID          int64     `json:"id"`
	ProductID   int64     `json:"productId"`
	PriceListID int64     `json:"priceListId"`
	CurrencyID  int64     `json:"currencyId"`
	OldPrice    string    `json:"oldPrice"`
	NewPrice    string    `json:"newPrice"`
	UpdatedBy   int64     `json:"updatedBy"`
	CreatedAt   time.Time `json:"createdAt"`
}

type PaginatedPriceHistory struct {
	Data  []PriceHistoryDTO `json:"data"`
	Total int               `json:"total"`
}

type CreatePriceHistoryInput struct {
	ProductID   int64
	PriceListID int64
	CurrencyID  int64
	OldPrice    string
	NewPrice    string
	UpdatedBy   int64
}

type PriceHistoryRepository struct {
	db Querier
}

func NewPriceHistoryRepository(db Querier) *PriceHistoryRepository {
	return &PriceHistoryRepository{db: db}
}

func (r *PriceHistoryRepository) FindByID(ctx context.Context, id int64) (*PriceHistoryDTO, error) {
	query := `
		SELECT id, product_id, price_list_id, currency_id, old_price, new_price, updated_by, created_at
		FROM ecom_price_history
		WHERE id = ?
	`

	var p PriceHistoryDTO
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.ProductID, &p.PriceListID, &p.CurrencyID,
		&p.OldPrice, &p.NewPrice, &p.UpdatedBy, &p.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPriceHistoryNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *PriceHistoryRepository) FindByProductID(ctx context.Context, productID int64, offset, pageSize int) (*PaginatedPriceHistory, error) {
	query := `
		SELECT id, product_id, price_list_id, currency_id, old_price, new_price, updated_by, created_at
		FROM ecom_price_history
		WHERE product_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, productID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []PriceHistoryDTO
	for rows.Next() {
		var p PriceHistoryDTO
		if err := rows.Scan(&p.ID, &p.ProductID, &p.PriceListID, &p.CurrencyID,
			&p.OldPrice, &p.NewPrice, &p.UpdatedBy, &p.CreatedAt); err != nil {
			return nil, err
		}
		history = append(history, p)
	}

	countQuery := "SELECT COUNT(*) FROM ecom_price_history WHERE product_id = ?"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, productID).Scan(&total); err != nil {
		return nil, err
	}

	return &PaginatedPriceHistory{Data: history, Total: total}, nil
}

func (r *PriceHistoryRepository) Create(ctx context.Context, input CreatePriceHistoryInput) (*PriceHistoryDTO, error) {
	query := `
		INSERT INTO ecom_price_history (product_id, price_list_id, currency_id, old_price, new_price, updated_by)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, input.ProductID, input.PriceListID, input.CurrencyID,
		input.OldPrice, input.NewPrice, input.UpdatedBy)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, id)
}

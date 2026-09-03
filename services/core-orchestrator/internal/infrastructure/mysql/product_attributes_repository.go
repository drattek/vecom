package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrProductAttributeNotFound = errors.New("product attribute not found")

type ProductAttributeDTO struct {
	ID          int64      `json:"id"`
	ProductID   int64      `json:"productId"`
	AttributeID int64      `json:"attributeId"`
	ValueText   *string    `json:"valueText,omitempty"`
	ValueNumber *float64   `json:"valueNumber,omitempty"`
	ValueBool   *bool      `json:"valueBoolean,omitempty"`
	ValueDate   *time.Time `json:"valueDate,omitempty"`
	OptionID    *int64     `json:"optionId,omitempty"`
	CreatedBy   int64      `json:"createdBy"`
	UpdatedBy   *int64     `json:"updatedBy,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
}

// UpsertProductAttributeInput sets the value of one attribute on one
// product. ecom_product_attributes has a unique key on (product_id,
// attribute_id) — only one of ValueText/ValueNumber/ValueBool/ValueDate/
// OptionID is expected to be set, matching the attribute's data_type; which
// one is the caller's (service layer) responsibility to enforce.
type UpsertProductAttributeInput struct {
	ProductID   int64
	AttributeID int64
	ValueText   *string
	ValueNumber *float64
	ValueBool   *bool
	ValueDate   *time.Time
	OptionID    *int64
	ActorID     int64
}

type ProductAttributesRepository struct {
	db Querier
}

func NewProductAttributesRepository(db Querier) *ProductAttributesRepository {
	return &ProductAttributesRepository{db: db}
}

func (r *ProductAttributesRepository) FindByProductID(ctx context.Context, productID int64) ([]ProductAttributeDTO, error) {
	query := `
		SELECT id, product_id, attribute_id, value_text, value_number, value_boolean, value_date, option_id,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_product_attributes
		WHERE product_id = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("error querying product attributes: %w", err)
	}
	defer rows.Close()

	attributes := make([]ProductAttributeDTO, 0)
	for rows.Next() {
		attribute, scanErr := scanProductAttribute(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		attributes = append(attributes, attribute)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating product attributes: %w", err)
	}

	return attributes, nil
}

func (r *ProductAttributesRepository) FindByProductAndAttribute(ctx context.Context, productID, attributeID int64) (*ProductAttributeDTO, error) {
	query := `
		SELECT id, product_id, attribute_id, value_text, value_number, value_boolean, value_date, option_id,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_product_attributes
		WHERE product_id = ? AND attribute_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanProductAttributeRow(r.db.QueryRowContext(ctx, query, productID, attributeID))
}

func (r *ProductAttributesRepository) findByID(ctx context.Context, id int64) (*ProductAttributeDTO, error) {
	query := `
		SELECT id, product_id, attribute_id, value_text, value_number, value_boolean, value_date, option_id,
		       created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_product_attributes
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	return scanProductAttributeRow(r.db.QueryRowContext(ctx, query, id))
}

// Upsert creates the (product, attribute) value row on first write, or
// replaces its value on subsequent writes — mirrors
// ChannelCategoryMapRepository.Upsert.
func (r *ProductAttributesRepository) Upsert(ctx context.Context, input UpsertProductAttributeInput) (*ProductAttributeDTO, error) {
	existing, err := r.FindByProductAndAttribute(ctx, input.ProductID, input.AttributeID)
	if err != nil && !errors.Is(err, ErrProductAttributeNotFound) {
		return nil, fmt.Errorf("error loading product attribute: %w", err)
	}

	if existing == nil {
		query := `
			INSERT INTO ecom_product_attributes
				(product_id, attribute_id, value_text, value_number, value_boolean, value_date, option_id, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`

		result, err := r.db.ExecContext(ctx, query, input.ProductID, input.AttributeID, input.ValueText, input.ValueNumber, input.ValueBool, input.ValueDate, input.OptionID, input.ActorID)
		if err != nil {
			return nil, fmt.Errorf("error creating product attribute: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("error getting last insert id: %w", err)
		}

		return r.findByID(ctx, id)
	}

	query := `
		UPDATE ecom_product_attributes
		SET value_text = ?, value_number = ?, value_boolean = ?, value_date = ?, option_id = ?,
		    updated_by = ?, updated_at = NOW()
		WHERE id = ?
	`

	if _, err := r.db.ExecContext(ctx, query, input.ValueText, input.ValueNumber, input.ValueBool, input.ValueDate, input.OptionID, input.ActorID, existing.ID); err != nil {
		return nil, fmt.Errorf("error updating product attribute: %w", err)
	}

	return r.findByID(ctx, existing.ID)
}

func (r *ProductAttributesRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `UPDATE ecom_product_attributes SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting product attribute: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}
	if affected == 0 {
		return ErrProductAttributeNotFound
	}

	return nil
}

func scanProductAttribute(rows *sql.Rows) (ProductAttributeDTO, error) {
	var attribute ProductAttributeDTO
	err := rows.Scan(
		&attribute.ID,
		&attribute.ProductID,
		&attribute.AttributeID,
		&attribute.ValueText,
		&attribute.ValueNumber,
		&attribute.ValueBool,
		&attribute.ValueDate,
		&attribute.OptionID,
		&attribute.CreatedBy,
		&attribute.UpdatedBy,
		&attribute.CreatedAt,
		&attribute.UpdatedAt,
		&attribute.DeletedAt,
	)
	if err != nil {
		return ProductAttributeDTO{}, fmt.Errorf("error scanning product attribute: %w", err)
	}
	return attribute, nil
}

func scanProductAttributeRow(row *sql.Row) (*ProductAttributeDTO, error) {
	var attribute ProductAttributeDTO
	err := row.Scan(
		&attribute.ID,
		&attribute.ProductID,
		&attribute.AttributeID,
		&attribute.ValueText,
		&attribute.ValueNumber,
		&attribute.ValueBool,
		&attribute.ValueDate,
		&attribute.OptionID,
		&attribute.CreatedBy,
		&attribute.UpdatedBy,
		&attribute.CreatedAt,
		&attribute.UpdatedAt,
		&attribute.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProductAttributeNotFound
		}
		return nil, fmt.Errorf("error scanning product attribute: %w", err)
	}
	return &attribute, nil
}

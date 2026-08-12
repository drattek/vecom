package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrPendingProductVehicleFitmentAlreadyExists = errors.New("pending product vehicle fitment already exists")
var ErrPendingProductVehicleFitmentInvalidReference = errors.New("pending product vehicle fitment invalid reference")

type PendingProductVehicleFitmentDTO struct {
	ID                int64      `json:"id"`
	SKU               string     `json:"sku"`
	VehicleFitmentID  int64      `json:"vehicleFitmentId"`
	Motor             string     `json:"motor"`
	Position          string     `json:"position"`
	Side              string     `json:"side"`
	ResolvedAt        *time.Time `json:"resolvedAt,omitempty"`
	ResolvedProductID *int64     `json:"resolvedProductId,omitempty"`
	CreatedBy         int64      `json:"createdBy"`
	UpdatedBy         *int64     `json:"updatedBy,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	DeletedAt         *time.Time `json:"deletedAt,omitempty"`
}

type CreatePendingProductVehicleFitmentInput struct {
	SKU              string
	VehicleFitmentID int64
	Motor            string
	Position         string
	Side             string
	CreatedBy        int64
}

type PendingProductVehicleFitmentsRepository struct {
	db *sql.DB
}

func NewPendingProductVehicleFitmentsRepository(db *sql.DB) *PendingProductVehicleFitmentsRepository {
	return &PendingProductVehicleFitmentsRepository{db: db}
}

func (r *PendingProductVehicleFitmentsRepository) Create(input CreatePendingProductVehicleFitmentInput) (*PendingProductVehicleFitmentDTO, error) {
	query := `
		INSERT INTO ecom_pending_product_vehicle_fitments (sku, vehicle_fitment_id, motor, position, side, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.Exec(query, input.SKU, input.VehicleFitmentID, input.Motor, input.Position, input.Side, input.CreatedBy)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrPendingProductVehicleFitmentAlreadyExists
		}
		if isForeignKeyConstraintError(err) {
			return nil, ErrPendingProductVehicleFitmentInvalidReference
		}
		return nil, fmt.Errorf("error creating pending product vehicle fitment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(id)
}

func (r *PendingProductVehicleFitmentsRepository) FindByID(id int64) (*PendingProductVehicleFitmentDTO, error) {
	query := `
		SELECT id, sku, vehicle_fitment_id, motor, position, side, resolved_at, resolved_product_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_pending_product_vehicle_fitments
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, id)
	return scanPendingProductVehicleFitmentRow(row)
}

// FindUnresolvedBySKU returns pending fitments awaiting a product with this
// SKU, so it only matches rows that haven't been resolved or soft-deleted yet.
func (r *PendingProductVehicleFitmentsRepository) FindUnresolvedBySKU(sku string) ([]PendingProductVehicleFitmentDTO, error) {
	query := `
		SELECT id, sku, vehicle_fitment_id, motor, position, side, resolved_at, resolved_product_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_pending_product_vehicle_fitments
		WHERE sku = ? AND resolved_at IS NULL AND deleted_at IS NULL
		ORDER BY id ASC
	`

	rows, err := r.db.Query(query, sku)
	if err != nil {
		return nil, fmt.Errorf("error querying pending product vehicle fitments: %w", err)
	}
	defer rows.Close()

	pending := make([]PendingProductVehicleFitmentDTO, 0)
	for rows.Next() {
		item, scanErr := scanPendingProductVehicleFitment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		pending = append(pending, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending product vehicle fitments: %w", err)
	}

	return pending, nil
}

// FindAllUnresolved returns every pending fitment across all SKUs that
// hasn't been resolved or soft-deleted yet, for batch-resolving against
// products that may have been created since the pending rows were staged.
func (r *PendingProductVehicleFitmentsRepository) FindAllUnresolved() ([]PendingProductVehicleFitmentDTO, error) {
	query := `
		SELECT id, sku, vehicle_fitment_id, motor, position, side, resolved_at, resolved_product_id, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_pending_product_vehicle_fitments
		WHERE resolved_at IS NULL AND deleted_at IS NULL
		ORDER BY sku ASC, id ASC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying pending product vehicle fitments: %w", err)
	}
	defer rows.Close()

	pending := make([]PendingProductVehicleFitmentDTO, 0)
	for rows.Next() {
		item, scanErr := scanPendingProductVehicleFitment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		pending = append(pending, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending product vehicle fitments: %w", err)
	}

	return pending, nil
}

// MarkResolved stamps the pending row with the product that satisfied it,
// keeping it around (rather than deleting it) as an audit trail of when and
// by which product the compatibility ended up being created automatically.
func (r *PendingProductVehicleFitmentsRepository) MarkResolved(id, productID, actorID int64) error {
	query := `
		UPDATE ecom_pending_product_vehicle_fitments
		SET resolved_at = NOW(), resolved_product_id = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	_, err := r.db.Exec(query, productID, actorID, id)
	if err != nil {
		return fmt.Errorf("error marking pending product vehicle fitment resolved: %w", err)
	}

	return nil
}

func scanPendingProductVehicleFitment(rows *sql.Rows) (PendingProductVehicleFitmentDTO, error) {
	var item PendingProductVehicleFitmentDTO
	var resolvedAt sql.NullTime
	var resolvedProductID sql.NullInt64
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := rows.Scan(
		&item.ID,
		&item.SKU,
		&item.VehicleFitmentID,
		&item.Motor,
		&item.Position,
		&item.Side,
		&resolvedAt,
		&resolvedProductID,
		&item.CreatedBy,
		&updatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return PendingProductVehicleFitmentDTO{}, fmt.Errorf("error scanning pending product vehicle fitment: %w", err)
	}

	if resolvedAt.Valid {
		item.ResolvedAt = &resolvedAt.Time
	}
	if resolvedProductID.Valid {
		item.ResolvedProductID = &resolvedProductID.Int64
	}
	if updatedBy.Valid {
		item.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}

	return item, nil
}

func scanPendingProductVehicleFitmentRow(row *sql.Row) (*PendingProductVehicleFitmentDTO, error) {
	var item PendingProductVehicleFitmentDTO
	var resolvedAt sql.NullTime
	var resolvedProductID sql.NullInt64
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := row.Scan(
		&item.ID,
		&item.SKU,
		&item.VehicleFitmentID,
		&item.Motor,
		&item.Position,
		&item.Side,
		&resolvedAt,
		&resolvedProductID,
		&item.CreatedBy,
		&updatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("pending product vehicle fitment not found: %w", err)
		}
		return nil, fmt.Errorf("error scanning pending product vehicle fitment: %w", err)
	}

	if resolvedAt.Valid {
		item.ResolvedAt = &resolvedAt.Time
	}
	if resolvedProductID.Valid {
		item.ResolvedProductID = &resolvedProductID.Int64
	}
	if updatedBy.Valid {
		item.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}

	return &item, nil
}

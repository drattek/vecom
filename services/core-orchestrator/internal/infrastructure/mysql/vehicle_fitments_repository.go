package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrVehicleFitmentNotFound = errors.New("vehicle fitment not found")
var ErrVehicleFitmentAlreadyExists = errors.New("vehicle fitment already exists")
var ErrVehicleFitmentInvalidReference = errors.New("vehicle fitment invalid reference")

type VehicleFitmentDTO struct {
	ID        int64  `json:"id"`
	BrandID   int64  `json:"brandId"`
	BrandName string `json:"brandName"`
	Model     string `json:"model"`
	YearStart int    `json:"yearStart"`
	YearEnd   *int   `json:"yearEnd,omitempty"`
	// ProductCount is how many products are currently linked to this fitment
	// (live rows of ecom_product_vehicle_compatibility).
	ProductCount int64      `json:"productCount"`
	CreatedBy    int64      `json:"createdBy"`
	UpdatedBy    *int64     `json:"updatedBy,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	DeletedAt    *time.Time `json:"deletedAt,omitempty"`
}

type PaginatedVehicleFitments struct {
	Total    int64               `json:"total"`
	Offset   int                 `json:"offset"`
	PageSize int                 `json:"pageSize"`
	Fitments []VehicleFitmentDTO `json:"fitments"`
}

// VehicleFitmentFilter narrows FindPaginated. Query matches brand name, model
// or — when numeric — any fitment whose year range contains that year.
type VehicleFitmentFilter struct {
	Query   string
	BrandID int64
}

type CreateVehicleFitmentInput struct {
	BrandID   int64
	Model     string
	YearStart int
	YearEnd   *int
	CreatedBy int64
}

type UpdateVehicleFitmentInput struct {
	BrandID   int64
	Model     string
	YearStart int
	YearEnd   *int
	UpdatedBy int64
}

type VehicleFitmentsRepository struct {
	db Querier
}

func NewVehicleFitmentsRepository(db Querier) *VehicleFitmentsRepository {
	return &VehicleFitmentsRepository{db: db}
}

const vehicleFitmentSelect = `
	SELECT vf.id, vf.brand_id, COALESCE(b.name, ''), vf.model, vf.year_start, vf.year_end,
	       (SELECT COUNT(*) FROM ecom_product_vehicle_compatibility pvc
	         WHERE pvc.vehicle_fitment_id = vf.id AND pvc.deleted_at IS NULL),
	       vf.created_by, vf.updated_by, vf.created_at, vf.updated_at, vf.deleted_at
	FROM ecom_vehicle_fitments vf
	LEFT JOIN ecom_brands b ON b.id = vf.brand_id
`

// likePattern wraps q for a LIKE '%q%' match, escaping the wildcards so a
// search for "50%" doesn't match everything.
func likePattern(q string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(q) + "%"
}

func (r *VehicleFitmentsRepository) FindPaginated(ctx context.Context, offset, pageSize int) (*PaginatedVehicleFitments, error) {
	return r.Search(ctx, VehicleFitmentFilter{}, offset, pageSize)
}

func (r *VehicleFitmentsRepository) Search(ctx context.Context, filter VehicleFitmentFilter, offset, pageSize int) (*PaginatedVehicleFitments, error) {
	where := []string{"vf.deleted_at IS NULL"}
	args := []any{}

	if filter.BrandID > 0 {
		where = append(where, "vf.brand_id = ?")
		args = append(args, filter.BrandID)
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		clause := "(vf.model LIKE ? OR b.name LIKE ?"
		pattern := likePattern(q)
		args = append(args, pattern, pattern)
		if year, err := strconv.Atoi(q); err == nil && year > 0 {
			clause += " OR (vf.year_start <= ? AND COALESCE(vf.year_end, vf.year_start) >= ?)"
			args = append(args, year, year)
		}
		where = append(where, clause+")")
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM ecom_vehicle_fitments vf LEFT JOIN ecom_brands b ON b.id = vf.brand_id WHERE "+whereSQL,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting vehicle fitments: %w", err)
	}

	query := vehicleFitmentSelect + " WHERE " + whereSQL + " ORDER BY b.name ASC, vf.model ASC, vf.year_start ASC, vf.id ASC LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, query, append(args, pageSize, offset)...)
	if err != nil {
		return nil, fmt.Errorf("error querying vehicle fitments: %w", err)
	}
	defer rows.Close()

	fitments := make([]VehicleFitmentDTO, 0)
	for rows.Next() {
		fitment, scanErr := scanVehicleFitment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		fitments = append(fitments, fitment)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating vehicle fitments: %w", err)
	}

	return &PaginatedVehicleFitments{
		Total:    total,
		Offset:   offset,
		PageSize: pageSize,
		Fitments: fitments,
	}, nil
}

func (r *VehicleFitmentsRepository) FindByID(ctx context.Context, id int64) (*VehicleFitmentDTO, error) {
	row := r.db.QueryRowContext(ctx, vehicleFitmentSelect+" WHERE vf.id = ? AND vf.deleted_at IS NULL LIMIT 1", id)
	return scanVehicleFitmentRow(row)
}

func (r *VehicleFitmentsRepository) FindByUniqueKey(ctx context.Context, brandID int64, model string, yearStart int, yearEnd *int) (*VehicleFitmentDTO, error) {
	query := vehicleFitmentSelect + `
		WHERE vf.brand_id = ? AND vf.model = ? AND vf.year_start = ? AND vf.year_end <=> ? AND vf.deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, brandID, model, yearStart, yearEnd)
	return scanVehicleFitmentRow(row)
}

func (r *VehicleFitmentsRepository) Create(ctx context.Context, input CreateVehicleFitmentInput) (*VehicleFitmentDTO, error) {
	query := `
		INSERT INTO ecom_vehicle_fitments (brand_id, model, year_start, year_end, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, input.BrandID, input.Model, input.YearStart, input.YearEnd, input.CreatedBy)
	if err != nil {
		if isDuplicateKeyError(err) {
			// uq_vehicle_fitment also covers soft-deleted rows: if the colliding
			// row was deleted, bring it back instead of reporting a duplicate the
			// user can't even see.
			if revived, reviveErr := r.reviveDeleted(ctx, input); reviveErr != nil {
				return nil, reviveErr
			} else if revived != nil {
				return revived, nil
			}
			return nil, ErrVehicleFitmentAlreadyExists
		}
		if isForeignKeyConstraintError(err) {
			return nil, ErrVehicleFitmentInvalidReference
		}
		return nil, fmt.Errorf("error creating vehicle fitment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *VehicleFitmentsRepository) reviveDeleted(ctx context.Context, input CreateVehicleFitmentInput) (*VehicleFitmentDTO, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		SELECT id FROM ecom_vehicle_fitments
		WHERE brand_id = ? AND model = ? AND year_start = ? AND year_end <=> ? AND deleted_at IS NOT NULL
		LIMIT 1
	`, input.BrandID, input.Model, input.YearStart, input.YearEnd).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error looking up deleted vehicle fitment: %w", err)
	}

	if _, err := r.db.ExecContext(ctx,
		"UPDATE ecom_vehicle_fitments SET deleted_at = NULL, updated_by = ?, updated_at = NOW() WHERE id = ?",
		input.CreatedBy, id,
	); err != nil {
		return nil, fmt.Errorf("error restoring vehicle fitment: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *VehicleFitmentsRepository) Update(ctx context.Context, id int64, input UpdateVehicleFitmentInput) (*VehicleFitmentDTO, error) {
	query := `
		UPDATE ecom_vehicle_fitments
		SET brand_id = ?, model = ?, year_start = ?, year_end = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.BrandID, input.Model, input.YearStart, input.YearEnd, input.UpdatedBy, id)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrVehicleFitmentAlreadyExists
		}
		if isForeignKeyConstraintError(err) {
			return nil, ErrVehicleFitmentInvalidReference
		}
		return nil, fmt.Errorf("error updating vehicle fitment: %w", err)
	}

	if _, err := result.RowsAffected(); err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}

	// MySQL reports 0 affected rows when the new values equal the old ones, so
	// FindByID (ErrVehicleFitmentNotFound when absent) decides not-found.
	return r.FindByID(ctx, id)
}

func (r *VehicleFitmentsRepository) SoftDelete(ctx context.Context, id int64) error {
	query := `
		UPDATE ecom_vehicle_fitments
		SET deleted_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting vehicle fitment: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if affected == 0 {
		return ErrVehicleFitmentNotFound
	}

	return nil
}

// SoftDeleteLinks soft-deletes every live product link and every unresolved
// pending link of the fitment, so deleting the fitment doesn't leave dangling
// rows that later resurface (e.g. the pending resolver re-linking SKUs to a
// deleted fitment). Returns how many product links were removed.
func (r *VehicleFitmentsRepository) SoftDeleteLinks(ctx context.Context, fitmentID, actorID int64) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE ecom_product_vehicle_compatibility
		SET deleted_at = NOW(), updated_by = ?
		WHERE vehicle_fitment_id = ? AND deleted_at IS NULL
	`, actorID, fitmentID)
	if err != nil {
		return 0, fmt.Errorf("error deleting product links of vehicle fitment: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error getting rows affected: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, `
		UPDATE ecom_pending_product_vehicle_fitments
		SET deleted_at = NOW(), updated_by = ?
		WHERE vehicle_fitment_id = ? AND deleted_at IS NULL
	`, actorID, fitmentID); err != nil {
		return 0, fmt.Errorf("error deleting pending links of vehicle fitment: %w", err)
	}

	return affected, nil
}

func scanVehicleFitment(rows *sql.Rows) (VehicleFitmentDTO, error) {
	var fitment VehicleFitmentDTO
	var yearEnd sql.NullInt64
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := rows.Scan(
		&fitment.ID,
		&fitment.BrandID,
		&fitment.BrandName,
		&fitment.Model,
		&fitment.YearStart,
		&yearEnd,
		&fitment.ProductCount,
		&fitment.CreatedBy,
		&updatedBy,
		&fitment.CreatedAt,
		&fitment.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return VehicleFitmentDTO{}, fmt.Errorf("error scanning vehicle fitment: %w", err)
	}

	applyVehicleFitmentNullables(&fitment, yearEnd, updatedBy, deletedAt)

	return fitment, nil
}

func scanVehicleFitmentRow(row *sql.Row) (*VehicleFitmentDTO, error) {
	var fitment VehicleFitmentDTO
	var yearEnd sql.NullInt64
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := row.Scan(
		&fitment.ID,
		&fitment.BrandID,
		&fitment.BrandName,
		&fitment.Model,
		&fitment.YearStart,
		&yearEnd,
		&fitment.ProductCount,
		&fitment.CreatedBy,
		&updatedBy,
		&fitment.CreatedAt,
		&fitment.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVehicleFitmentNotFound
		}
		return nil, fmt.Errorf("error scanning vehicle fitment: %w", err)
	}

	applyVehicleFitmentNullables(&fitment, yearEnd, updatedBy, deletedAt)

	return &fitment, nil
}

func applyVehicleFitmentNullables(fitment *VehicleFitmentDTO, yearEnd, updatedBy sql.NullInt64, deletedAt sql.NullTime) {
	if yearEnd.Valid {
		year := int(yearEnd.Int64)
		fitment.YearEnd = &year
	}
	if updatedBy.Valid {
		fitment.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		fitment.DeletedAt = &deletedAt.Time
	}
}

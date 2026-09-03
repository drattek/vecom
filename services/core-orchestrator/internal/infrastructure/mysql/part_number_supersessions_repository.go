package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrPartNumberSupersessionNotFound = errors.New("part number supersession not found")
var ErrPartNumberSupersessionAlreadyExists = errors.New("part number supersession already exists")
var ErrPartNumberSupersessionInvalidReference = errors.New("part number supersession invalid reference")

type PartNumberSupersessionDTO struct {
	ID            int64      `json:"id"`
	SourceID      int64      `json:"sourceId"`
	OldPartNumber string     `json:"oldPartNumber"`
	NewPartNumber string     `json:"newPartNumber"`
	OldProductID  *int64     `json:"oldProductId,omitempty"`
	NewProductID  *int64     `json:"newProductId,omitempty"`
	OldResolvedAt *time.Time `json:"oldResolvedAt,omitempty"`
	NewResolvedAt *time.Time `json:"newResolvedAt,omitempty"`
	CreatedBy     int64      `json:"createdBy"`
	UpdatedBy     *int64     `json:"updatedBy,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
}

type CreatePartNumberSupersessionInput struct {
	SourceID      int64
	OldPartNumber string
	NewPartNumber string
	CreatedBy     int64
}

type UpdatePartNumberSupersessionInput struct {
	NewPartNumber string
	UpdatedBy     int64
}

type PartNumberSupersessionsRepository struct {
	db Querier
}

func NewPartNumberSupersessionsRepository(db Querier) *PartNumberSupersessionsRepository {
	return &PartNumberSupersessionsRepository{db: db}
}

// FindByOldPartNumber busca el estado actual (haya sido resuelto o no) de la sucesión para
// esta pieza dentro de esta fuente. Como ecom_part_number_supersessions guarda estado
// actual (a lo más una fila por source_id+old_part_number, igual que ecom_product_stock o
// ecom_product_prices, no un log de eventos), esto es lo que se usa para decidir si lo que
// llegó del ERP ya está reflejado o si hay que crear/actualizar.
func (r *PartNumberSupersessionsRepository) FindByOldPartNumber(ctx context.Context, sourceID int64, oldPartNumber string) (*PartNumberSupersessionDTO, error) {
	query := `
		SELECT id, source_id, old_part_number, new_part_number, old_product_id, new_product_id,
			old_resolved_at, new_resolved_at, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_part_number_supersessions
		WHERE source_id = ? AND old_part_number = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, sourceID, oldPartNumber)
	return scanPartNumberSupersessionRow(row)
}

func (r *PartNumberSupersessionsRepository) Create(ctx context.Context, input CreatePartNumberSupersessionInput) (*PartNumberSupersessionDTO, error) {
	query := `
		INSERT INTO ecom_part_number_supersessions (source_id, old_part_number, new_part_number, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, input.SourceID, input.OldPartNumber, input.NewPartNumber, input.CreatedBy)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrPartNumberSupersessionAlreadyExists
		}
		if isForeignKeyConstraintError(err) {
			return nil, ErrPartNumberSupersessionInvalidReference
		}
		return nil, fmt.Errorf("error creating part number supersession: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

// Update cambia el número de parte sucesor de una sucesión existente (el ERP corrigió a qué
// pieza apunta PROD_SUPERSESION). old_product_id/old_resolved_at no se tocan porque
// old_part_number no cambió, pero new_product_id/new_resolved_at se limpian: el vínculo
// resuelto anteriormente apuntaba al número de parte viejo, ya no es válido, y
// ResolveForProduct lo volverá a resolver contra el nuevo valor cuando corresponda.
func (r *PartNumberSupersessionsRepository) Update(ctx context.Context, id int64, input UpdatePartNumberSupersessionInput) (*PartNumberSupersessionDTO, error) {
	query := `
		UPDATE ecom_part_number_supersessions
		SET new_part_number = ?, new_product_id = NULL, new_resolved_at = NULL, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, input.NewPartNumber, input.UpdatedBy, id)
	if err != nil {
		return nil, fmt.Errorf("error updating part number supersession: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error getting rows affected: %w", err)
	}
	if affected == 0 {
		return nil, ErrPartNumberSupersessionNotFound
	}

	return r.FindByID(ctx, id)
}

func (r *PartNumberSupersessionsRepository) FindByID(ctx context.Context, id int64) (*PartNumberSupersessionDTO, error) {
	query := `
		SELECT id, source_id, old_part_number, new_part_number, old_product_id, new_product_id,
			old_resolved_at, new_resolved_at, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_part_number_supersessions
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanPartNumberSupersessionRow(row)
}

// FindByNewPartNumber devuelve toda sucesión (resuelta o no) donde este número de parte es
// el lado "nuevo" — es decir, toda pieza que fue reemplazada POR este número de parte. A
// diferencia de FindUnresolvedByNewPartNumber, no filtra por new_product_id IS NULL: se usa
// para recorrer la cadena de sucesión hacia atrás (predecesores), no para resolver FKs.
func (r *PartNumberSupersessionsRepository) FindByNewPartNumber(ctx context.Context, sourceID int64, newPartNumber string) ([]PartNumberSupersessionDTO, error) {
	query := `
		SELECT id, source_id, old_part_number, new_part_number, old_product_id, new_product_id,
			old_resolved_at, new_resolved_at, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_part_number_supersessions
		WHERE source_id = ? AND new_part_number = ? AND deleted_at IS NULL
		ORDER BY id ASC
	`

	return r.queryPartNumberSupersessions(ctx, query, sourceID, newPartNumber)
}

// FindUnresolvedByOldPartNumber devuelve las sucesiones donde este número de parte es el
// lado "viejo" (la pieza reemplazada) y todavía no se vinculó a un ecom_products.id.
func (r *PartNumberSupersessionsRepository) FindUnresolvedByOldPartNumber(ctx context.Context, sourceID int64, partNumber string) ([]PartNumberSupersessionDTO, error) {
	query := `
		SELECT id, source_id, old_part_number, new_part_number, old_product_id, new_product_id,
			old_resolved_at, new_resolved_at, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_part_number_supersessions
		WHERE source_id = ? AND old_part_number = ? AND old_product_id IS NULL AND deleted_at IS NULL
		ORDER BY id ASC
	`

	return r.queryPartNumberSupersessions(ctx, query, sourceID, partNumber)
}

// FindUnresolvedByNewPartNumber devuelve las sucesiones donde este número de parte es el
// lado "nuevo" (la pieza que reemplaza) y todavía no se vinculó a un ecom_products.id.
func (r *PartNumberSupersessionsRepository) FindUnresolvedByNewPartNumber(ctx context.Context, sourceID int64, partNumber string) ([]PartNumberSupersessionDTO, error) {
	query := `
		SELECT id, source_id, old_part_number, new_part_number, old_product_id, new_product_id,
			old_resolved_at, new_resolved_at, created_by, updated_by, created_at, updated_at, deleted_at
		FROM ecom_part_number_supersessions
		WHERE source_id = ? AND new_part_number = ? AND new_product_id IS NULL AND deleted_at IS NULL
		ORDER BY id ASC
	`

	return r.queryPartNumberSupersessions(ctx, query, sourceID, partNumber)
}

func (r *PartNumberSupersessionsRepository) queryPartNumberSupersessions(ctx context.Context, query string, args ...interface{}) ([]PartNumberSupersessionDTO, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying part number supersessions: %w", err)
	}
	defer rows.Close()

	items := make([]PartNumberSupersessionDTO, 0)
	for rows.Next() {
		item, scanErr := scanPartNumberSupersession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating part number supersessions: %w", err)
	}

	return items, nil
}

// MarkOldResolved vincula el lado "viejo" de la sucesión con el producto que se acaba de
// crear/actualizar para ese número de parte. La fila se conserva (no se borra) como registro
// de auditoría de cuándo y con qué producto se resolvió.
func (r *PartNumberSupersessionsRepository) MarkOldResolved(ctx context.Context, id, productID, actorID int64) error {
	query := `
		UPDATE ecom_part_number_supersessions
		SET old_product_id = ?, old_resolved_at = NOW(), updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, productID, actorID, id)
	if err != nil {
		return fmt.Errorf("error marking part number supersession resolved (old side): %w", err)
	}

	return nil
}

// MarkNewResolved vincula el lado "nuevo" de la sucesión con el producto que se acaba de
// crear/actualizar para ese número de parte.
func (r *PartNumberSupersessionsRepository) MarkNewResolved(ctx context.Context, id, productID, actorID int64) error {
	query := `
		UPDATE ecom_part_number_supersessions
		SET new_product_id = ?, new_resolved_at = NOW(), updated_by = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, productID, actorID, id)
	if err != nil {
		return fmt.Errorf("error marking part number supersession resolved (new side): %w", err)
	}

	return nil
}

// ResolveForProduct vincula cualquier sucesión pendiente (lado viejo o lado nuevo) que
// estuviera esperando este producto, identificado por su source_id y part_number. Es segura
// de llamar en cada sync de un producto, no solo al crearlo: para sucesiones ya resueltas
// FindUnresolvedBy* no devuelve nada, así que no hace ningún UPDATE de más.
func (r *PartNumberSupersessionsRepository) ResolveForProduct(ctx context.Context, sourceID, productID int64, partNumber string, actorID int64) error {
	oldSide, err := r.FindUnresolvedByOldPartNumber(ctx, sourceID, partNumber)
	if err != nil {
		return fmt.Errorf("looking up pending part number supersessions (old side) for part number %s: %w", partNumber, err)
	}
	for _, item := range oldSide {
		if err := r.MarkOldResolved(ctx, item.ID, productID, actorID); err != nil {
			return fmt.Errorf("resolving part number supersession %d (old side) for part number %s: %w", item.ID, partNumber, err)
		}
	}

	newSide, err := r.FindUnresolvedByNewPartNumber(ctx, sourceID, partNumber)
	if err != nil {
		return fmt.Errorf("looking up pending part number supersessions (new side) for part number %s: %w", partNumber, err)
	}
	for _, item := range newSide {
		if err := r.MarkNewResolved(ctx, item.ID, productID, actorID); err != nil {
			return fmt.Errorf("resolving part number supersession %d (new side) for part number %s: %w", item.ID, partNumber, err)
		}
	}

	return nil
}

// FindPendingPartNumbers devuelve, para esta fuente, el conjunto de part numbers con una
// sucesión pendiente de resolver por el lado viejo (oldPending) y por el lado nuevo
// (newPending). Se precarga una sola vez por corrida de sync (ver nissanSyncCache en
// sync_nissan.go) para evitar los 2 SELECT que ResolveForProduct hacía por cada uno de los
// miles de SKUs que nunca tienen una sucesión relacionada.
func (r *PartNumberSupersessionsRepository) FindPendingPartNumbers(ctx context.Context, sourceID int64) (oldPending, newPending map[string]bool, err error) {
	oldPending, err = r.queryPendingPartNumberSet(ctx, `
		SELECT DISTINCT old_part_number FROM ecom_part_number_supersessions
		WHERE source_id = ? AND old_product_id IS NULL AND deleted_at IS NULL
	`, sourceID)
	if err != nil {
		return nil, nil, fmt.Errorf("loading pending old part numbers: %w", err)
	}

	newPending, err = r.queryPendingPartNumberSet(ctx, `
		SELECT DISTINCT new_part_number FROM ecom_part_number_supersessions
		WHERE source_id = ? AND new_product_id IS NULL AND deleted_at IS NULL
	`, sourceID)
	if err != nil {
		return nil, nil, fmt.Errorf("loading pending new part numbers: %w", err)
	}

	return oldPending, newPending, nil
}

func (r *PartNumberSupersessionsRepository) queryPendingPartNumberSet(ctx context.Context, query string, sourceID int64) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, query, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	set := make(map[string]bool)
	for rows.Next() {
		var partNumber string
		if err := rows.Scan(&partNumber); err != nil {
			return nil, err
		}
		set[partNumber] = true
	}

	return set, rows.Err()
}

func scanPartNumberSupersession(rows *sql.Rows) (PartNumberSupersessionDTO, error) {
	var item PartNumberSupersessionDTO
	var oldProductID, newProductID sql.NullInt64
	var oldResolvedAt, newResolvedAt sql.NullTime
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := rows.Scan(
		&item.ID, &item.SourceID, &item.OldPartNumber, &item.NewPartNumber,
		&oldProductID, &newProductID, &oldResolvedAt, &newResolvedAt,
		&item.CreatedBy, &updatedBy, &item.CreatedAt, &item.UpdatedAt, &deletedAt,
	)
	if err != nil {
		return PartNumberSupersessionDTO{}, fmt.Errorf("error scanning part number supersession: %w", err)
	}

	applyPartNumberSupersessionNullables(&item, oldProductID, newProductID, oldResolvedAt, newResolvedAt, updatedBy, deletedAt)

	return item, nil
}

func scanPartNumberSupersessionRow(row *sql.Row) (*PartNumberSupersessionDTO, error) {
	var item PartNumberSupersessionDTO
	var oldProductID, newProductID sql.NullInt64
	var oldResolvedAt, newResolvedAt sql.NullTime
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := row.Scan(
		&item.ID, &item.SourceID, &item.OldPartNumber, &item.NewPartNumber,
		&oldProductID, &newProductID, &oldResolvedAt, &newResolvedAt,
		&item.CreatedBy, &updatedBy, &item.CreatedAt, &item.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPartNumberSupersessionNotFound
		}
		return nil, fmt.Errorf("error scanning part number supersession: %w", err)
	}

	applyPartNumberSupersessionNullables(&item, oldProductID, newProductID, oldResolvedAt, newResolvedAt, updatedBy, deletedAt)

	return &item, nil
}

func applyPartNumberSupersessionNullables(
	item *PartNumberSupersessionDTO,
	oldProductID, newProductID sql.NullInt64,
	oldResolvedAt, newResolvedAt sql.NullTime,
	updatedBy sql.NullInt64,
	deletedAt sql.NullTime,
) {
	if oldProductID.Valid {
		item.OldProductID = &oldProductID.Int64
	}
	if newProductID.Valid {
		item.NewProductID = &newProductID.Int64
	}
	if oldResolvedAt.Valid {
		item.OldResolvedAt = &oldResolvedAt.Time
	}
	if newResolvedAt.Valid {
		item.NewResolvedAt = &newResolvedAt.Time
	}
	if updatedBy.Valid {
		item.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
}

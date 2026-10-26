package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Estados de ecom_meli_import_items (ver infrastructure/mysql/
// 2026-10-23_meli_bulk_import.sql).
const (
	MeliImportPending   = "pending"
	MeliImportRunning   = "running"
	MeliImportDone      = "done"
	MeliImportFailed    = "failed"
	MeliImportMissing   = "missing"
	MeliImportDuplicate = "duplicate"
)

var ErrMeliImportItemNotFound = errors.New("meli import item not found")

// MeliImportRepository guarda los lotes y filas de la importación masiva de
// publicaciones de MercadoLibre por Excel.
type MeliImportRepository struct {
	db Querier
}

func NewMeliImportRepository(db Querier) *MeliImportRepository {
	return &MeliImportRepository{db: db}
}

type CreateMeliImportBatchInput struct {
	ConnectionID int64
	FileName     string
	TotalRows    int
	ActorID      int64
}

type CreateMeliImportItemInput struct {
	BatchID      int64
	ConnectionID int64
	SheetRow     int
	SKU          string
	MeliIDRaw    string
	ExternalID   string
	ProductID    *int64
	Status       string
	ErrorMessage string
	ActorID      int64
}

// MeliImportItemDTO es una fila del Excel con su avance. ProductName viene de
// ecom_products (nil si el SKU no existe: estado "missing").
type MeliImportItemDTO struct {
	ID           int64           `json:"id"`
	BatchID      int64           `json:"batchId"`
	BatchFile    string          `json:"batchFile"`
	ConnectionID int64           `json:"connectionId"`
	SheetRow     int             `json:"sheetRow"`
	SKU          string          `json:"sku"`
	MeliIDRaw    string          `json:"meliId"`
	ExternalID   *string         `json:"externalId,omitempty"`
	ProductID    *int64          `json:"productId,omitempty"`
	ProductName  *string         `json:"productName,omitempty"`
	Status       string          `json:"status"`
	ErrorMessage *string         `json:"errorMessage,omitempty"`
	ListingTitle *string         `json:"listingTitle,omitempty"`
	Steps        json.RawMessage `json:"steps,omitempty"`
	Attempts     int             `json:"attempts"`
	StartedAt    *time.Time      `json:"startedAt,omitempty"`
	FinishedAt   *time.Time      `json:"finishedAt,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

// MeliImportItemFilter: todos los campos son opcionales (valor cero = sin filtro).
type MeliImportItemFilter struct {
	Statuses     []string
	BatchID      int64
	ConnectionID int64
	Search       string
	// FinishedOrder ordena por fecha de término descendente (historial); si no,
	// por id ascendente (el orden de la cola).
	FinishedOrder bool
	Offset        int
	PageSize      int
}

type MeliImportItemsPage struct {
	Items []MeliImportItemDTO `json:"items"`
	Total int                 `json:"total"`
}

type MeliImportClaim struct {
	ID           int64
	BatchID      int64
	ConnectionID int64
	ProductID    *int64
	SKU          string
	ExternalID   string
	ActorID      int64
}

type MeliImportOutcome struct {
	Status       string
	ErrorMessage string
	ListingTitle string
	Steps        json.RawMessage
	ActorID      int64
}

func (r *MeliImportRepository) CreateBatch(ctx context.Context, in CreateMeliImportBatchInput) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_meli_import_batches (connection_id, file_name, total_rows, created_by)
		VALUES (?, ?, ?, ?)`, in.ConnectionID, in.FileName, in.TotalRows, in.ActorID)
	if err != nil {
		return 0, fmt.Errorf("error creating meli import batch: %w", err)
	}
	return res.LastInsertId()
}

func (r *MeliImportRepository) CreateItem(ctx context.Context, in CreateMeliImportItemInput) error {
	var finished any
	if in.Status != MeliImportPending {
		finished = time.Now()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_meli_import_items
			(batch_id, connection_id, sheet_row, sku, meli_id_raw, external_id, product_id,
			 status, error_message, finished_at, created_by)
		VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, ?)`,
		in.BatchID, in.ConnectionID, in.SheetRow, in.SKU, in.MeliIDRaw, in.ExternalID, in.ProductID,
		in.Status, in.ErrorMessage, finished, in.ActorID)
	if err != nil {
		return fmt.Errorf("error creating meli import item: %w", err)
	}
	return nil
}

// ClaimNext toma la fila pending más antigua y la marca running. Debe llamarse
// dentro de una transacción (WithinTx): FOR UPDATE SKIP LOCKED evita que dos
// réplicas tomen la misma fila. Devuelve (nil, nil) si no hay nada pendiente.
func (r *MeliImportRepository) ClaimNext(ctx context.Context) (*MeliImportClaim, error) {
	var claim MeliImportClaim
	var productID sql.NullInt64
	var externalID sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT i.id, i.batch_id, i.connection_id, i.product_id, i.sku, i.external_id, b.created_by
		FROM ecom_meli_import_items i
		JOIN ecom_meli_import_batches b ON b.id = i.batch_id
		WHERE i.status = 'pending' AND i.deleted_at IS NULL
		ORDER BY i.id
		LIMIT 1
		FOR UPDATE SKIP LOCKED`).Scan(
		&claim.ID, &claim.BatchID, &claim.ConnectionID, &productID, &claim.SKU, &externalID, &claim.ActorID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error claiming meli import item: %w", err)
	}
	if productID.Valid {
		claim.ProductID = &productID.Int64
	}
	claim.ExternalID = externalID.String

	if _, err := r.db.ExecContext(ctx, `
		UPDATE ecom_meli_import_items
		SET status = 'running', attempts = attempts + 1, started_at = NOW(), finished_at = NULL
		WHERE id = ?`, claim.ID); err != nil {
		return nil, fmt.Errorf("error marking meli import item %d running: %w", claim.ID, err)
	}
	return &claim, nil
}

// RequeueStale devuelve a pending las filas running cuyo worker murió (el
// servicio se reinició a media importación). Cada paso del import es
// idempotente, así que reintentar es seguro.
func (r *MeliImportRepository) RequeueStale(ctx context.Context, olderThan time.Duration) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ecom_meli_import_items
		SET status = 'pending'
		WHERE status = 'running' AND started_at < ?`, time.Now().Add(-olderThan))
	if err != nil {
		return 0, fmt.Errorf("error requeueing stale meli import items: %w", err)
	}
	return res.RowsAffected()
}

func (r *MeliImportRepository) SetOutcome(ctx context.Context, id int64, out MeliImportOutcome) error {
	var steps any
	if len(out.Steps) > 0 {
		steps = string(out.Steps)
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_meli_import_items
		SET status = ?, error_message = NULLIF(?, ''), listing_title = NULLIF(?, ''), steps = ?,
		    finished_at = NOW(), updated_by = ?
		WHERE id = ?`, out.Status, out.ErrorMessage, out.ListingTitle, steps, out.ActorID, id)
	if err != nil {
		return fmt.Errorf("error saving meli import item %d outcome: %w", id, err)
	}
	return nil
}

// Requeue pone una fila failed/missing de nuevo en pending (con el producto ya
// resuelto, si lo hay). Devuelve ErrMeliImportItemNotFound si no está en un
// estado reintentable.
func (r *MeliImportRepository) Requeue(ctx context.Context, id int64, productID *int64, actorID int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ecom_meli_import_items
		SET status = 'pending', product_id = ?, error_message = NULL, steps = NULL,
		    started_at = NULL, finished_at = NULL, updated_by = ?
		WHERE id = ? AND status IN ('failed', 'missing') AND deleted_at IS NULL`, productID, actorID, id)
	if err != nil {
		return fmt.Errorf("error requeueing meli import item %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrMeliImportItemNotFound
	}
	return nil
}

// FindItem devuelve una fila por id.
func (r *MeliImportRepository) FindItem(ctx context.Context, id int64) (*MeliImportItemDTO, error) {
	page, err := r.listItems(ctx, "i.id = ?", []any{id}, "i.id", 0, 1)
	if err != nil {
		return nil, err
	}
	if len(page.Items) == 0 {
		return nil, ErrMeliImportItemNotFound
	}
	return &page.Items[0], nil
}

// ListByStatus devuelve todas las filas en el estado dado (para reintentar en
// bloque los failed o missing), opcionalmente de una sola conexión.
func (r *MeliImportRepository) ListByStatus(ctx context.Context, status string, connectionID int64) ([]MeliImportItemDTO, error) {
	where, args := meliImportWhere(MeliImportItemFilter{Statuses: []string{status}, ConnectionID: connectionID})
	page, err := r.listItems(ctx, where, args, "i.id", 0, 100000)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func (r *MeliImportRepository) FindItems(ctx context.Context, f MeliImportItemFilter) (*MeliImportItemsPage, error) {
	where, args := meliImportWhere(f)
	order := "i.id"
	if f.FinishedOrder {
		order = "i.finished_at DESC, i.id DESC"
	}
	pageSize := f.PageSize
	if pageSize <= 0 {
		pageSize = 25
	}
	return r.listItems(ctx, where, args, order, f.Offset, pageSize)
}

func meliImportWhere(f MeliImportItemFilter) (string, []any) {
	conds := []string{"1=1"}
	var args []any
	if len(f.Statuses) > 0 {
		conds = append(conds, "i.status IN ("+strings.TrimSuffix(strings.Repeat("?,", len(f.Statuses)), ",")+")")
		for _, s := range f.Statuses {
			args = append(args, s)
		}
	}
	if f.BatchID > 0 {
		conds = append(conds, "i.batch_id = ?")
		args = append(args, f.BatchID)
	}
	if f.ConnectionID > 0 {
		conds = append(conds, "i.connection_id = ?")
		args = append(args, f.ConnectionID)
	}
	if search := strings.TrimSpace(f.Search); search != "" {
		like := "%" + search + "%"
		conds = append(conds, "(i.sku LIKE ? OR i.meli_id_raw LIKE ? OR i.external_id LIKE ? OR p.name LIKE ?)")
		args = append(args, like, like, like, like)
	}
	return strings.Join(conds, " AND "), args
}

func (r *MeliImportRepository) listItems(ctx context.Context, where string, args []any, order string, offset, pageSize int) (*MeliImportItemsPage, error) {
	from := `
		FROM ecom_meli_import_items i
		JOIN ecom_meli_import_batches b ON b.id = i.batch_id
		LEFT JOIN ecom_products p ON p.id = i.product_id
		WHERE i.deleted_at IS NULL AND ` + where

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) "+from, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("error counting meli import items: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT i.id, i.batch_id, b.file_name, i.connection_id, i.sheet_row, i.sku, i.meli_id_raw,
		       i.external_id, i.product_id, p.name, i.status, i.error_message, i.listing_title,
		       i.steps, i.attempts, i.started_at, i.finished_at, i.created_at `+from+`
		ORDER BY `+order+` LIMIT ? OFFSET ?`, append(append([]any{}, args...), pageSize, offset)...)
	if err != nil {
		return nil, fmt.Errorf("error listing meli import items: %w", err)
	}
	defer rows.Close()

	items := []MeliImportItemDTO{}
	for rows.Next() {
		var it MeliImportItemDTO
		var externalID, productName, errMsg, title sql.NullString
		var productID sql.NullInt64
		var steps []byte
		var started, finished sql.NullTime
		if err := rows.Scan(&it.ID, &it.BatchID, &it.BatchFile, &it.ConnectionID, &it.SheetRow, &it.SKU, &it.MeliIDRaw,
			&externalID, &productID, &productName, &it.Status, &errMsg, &title,
			&steps, &it.Attempts, &started, &finished, &it.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning meli import item: %w", err)
		}
		if externalID.Valid {
			it.ExternalID = &externalID.String
		}
		if productID.Valid {
			it.ProductID = &productID.Int64
		}
		if productName.Valid {
			it.ProductName = &productName.String
		}
		if errMsg.Valid {
			it.ErrorMessage = &errMsg.String
		}
		if title.Valid {
			it.ListingTitle = &title.String
		}
		if len(steps) > 0 {
			it.Steps = json.RawMessage(steps)
		}
		if started.Valid {
			it.StartedAt = &started.Time
		}
		if finished.Valid {
			it.FinishedAt = &finished.Time
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading meli import items: %w", err)
	}
	return &MeliImportItemsPage{Items: items, Total: total}, nil
}

// MeliImportBatchDTO es un lote (un Excel subido).
type MeliImportBatchDTO struct {
	ID           int64     `json:"id"`
	ConnectionID int64     `json:"connectionId"`
	FileName     string    `json:"fileName"`
	TotalRows    int       `json:"totalRows"`
	CreatedAt    time.Time `json:"createdAt"`
}

var ErrMeliImportBatchNotFound = errors.New("meli import batch not found")

// ListBatches devuelve los lotes más recientes primero, opcionalmente de una
// sola conexión.
func (r *MeliImportRepository) ListBatches(ctx context.Context, connectionID int64, limit int) ([]MeliImportBatchDTO, error) {
	query := `
		SELECT id, connection_id, file_name, total_rows, created_at
		FROM ecom_meli_import_batches
		WHERE deleted_at IS NULL`
	var args []any
	if connectionID > 0 {
		query += " AND connection_id = ?"
		args = append(args, connectionID)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error listing meli import batches: %w", err)
	}
	defer rows.Close()

	batches := []MeliImportBatchDTO{}
	for rows.Next() {
		var b MeliImportBatchDTO
		if err := rows.Scan(&b.ID, &b.ConnectionID, &b.FileName, &b.TotalRows, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning meli import batch: %w", err)
		}
		batches = append(batches, b)
	}
	return batches, rows.Err()
}

// FindBatch devuelve un lote por id.
func (r *MeliImportRepository) FindBatch(ctx context.Context, id int64) (*MeliImportBatchDTO, error) {
	var b MeliImportBatchDTO
	err := r.db.QueryRowContext(ctx, `
		SELECT id, connection_id, file_name, total_rows, created_at
		FROM ecom_meli_import_batches
		WHERE id = ? AND deleted_at IS NULL`, id).Scan(&b.ID, &b.ConnectionID, &b.FileName, &b.TotalRows, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMeliImportBatchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error finding meli import batch %d: %w", id, err)
	}
	return &b, nil
}

// ListByBatch devuelve todas las filas de un lote en el orden del Excel.
func (r *MeliImportRepository) ListByBatch(ctx context.Context, batchID int64) ([]MeliImportItemDTO, error) {
	where, args := meliImportWhere(MeliImportItemFilter{BatchID: batchID})
	page, err := r.listItems(ctx, where, args, "i.sheet_row, i.id", 0, 100000)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// CountByStatus devuelve cuántas filas hay por estado (para los contadores de
// los filtros), opcionalmente de una sola conexión o lote.
func (r *MeliImportRepository) CountByStatus(ctx context.Context, connectionID, batchID int64) (map[string]int, error) {
	where, args := meliImportWhere(MeliImportItemFilter{ConnectionID: connectionID, BatchID: batchID})
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.status, COUNT(*)
		FROM ecom_meli_import_items i
		WHERE i.deleted_at IS NULL AND `+where+`
		GROUP BY i.status`, args...)
	if err != nil {
		return nil, fmt.Errorf("error counting meli import items: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, fmt.Errorf("error scanning meli import counts: %w", err)
		}
		counts[status] = n
	}
	return counts, rows.Err()
}

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

// Estados de ecom_product_bulk_update_items (ver infrastructure/mysql/
// 2026-11-10_product_bulk_update.sql).
const (
	ProductBulkUpdatePending = "pending"
	ProductBulkUpdateRunning = "running"
	ProductBulkUpdateDone    = "done"
	ProductBulkUpdateFailed  = "failed"
	ProductBulkUpdateMissing = "missing"
)

var (
	ErrProductBulkUpdateItemNotFound  = errors.New("product bulk update item not found")
	ErrProductBulkUpdateBatchNotFound = errors.New("product bulk update batch not found")
)

// ProductBulkUpdateRepository guarda los lotes y filas de la actualización
// masiva de productos por Excel.
type ProductBulkUpdateRepository struct {
	db Querier
}

func NewProductBulkUpdateRepository(db Querier) *ProductBulkUpdateRepository {
	return &ProductBulkUpdateRepository{db: db}
}

type CreateProductBulkUpdateBatchInput struct {
	SourceID      int64
	StorageDiskID int64
	// SyncConnectionIDs son las conexiones donde, tras guardar cada fila, se
	// resincroniza el producto. Vacío = no resincronizar.
	SyncConnectionIDs []int64
	FileName          string
	TotalRows         int
	ActorID           int64
}

type CreateProductBulkUpdateItemInput struct {
	BatchID      int64
	SheetRow     int
	SKU          string
	ProductID    *int64
	Payload      json.RawMessage
	Status       string
	ErrorMessage string
	ActorID      int64
}

// ProductBulkUpdateItemDTO es una fila del Excel con su avance. ProductName
// viene de ecom_products (nil si el SKU no existe: estado "missing").
type ProductBulkUpdateItemDTO struct {
	ID           int64           `json:"id"`
	BatchID      int64           `json:"batchId"`
	BatchFile    string          `json:"batchFile"`
	SheetRow     int             `json:"sheetRow"`
	SKU          string          `json:"sku"`
	ProductID    *int64          `json:"productId,omitempty"`
	ProductName  *string         `json:"productName,omitempty"`
	Status       string          `json:"status"`
	ErrorMessage *string         `json:"errorMessage,omitempty"`
	Steps        json.RawMessage `json:"steps,omitempty"`
	Attempts     int             `json:"attempts"`
	StartedAt    *time.Time      `json:"startedAt,omitempty"`
	FinishedAt   *time.Time      `json:"finishedAt,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	// Retryable: la fila failed/missing conserva lo que pedía (payload) y se
	// puede volver a encolar. Las filas inválidas del Excel (sin sku, medida mal
	// escrita...) no: hay que corregir el archivo y subirlo de nuevo.
	Retryable bool `json:"retryable"`
}

// ProductBulkUpdateItemFilter: todos los campos son opcionales (valor cero = sin filtro).
type ProductBulkUpdateItemFilter struct {
	Statuses []string
	BatchID  int64
	Search   string
	// FinishedOrder ordena por fecha de término descendente (historial); si no,
	// por id ascendente (el orden de la cola).
	FinishedOrder bool
	Offset        int
	PageSize      int
}

type ProductBulkUpdateItemsPage struct {
	Items []ProductBulkUpdateItemDTO `json:"items"`
	Total int                        `json:"total"`
}

// ProductBulkUpdateClaim es la fila que el worker tomó de la cola, junto con
// los datos del lote que necesita para procesarla.
type ProductBulkUpdateClaim struct {
	ID                int64
	BatchID           int64
	SourceID          int64
	StorageDiskID     int64
	SyncConnectionIDs []int64
	ProductID         *int64
	SKU               string
	Payload           json.RawMessage
	ActorID           int64
}

// decodeConnectionIDs lee la columna JSON sync_connection_ids (NULL = ninguna).
func decodeConnectionIDs(raw []byte) []int64 {
	if len(raw) == 0 {
		return nil
	}
	var ids []int64
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil
	}
	return ids
}

type ProductBulkUpdateOutcome struct {
	Status       string
	ErrorMessage string
	Steps        json.RawMessage
	ActorID      int64
}

func (r *ProductBulkUpdateRepository) CreateBatch(ctx context.Context, in CreateProductBulkUpdateBatchInput) (int64, error) {
	var syncIDs any
	if len(in.SyncConnectionIDs) > 0 {
		encoded, err := json.Marshal(in.SyncConnectionIDs)
		if err != nil {
			return 0, fmt.Errorf("error encoding sync connection ids: %w", err)
		}
		syncIDs = string(encoded)
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_product_bulk_update_batches (source_id, storage_disk_id, sync_connection_ids, file_name, total_rows, created_by)
		VALUES (?, ?, ?, ?, ?, ?)`, in.SourceID, in.StorageDiskID, syncIDs, in.FileName, in.TotalRows, in.ActorID)
	if err != nil {
		return 0, fmt.Errorf("error creating product bulk update batch: %w", err)
	}
	return res.LastInsertId()
}

func (r *ProductBulkUpdateRepository) CreateItem(ctx context.Context, in CreateProductBulkUpdateItemInput) error {
	var finished any
	if in.Status != ProductBulkUpdatePending {
		finished = time.Now()
	}
	var payload any
	if len(in.Payload) > 0 {
		payload = string(in.Payload)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_product_bulk_update_items
			(batch_id, sheet_row, sku, product_id, payload, status, error_message, finished_at, created_by)
		VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)`,
		in.BatchID, in.SheetRow, in.SKU, in.ProductID, payload, in.Status, in.ErrorMessage, finished, in.ActorID)
	if err != nil {
		return fmt.Errorf("error creating product bulk update item: %w", err)
	}
	return nil
}

// ClaimNext toma la fila pending más antigua y la marca running. Debe llamarse
// dentro de una transacción (WithinTx): FOR UPDATE SKIP LOCKED evita que dos
// workers tomen la misma fila. Devuelve (nil, nil) si no hay nada pendiente.
func (r *ProductBulkUpdateRepository) ClaimNext(ctx context.Context) (*ProductBulkUpdateClaim, error) {
	var claim ProductBulkUpdateClaim
	var productID sql.NullInt64
	var payload, syncIDs []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT i.id, i.batch_id, b.source_id, b.storage_disk_id, b.sync_connection_ids, i.product_id, i.sku, i.payload, b.created_by
		FROM ecom_product_bulk_update_items i
		JOIN ecom_product_bulk_update_batches b ON b.id = i.batch_id
		WHERE i.status = 'pending' AND i.deleted_at IS NULL
		ORDER BY i.id
		LIMIT 1
		FOR UPDATE OF i SKIP LOCKED`).Scan(
		&claim.ID, &claim.BatchID, &claim.SourceID, &claim.StorageDiskID, &syncIDs, &productID, &claim.SKU, &payload, &claim.ActorID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error claiming product bulk update item: %w", err)
	}
	if productID.Valid {
		claim.ProductID = &productID.Int64
	}
	claim.Payload = json.RawMessage(payload)
	claim.SyncConnectionIDs = decodeConnectionIDs(syncIDs)

	if _, err := r.db.ExecContext(ctx, `
		UPDATE ecom_product_bulk_update_items
		SET status = 'running', attempts = attempts + 1, started_at = NOW(), finished_at = NULL
		WHERE id = ?`, claim.ID); err != nil {
		return nil, fmt.Errorf("error marking product bulk update item %d running: %w", claim.ID, err)
	}
	return &claim, nil
}

// RequeueStale devuelve a pending las filas running cuyo worker murió (el
// servicio se reinició a media actualización). Cada paso es idempotente, así
// que reintentar es seguro.
func (r *ProductBulkUpdateRepository) RequeueStale(ctx context.Context, olderThan time.Duration) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ecom_product_bulk_update_items
		SET status = 'pending'
		WHERE status = 'running' AND started_at < ?`, time.Now().Add(-olderThan))
	if err != nil {
		return 0, fmt.Errorf("error requeueing stale product bulk update items: %w", err)
	}
	return res.RowsAffected()
}

func (r *ProductBulkUpdateRepository) SetOutcome(ctx context.Context, id int64, out ProductBulkUpdateOutcome) error {
	var steps any
	if len(out.Steps) > 0 {
		steps = string(out.Steps)
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_product_bulk_update_items
		SET status = ?, error_message = NULLIF(?, ''), steps = ?, finished_at = NOW(), updated_by = ?
		WHERE id = ?`, out.Status, out.ErrorMessage, steps, out.ActorID, id)
	if err != nil {
		return fmt.Errorf("error saving product bulk update item %d outcome: %w", id, err)
	}
	return nil
}

// Requeue pone una fila failed/missing de nuevo en pending (con el producto ya
// resuelto, si lo hay). Devuelve ErrProductBulkUpdateItemNotFound si no está en
// un estado reintentable.
func (r *ProductBulkUpdateRepository) Requeue(ctx context.Context, id int64, productID *int64, actorID int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ecom_product_bulk_update_items
		SET status = 'pending', product_id = ?, error_message = NULL, steps = NULL,
		    started_at = NULL, finished_at = NULL, updated_by = ?
		WHERE id = ? AND status IN ('failed', 'missing') AND payload IS NOT NULL AND deleted_at IS NULL`,
		productID, actorID, id)
	if err != nil {
		return fmt.Errorf("error requeueing product bulk update item %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrProductBulkUpdateItemNotFound
	}
	return nil
}

// FindItem devuelve una fila por id.
func (r *ProductBulkUpdateRepository) FindItem(ctx context.Context, id int64) (*ProductBulkUpdateItemDTO, error) {
	page, err := r.listItems(ctx, "i.id = ?", []any{id}, "i.id", 0, 1)
	if err != nil {
		return nil, err
	}
	if len(page.Items) == 0 {
		return nil, ErrProductBulkUpdateItemNotFound
	}
	return &page.Items[0], nil
}

// ListByStatus devuelve todas las filas en el estado dado (para reintentar en
// bloque los failed o missing), opcionalmente de un solo lote.
func (r *ProductBulkUpdateRepository) ListByStatus(ctx context.Context, status string, batchID int64) ([]ProductBulkUpdateItemDTO, error) {
	where, args := productBulkUpdateWhere(ProductBulkUpdateItemFilter{Statuses: []string{status}, BatchID: batchID})
	page, err := r.listItems(ctx, where, args, "i.id", 0, 100000)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func (r *ProductBulkUpdateRepository) FindItems(ctx context.Context, f ProductBulkUpdateItemFilter) (*ProductBulkUpdateItemsPage, error) {
	where, args := productBulkUpdateWhere(f)
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

func productBulkUpdateWhere(f ProductBulkUpdateItemFilter) (string, []any) {
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
	if search := strings.TrimSpace(f.Search); search != "" {
		like := "%" + search + "%"
		conds = append(conds, "(i.sku LIKE ? OR p.name LIKE ?)")
		args = append(args, like, like)
	}
	return strings.Join(conds, " AND "), args
}

func (r *ProductBulkUpdateRepository) listItems(ctx context.Context, where string, args []any, order string, offset, pageSize int) (*ProductBulkUpdateItemsPage, error) {
	from := `
		FROM ecom_product_bulk_update_items i
		JOIN ecom_product_bulk_update_batches b ON b.id = i.batch_id
		LEFT JOIN ecom_products p ON p.id = i.product_id
		WHERE i.deleted_at IS NULL AND ` + where

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) "+from, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("error counting product bulk update items: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT i.id, i.batch_id, b.file_name, i.sheet_row, i.sku, i.product_id, p.name, i.status,
		       i.error_message, i.steps, i.attempts, i.started_at, i.finished_at, i.created_at,
		       (i.payload IS NOT NULL) `+from+`
		ORDER BY `+order+` LIMIT ? OFFSET ?`, append(append([]any{}, args...), pageSize, offset)...)
	if err != nil {
		return nil, fmt.Errorf("error listing product bulk update items: %w", err)
	}
	defer rows.Close()

	items := []ProductBulkUpdateItemDTO{}
	for rows.Next() {
		var it ProductBulkUpdateItemDTO
		var productName, errMsg sql.NullString
		var productID sql.NullInt64
		var steps []byte
		var started, finished sql.NullTime
		if err := rows.Scan(&it.ID, &it.BatchID, &it.BatchFile, &it.SheetRow, &it.SKU, &productID, &productName,
			&it.Status, &errMsg, &steps, &it.Attempts, &started, &finished, &it.CreatedAt, &it.Retryable); err != nil {
			return nil, fmt.Errorf("error scanning product bulk update item: %w", err)
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
		if len(steps) > 0 {
			it.Steps = json.RawMessage(steps)
		}
		if started.Valid {
			it.StartedAt = &started.Time
		}
		if finished.Valid {
			it.FinishedAt = &finished.Time
		}
		it.Retryable = it.Retryable && (it.Status == ProductBulkUpdateFailed || it.Status == ProductBulkUpdateMissing)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading product bulk update items: %w", err)
	}
	return &ProductBulkUpdateItemsPage{Items: items, Total: total}, nil
}

// ProductBulkUpdateBatchDTO es un lote (un Excel subido).
type ProductBulkUpdateBatchDTO struct {
	ID                int64     `json:"id"`
	SourceID          int64     `json:"sourceId"`
	StorageDiskID     int64     `json:"storageDiskId"`
	SyncConnectionIDs []int64   `json:"syncConnectionIds"`
	FileName          string    `json:"fileName"`
	TotalRows         int       `json:"totalRows"`
	CreatedAt         time.Time `json:"createdAt"`
}

// ListBatches devuelve los lotes más recientes primero.
func (r *ProductBulkUpdateRepository) ListBatches(ctx context.Context, limit int) ([]ProductBulkUpdateBatchDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, source_id, storage_disk_id, sync_connection_ids, file_name, total_rows, created_at
		FROM ecom_product_bulk_update_batches
		WHERE deleted_at IS NULL
		ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("error listing product bulk update batches: %w", err)
	}
	defer rows.Close()

	batches := []ProductBulkUpdateBatchDTO{}
	for rows.Next() {
		var b ProductBulkUpdateBatchDTO
		var syncIDs []byte
		if err := rows.Scan(&b.ID, &b.SourceID, &b.StorageDiskID, &syncIDs, &b.FileName, &b.TotalRows, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("error scanning product bulk update batch: %w", err)
		}
		b.SyncConnectionIDs = decodeConnectionIDs(syncIDs)
		if b.SyncConnectionIDs == nil {
			b.SyncConnectionIDs = []int64{}
		}
		batches = append(batches, b)
	}
	return batches, rows.Err()
}

// FindBatch devuelve un lote por id.
func (r *ProductBulkUpdateRepository) FindBatch(ctx context.Context, id int64) (*ProductBulkUpdateBatchDTO, error) {
	var b ProductBulkUpdateBatchDTO
	var syncIDs []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, source_id, storage_disk_id, sync_connection_ids, file_name, total_rows, created_at
		FROM ecom_product_bulk_update_batches
		WHERE id = ? AND deleted_at IS NULL`, id).Scan(&b.ID, &b.SourceID, &b.StorageDiskID, &syncIDs, &b.FileName, &b.TotalRows, &b.CreatedAt)
	b.SyncConnectionIDs = decodeConnectionIDs(syncIDs)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProductBulkUpdateBatchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error finding product bulk update batch %d: %w", id, err)
	}
	return &b, nil
}

// ListByBatch devuelve todas las filas de un lote en el orden del Excel.
func (r *ProductBulkUpdateRepository) ListByBatch(ctx context.Context, batchID int64) ([]ProductBulkUpdateItemDTO, error) {
	where, args := productBulkUpdateWhere(ProductBulkUpdateItemFilter{BatchID: batchID})
	page, err := r.listItems(ctx, where, args, "i.sheet_row, i.id", 0, 100000)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// CountByStatus devuelve cuántas filas hay por estado (para los contadores de
// los filtros y la barra de progreso), opcionalmente de un solo lote.
func (r *ProductBulkUpdateRepository) CountByStatus(ctx context.Context, batchID int64) (map[string]int, error) {
	where, args := productBulkUpdateWhere(ProductBulkUpdateItemFilter{BatchID: batchID})
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.status, COUNT(*)
		FROM ecom_product_bulk_update_items i
		WHERE i.deleted_at IS NULL AND `+where+`
		GROUP BY i.status`, args...)
	if err != nil {
		return nil, fmt.Errorf("error counting product bulk update items: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, fmt.Errorf("error scanning product bulk update counts: %w", err)
		}
		counts[status] = n
	}
	return counts, rows.Err()
}

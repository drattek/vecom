package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// NotificationRecipientDTO es un usuario que quiere recibir un tipo de aviso.
// Sus permisos se comprueban aparte.
type NotificationRecipientDTO struct {
	UserID int64
	Role   string
	RoleID *int64
}

// Tipos de lote masivo.
const (
	BulkKindMeliImport    = "meli_import"
	BulkKindProductUpdate = "product_bulk_update"
)

// FinishedBatchDTO es un lote sin filas pendientes ni en proceso.
type FinishedBatchDTO struct {
	Kind      string
	ID        int64
	FileName  string
	CreatedBy int64
	// FinishedAtUnix es el fin de la última fila: cambia si se reintenta.
	FinishedAtUnix int64
	Total          int
	Done           int
	Failed         int
	Missing        int
	Duplicate      int
}

// FailedBatchRowDTO es una fila de un lote que no terminó bien.
type FailedBatchRowDTO struct {
	SheetRow int
	SKU      string
	Status   string
	Error    string
}

// ListingStateDTO es el último intento registrado sobre una publicación
// (producto, conexión, compatibilidad).
type ListingStateDTO struct {
	ProductID      int64
	SKU            string
	ProductName    string
	ConnectionID   int64
	ConnectionName string
	Action         string
	Outcome        string
	Message        string
	// Age es el tiempo desde el evento según el reloj de la base.
	Age time.Duration
}

// ProductListingDTO es una publicación viva de un producto.
type ProductListingDTO struct {
	ProductID      int64
	SKU            string
	ProductName    string
	ConnectionName string
	ExternalID     string
	Title          string
	Status         string
	PausedReason   string
}

// NotificationQueryRepository reúne las lecturas que cruzan tablas para decidir
// qué avisar y a quién.
type NotificationQueryRepository struct {
	db Querier
}

func NewNotificationQueryRepository(db Querier) *NotificationQueryRepository {
	return &NotificationQueryRepository{db: db}
}

// Recipients: usuarios activos, vinculados al bot en uso, con las
// notificaciones encendidas y ese aviso activado. onlyUserID > 0 limita a uno.
func (r *NotificationQueryRepository) Recipients(ctx context.Context, notificationType, botUsername string, onlyUserID int64) ([]NotificationRecipientDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.role, u.role_id
		FROM ecom_notification_preferences p
		JOIN ecom_api_user u ON u.id = p.user_id AND u.is_active = 1
		JOIN ecom_user_telegram_links l ON l.user_id = u.id AND l.deleted_at IS NULL
		WHERE p.notification_type = ? AND p.is_enabled = 1 AND p.deleted_at IS NULL
		  AND l.status = 'linked' AND l.notifications_enabled = 1 AND l.bot_username = ?
		  AND (? = 0 OR u.id = ?)
		ORDER BY u.id
	`, notificationType, botUsername, onlyUserID, onlyUserID)
	if err != nil {
		return nil, fmt.Errorf("listing recipients of %s: %w", notificationType, err)
	}
	defer rows.Close()

	recipients := make([]NotificationRecipientDTO, 0)
	for rows.Next() {
		var (
			recipient NotificationRecipientDTO
			roleID    sql.NullInt64
		)
		if err := rows.Scan(&recipient.UserID, &recipient.Role, &roleID); err != nil {
			return nil, fmt.Errorf("reading recipient: %w", err)
		}
		if roleID.Valid {
			recipient.RoleID = &roleID.Int64
		}
		recipients = append(recipients, recipient)
	}
	return recipients, rows.Err()
}

type bulkTables struct{ batches, items string }

var bulkTablesByKind = map[string]bulkTables{
	BulkKindMeliImport:    {"ecom_meli_import_batches", "ecom_meli_import_items"},
	BulkKindProductUpdate: {"ecom_product_bulk_update_batches", "ecom_product_bulk_update_items"},
}

// FinishedBatches devuelve los lotes terminados cuya última fila acabó dentro de
// window. Se compara con el reloj de la base, que es quien llena finished_at.
func (r *NotificationQueryRepository) FinishedBatches(ctx context.Context, kind string, window time.Duration) ([]FinishedBatchDTO, error) {
	tables, ok := bulkTablesByKind[kind]
	if !ok {
		return nil, fmt.Errorf("unknown bulk kind %q", kind)
	}
	// Solo se interpolan nombres de tabla fijos.
	query := fmt.Sprintf(`
		SELECT b.id, b.file_name, b.created_by, UNIX_TIMESTAMP(MAX(i.finished_at)), COUNT(*),
		       SUM(i.status = 'done'), SUM(i.status = 'failed'), SUM(i.status = 'missing'),
		       SUM(i.status = 'duplicate')
		FROM %[1]s b
		JOIN %[2]s i ON i.batch_id = b.id AND i.deleted_at IS NULL
		WHERE b.deleted_at IS NULL
		  AND b.id IN (SELECT r.batch_id FROM %[2]s r WHERE r.finished_at >= NOW() - INTERVAL ? SECOND)
		GROUP BY b.id, b.file_name, b.created_by
		HAVING SUM(i.status IN ('pending', 'running')) = 0
		ORDER BY b.id
	`, tables.batches, tables.items)

	rows, err := r.db.QueryContext(ctx, query, int64(window.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("listing finished %s batches: %w", kind, err)
	}
	defer rows.Close()

	batches := make([]FinishedBatchDTO, 0)
	for rows.Next() {
		batch := FinishedBatchDTO{Kind: kind}
		if err := rows.Scan(&batch.ID, &batch.FileName, &batch.CreatedBy, &batch.FinishedAtUnix, &batch.Total,
			&batch.Done, &batch.Failed, &batch.Missing, &batch.Duplicate); err != nil {
			return nil, fmt.Errorf("reading finished batch: %w", err)
		}
		batches = append(batches, batch)
	}
	return batches, rows.Err()
}

func (r *NotificationQueryRepository) FailedBatchRows(ctx context.Context, kind string, batchID int64) ([]FailedBatchRowDTO, error) {
	tables, ok := bulkTablesByKind[kind]
	if !ok {
		return nil, fmt.Errorf("unknown bulk kind %q", kind)
	}
	query := fmt.Sprintf(`
		SELECT sheet_row, sku, status, error_message
		FROM %s
		WHERE batch_id = ? AND status <> 'done' AND deleted_at IS NULL
		ORDER BY sheet_row, id
	`, tables.items)

	rows, err := r.db.QueryContext(ctx, query, batchID)
	if err != nil {
		return nil, fmt.Errorf("listing failed rows of %s batch %d: %w", kind, batchID, err)
	}
	defer rows.Close()

	failed := make([]FailedBatchRowDTO, 0)
	for rows.Next() {
		var (
			row     FailedBatchRowDTO
			message sql.NullString
		)
		if err := rows.Scan(&row.SheetRow, &row.SKU, &row.Status, &message); err != nil {
			return nil, fmt.Errorf("reading failed row: %w", err)
		}
		row.Error = message.String
		failed = append(failed, row)
	}
	return failed, rows.Err()
}

// LatestListingStates devuelve, por publicación, el último intento de publicar o
// resincronizar registrado dentro de window en conexiones activas. Los errores
// con el mensaje ignoredError no cuentan como intento. Los cambios de estado que
// informa el marketplace (status_sync) tampoco.
func (r *NotificationQueryRepository) LatestListingStates(ctx context.Context, window time.Duration, ignoredError string) ([]ListingStateDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.product_id, p.sku, p.name, e.connection_id, cc.name, e.action, e.outcome, e.message,
		       TIMESTAMPDIFF(SECOND, e.created_at, NOW())
		FROM (
			SELECT MAX(id) AS id
			FROM ecom_channel_listing_events
			WHERE created_at >= NOW() - INTERVAL ? SECOND
			  AND action IN ('publish', 'refresh', 'resync', 'pause', 'reactivate')
			  AND NOT (outcome = 'error' AND COALESCE(message, '') = ?)
			GROUP BY product_id, connection_id, vehicle_fitment_key
		) latest
		JOIN ecom_channel_listing_events e ON e.id = latest.id
		JOIN ecom_products p ON p.id = e.product_id AND p.deleted_at IS NULL
		JOIN ecom_channel_connections cc ON cc.id = e.connection_id
		     AND cc.deleted_at IS NULL AND cc.status = 'active'
		ORDER BY p.sku, e.product_id, e.connection_id, e.id
	`, int64(window.Seconds()), ignoredError)
	if err != nil {
		return nil, fmt.Errorf("listing latest listing states: %w", err)
	}
	defer rows.Close()

	states := make([]ListingStateDTO, 0)
	for rows.Next() {
		var (
			state      ListingStateDTO
			message    sql.NullString
			ageSeconds int64
		)
		if err := rows.Scan(&state.ProductID, &state.SKU, &state.ProductName, &state.ConnectionID,
			&state.ConnectionName, &state.Action, &state.Outcome, &message, &ageSeconds); err != nil {
			return nil, fmt.Errorf("reading listing state: %w", err)
		}
		state.Message = message.String
		state.Age = time.Duration(ageSeconds) * time.Second
		states = append(states, state)
	}
	return states, rows.Err()
}

// StockPausedListings devuelve las publicaciones no cerradas, en conexiones
// activas, de los productos que en los últimos window tuvieron una pausa y aún
// tienen alguna pausada por falta de stock. Cuentan las pausas propias
// ('zero_stock') y las del marketplace: sin motivo y con un evento de pausa con
// el mensaje marketplacePauseMessage. Estas se devuelven como 'zero_stock'.
func (r *NotificationQueryRepository) StockPausedListings(ctx context.Context, window time.Duration, marketplacePauseMessage string) ([]ProductListingDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.product_id, p.sku, p.name, cc.name, m.external_id, m.listing_title, m.status,
		       CASE
		         WHEN m.status = 'paused' AND m.paused_reason IS NULL AND EXISTS (
		           SELECT 1 FROM ecom_channel_listing_events x
		           WHERE x.channel_product_map_id = m.id AND x.action = 'pause' AND x.message = ?
		         ) THEN 'zero_stock'
		         ELSE m.paused_reason
		       END
		FROM ecom_channel_product_map m
		JOIN ecom_products p ON p.id = m.product_id AND p.deleted_at IS NULL
		JOIN ecom_channel_connections cc ON cc.id = m.connection_id
		     AND cc.deleted_at IS NULL AND cc.status = 'active'
		WHERE m.deleted_at IS NULL AND m.status <> 'closed'
		  AND m.product_id IN (
			SELECT e.product_id
			FROM ecom_channel_listing_events e
			JOIN ecom_channel_product_map z ON z.product_id = e.product_id AND z.deleted_at IS NULL
			     AND z.status = 'paused'
			     AND (z.paused_reason = 'zero_stock' OR (z.paused_reason IS NULL AND EXISTS (
			       SELECT 1 FROM ecom_channel_listing_events y
			       WHERE y.channel_product_map_id = z.id AND y.action = 'pause' AND y.message = ?
			     )))
			WHERE e.created_at >= NOW() - INTERVAL ? SECOND AND e.outcome = 'success'
			  AND (e.action = 'pause' OR (e.action = 'publish' AND e.status_after = 'paused'))
		  )
		ORDER BY p.sku, m.product_id, cc.name, m.id
	`, marketplacePauseMessage, marketplacePauseMessage, int64(window.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("listing stock paused products: %w", err)
	}
	defer rows.Close()

	listings := make([]ProductListingDTO, 0)
	for rows.Next() {
		var (
			listing                         ProductListingDTO
			externalID, title, pausedReason sql.NullString
		)
		if err := rows.Scan(&listing.ProductID, &listing.SKU, &listing.ProductName, &listing.ConnectionName,
			&externalID, &title, &listing.Status, &pausedReason); err != nil {
			return nil, fmt.Errorf("reading stock paused listing: %w", err)
		}
		listing.ExternalID = externalID.String
		listing.Title = title.String
		listing.PausedReason = pausedReason.String
		listings = append(listings, listing)
	}
	return listings, rows.Err()
}

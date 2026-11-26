package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Filtros de la vista "Preparación del catálogo": cada uno es un motivo por el
// que un producto puede no estar vendiéndose en un canal.
const (
	CatalogFilterNoImage          = "no_image"
	CatalogFilterNoPrice          = "no_price"
	CatalogFilterNoBrand          = "no_brand"
	CatalogFilterNoCategory       = "no_category"
	CatalogFilterNoStock          = "no_stock"
	CatalogFilterCategoryUnmapped = "category_unmapped"
	CatalogFilterReadyUnpublished = "ready_unpublished"
)

// DashboardAttentionRow es una publicación (o intento de publicación) que
// requiere atención.
type DashboardAttentionRow struct {
	Kind             string     `json:"kind"`
	ID               int64      `json:"id"`
	ProductID        int64      `json:"productId"`
	SKU              string     `json:"sku"`
	ProductName      string     `json:"productName"`
	ConnectionName   string     `json:"connectionName"`
	VehicleFitmentID *int64     `json:"vehicleFitmentId,omitempty"`
	ListingTitle     *string    `json:"listingTitle,omitempty"`
	ExternalID       *string    `json:"externalId,omitempty"`
	LastError        *string    `json:"lastError,omitempty"`
	LastActivityAt   *time.Time `json:"lastActivityAt,omitempty"`
}

// DashboardCatalogRow es un producto con el resultado de cada requisito de
// publicación, para mostrar todo lo que le falta y no solo el filtro activo.
type DashboardCatalogRow struct {
	ProductID    int64   `json:"productId"`
	SKU          string  `json:"sku"`
	ProductName  string  `json:"productName"`
	BrandName    *string `json:"brandName,omitempty"`
	CategoryName *string `json:"categoryName,omitempty"`
	HasImage     bool    `json:"hasImage"`
	HasPrice     bool    `json:"hasPrice"`
	HasBrand     bool    `json:"hasBrand"`
	HasCategory  bool    `json:"hasCategory"`
	HasStock     bool    `json:"hasStock"`
}

type DashboardConnectionHealth struct {
	ConnectionID   int64      `json:"connectionId"`
	ConnectionName string     `json:"connectionName"`
	ChannelName    string     `json:"channelName"`
	ChannelCode    string     `json:"channelCode"`
	Status         string     `json:"status"`
	Environment    string     `json:"environment"`
	HasStatus      bool       `json:"hasStatus"`
	Authenticated  bool       `json:"authenticated"`
	LastAuth       *time.Time `json:"lastAuth,omitempty"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	LastError      *string    `json:"lastError,omitempty"`
}

type DashboardInventoryHealth struct {
	SourceID     int64      `json:"sourceId"`
	SourceName   string     `json:"sourceName"`
	StockRows    int64      `json:"stockRows"`
	LastChangeAt *time.Time `json:"lastChangeAt,omitempty"`
}

type DashboardQueueHealth struct {
	ConnectionID    int64      `json:"connectionId"`
	ConnectionName  string     `json:"connectionName"`
	SyncType        string     `json:"syncType"`
	Pending         int64      `json:"pending"`
	Processing      int64      `json:"processing"`
	Failed          int64      `json:"failed"`
	OldestPendingAt *time.Time `json:"oldestPendingAt,omitempty"`
}

// DashboardRepository concentra las consultas de solo lectura del Dashboard.
type DashboardRepository struct {
	db Querier
}

func NewDashboardRepository(db Querier) *DashboardRepository {
	return &DashboardRepository{db: db}
}

// attentionFrom devuelve el FROM de un estado y sus argumentos.
// "not_published" sale de la bitácora; el resto, de las publicaciones vivas.
func attentionFrom(state string) (string, []any) {
	if state == ListingStateNotPublished {
		return ` FROM (` + listingViewNotPublishedSource + `) x`, nil
	}
	return ` FROM (` + listingViewLiveSource + `) x WHERE x.state = ?`, []any{state}
}

// FindAttentionPage devuelve una página de las publicaciones en el estado dado
// (todos los canales) junto con el total de ese estado.
func (r *DashboardRepository) FindAttentionPage(ctx context.Context, state string, offset, limit int) ([]DashboardAttentionRow, int64, error) {
	from, args := attentionFrom(state)

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("error counting listings in state %s: %w", state, err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT x.kind, x.id, x.product_id, x.sku, x.product_name, x.connection_name,
		       x.vehicle_fitment_id, x.listing_title, x.external_id, x.last_error, x.last_activity_at`+
		from+` ORDER BY x.last_activity_at DESC, x.id DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("error querying listings in state %s: %w", state, err)
	}
	defer rows.Close()

	out := make([]DashboardAttentionRow, 0)
	for rows.Next() {
		var v DashboardAttentionRow
		if err := rows.Scan(&v.Kind, &v.ID, &v.ProductID, &v.SKU, &v.ProductName, &v.ConnectionName,
			&v.VehicleFitmentID, &v.ListingTitle, &v.ExternalID, &v.LastError, &v.LastActivityAt); err != nil {
			return nil, 0, fmt.Errorf("error scanning listing in state %s: %w", state, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating listings in state %s: %w", state, err)
	}
	return out, total, nil
}

// Requisitos de publicación (mismo criterio que el descubrimiento del ADR 0003).
const (
	catalogHasImageSQL = `EXISTS (
		SELECT 1 FROM ecom_product_images pi
		WHERE pi.product_id = p.id AND pi.is_first = 1 AND pi.deleted_at IS NULL)`

	// Precio base efectivo: gana la lista vigente de mayor prioridad, igual que
	// ProductPricesRepository.FindEffectivePrice, y debe ser > 0. No evalúa
	// tipo de cambio ni la fórmula de la conexión.
	catalogHasPriceSQL = `COALESCE((
		SELECT pp.price FROM ecom_product_prices pp
		JOIN ecom_price_list pl ON pl.id = pp.price_list_id AND pl.deleted_at IS NULL
		WHERE pp.product_id = p.id
		  AND pl.status = 'active' AND CURDATE() BETWEEN pl.valid_from AND pl.valid_to
		ORDER BY pl.priority DESC, pp.updated_at DESC
		LIMIT 1), 0) > 0`

	catalogHasStockSQL = `COALESCE((
		SELECT SUM(ps.available_qty) FROM ecom_product_stock ps
		WHERE ps.product_id = p.id), 0) > 0`

	// status es nullable; NULL se trata como activo, igual que
	// ProductRepository.FindZeroStockEligibleForConnection.
	catalogBaseWhereSQL = `p.deleted_at IS NULL
		AND (p.status IS NULL OR p.status = 'active')
		AND p.is_sellable = 1`
)

// catalogReadySQL: cumple todos los requisitos y existe al menos una conexión
// activa (o la indicada) con la categoría mapeada, que acepte su stock y donde
// todavía no tenga publicación viva.
func catalogReadySQL(connectionID int64) (string, []any) {
	codes := make([]string, 0, len(zeroStockChannelCodes))
	for code := range zeroStockChannelCodes {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	args := make([]any, 0, len(codes)+1)
	connectionClause := ""
	if connectionID > 0 {
		connectionClause = " AND cc.id = ?"
		args = append(args, connectionID)
	}
	for _, code := range codes {
		args = append(args, code)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(codes)), ",")

	query := catalogHasImageSQL + `
		AND ` + catalogHasPriceSQL + `
		AND p.brand_id IS NOT NULL
		AND p.category_id IS NOT NULL
		AND EXISTS (
			SELECT 1 FROM ecom_channel_category_map ccm
			JOIN ecom_channel_connections cc ON cc.id = ccm.connection_id
			JOIN ecom_channels ch ON ch.id = cc.channel_id
			WHERE ccm.category_id = p.category_id AND ccm.deleted_at IS NULL
			  AND cc.deleted_at IS NULL AND cc.status = 'active'` + connectionClause + `
			  AND (` + catalogHasStockSQL + ` OR UPPER(TRIM(ch.code)) IN (` + placeholders + `))
			  AND NOT EXISTS (
				SELECT 1 FROM ecom_channel_product_map cpm
				WHERE cpm.product_id = p.id AND cpm.connection_id = cc.id
				  AND cpm.deleted_at IS NULL AND cpm.status <> 'closed'))`
	return query, args
}

// catalogWhere arma el WHERE de un filtro. Con connectionID > 0 el universo se
// reduce a los productos sin publicación viva en esa conexión, es decir, los
// que hoy no se están vendiendo ahí.
func catalogWhere(filter string, connectionID int64) (string, []any, error) {
	where := catalogBaseWhereSQL
	var args []any

	if connectionID > 0 {
		where += `
			AND NOT EXISTS (
				SELECT 1 FROM ecom_channel_product_map live
				WHERE live.product_id = p.id AND live.connection_id = ?
				  AND live.deleted_at IS NULL AND live.status <> 'closed')`
		args = append(args, connectionID)
	}

	switch filter {
	case CatalogFilterNoImage:
		where += ` AND NOT ` + catalogHasImageSQL
	case CatalogFilterNoPrice:
		where += ` AND NOT (` + catalogHasPriceSQL + `)`
	case CatalogFilterNoBrand:
		where += ` AND p.brand_id IS NULL`
	case CatalogFilterNoCategory:
		where += ` AND p.category_id IS NULL`
	case CatalogFilterNoStock:
		where += ` AND NOT (` + catalogHasStockSQL + `)`
	case CatalogFilterCategoryUnmapped:
		where += ` AND p.category_id IS NOT NULL
			AND NOT EXISTS (
				SELECT 1 FROM ecom_channel_category_map um
				WHERE um.category_id = p.category_id AND um.connection_id = ? AND um.deleted_at IS NULL)`
		args = append(args, connectionID)
	case CatalogFilterReadyUnpublished:
		ready, readyArgs := catalogReadySQL(connectionID)
		where += ` AND ` + ready
		args = append(args, readyArgs...)
	default:
		return "", nil, fmt.Errorf("unknown catalog filter %q", filter)
	}

	return where, args, nil
}

// CountCatalogProducts cuenta los productos que caen en un filtro.
func (r *DashboardRepository) CountCatalogProducts(ctx context.Context, filter string, connectionID int64) (int64, error) {
	where, args, err := catalogWhere(filter, connectionID)
	if err != nil {
		return 0, err
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ecom_products p WHERE `+where, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("error counting catalog filter %s: %w", filter, err)
	}
	return total, nil
}

// FindCatalogProducts devuelve una página de un filtro, productos más nuevos
// primero.
func (r *DashboardRepository) FindCatalogProducts(ctx context.Context, filter string, connectionID int64, offset, limit int) ([]DashboardCatalogRow, error) {
	where, args, err := catalogWhere(filter, connectionID)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.sku, p.name, b.name, c.name,
		       `+catalogHasImageSQL+`,
		       `+catalogHasPriceSQL+`,
		       p.brand_id IS NOT NULL,
		       p.category_id IS NOT NULL,
		       `+catalogHasStockSQL+`
		FROM ecom_products p
		LEFT JOIN ecom_brands b ON b.id = p.brand_id
		LEFT JOIN ecom_categories c ON c.id = p.category_id
		WHERE `+where+`
		ORDER BY p.id DESC
		LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, fmt.Errorf("error querying catalog filter %s: %w", filter, err)
	}
	defer rows.Close()

	out := make([]DashboardCatalogRow, 0)
	for rows.Next() {
		var v DashboardCatalogRow
		if err := rows.Scan(&v.ProductID, &v.SKU, &v.ProductName, &v.BrandName, &v.CategoryName,
			&v.HasImage, &v.HasPrice, &v.HasBrand, &v.HasCategory, &v.HasStock); err != nil {
			return nil, fmt.Errorf("error scanning catalog filter %s: %w", filter, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating catalog filter %s: %w", filter, err)
	}
	return out, nil
}

// FindConnectionsHealth devuelve cada conexión no eliminada con su último
// estado de autenticación (si existe).
func (r *DashboardRepository) FindConnectionsHealth(ctx context.Context) ([]DashboardConnectionHealth, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT cc.id, cc.name, ch.name, ch.code, COALESCE(cc.status, ''), COALESCE(cc.environment, ''),
		       cs.id IS NOT NULL, COALESCE(cs.authenticated, 0), cs.last_auth, cs.expires_at, cs.last_error
		FROM ecom_channel_connections cc
		JOIN ecom_channels ch ON ch.id = cc.channel_id
		LEFT JOIN ecom_connection_status cs ON cs.connection_id = cc.id
		WHERE cc.deleted_at IS NULL AND ch.deleted_at IS NULL
		ORDER BY ch.name, cc.name`)
	if err != nil {
		return nil, fmt.Errorf("error querying connections health: %w", err)
	}
	defer rows.Close()

	out := make([]DashboardConnectionHealth, 0)
	for rows.Next() {
		var v DashboardConnectionHealth
		if err := rows.Scan(&v.ConnectionID, &v.ConnectionName, &v.ChannelName, &v.ChannelCode, &v.Status,
			&v.Environment, &v.HasStatus, &v.Authenticated, &v.LastAuth, &v.ExpiresAt, &v.LastError); err != nil {
			return nil, fmt.Errorf("error scanning connection health: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating connections health: %w", err)
	}
	return out, nil
}

// FindInventoryHealth devuelve, por fuente, la última vez que una
// sincronización escribió una cantidad. last_sync_at no se sella cuando la
// cantidad no cambia, así que no equivale a "última corrida".
func (r *DashboardRepository) FindInventoryHealth(ctx context.Context) ([]DashboardInventoryHealth, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.name, COUNT(*), MAX(ps.last_sync_at)
		FROM ecom_product_stock ps
		JOIN ecom_products p ON p.id = ps.product_id AND p.deleted_at IS NULL
		JOIN ecom_sources s ON s.id = p.source_id
		GROUP BY s.id, s.name
		ORDER BY s.name`)
	if err != nil {
		return nil, fmt.Errorf("error querying inventory health: %w", err)
	}
	defer rows.Close()

	out := make([]DashboardInventoryHealth, 0)
	for rows.Next() {
		var v DashboardInventoryHealth
		var lastChange sql.NullTime
		if err := rows.Scan(&v.SourceID, &v.SourceName, &v.StockRows, &lastChange); err != nil {
			return nil, fmt.Errorf("error scanning inventory health: %w", err)
		}
		if lastChange.Valid {
			v.LastChangeAt = &lastChange.Time
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating inventory health: %w", err)
	}
	return out, nil
}

// FindQueueHealth resume lo que sigue sin completarse en la cola de
// sincronización, por conexión y tipo.
func (r *DashboardRepository) FindQueueHealth(ctx context.Context) ([]DashboardQueueHealth, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT q.connection_id, cc.name, q.sync_type,
		       CAST(SUM(q.status = 'pending') AS SIGNED),
		       CAST(SUM(q.status = 'processing') AS SIGNED),
		       CAST(SUM(q.status = 'failed') AS SIGNED),
		       MIN(CASE WHEN q.status = 'pending' THEN q.requested_at END)
		FROM ecom_channel_sync_queue q
		JOIN ecom_channel_connections cc ON cc.id = q.connection_id
		WHERE q.deleted_at IS NULL AND q.status IN ('pending', 'processing', 'failed')
		GROUP BY q.connection_id, cc.name, q.sync_type
		ORDER BY cc.name, q.sync_type`)
	if err != nil {
		return nil, fmt.Errorf("error querying sync queue health: %w", err)
	}
	defer rows.Close()

	out := make([]DashboardQueueHealth, 0)
	for rows.Next() {
		var v DashboardQueueHealth
		var oldest sql.NullTime
		if err := rows.Scan(&v.ConnectionID, &v.ConnectionName, &v.SyncType,
			&v.Pending, &v.Processing, &v.Failed, &oldest); err != nil {
			return nil, fmt.Errorf("error scanning sync queue health: %w", err)
		}
		if oldest.Valid {
			v.OldestPendingAt = &oldest.Time
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sync queue health: %w", err)
	}
	return out, nil
}

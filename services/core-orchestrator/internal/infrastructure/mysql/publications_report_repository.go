package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// Códigos de canal (ecom_channels.code) que el reporte de publicaciones
// expone como columnas propias.
const (
	reportChannelMercadoLibre = "MERCADOLIBRE"
	reportChannelOdoo         = "ODOO"
)

// publishedMapStatuses son los estados de ecom_channel_product_map que cuentan
// como "producto publicado/sincronizado" en un canal. 'pending' y 'error' aún
// no tienen publicación viva y 'closed' ya no la tiene.
const publishedMapStatuses = `'synced', 'under_review', 'paused'`

// PublicationsReportFilter acota el reporte. Los campos en cero no filtran.
type PublicationsReportFilter struct {
	SourceID     int64
	ChannelCode  string
	ConnectionID int64
	Search       string
	Offset       int
	PageSize     int
	// MissingChannelCode / MissingConnectionID restringen a productos que NO
	// están publicados en ese canal o conexión (comparación de faltantes).
	// Combinados con ChannelCode/ConnectionID dan "publicado en A y falta en B".
	MissingChannelCode  string
	MissingConnectionID int64
}

// PublicationReportConnection es la publicación de un producto en una
// conexión concreta. Si la conexión permite varias publicaciones por producto,
// Listings las cuenta y State es el estado "más vivo" de ellas.
type PublicationReportConnection struct {
	ConnectionID   int64  `json:"connectionId"`
	ConnectionName string `json:"connectionName"`
	State          string `json:"state"`
	Listings       int    `json:"listings"`
}

type PublicationReportPrice struct {
	Amount         string `json:"amount"`
	CurrencyCode   string `json:"currencyCode"`
	CurrencySymbol string `json:"currencySymbol"`
}

type PublicationReportRow struct {
	ProductID    int64                         `json:"productId"`
	SKU          string                        `json:"sku"`
	Name         string                        `json:"name"`
	PartNumber   string                        `json:"partNumber"`
	BrandName    *string                       `json:"brandName,omitempty"`
	SourceID     int64                         `json:"sourceId"`
	SourceName   string                        `json:"sourceName"`
	ImageCount   int64                         `json:"imageCount"`
	HasMktImages bool                          `json:"hasMktImages"`
	Stock        int64                         `json:"stock"`
	Price        *PublicationReportPrice       `json:"price,omitempty"`
	Odoo         []PublicationReportConnection `json:"odoo"`
	MercadoLibre []PublicationReportConnection `json:"mercadoLibre"`
}

type PaginatedPublicationsReport struct {
	Total    int64                  `json:"total"`
	Offset   int                    `json:"offset"`
	PageSize int                    `json:"pageSize"`
	Rows     []PublicationReportRow `json:"rows"`
}

type PublicationsReportRepository struct {
	db Querier
}

func NewPublicationsReportRepository(db Querier) *PublicationsReportRepository {
	return &PublicationsReportRepository{db: db}
}

// listingStateCase traduce ecom_channel_product_map.status/paused_reason a los
// mismos estados que usa la pantalla de Publicaciones (listingViewSource).
const listingStateCase = `CASE
	WHEN cpm.status = 'synced' THEN 'published'
	WHEN cpm.status = 'paused' AND cpm.paused_reason = 'zero_stock' THEN 'paused_zero_stock'
	WHEN cpm.status = 'paused' THEN 'paused_manual'
	WHEN cpm.status = 'closed' THEN 'closed'
	ELSE 'under_review'
END`

// listingStateRank ordena los estados de más a menos "vivo" para resumir
// varias publicaciones de una misma conexión en una sola.
var listingStateRank = map[string]int{
	ListingStatePublished:       0,
	ListingStateUnderReview:     1,
	ListingStatePausedManual:    2,
	ListingStatePausedZeroStock: 3,
	ListingStateClosed:          4,
}

// FindPublicationsReport devuelve una página de productos que están publicados
// en al menos un canal (que cumpla los filtros de canal/conexión), con su
// resumen de imágenes, stock, precio y las conexiones donde están publicados en
// Odoo y MercadoLibre.
func (r *PublicationsReportRepository) FindPublicationsReport(ctx context.Context, f PublicationsReportFilter) (*PaginatedPublicationsReport, error) {
	if f.PageSize <= 0 {
		f.PageSize = 50
	}
	if f.PageSize > 200 {
		f.PageSize = 200
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	// Subconsulta de "publicado": se reutiliza para filtrar y, con otro
	// alcance, para traer las conexiones de la página.
	args := []any{}
	publishedIn := func(code string, connectionID int64) string {
		q := `EXISTS (
		SELECT 1 FROM ecom_channel_product_map cpm
		JOIN ecom_channel_connections cc ON cc.id = cpm.connection_id AND cc.deleted_at IS NULL
		JOIN ecom_channels ch ON ch.id = cc.channel_id AND ch.deleted_at IS NULL
		WHERE cpm.product_id = p.id AND cpm.deleted_at IS NULL
		  AND cpm.status IN (` + publishedMapStatuses + `)`
		if code != "" {
			q += ` AND UPPER(TRIM(ch.code)) = UPPER(?)`
			args = append(args, code)
		}
		if connectionID > 0 {
			q += ` AND cc.id = ?`
			args = append(args, connectionID)
		}
		return q + `)`
	}

	where := []string{"p.deleted_at IS NULL", publishedIn(strings.TrimSpace(f.ChannelCode), f.ConnectionID)}
	missingCode := strings.TrimSpace(f.MissingChannelCode)
	if missingCode != "" || f.MissingConnectionID > 0 {
		where = append(where, "NOT "+publishedIn(missingCode, f.MissingConnectionID))
	}
	if f.SourceID > 0 {
		where = append(where, "p.source_id = ?")
		args = append(args, f.SourceID)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + s + "%"
		where = append(where, "(p.sku LIKE ? OR p.name LIKE ? OR p.part_number LIKE ?)")
		args = append(args, like, like, like)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ecom_products p WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("error counting publications report: %w", err)
	}

	// Mismo criterio de precio vigente que ProductRepository.FindPaginated:
	// la lista de precios activa/vigente de mayor prioridad.
	priceRow := `FROM ecom_product_prices pp
		JOIN ecom_price_list pl ON pl.id = pp.price_list_id AND pl.deleted_at IS NULL
		JOIN ecom_currencies cur ON cur.id = pp.currency
		WHERE pp.product_id = p.id
		  AND pl.status = 'active' AND CURDATE() BETWEEN pl.valid_from AND pl.valid_to
		ORDER BY pl.priority DESC, pp.updated_at DESC LIMIT 1`

	query := `
		SELECT p.id, p.sku, p.name, p.part_number,
			(SELECT b.name FROM ecom_brands b WHERE b.id = p.brand_id AND b.deleted_at IS NULL) AS brand_name,
			p.source_id,
			COALESCE((SELECT s.name FROM ecom_sources s WHERE s.id = p.source_id), '') AS source_name,
			(SELECT COUNT(*) FROM ecom_product_images pi
			   JOIN ecom_files f ON f.id = pi.file_id AND f.deleted_at IS NULL
			   WHERE pi.product_id = p.id AND pi.deleted_at IS NULL) AS image_count,
			EXISTS (SELECT 1 FROM ecom_product_images pi
			   JOIN ecom_files f ON f.id = pi.file_id AND f.deleted_at IS NULL
			   LEFT JOIN ecom_storage_disks sd ON sd.id = f.disk_id AND sd.deleted_at IS NULL
			   WHERE pi.product_id = p.id AND pi.deleted_at IS NULL
			     AND INSTR(CASE WHEN f.path LIKE 'http://%' OR f.path LIKE 'https://%'
			                    THEN f.path ELSE CONCAT(COALESCE(sd.base_url, ''), '/', f.path) END,
			               '_MKT_') > 0) AS has_mkt_images,
			CAST(COALESCE((SELECT SUM(ps.available_qty) FROM ecom_product_stock ps
			   WHERE ps.product_id = p.id), 0) AS SIGNED) AS total_stock,
			(SELECT pp.price ` + priceRow + `) AS price_amount,
			(SELECT cur.code ` + priceRow + `) AS price_currency_code,
			(SELECT cur.symbol ` + priceRow + `) AS price_currency_symbol
		FROM ecom_products p
		WHERE ` + whereSQL + `
		ORDER BY p.sku ASC, p.id ASC
		LIMIT ? OFFSET ?`

	pageArgs := append(append([]any{}, args...), f.PageSize, f.Offset)
	rows, err := r.db.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, fmt.Errorf("error querying publications report: %w", err)
	}
	defer rows.Close()

	out := make([]PublicationReportRow, 0)
	index := map[int64]int{}
	ids := make([]any, 0)
	for rows.Next() {
		var row PublicationReportRow
		var brand, priceAmount, priceCode, priceSymbol sql.NullString
		if err := rows.Scan(&row.ProductID, &row.SKU, &row.Name, &row.PartNumber, &brand,
			&row.SourceID, &row.SourceName, &row.ImageCount, &row.HasMktImages, &row.Stock,
			&priceAmount, &priceCode, &priceSymbol); err != nil {
			return nil, fmt.Errorf("error scanning publications report row: %w", err)
		}
		if brand.Valid {
			row.BrandName = &brand.String
		}
		if priceAmount.Valid {
			row.Price = &PublicationReportPrice{
				Amount:         priceAmount.String,
				CurrencyCode:   priceCode.String,
				CurrencySymbol: priceSymbol.String,
			}
		}
		row.Odoo = []PublicationReportConnection{}
		row.MercadoLibre = []PublicationReportConnection{}
		index[row.ProductID] = len(out)
		ids = append(ids, row.ProductID)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating publications report: %w", err)
	}

	if len(ids) > 0 {
		if err := r.attachConnections(ctx, out, index, ids); err != nil {
			return nil, err
		}
	}

	return &PaginatedPublicationsReport{
		Total:    total,
		Offset:   f.Offset,
		PageSize: f.PageSize,
		Rows:     out,
	}, nil
}

// attachConnections llena las columnas de Odoo y MercadoLibre de los productos
// de la página. Se resuelve con una segunda consulta (en vez de GROUP_CONCAT)
// para no toparse con group_concat_max_len en productos con muchas
// publicaciones por compatibilidad.
func (r *PublicationsReportRepository) attachConnections(ctx context.Context, out []PublicationReportRow, index map[int64]int, ids []any) error {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	rows, err := r.db.QueryContext(ctx, `
		SELECT cpm.product_id, UPPER(TRIM(ch.code)), cc.id, cc.name, `+listingStateCase+`
		FROM ecom_channel_product_map cpm
		JOIN ecom_channel_connections cc ON cc.id = cpm.connection_id AND cc.deleted_at IS NULL
		JOIN ecom_channels ch ON ch.id = cc.channel_id AND ch.deleted_at IS NULL
		WHERE cpm.product_id IN (`+placeholders+`) AND cpm.deleted_at IS NULL
		  AND cpm.status IN (`+publishedMapStatuses+`, 'closed')
		  AND UPPER(TRIM(ch.code)) IN ('`+reportChannelOdoo+`', '`+reportChannelMercadoLibre+`')
	`, ids...)
	if err != nil {
		return fmt.Errorf("error querying publications report connections: %w", err)
	}
	defer rows.Close()

	type key struct {
		productID    int64
		channel      string
		connectionID int64
	}
	agg := map[key]*PublicationReportConnection{}
	order := make([]key, 0)
	for rows.Next() {
		var k key
		var name, state string
		if err := rows.Scan(&k.productID, &k.channel, &k.connectionID, &name, &state); err != nil {
			return fmt.Errorf("error scanning publications report connection: %w", err)
		}
		if c, ok := agg[k]; ok {
			switch {
			case c.State == ListingStateClosed && state != ListingStateClosed:
				// Primera publicación viva: descarta el conteo de las cerradas.
				c.State, c.Listings = state, 1
			case state == ListingStateClosed && c.State != ListingStateClosed:
				// Una cerrada junto a publicaciones vivas no cuenta.
			default:
				c.Listings++
				if listingStateRank[state] < listingStateRank[c.State] {
					c.State = state
				}
			}
			continue
		}
		agg[k] = &PublicationReportConnection{ConnectionID: k.connectionID, ConnectionName: name, State: state, Listings: 1}
		order = append(order, k)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating publications report connections: %w", err)
	}

	sort.Slice(order, func(i, j int) bool {
		return agg[order[i]].ConnectionName < agg[order[j]].ConnectionName
	})
	for _, k := range order {
		pos, ok := index[k.productID]
		if !ok {
			continue
		}
		switch k.channel {
		case reportChannelOdoo:
			out[pos].Odoo = append(out[pos].Odoo, *agg[k])
		case reportChannelMercadoLibre:
			out[pos].MercadoLibre = append(out[pos].MercadoLibre, *agg[k])
		}
	}
	return nil
}

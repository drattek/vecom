package meli_report

import (
	"context"
	"database/sql"
	"fmt"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// listingsQuery: publicaciones con id externo en conexiones activas de
// MercadoLibre, con el stock total del producto (lo que se envía). Los dos
// primeros parámetros filtran una conexión (0 = todas).
const listingsQuery = `
	SELECT m.id, m.connection_id, cc.name, m.product_id, p.brand_id, p.sku, p.name,
	       m.external_id, m.status, m.paused_reason, m.last_synced_at,
	       COALESCE(st.quantity, 0), st.changed_at
	FROM ecom_channel_product_map m
	JOIN ecom_channel_connections cc ON cc.id = m.connection_id
	     AND cc.deleted_at IS NULL AND cc.status = 'active'
	JOIN ecom_channels ch ON ch.id = cc.channel_id AND UPPER(TRIM(ch.code)) = 'MERCADOLIBRE'
	JOIN ecom_products p ON p.id = m.product_id
	LEFT JOIN (
		SELECT product_id, SUM(available_qty) AS quantity, MAX(last_sync_at) AS changed_at
		FROM ecom_product_stock
		GROUP BY product_id
	) st ON st.product_id = m.product_id
	WHERE m.deleted_at IS NULL AND m.external_id IS NOT NULL AND m.external_id <> ''
	  AND (? = 0 OR m.connection_id = ?)
	ORDER BY m.connection_id, p.sku, m.id
`

// zeroStockPausesQuery: filas cuya última pausa por stock de este sistema es
// posterior a su última reactivación, según la bitácora.
const zeroStockPausesQuery = `
	SELECT channel_product_map_id
	FROM ecom_channel_listing_events
	WHERE channel_product_map_id IS NOT NULL AND outcome = 'success'
	GROUP BY channel_product_map_id
	HAVING MAX(CASE
	         WHEN (action = 'pause' AND message = 'pausado: sin stock disponible')
	           OR (action = 'publish' AND message LIKE 'publicado pausado (sin stock)%')
	         THEN id ELSE 0 END)
	     > MAX(CASE WHEN action = 'reactivate' THEN id ELSE 0 END)
`

// PriceCalculator calcula el precio en MXN que se enviaría hoy.
type PriceCalculator func(ctx context.Context, listing Listing) (price float64, changedAt sql.NullTime, err error)

// LoadListings lee las publicaciones a comparar. Solo hace SELECT.
func LoadListings(ctx context.Context, db mysqlInfra.Querier, connectionID int64, price PriceCalculator) ([]Listing, error) {
	ownPauses, err := loadOwnPauses(ctx, db)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, listingsQuery, connectionID, connectionID)
	if err != nil {
		return nil, fmt.Errorf("listing publications: %w", err)
	}
	defer rows.Close()

	listings := make([]Listing, 0)
	for rows.Next() {
		var (
			listing           Listing
			mapID             int64
			brandID           sql.NullInt64
			reason            sql.NullString
			pushedAt, changed sql.NullTime
		)
		if err := rows.Scan(&mapID, &listing.ConnectionID, &listing.ConnectionName, &listing.ProductID, &brandID,
			&listing.SKU, &listing.ProductName, &listing.ExternalID, &listing.Status, &reason, &pushedAt,
			&listing.Stock, &changed); err != nil {
			return nil, fmt.Errorf("reading publication: %w", err)
		}
		if brandID.Valid {
			listing.BrandID = &brandID.Int64
		}
		listing.PausedReason = reason.String
		if pushedAt.Valid {
			listing.LastPushAt = &pushedAt.Time
		}
		if changed.Valid {
			listing.StockChangedAt = &changed.Time
		}
		listing.PausedForStockByUs = ownPauses[mapID]
		listings = append(listings, listing)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing publications: %w", err)
	}
	rows.Close()

	if price == nil {
		return listings, nil
	}
	// Después de cerrar la consulta anterior, para no ocupar dos conexiones.
	type priceKey struct{ product, connection int64 }
	type priced struct {
		value     float64
		changedAt sql.NullTime
		err       error
	}
	cache := map[priceKey]priced{}
	for i := range listings {
		key := priceKey{listings[i].ProductID, listings[i].ConnectionID}
		result, ok := cache[key]
		if !ok {
			result.value, result.changedAt, result.err = price(ctx, listings[i])
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			cache[key] = result
		}
		if result.err != nil {
			listings[i].PriceNote = "No se pudo calcular: " + result.err.Error()
			continue
		}
		value := result.value
		listings[i].ExpectedPrice = &value
		if result.changedAt.Valid {
			changedAt := result.changedAt.Time
			listings[i].PriceChangedAt = &changedAt
		}
	}
	return listings, nil
}

func loadOwnPauses(ctx context.Context, db mysqlInfra.Querier) (map[int64]bool, error) {
	rows, err := db.QueryContext(ctx, zeroStockPausesQuery)
	if err != nil {
		return nil, fmt.Errorf("reading pause history: %w", err)
	}
	defer rows.Close()

	paused := map[int64]bool{}
	for rows.Next() {
		var mapID int64
		if err := rows.Scan(&mapID); err != nil {
			return nil, fmt.Errorf("reading pause history: %w", err)
		}
		paused[mapID] = true
	}
	return paused, rows.Err()
}

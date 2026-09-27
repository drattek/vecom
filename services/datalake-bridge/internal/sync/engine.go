// Package sync implementa el diff auto-contenido D365 (datalake) -> Odoo para
// máquinas. No usa RabbitMQ ni base intermedia: en cada pasada lee el estado
// deseado del datalake y el estado actual de Odoo, y solo escribe las
// diferencias (series nuevas, series que desaparecieron). En estado estable
// (sin cambios) hace solo lecturas — misma optimización que el gate de cambios
// del sync de refacciones, sin el peso de las colas para tan poco volumen.
package sync

import (
	"context"
	"fmt"
	"log/slog"

	"datalake-bridge/internal/datalake"
	"datalake-bridge/internal/odoo"
	"datalake-bridge/internal/warehouse"
)

// Reader es lo que el motor necesita del datalake (facilita las pruebas).
type Reader interface {
	Machines(ctx context.Context) ([]datalake.Unit, error)
}

// Engine orquesta una pasada de sincronización.
type Engine struct {
	reader   Reader
	odoo     *odoo.Client
	category string
	dryRun   bool
	log      *slog.Logger
}

// New crea el motor de sync.
func New(reader Reader, oc *odoo.Client, category string, dryRun bool, log *slog.Logger) *Engine {
	return &Engine{reader: reader, odoo: oc, category: category, dryRun: dryRun, log: log}
}

// Result resume lo que hizo una pasada.
type Result struct {
	DesiredUnits    int
	ProductsCreated int
	LotsCreated     int
	Placed          int // series colocadas/actualizadas a 1
	Removed         int // series puestas en 0 (ya no están en D365)
	Unchanged       int // series que ya estaban bien (no se tocó Odoo)
	Skipped         int // filas descartadas (almacén sin mapear, etc.)
}

// productVariantID de la variante para un default_code, con caché por pasada.
type engineState struct {
	categoryID int64
	warehouses map[string]odoo.Warehouse // código Odoo -> almacén resuelto
	products   map[string]odoo.Product   // default_code (ItemID) -> variante
	lots       map[string]int64          // "productID|serial" -> lotID
}

// desiredKey identifica una unidad deseada por su ubicación física destino.
type placedKey struct {
	productID  int64
	lotID      int64
	locationID int64
}

// Run ejecuta una pasada completa del diff.
func (e *Engine) Run(ctx context.Context) (Result, error) {
	var res Result

	units, err := e.reader.Machines(ctx)
	if err != nil {
		return res, fmt.Errorf("leer máquinas del datalake: %w", err)
	}
	res.DesiredUnits = len(units)
	e.log.Info("máquinas leídas del datalake", "unidades", len(units))

	st := &engineState{
		warehouses: map[string]odoo.Warehouse{},
		products:   map[string]odoo.Product{},
		lots:       map[string]int64{},
	}

	// Categoría de máquinas (crea si falta).
	if !e.dryRun {
		id, err := e.odoo.EnsureCategory(ctx, e.category)
		if err != nil {
			return res, fmt.Errorf("asegurar categoría %q: %w", e.category, err)
		}
		st.categoryID = id
	}

	// Resolver almacenes usados y reunir sus ubicaciones de stock.
	locationIDs, err := e.resolveWarehouses(ctx, units, st)
	if err != nil {
		return res, err
	}

	// Estado actual en Odoo: existencias por serie (lote) en esas ubicaciones.
	current, err := e.odoo.SerialQuants(ctx, locationIDs)
	if err != nil {
		return res, fmt.Errorf("leer existencias actuales de Odoo: %w", err)
	}
	currentByKey := make(map[placedKey]odoo.Quant, len(current))
	for _, q := range current {
		currentByKey[placedKey{q.ProductID.ID, q.LotID.ID, q.LocationID.ID}] = q
	}
	e.log.Info("existencias de serie actuales en Odoo", "quants", len(current))

	// Recorrer el estado deseado y colocar lo que cambie.
	seen := make(map[placedKey]bool, len(units))
	for _, u := range units {
		code, err := warehouse.OdooCode(u.Company, u.Site)
		if err != nil {
			e.log.Warn("unidad descartada", "serie", u.Serial, "motivo", err.Error())
			res.Skipped++
			continue
		}
		wh, ok := st.warehouses[code]
		if !ok {
			res.Skipped++
			continue
		}
		locID := wh.LotStockID.ID

		// Producto (modelo) — cotizable exista o no exista stock.
		prod, err := e.ensureProduct(ctx, u, st, &res)
		if err != nil {
			return res, err
		}

		// Lote (serie).
		lotID, err := e.ensureLot(ctx, prod.ID, wh.CompanyID.ID, u.Serial, st, &res)
		if err != nil {
			return res, err
		}

		key := placedKey{prod.ID, lotID, locID}
		seen[key] = true

		if q, ok := currentByKey[key]; ok && q.Quantity == 1 {
			res.Unchanged++
			continue
		}

		if e.dryRun {
			e.log.Info("[dry-run] colocaría serie", "serie", u.Serial, "modelo", u.ItemID, "almacen", code)
			res.Placed++
			continue
		}
		if err := e.odoo.SetInventory(ctx, prod.ID, lotID, locID, 1); err != nil {
			return res, fmt.Errorf("colocar serie %q (%s) en %s: %w", u.Serial, u.ItemID, code, err)
		}
		res.Placed++
	}

	// Reconciliación: series que en Odoo tienen existencia pero ya no están en
	// D365 (vendidas/movidas) -> a 0.
	for key, q := range currentByKey {
		if seen[key] || q.Quantity == 0 {
			continue
		}
		if e.dryRun {
			e.log.Info("[dry-run] retiraría serie", "lot_id", key.lotID, "location_id", key.locationID)
			res.Removed++
			continue
		}
		if err := e.odoo.SetInventory(ctx, key.productID, key.lotID, key.locationID, 0); err != nil {
			return res, fmt.Errorf("retirar serie lot_id=%d loc=%d: %w", key.lotID, key.locationID, err)
		}
		res.Removed++
	}

	e.log.Info("pasada de sync completada",
		"deseadas", res.DesiredUnits,
		"productos_creados", res.ProductsCreated,
		"lotes_creados", res.LotsCreated,
		"colocadas", res.Placed,
		"retiradas", res.Removed,
		"sin_cambio", res.Unchanged,
		"descartadas", res.Skipped,
		"dry_run", e.dryRun,
	)
	return res, nil
}

// resolveWarehouses resuelve, contra Odoo, cada código de almacén que aparece en
// las unidades y devuelve las ubicaciones de stock destino.
func (e *Engine) resolveWarehouses(ctx context.Context, units []datalake.Unit, st *engineState) ([]int64, error) {
	needed := map[string]bool{}
	for _, u := range units {
		code, err := warehouse.OdooCode(u.Company, u.Site)
		if err != nil {
			continue // se reporta luego, por unidad
		}
		needed[code] = true
	}
	var locationIDs []int64
	for code := range needed {
		wh, ok, err := e.odoo.ResolveWarehouse(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("resolver almacén %q: %w", code, err)
		}
		if !ok {
			e.log.Warn("almacén no existe en Odoo; sus unidades se descartan", "code", code)
			continue
		}
		st.warehouses[code] = wh
		locationIDs = append(locationIDs, wh.LotStockID.ID)
	}
	if len(locationIDs) == 0 {
		return nil, fmt.Errorf("ningún almacén resuelto en Odoo (revisa el mapeo)")
	}
	return locationIDs, nil
}

func (e *Engine) ensureProduct(ctx context.Context, u datalake.Unit, st *engineState, res *Result) (odoo.Product, error) {
	if p, ok := st.products[u.ItemID]; ok {
		return p, nil
	}
	p, found, err := e.odoo.FindProduct(ctx, u.ItemID)
	if err != nil {
		return odoo.Product{}, fmt.Errorf("buscar producto %q: %w", u.ItemID, err)
	}
	if !found {
		if e.dryRun {
			e.log.Info("[dry-run] crearía producto", "modelo", u.ItemID, "nombre", u.Name)
			res.ProductsCreated++
			// En dry-run no hay id real; se usa 0 y se cachea para no repetir.
			st.products[u.ItemID] = odoo.Product{DefaultCode: u.ItemID}
			return st.products[u.ItemID], nil
		}
		p, err = e.odoo.CreateMachineProduct(ctx, u.ItemID, u.Name, st.categoryID)
		if err != nil {
			return odoo.Product{}, fmt.Errorf("crear producto %q: %w", u.ItemID, err)
		}
		res.ProductsCreated++
	}
	st.products[u.ItemID] = p
	return p, nil
}

func (e *Engine) ensureLot(ctx context.Context, productID, companyID int64, serial string, st *engineState, res *Result) (int64, error) {
	key := fmt.Sprintf("%d|%s", productID, serial)
	if id, ok := st.lots[key]; ok {
		return id, nil
	}
	if e.dryRun && productID == 0 {
		// producto ficticio de dry-run; no se puede buscar lote real
		res.LotsCreated++
		return 0, nil
	}
	lot, found, err := e.odoo.FindLot(ctx, productID, serial)
	if err != nil {
		return 0, fmt.Errorf("buscar lote %q: %w", serial, err)
	}
	if !found {
		if e.dryRun {
			res.LotsCreated++
			st.lots[key] = 0
			return 0, nil
		}
		lot, err = e.odoo.CreateLot(ctx, productID, companyID, serial)
		if err != nil {
			return 0, fmt.Errorf("crear lote %q: %w", serial, err)
		}
		res.LotsCreated++
	}
	st.lots[key] = lot.ID
	return lot.ID, nil
}

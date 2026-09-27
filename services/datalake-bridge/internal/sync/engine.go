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
	"math"

	"datalake-bridge/internal/brand"
	"datalake-bridge/internal/datalake"
	"datalake-bridge/internal/odoo"
	"datalake-bridge/internal/warehouse"
)

// Reader es lo que el motor necesita del datalake (facilita las pruebas).
type Reader interface {
	Machines(ctx context.Context) ([]datalake.Unit, error)
	Models(ctx context.Context) ([]datalake.Model, error)
}

// uomUnitsName es la unidad de Odoo a la que mapea la unidad "pz" de D365.
const uomUnitsName = "Units"

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
	ModelsEnriched  int // modelos existentes cuyos campos (marca/precio/…) se actualizaron
	BrandsCreated   int // product.brand creados
	LotsCreated     int
	Placed          int // series colocadas/actualizadas a 1
	Removed         int // series puestas en 0 (ya no están en D365)
	Unchanged       int // series que ya estaban bien (no se tocó Odoo)
	Skipped         int // filas descartadas (almacén sin mapear, etc.)
}

// engineState es la caché por pasada.
type engineState struct {
	categoryID int64
	uomID      int64                     // id de la UoM "Units" (0 = no resuelta)
	capField   string                    // campo custom de capacidad ("" = no disponible)
	warehouses map[string]odoo.Warehouse // código Odoo -> almacén resuelto
	products   map[string]odoo.Product   // default_code (ItemID) -> variante
	models     map[string]datalake.Model // ItemID -> datos de modelo (datalake)
	brands     map[string]int64          // nombre de marca -> product.brand id
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

	models, err := e.reader.Models(ctx)
	if err != nil {
		return res, fmt.Errorf("leer modelos del datalake: %w", err)
	}

	st := &engineState{
		warehouses: map[string]odoo.Warehouse{},
		products:   map[string]odoo.Product{},
		models:     make(map[string]datalake.Model, len(models)),
		brands:     map[string]int64{},
		lots:       map[string]int64{},
	}
	for _, m := range models {
		if _, ok := st.models[m.ItemID]; !ok {
			st.models[m.ItemID] = m
		}
	}
	e.log.Info("modelos leídos del datalake", "modelos", len(st.models))

	// Categoría de máquinas (crea si falta).
	if !e.dryRun {
		id, err := e.odoo.EnsureCategory(ctx, e.category)
		if err != nil {
			return res, fmt.Errorf("asegurar categoría %q: %w", e.category, err)
		}
		st.categoryID = id
	}

	// Campo custom de capacidad (se crea si falta; en dry-run solo se detecta).
	capField, err := e.odoo.EnsureCapacidadField(ctx, e.dryRun)
	if err != nil {
		return res, fmt.Errorf("asegurar campo de capacidad: %w", err)
	}
	st.capField = capField
	e.odoo.SetCapacidadField(capField)

	// UoM "Units" (a la que mapea "pz" de D365).
	if uomID, ok, err := e.odoo.ResolveUomID(ctx, uomUnitsName); err != nil {
		return res, fmt.Errorf("resolver UoM %q: %w", uomUnitsName, err)
	} else if ok {
		st.uomID = uomID
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
		"modelos_enriquecidos", res.ModelsEnriched,
		"marcas_creadas", res.BrandsCreated,
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

	model := st.models[u.ItemID] // vacío si no está; los campos vienen en 0/""
	brandID, err := e.brandID(ctx, model, st, res)
	if err != nil {
		return odoo.Product{}, err
	}
	capacidad := ""
	if st.capField != "" {
		capacidad = capacityFromName(model.Name)
	}

	p, found, err := e.odoo.FindProduct(ctx, u.ItemID)
	if err != nil {
		return odoo.Product{}, fmt.Errorf("buscar producto %q: %w", u.ItemID, err)
	}
	if !found {
		if e.dryRun {
			e.log.Info("[dry-run] crearía producto", "modelo", u.ItemID, "nombre", u.Name,
				"marca_id", brandID, "precio", model.SalePrice, "capacidad", capacidad)
			res.ProductsCreated++
			st.products[u.ItemID] = odoo.Product{DefaultCode: u.ItemID}
			return st.products[u.ItemID], nil
		}
		extra := e.createVals(model, brandID, capacidad, st)
		p, err = e.odoo.CreateMachineProduct(ctx, u.ItemID, u.Name, st.categoryID, extra)
		if err != nil {
			return odoo.Product{}, fmt.Errorf("crear producto %q: %w", u.ItemID, err)
		}
		res.ProductsCreated++
		st.products[u.ItemID] = p
		return p, nil
	}

	// Producto existente: gate — solo escribe los campos de modelo que cambiaron.
	diff := modelDiff(p, model, brandID, capacidad)
	if len(diff) > 0 {
		if e.dryRun {
			e.log.Info("[dry-run] enriquecería modelo", "modelo", u.ItemID, "campos", keys(diff))
			res.ModelsEnriched++
		} else if err := e.odoo.UpdateModel(ctx, p.Tmpl.ID, diff); err != nil {
			return odoo.Product{}, fmt.Errorf("enriquecer modelo %q: %w", u.ItemID, err)
		} else {
			res.ModelsEnriched++
		}
	}
	st.products[u.ItemID] = p
	return p, nil
}

// brandID resuelve (y cachea) el id de product.brand para un modelo; 0 si no se
// pudo determinar la marca.
func (e *Engine) brandID(ctx context.Context, model datalake.Model, st *engineState, res *Result) (int64, error) {
	name := brand.Resolve(model.BrandCode, model.MarcaCod)
	if name == "" {
		return 0, nil
	}
	if id, ok := st.brands[name]; ok {
		return id, nil
	}
	if e.dryRun {
		st.brands[name] = 0
		return 0, nil
	}
	id, created, err := e.odoo.EnsureBrand(ctx, name)
	if err != nil {
		return 0, fmt.Errorf("asegurar marca %q: %w", name, err)
	}
	if created {
		res.BrandsCreated++
	}
	st.brands[name] = id
	return id, nil
}

// createVals arma los campos de modelo a fijar al crear el producto.
func (e *Engine) createVals(model datalake.Model, brandID int64, capacidad string, st *engineState) map[string]any {
	extra := map[string]any{}
	if brandID > 0 {
		extra["product_brand_id"] = brandID
	}
	if model.SalePrice > 0 {
		extra["list_price"] = model.SalePrice
	}
	if model.Cost > 0 {
		extra["standard_price"] = model.Cost
	}
	if model.Weight > 0 {
		extra["weight"] = model.Weight
	}
	if model.Volume > 0 {
		extra["volume"] = model.Volume
	}
	if st.uomID > 0 {
		extra["uom_id"] = st.uomID
	}
	if st.capField != "" && capacidad != "" {
		extra[st.capField] = capacidad
	}
	return extra
}

// modelDiff devuelve solo los campos de modelo que difieren del producto actual
// (nunca sobrescribe con 0/"" un valor existente).
func modelDiff(p odoo.Product, model datalake.Model, brandID int64, capacidad string) map[string]any {
	diff := map[string]any{}
	if brandID > 0 && p.BrandID.ID != brandID {
		diff["product_brand_id"] = brandID
	}
	if model.SalePrice > 0 && !floatEq(p.ListPrice, model.SalePrice) {
		diff["list_price"] = model.SalePrice
	}
	if model.Cost > 0 && !floatEq(p.StandardPrice, model.Cost) {
		diff["standard_price"] = model.Cost
	}
	if model.Weight > 0 && !floatEq(p.Weight, model.Weight) {
		diff["weight"] = model.Weight
	}
	if model.Volume > 0 && !floatEq(p.Volume, model.Volume) {
		diff["volume"] = model.Volume
	}
	if capacidad != "" && p.Capacidad != capacidad {
		diff["x_capacidad"] = capacidad
	}
	return diff
}

func floatEq(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
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

package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"core-orchestrator/internal/domain"
	mysqlRepo "core-orchestrator/internal/infrastructure/mysql"
)

const (
	nissanSourceCode = "NISSAN"
	nissanSourceName = "Nissan"
	nissanPriceList  = "precios_nissan"
	mxnCurrencyCode  = "MXN"
)

// Mapeo de PROD_TIPOREFA (campo "type" en domain.Existencia) a ecom_products.product_type.
// Un tipo no reconocido cae en "part" (mismo default de la columna).
func nissanProductType(t string) string {
	switch t {
	case "REFNIS", "ARTVAR", "PROMOC", "AIRNIS", "OTRREF":
		return "part"
	case "ACCNIS", "OTRACC":
		return "accessory"
	case "LUBNIS", "OTRLUB":
		return "consumable"
	default:
		return "part"
	}
}

// nissanSyncCache resuelve una sola vez, por corrida de sync, los valores que antes se
// volvían a buscar en MySQL para cada uno de los 50k+ SKUs (fuente Nissan, moneda MXN,
// lista de precios, y las pocas decenas de sucursales/almacenes distintos). Reutilizarlos
// desde memoria elimina la mayoría de los SELECT redundantes que hacían que el sync
// completo tomara horas. No requiere locking: se usa secuencialmente, un SKU a la vez.
type nissanSyncCache struct {
	sourceID      int64
	mxnCurrencyID int64
	priceListID   int64
	branchIDs     map[string]int64
	warehouseIDs  map[int64]int64

	// pendingOldPartNumbers/pendingNewPartNumbers son los part numbers que, al momento de
	// arrancar la corrida, tenían una sucesión (ecom_part_number_supersessions) pendiente de
	// resolver por ese lado. run() solo llama a ResolveForProduct para un SKU si aparece en
	// alguno de estos sets (o si el propio registro declara una sucesión), en vez de hacerlo
	// siempre — evita 2 SELECT redundantes por cada uno de los miles de SKUs que nunca tienen
	// una sucesión relacionada. syncPartNumberSupersession agrega a pendingNewPartNumbers en
	// caliente cuando crea/actualiza una sucesión cuyo sucesor todavía no existe, para que si
	// ese SKU aparece más adelante en esta misma corrida, también dispare su resolución.
	pendingOldPartNumbers map[string]bool
	pendingNewPartNumbers map[string]bool
}

// prepareNissanSyncCache resuelve fuente/moneda/lista de precios una vez, antes de procesar
// cualquier SKU, en vez de dentro de la transacción de cada uno.
func prepareNissanSyncCache(ctx context.Context, db *sql.DB) (*nissanSyncCache, error) {
	sourcesRepo := mysqlRepo.NewSourcesRepository(db)
	currenciesRepo := mysqlRepo.NewCurrenciesRepository(db)
	priceListRepo := mysqlRepo.NewPriceListRepository(db)

	source, err := findOrCreateNissanSource(ctx, sourcesRepo)
	if err != nil {
		return nil, fmt.Errorf("resolving Nissan source: %w", err)
	}

	mxn, err := currenciesRepo.FindByCode(ctx, mxnCurrencyCode)
	if err != nil {
		return nil, fmt.Errorf("MXN currency not found in ecom_currencies: %w", err)
	}

	priceList, err := findOrCreateNissanPriceList(ctx, priceListRepo, mxn.ID)
	if err != nil {
		return nil, fmt.Errorf("resolving price list %q: %w", nissanPriceList, err)
	}

	partNumberSupersessionsRepo := mysqlRepo.NewPartNumberSupersessionsRepository(db)
	pendingOld, pendingNew, err := partNumberSupersessionsRepo.FindPendingPartNumbers(ctx, source.ID)
	if err != nil {
		return nil, fmt.Errorf("preloading pending part number supersessions: %w", err)
	}

	return &nissanSyncCache{
		sourceID:              source.ID,
		mxnCurrencyID:         mxn.ID,
		priceListID:           priceList.ID,
		branchIDs:             make(map[string]int64),
		warehouseIDs:          make(map[int64]int64),
		pendingOldPartNumbers: pendingOld,
		pendingNewPartNumbers: pendingNew,
	}, nil
}

func findOrCreateNissanSource(ctx context.Context, repo *mysqlRepo.SourcesRepository) (*mysqlRepo.SourceDTO, error) {
	source, err := repo.FindByCode(ctx, nissanSourceCode)
	if err == nil {
		return source, nil
	}
	if !errors.Is(err, mysqlRepo.ErrSourceNotFound) {
		return nil, err
	}

	return repo.Create(ctx, mysqlRepo.CreateSourceInput{
		Code:      nissanSourceCode,
		Name:      nissanSourceName,
		CreatedBy: systemUserID,
	})
}

func findOrCreateNissanPriceList(ctx context.Context, repo *mysqlRepo.PriceListRepository, currencyID int64) (*mysqlRepo.PriceListDTO, error) {
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	priceList, err := repo.FindByName(ctx, nissanPriceList)
	if err != nil {
		if !errors.Is(err, mysqlRepo.ErrPriceListNotFound) {
			return nil, err
		}

		priceList, err = repo.Create(ctx, mysqlRepo.CreatePriceListInput{
			Name:      nissanPriceList,
			Currency:  currencyID,
			Priority:  0,
			Status:    "active",
			ValidFrom: today,
			ValidTo:   tomorrow,
			CreatedBy: systemUserID,
		})
		if err != nil {
			return nil, err
		}
	}

	// La lista debe extenderse en cada sync, exista o se acabe de crear.
	return repo.Update(ctx, priceList.ID, mysqlRepo.UpdatePriceListInput{
		Name:      nissanPriceList,
		Currency:  currencyID,
		Priority:  0,
		Status:    priceList.Status,
		ValidFrom: priceList.ValidFrom,
		ValidTo:   tomorrow,
		UpdatedBy: systemUserID,
	})
}

// nissanSyncBatchSize es cuántos registros de Nissan se procesan dentro de una misma
// transacción MySQL. Antes cada uno de los 8000+ SKUs abría y confirmaba su propia
// transacción, y cada COMMIT implica un fsync — con miles de SKUs eso por sí solo dominaba
// el tiempo total del sync. Compartir la transacción entre un lote reduce esa cantidad de
// commits en el mismo orden que el tamaño del lote. Un registro que falla dentro del lote no
// aborta a los demás (ver ProcessNissanExistenciaBatch): en MySQL, a diferencia de Postgres,
// un error de aplicación (fila no encontrada, etc.) no deja la transacción inutilizable para
// las siguientes sentencias.
const nissanSyncBatchSize = 250

// ProcessNissanExistenciaBatch persiste en MySQL un lote de registros de existencias de
// Nissan leídos de Redis: producto (ecom_products), stock (ecom_product_stock/
// ecom_stock_movements) y precio (ecom_product_prices/ecom_price_history), todos dentro de
// una única transacción compartida por el lote (ver nissanSyncBatchSize). Devuelve cuántos
// registros se sincronizaron correctamente; los que fallan se saltan sin abortar el resto
// del lote, igual que antes cuando cada registro tenía su propia transacción.
func (s *SyncService) ProcessNissanExistenciaBatch(ctx context.Context, batch []*domain.Existencia, cache *nissanSyncCache, seen *stockSeen) (synced, failed int, err error) {
	// Un solo BEGIN/COMMIT por lote (ver nissanSyncBatchSize): con miles de SKUs,
	// una transacción por registro hacía que el fsync del COMMIT dominara el sync.
	// Un registro que falla se saltea sin abortar el lote (en MySQL un error de
	// aplicación no inutiliza la transacción para las siguientes sentencias).
	txErr := mysqlRepo.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		t := newNissanSyncTx(tx, seen)
		for _, e := range batch {
			if runErr := t.run(ctx, e, cache); runErr != nil {
				failed++
				continue
			}
			synced++
		}
		return nil
	})
	if txErr != nil {
		return synced, failed, fmt.Errorf("committing batch transaction: %w", txErr)
	}

	return synced, failed, nil
}

// nissanSyncTx agrupa los repos MySQL "scoped" a una única *sql.Tx: cada instancia se usa
// para un lote entero de registros (ver ProcessNissanExistenciaBatch) y se descarta al
// terminar (el commit se hace por fuera, en ProcessNissanExistenciaBatch). Fuente/moneda/
// lista de precios ya no viven aquí: se resuelven una sola vez por corrida en
// nissanSyncCache.
type nissanSyncTx struct {
	productRepo                 *mysqlRepo.ProductRepository
	branchesRepo                *mysqlRepo.BranchesRepository
	warehousesRepo              *mysqlRepo.WarehousesRepository
	productStockRepo            *mysqlRepo.ProductStockRepository
	stockMovementsRepo          *mysqlRepo.StockMovementsRepository
	productPricesRepo           *mysqlRepo.ProductPricesRepository
	priceHistoryRepo            *mysqlRepo.PriceHistoryRepository
	partNumberSupersessionsRepo *mysqlRepo.PartNumberSupersessionsRepository

	// seen acumula qué (producto, sucursal, almacén) vino en la corrida, para la
	// reconciliación por ausencia que corre al final (ver sync_stock_reconcile.go).
	// Es compartido por todos los lotes de la corrida.
	seen *stockSeen
}

func newNissanSyncTx(tx *sql.Tx, seen *stockSeen) *nissanSyncTx {
	return &nissanSyncTx{
		productRepo:                 mysqlRepo.NewProductRepository(tx),
		branchesRepo:                mysqlRepo.NewBranchesRepository(tx),
		warehousesRepo:              mysqlRepo.NewWarehousesRepository(tx),
		productStockRepo:            mysqlRepo.NewProductStockRepository(tx),
		stockMovementsRepo:          mysqlRepo.NewStockMovementsRepository(tx),
		productPricesRepo:           mysqlRepo.NewProductPricesRepository(tx),
		priceHistoryRepo:            mysqlRepo.NewPriceHistoryRepository(tx),
		partNumberSupersessionsRepo: mysqlRepo.NewPartNumberSupersessionsRepository(tx),
		seen:                        seen,
	}
}

// run ejecuta el flujo para un registro. Si el SKU ya existe en ecom_products, esa fila
// nunca se toca: run solo crea el producto la primera vez que aparece (createNissanProduct)
// y en cualquier corrida posterior va derecho a stock/precio. syncProductStock y
// syncNissanPrice ya comparan contra lo que hay en MySQL antes de escribir (mismo criterio
// de "no escribir si no cambió" que evitaba las UPDATE/INSERT redundantes en los 50k+ SKUs
// que, en estado estable, no cambian entre una corrida y la siguiente), así que no hace
// falta un chequeo combinado por separado.
func (t *nissanSyncTx) run(ctx context.Context, e *domain.Existencia, cache *nissanSyncCache) error {
	existingProduct, err := t.productRepo.FindBySKU(ctx, e.Code)
	isNewProduct := errors.Is(err, mysqlRepo.ErrProductNotFound)
	if err != nil && !isNewProduct {
		return fmt.Errorf("looking up product %s: %w", e.Code, err)
	}

	branchID, err := t.resolveBranch(ctx, e.AgencyName, cache)
	if err != nil {
		return fmt.Errorf("resolving branch %q: %w", e.AgencyName, err)
	}

	warehouseID, err := t.resolveWarehouse(ctx, e.AgencyName, branchID, cache)
	if err != nil {
		return fmt.Errorf("resolving warehouse %q: %w", e.AgencyName, err)
	}

	if err := t.syncPartNumberSupersession(ctx, cache, e); err != nil {
		return fmt.Errorf("syncing part number supersession for product %s: %w", e.Code, err)
	}

	productID := int64(0)
	if isNewProduct {
		product, err := t.createNissanProduct(ctx, e, cache)
		if err != nil {
			return fmt.Errorf("creating product %s: %w", e.Code, err)
		}
		productID = product.ID
	} else {
		productID = existingProduct.ID
	}

	// Solo vale la pena llamar a ResolveForProduct (2 SELECT) si este SKU realmente puede
	// tener algo que resolver: declara una sucesión él mismo, o aparece en alguno de los sets
	// precargados en cache (ver pendingOldPartNumbers/pendingNewPartNumbers). Para la gran
	// mayoría de los SKUs, que nunca participan de una sucesión, esto evita la consulta.
	if e.SupersededByPartNumber != "" || cache.pendingOldPartNumbers[e.Code] || cache.pendingNewPartNumbers[e.Code] {
		if err := t.partNumberSupersessionsRepo.ResolveForProduct(ctx, cache.sourceID, productID, e.Code, systemUserID); err != nil {
			return fmt.Errorf("resolving part number supersessions for product %s: %w", e.Code, err)
		}
	}

	// Se marca antes de escribir: la fuente reportó este SKU@almacén con stock > 0,
	// así que la reconciliación por ausencia no debe bajarlo a 0 aunque el write falle.
	t.seen.mark(productID, branchID, warehouseID)

	if err := t.syncProductStock(ctx, productID, branchID, warehouseID, e.Stock); err != nil {
		return fmt.Errorf("syncing stock for product %s: %w", e.Code, err)
	}

	if err := t.syncNissanPrice(ctx, productID, cache.mxnCurrencyID, cache.priceListID, e.CostProm); err != nil {
		return fmt.Errorf("syncing price for product %s: %w", e.Code, err)
	}

	return nil
}

// syncPartNumberSupersession guarda o actualiza el estado actual de la sucesión reportada por
// PROD_SUPERSESION para este renglón, con el mismo criterio de "no escribir si no cambió" que
// syncProductStock/syncNissanPrice: la gran mayoría de los SKUs no
// traen supersesión, y entre los que sí, la mayoría no cambia de una corrida a otra.
//
// El lado "viejo" (old_part_number) no necesita resolverse aquí: siempre es el propio e.Code,
// y run() ya lo resuelve unas líneas más abajo contra el producto que se acaba de crear o
// encontrar en esta misma transacción. El lado "nuevo" (new_part_number) sí se busca
// activamente aquí — si el producto sucesor ya existe, se vincula en el momento en vez de
// esperar a que ese producto pase por su propio run() (lo cual solo pasa si su SKU aparece en
// esta misma corrida o en una futura).
func (t *nissanSyncTx) syncPartNumberSupersession(ctx context.Context, cache *nissanSyncCache, e *domain.Existencia) error {
	if e.SupersededByPartNumber == "" {
		return nil
	}

	var row *mysqlRepo.PartNumberSupersessionDTO

	existing, err := t.partNumberSupersessionsRepo.FindByOldPartNumber(ctx, cache.sourceID, e.Code)
	switch {
	case err == nil:
		row = existing
		if existing.NewPartNumber != e.SupersededByPartNumber {
			row, err = t.partNumberSupersessionsRepo.Update(ctx, existing.ID, mysqlRepo.UpdatePartNumberSupersessionInput{
				NewPartNumber: e.SupersededByPartNumber,
				UpdatedBy:     systemUserID,
			})
			if err != nil {
				return err
			}
		}
	case errors.Is(err, mysqlRepo.ErrPartNumberSupersessionNotFound):
		row, err = t.partNumberSupersessionsRepo.Create(ctx, mysqlRepo.CreatePartNumberSupersessionInput{
			SourceID:      cache.sourceID,
			OldPartNumber: e.Code,
			NewPartNumber: e.SupersededByPartNumber,
			CreatedBy:     systemUserID,
		})
		if err != nil {
			return err
		}
	default:
		return err
	}

	if row.NewProductID != nil {
		// Ya vinculado (de una corrida anterior, o porque arriba no cambió el valor y ya
		// estaba resuelto). Nada más que hacer.
		return nil
	}

	// El ERP solo reporta el part_number del sucesor, no un sku — si más de un producto de
	// este source llegara a compartir ese part_number (ecom_products ya no lo impide a nivel
	// de base de datos), FindBySourceAndPartNumber resuelve al más antiguo (ORDER BY id ASC)
	// de forma determinística en vez de a uno arbitrario.
	successor, err := t.productRepo.FindBySourceAndPartNumber(ctx, cache.sourceID, e.SupersededByPartNumber)
	if errors.Is(err, mysqlRepo.ErrProductNotFound) {
		// El sucesor todavía no existe como producto: la fila queda con new_product_id en
		// NULL. Se marca en cache para que, si ese SKU aparece más adelante en esta misma
		// corrida, run() sepa que debe llamar a ResolveForProduct (si no aparece en esta
		// corrida, se resolverá en la siguiente vía pendingNewPartNumbers precargado en
		// prepareNissanSyncCache).
		cache.pendingNewPartNumbers[e.SupersededByPartNumber] = true
		return nil
	}
	if err != nil {
		return fmt.Errorf("looking up successor product for part number %s: %w", e.SupersededByPartNumber, err)
	}

	return t.partNumberSupersessionsRepo.MarkNewResolved(ctx, row.ID, successor.ID, systemUserID)
}

func (t *nissanSyncTx) resolveBranch(ctx context.Context, name string, cache *nissanSyncCache) (int64, error) {
	if id, ok := cache.branchIDs[name]; ok {
		return id, nil
	}

	branch, err := t.findOrCreateBranch(ctx, name)
	if err != nil {
		return 0, err
	}

	cache.branchIDs[name] = branch.ID
	return branch.ID, nil
}

func (t *nissanSyncTx) resolveWarehouse(ctx context.Context, name string, branchID int64, cache *nissanSyncCache) (int64, error) {
	if id, ok := cache.warehouseIDs[branchID]; ok {
		return id, nil
	}

	warehouse, err := t.findOrCreateWarehouse(ctx, name, branchID)
	if err != nil {
		return 0, err
	}

	cache.warehouseIDs[branchID] = warehouse.ID
	return warehouse.ID, nil
}

// createNissanProduct crea la fila en ecom_products la primera vez que un SKU de Nissan
// aparece. Un SKU ya existente nunca pasa por acá: run va derecho a stock/precio sin tocar
// ecom_products, para no pisar curación manual hecha desde el admin-dashboard (marca,
// categoría, status, descripción, e incluso name/product_type, que antes se resincronizaban
// desde Nissan en cada refresh de stock/precio).
func (t *nissanSyncTx) createNissanProduct(ctx context.Context, e *domain.Existencia, cache *nissanSyncCache) (*mysqlRepo.ProductDTO, error) {
	return t.productRepo.Create(ctx, mysqlRepo.CreateProductInput{
		SKU:         e.Code,
		PartNumber:  e.Code,
		Name:        e.Description,
		ProductType: nissanProductType(e.Type),
		IsSellable:  true,
		IsStockable: true,
		SourceID:    cache.sourceID,
		CreatedBy:   systemUserID,
	})
}

func (t *nissanSyncTx) findOrCreateBranch(ctx context.Context, name string) (*mysqlRepo.BranchDTO, error) {
	branch, err := t.branchesRepo.FindByName(ctx, name)
	if err == nil {
		return branch, nil
	}
	if !errors.Is(err, mysqlRepo.ErrBranchNotFound) {
		return nil, err
	}

	return t.branchesRepo.Create(ctx, mysqlRepo.CreateBranchInput{
		Name:      name,
		CreatedBy: systemUserID,
	})
}

func (t *nissanSyncTx) findOrCreateWarehouse(ctx context.Context, name string, branchID int64) (*mysqlRepo.WarehouseDTO, error) {
	warehouse, err := t.warehousesRepo.FindByNameAndBranch(ctx, name, branchID)
	if err == nil {
		return warehouse, nil
	}
	if !errors.Is(err, mysqlRepo.ErrWarehouseNotFound) {
		return nil, err
	}

	return t.warehousesRepo.Create(ctx, mysqlRepo.CreateWarehouseInput{
		Name:      name,
		BranchID:  branchID,
		CreatedBy: systemUserID,
	})
}

func (t *nissanSyncTx) syncProductStock(ctx context.Context, productID, branchID, warehouseID int64, newQty int) error {
	before := 0

	existing, err := t.productStockRepo.FindByProductBranchWarehouse(ctx, productID, branchID, warehouseID)
	switch {
	case err == nil:
		if existing.AvailableQty == newQty {
			return nil
		}

		before = existing.AvailableQty
		if _, updateErr := t.productStockRepo.Update(ctx, existing.ID, mysqlRepo.UpdateProductStockInput{
			AvailableQty: newQty,
			UpdatedBy:    systemUserID,
		}); updateErr != nil {
			return updateErr
		}
	case errors.Is(err, mysqlRepo.ErrProductStockNotFound):
		if _, createErr := t.productStockRepo.Create(ctx, mysqlRepo.CreateProductStockInput{
			ProductID:    productID,
			BranchID:     branchID,
			WarehouseID:  warehouseID,
			AvailableQty: newQty,
		}); createErr != nil {
			return createErr
		}
	default:
		return err
	}

	_, err = t.stockMovementsRepo.Create(ctx, mysqlRepo.CreateStockMovementInput{
		ProductID:      productID,
		BranchID:       branchID,
		WarehouseID:    warehouseID,
		MovementType:   "sync",
		QuantityBefore: before,
		QuantityChange: newQty - before,
		QuantityAfter:  newQty,
		UpdatedBy:      systemUserID,
	})

	return err
}

func (t *nissanSyncTx) syncNissanPrice(ctx context.Context, productID, currencyID, priceListID int64, costProm float64) error {
	newPrice := fmt.Sprintf("%.2f", costProm)
	oldPrice := "0.00"

	existingPrice, err := t.productPricesRepo.FindByProductAndPriceList(ctx, productID, priceListID)
	switch {
	case err == nil:
		if existingPrice.Price == newPrice {
			return nil
		}

		oldPrice = existingPrice.Price
		if _, updateErr := t.productPricesRepo.Update(ctx, existingPrice.ID, mysqlRepo.UpdateProductPriceInput{
			Price:       newPrice,
			Margin:      existingPrice.Margin,
			TaxIncluded: false,
			UpdatedBy:   systemUserID,
		}); updateErr != nil {
			return updateErr
		}
	case errors.Is(err, mysqlRepo.ErrProductPriceNotFound):
		if _, createErr := t.productPricesRepo.Create(ctx, mysqlRepo.CreateProductPriceInput{
			ProductID:   productID,
			PriceListID: priceListID,
			Price:       newPrice,
			Currency:    currencyID,
			Margin:      "0.00",
			TaxIncluded: false,
			UpdatedBy:   systemUserID,
		}); createErr != nil {
			return createErr
		}
	default:
		return err
	}

	if oldPrice == newPrice {
		return nil
	}

	_, err = t.priceHistoryRepo.Create(ctx, mysqlRepo.CreatePriceHistoryInput{
		ProductID:   productID,
		PriceListID: priceListID,
		CurrencyID:  currencyID,
		OldPrice:    oldPrice,
		NewPrice:    newPrice,
		UpdatedBy:   systemUserID,
	})

	return err
}

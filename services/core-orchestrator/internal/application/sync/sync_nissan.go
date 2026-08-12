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
}

// prepareNissanSyncCache resuelve fuente/moneda/lista de precios una vez, antes de procesar
// cualquier SKU, en vez de dentro de la transacción de cada uno.
func prepareNissanSyncCache(db *sql.DB) (*nissanSyncCache, error) {
	sourcesRepo := mysqlRepo.NewSourcesRepository(db)
	currenciesRepo := mysqlRepo.NewCurrenciesRepository(db)
	priceListRepo := mysqlRepo.NewPriceListRepository(db)

	source, err := findOrCreateNissanSource(sourcesRepo)
	if err != nil {
		return nil, fmt.Errorf("resolving Nissan source: %w", err)
	}

	mxn, err := currenciesRepo.FindByCode(mxnCurrencyCode)
	if err != nil {
		return nil, fmt.Errorf("MXN currency not found in ecom_currencies: %w", err)
	}

	priceList, err := findOrCreateNissanPriceList(priceListRepo, mxn.ID)
	if err != nil {
		return nil, fmt.Errorf("resolving price list %q: %w", nissanPriceList, err)
	}

	return &nissanSyncCache{
		sourceID:      source.ID,
		mxnCurrencyID: mxn.ID,
		priceListID:   priceList.ID,
		branchIDs:     make(map[string]int64),
		warehouseIDs:  make(map[int64]int64),
	}, nil
}

func findOrCreateNissanSource(repo *mysqlRepo.SourcesRepository) (*mysqlRepo.SourceDTO, error) {
	source, err := repo.FindByCode(nissanSourceCode)
	if err == nil {
		return source, nil
	}
	if !errors.Is(err, mysqlRepo.ErrSourceNotFound) {
		return nil, err
	}

	return repo.Create(mysqlRepo.CreateSourceInput{
		Code:      nissanSourceCode,
		Name:      nissanSourceName,
		CreatedBy: systemUserID,
	})
}

func findOrCreateNissanPriceList(repo *mysqlRepo.PriceListRepository, currencyID int64) (*mysqlRepo.PriceListDTO, error) {
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	priceList, err := repo.FindByName(nissanPriceList)
	if err != nil {
		if !errors.Is(err, mysqlRepo.ErrPriceListNotFound) {
			return nil, err
		}

		priceList, err = repo.Create(mysqlRepo.CreatePriceListInput{
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
	return repo.Update(priceList.ID, mysqlRepo.UpdatePriceListInput{
		Name:      nissanPriceList,
		Currency:  currencyID,
		Priority:  0,
		Status:    priceList.Status,
		ValidFrom: priceList.ValidFrom,
		ValidTo:   tomorrow,
		UpdatedBy: systemUserID,
	})
}

// ProcessNissanExistencia persiste en MySQL un registro de existencias de Nissan leído
// de Redis: producto (ecom_products), stock (ecom_product_stock/ecom_stock_movements) y
// precio (ecom_product_prices/ecom_price_history). Las escrituras que esto implica corren
// dentro de una única transacción: si cualquier paso falla, se revierte todo en vez de
// dejar, por ejemplo, stock actualizado sin su movimiento correspondiente. Fuente, moneda,
// lista de precios, sucursal y almacén ya vienen resueltos en cache, por lo que esta
// transacción solo toca producto/stock/precio.
func (s *SyncService) ProcessNissanExistencia(ctx context.Context, e *domain.Existencia, cache *nissanSyncCache) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	if err := newNissanSyncTx(tx).run(e, cache); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// nissanSyncTx agrupa los repos MySQL "scoped" a una única *sql.Tx: cada instancia se usa
// para un solo ProcessNissanExistencia y se descarta al terminar (se hace commit/rollback
// por fuera, en ProcessNissanExistencia). Fuente/moneda/lista de precios ya no viven aquí:
// se resuelven una sola vez por corrida en nissanSyncCache.
type nissanSyncTx struct {
	productRepo                 *mysqlRepo.ProductRepository
	branchesRepo                *mysqlRepo.BranchesRepository
	warehousesRepo              *mysqlRepo.WarehousesRepository
	productStockRepo            *mysqlRepo.ProductStockRepository
	stockMovementsRepo          *mysqlRepo.StockMovementsRepository
	productPricesRepo           *mysqlRepo.ProductPricesRepository
	priceHistoryRepo            *mysqlRepo.PriceHistoryRepository
	partNumberSupersessionsRepo *mysqlRepo.PartNumberSupersessionsRepository
}

func newNissanSyncTx(tx *sql.Tx) *nissanSyncTx {
	return &nissanSyncTx{
		productRepo:                 mysqlRepo.NewProductRepository(tx),
		branchesRepo:                mysqlRepo.NewBranchesRepository(tx),
		warehousesRepo:              mysqlRepo.NewWarehousesRepository(tx),
		productStockRepo:            mysqlRepo.NewProductStockRepository(tx),
		stockMovementsRepo:          mysqlRepo.NewStockMovementsRepository(tx),
		productPricesRepo:           mysqlRepo.NewProductPricesRepository(tx),
		priceHistoryRepo:            mysqlRepo.NewPriceHistoryRepository(tx),
		partNumberSupersessionsRepo: mysqlRepo.NewPartNumberSupersessionsRepository(tx),
	}
}

// run ejecuta el flujo para un registro. Primero resuelve si el SKU ya existe: si existe
// y su stock/precio actuales en MySQL son idénticos a lo que llegó de Nissan, corta ahí
// (return nil) sin tocar ecom_products/ecom_product_stock/ecom_product_prices — en estado
// estable la gran mayoría de los 50k+ SKUs no cambian entre una corrida y la siguiente, y
// escribir cada uno de todas formas (UPDATE + INSERT movimiento + UPDATE precio) era lo que
// hacía tan lento el sync completo. Si el SKU es nuevo, o si cambió stock o precio, se
// ejecuta el flujo completo (product_id -> branch_id/warehouse_id -> stock -> precio).
func (t *nissanSyncTx) run(e *domain.Existencia, cache *nissanSyncCache) error {
	existingProduct, err := t.productRepo.FindBySKU(e.Code)
	isNewProduct := errors.Is(err, mysqlRepo.ErrProductNotFound)
	if err != nil && !isNewProduct {
		return fmt.Errorf("looking up product %s: %w", e.Code, err)
	}

	branchID, err := t.resolveBranch(e.AgencyName, cache)
	if err != nil {
		return fmt.Errorf("resolving branch %q: %w", e.AgencyName, err)
	}

	warehouseID, err := t.resolveWarehouse(e.AgencyName, branchID, cache)
	if err != nil {
		return fmt.Errorf("resolving warehouse %q: %w", e.AgencyName, err)
	}

	// Independiente del "unchanged" de stock/precio: PROD_SUPERSESION puede empezar a
	// reportarse (o cambiar) para un SKU cuyo stock/precio no se movieron en esta corrida.
	if err := t.syncPartNumberSupersession(cache.sourceID, e); err != nil {
		return fmt.Errorf("syncing part number supersession for product %s: %w", e.Code, err)
	}

	if !isNewProduct {
		unchanged, err := t.stockAndPriceUnchanged(existingProduct.ID, branchID, warehouseID, cache.priceListID, e)
		if err != nil {
			return fmt.Errorf("checking current state for %s: %w", e.Code, err)
		}
		if unchanged {
			// Aun sin cambios de stock/precio, hay que reintentar la resolución: una
			// sucesión pendiente pudo haber quedado esperando a este producto (que ya
			// existía) desde una corrida anterior.
			if err := t.partNumberSupersessionsRepo.ResolveForProduct(cache.sourceID, existingProduct.ID, e.Code, systemUserID); err != nil {
				return fmt.Errorf("resolving part number supersessions for product %s: %w", e.Code, err)
			}
			return nil
		}
	}

	product, err := t.upsertNissanProduct(e, existingProduct, isNewProduct, cache)
	if err != nil {
		return fmt.Errorf("upserting product %s: %w", e.Code, err)
	}

	if err := t.partNumberSupersessionsRepo.ResolveForProduct(cache.sourceID, product.ID, e.Code, systemUserID); err != nil {
		return fmt.Errorf("resolving part number supersessions for product %s: %w", e.Code, err)
	}

	if err := t.syncProductStock(product.ID, branchID, warehouseID, e.Stock); err != nil {
		return fmt.Errorf("syncing stock for product %s: %w", e.Code, err)
	}

	if err := t.syncNissanPrice(product.ID, cache.mxnCurrencyID, cache.priceListID, e.CostProm); err != nil {
		return fmt.Errorf("syncing price for product %s: %w", e.Code, err)
	}

	return nil
}

// syncPartNumberSupersession guarda o actualiza el estado actual de la sucesión reportada por
// PROD_SUPERSESION para este renglón, con el mismo criterio de "no escribir si no cambió" que
// stockAndPriceUnchanged/syncProductStock/syncNissanPrice: la gran mayoría de los SKUs no
// traen supersesión, y entre los que sí, la mayoría no cambia de una corrida a otra.
//
// El lado "viejo" (old_part_number) no necesita resolverse aquí: siempre es el propio e.Code,
// y run() ya lo resuelve unas líneas más abajo contra el producto que se acaba de crear o
// encontrar en esta misma transacción. El lado "nuevo" (new_part_number) sí se busca
// activamente aquí — si el producto sucesor ya existe, se vincula en el momento en vez de
// esperar a que ese producto pase por su propio run() (lo cual solo pasa si su SKU aparece en
// esta misma corrida o en una futura).
func (t *nissanSyncTx) syncPartNumberSupersession(sourceID int64, e *domain.Existencia) error {
	if e.SupersededByPartNumber == "" {
		return nil
	}

	var row *mysqlRepo.PartNumberSupersessionDTO

	existing, err := t.partNumberSupersessionsRepo.FindByOldPartNumber(sourceID, e.Code)
	switch {
	case err == nil:
		row = existing
		if existing.NewPartNumber != e.SupersededByPartNumber {
			row, err = t.partNumberSupersessionsRepo.Update(existing.ID, mysqlRepo.UpdatePartNumberSupersessionInput{
				NewPartNumber: e.SupersededByPartNumber,
				UpdatedBy:     systemUserID,
			})
			if err != nil {
				return err
			}
		}
	case errors.Is(err, mysqlRepo.ErrPartNumberSupersessionNotFound):
		row, err = t.partNumberSupersessionsRepo.Create(mysqlRepo.CreatePartNumberSupersessionInput{
			SourceID:      sourceID,
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

	successor, err := t.productRepo.FindBySourceAndPartNumber(sourceID, e.SupersededByPartNumber)
	if errors.Is(err, mysqlRepo.ErrProductNotFound) {
		// El sucesor todavía no existe como producto: la fila queda con new_product_id en
		// NULL, y se resolverá más adelante cuando ese SKU pase por su propio run() (ver
		// ResolveForProduct en run()).
		return nil
	}
	if err != nil {
		return fmt.Errorf("looking up successor product for part number %s: %w", e.SupersededByPartNumber, err)
	}

	return t.partNumberSupersessionsRepo.MarkNewResolved(row.ID, successor.ID, systemUserID)
}

// stockAndPriceUnchanged compara el stock/precio que ya está en MySQL contra lo que llegó
// de Nissan para este SKU. Si cualquiera de los dos no existe todavía (primera vez que este
// producto tiene stock o precio en esta sucursal/lista), se considera "cambiado" para que el
// flujo normal lo cree.
func (t *nissanSyncTx) stockAndPriceUnchanged(productID, branchID, warehouseID, priceListID int64, e *domain.Existencia) (bool, error) {
	stock, err := t.productStockRepo.FindByProductBranchWarehouse(productID, branchID, warehouseID)
	if errors.Is(err, mysqlRepo.ErrProductStockNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if stock.AvailableQty != e.Stock {
		return false, nil
	}

	price, err := t.productPricesRepo.FindByProductAndPriceList(productID, priceListID)
	if errors.Is(err, mysqlRepo.ErrProductPriceNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return price.Price == fmt.Sprintf("%.2f", e.CostProm), nil
}

func (t *nissanSyncTx) resolveBranch(name string, cache *nissanSyncCache) (int64, error) {
	if id, ok := cache.branchIDs[name]; ok {
		return id, nil
	}

	branch, err := t.findOrCreateBranch(name)
	if err != nil {
		return 0, err
	}

	cache.branchIDs[name] = branch.ID
	return branch.ID, nil
}

func (t *nissanSyncTx) resolveWarehouse(name string, branchID int64, cache *nissanSyncCache) (int64, error) {
	if id, ok := cache.warehouseIDs[branchID]; ok {
		return id, nil
	}

	warehouse, err := t.findOrCreateWarehouse(name, branchID)
	if err != nil {
		return 0, err
	}

	cache.warehouseIDs[branchID] = warehouse.ID
	return warehouse.ID, nil
}

func (t *nissanSyncTx) upsertNissanProduct(e *domain.Existencia, existing *mysqlRepo.ProductDTO, isNew bool, cache *nissanSyncCache) (*mysqlRepo.ProductDTO, error) {
	productType := nissanProductType(e.Type)

	if isNew {
		return t.productRepo.Create(mysqlRepo.CreateProductInput{
			SKU:         e.Code,
			PartNumber:  e.Code,
			Name:        e.Description,
			ProductType: productType,
			IsSellable:  true,
			IsStockable: true,
			SourceID:    cache.sourceID,
			CreatedBy:   systemUserID,
		})
	}

	// Preserva campos que no pertenecen a este sync (marca, categoría, status, descripción
	// larga) para no pisar curación manual hecha desde el admin-dashboard.
	return t.productRepo.Update(existing.ID, mysqlRepo.UpdateProductInput{
		SKU:              e.Code,
		PartNumber:       e.Code,
		Name:             e.Description,
		Description:      existing.Description,
		ShortDescription: existing.ShortDescription,
		BrandID:          existing.BrandID,
		CategoryID:       existing.CategoryID,
		ProductType:      productType,
		Status:           existing.Status,
		IsSellable:       existing.IsSellable,
		IsStockable:      existing.IsStockable,
		SourceID:         cache.sourceID,
		UpdatedBy:        systemUserID,
	})
}

func (t *nissanSyncTx) findOrCreateBranch(name string) (*mysqlRepo.BranchDTO, error) {
	branch, err := t.branchesRepo.FindByName(name)
	if err == nil {
		return branch, nil
	}
	if !errors.Is(err, mysqlRepo.ErrBranchNotFound) {
		return nil, err
	}

	return t.branchesRepo.Create(mysqlRepo.CreateBranchInput{
		Name:      name,
		CreatedBy: systemUserID,
	})
}

func (t *nissanSyncTx) findOrCreateWarehouse(name string, branchID int64) (*mysqlRepo.WarehouseDTO, error) {
	warehouse, err := t.warehousesRepo.FindByNameAndBranch(name, branchID)
	if err == nil {
		return warehouse, nil
	}
	if !errors.Is(err, mysqlRepo.ErrWarehouseNotFound) {
		return nil, err
	}

	return t.warehousesRepo.Create(mysqlRepo.CreateWarehouseInput{
		Name:      name,
		BranchID:  branchID,
		CreatedBy: systemUserID,
	})
}

func (t *nissanSyncTx) syncProductStock(productID, branchID, warehouseID int64, newQty int) error {
	before := 0

	existing, err := t.productStockRepo.FindByProductBranchWarehouse(productID, branchID, warehouseID)
	switch {
	case err == nil:
		if existing.AvailableQty == newQty {
			return nil
		}

		before = existing.AvailableQty
		if _, updateErr := t.productStockRepo.Update(existing.ID, mysqlRepo.UpdateProductStockInput{
			AvailableQty: newQty,
			UpdatedBy:    systemUserID,
		}); updateErr != nil {
			return updateErr
		}
	case errors.Is(err, mysqlRepo.ErrProductStockNotFound):
		if _, createErr := t.productStockRepo.Create(mysqlRepo.CreateProductStockInput{
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

	_, err = t.stockMovementsRepo.Create(mysqlRepo.CreateStockMovementInput{
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

func (t *nissanSyncTx) syncNissanPrice(productID, currencyID, priceListID int64, costProm float64) error {
	newPrice := fmt.Sprintf("%.2f", costProm)
	oldPrice := "0.00"

	existingPrice, err := t.productPricesRepo.FindByProductAndPriceList(productID, priceListID)
	switch {
	case err == nil:
		if existingPrice.Price == newPrice {
			return nil
		}

		oldPrice = existingPrice.Price
		if _, updateErr := t.productPricesRepo.Update(existingPrice.ID, mysqlRepo.UpdateProductPriceInput{
			Price:       newPrice,
			Margin:      existingPrice.Margin,
			TaxIncluded: false,
			UpdatedBy:   systemUserID,
		}); updateErr != nil {
			return updateErr
		}
	case errors.Is(err, mysqlRepo.ErrProductPriceNotFound):
		if _, createErr := t.productPricesRepo.Create(mysqlRepo.CreateProductPriceInput{
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

	_, err = t.priceHistoryRepo.Create(mysqlRepo.CreatePriceHistoryInput{
		ProductID:   productID,
		PriceListID: priceListID,
		CurrencyID:  currencyID,
		OldPrice:    oldPrice,
		NewPrice:    newPrice,
		UpdatedBy:   systemUserID,
	})

	return err
}

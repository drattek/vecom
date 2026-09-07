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

const machineryPriceList = "precios_maquinaria"

// erpStockSyncCache resuelve una sola vez, por corrida de sync, moneda y lista de precios
// (mismo criterio que nissanSyncCache en sync_nissan.go), y cachea sucursales/almacenes ya
// resueltos. A diferencia de Nissan, donde agencia y almacén son 1:1, en el ERP general una
// misma sucursal puede tener varios almacenes distintos, por lo que warehouseIDs se indexa
// por sucursal+almacén y no solo por sucursal.
type erpStockSyncCache struct {
	// sourceID es la fuente DYNAMICS: acota la reconciliación por ausencia
	// (reconcileZeroedStock) al stock de este ERP y no al de Nissan.
	sourceID      int64
	mxnCurrencyID int64
	priceListID   int64
	branchIDs     map[string]int64
	warehouseIDs  map[string]int64
}

func prepareErpStockSyncCache(ctx context.Context, db *sql.DB) (*erpStockSyncCache, error) {
	sourcesRepo := mysqlRepo.NewSourcesRepository(db)
	currenciesRepo := mysqlRepo.NewCurrenciesRepository(db)
	priceListRepo := mysqlRepo.NewPriceListRepository(db)

	source, err := findOrCreateDynamicsSource(ctx, sourcesRepo)
	if err != nil {
		return nil, fmt.Errorf("resolving Dynamics source: %w", err)
	}

	mxn, err := currenciesRepo.FindByCode(ctx, mxnCurrencyCode)
	if err != nil {
		return nil, fmt.Errorf("MXN currency not found in ecom_currencies: %w", err)
	}

	priceList, err := findOrCreateMachineryPriceList(ctx, priceListRepo, mxn.ID)
	if err != nil {
		return nil, fmt.Errorf("resolving price list %q: %w", machineryPriceList, err)
	}

	return &erpStockSyncCache{
		sourceID:      source.ID,
		mxnCurrencyID: mxn.ID,
		priceListID:   priceList.ID,
		branchIDs:     make(map[string]int64),
		warehouseIDs:  make(map[string]int64),
	}, nil
}

// findOrCreateMachineryPriceList sigue el mismo criterio rodante que findOrCreateNissanPriceList
// (se extiende en cada corrida, exista o se acabe de crear), pero con una ventana de validez de
// 10 años en vez de 1 día.
func findOrCreateMachineryPriceList(ctx context.Context, repo *mysqlRepo.PriceListRepository, currencyID int64) (*mysqlRepo.PriceListDTO, error) {
	today := time.Now().Format("2006-01-02")
	validTo := time.Now().AddDate(10, 0, 0).Format("2006-01-02")

	priceList, err := repo.FindByName(ctx, machineryPriceList)
	if err != nil {
		if !errors.Is(err, mysqlRepo.ErrPriceListNotFound) {
			return nil, err
		}

		priceList, err = repo.Create(ctx, mysqlRepo.CreatePriceListInput{
			Name:      machineryPriceList,
			Currency:  currencyID,
			Priority:  0,
			Status:    "active",
			ValidFrom: today,
			ValidTo:   validTo,
			CreatedBy: systemUserID,
		})
		if err != nil {
			return nil, err
		}
	}

	return repo.Update(ctx, priceList.ID, mysqlRepo.UpdatePriceListInput{
		Name:      machineryPriceList,
		Currency:  currencyID,
		Priority:  0,
		Status:    priceList.Status,
		ValidFrom: priceList.ValidFrom,
		ValidTo:   validTo,
		UpdatedBy: systemUserID,
	})
}

// ProcessERPStock persiste en MySQL un registro de stock/precio del ERP general
// (dyn.ItemInventLocation) leído de Redis, con el mismo criterio que ProcessNissanExistencia
// (sync_nissan.go): stock (ecom_product_stock/ecom_stock_movements) y precio
// (ecom_product_prices/ecom_price_history) dentro de una única transacción.
//
// A diferencia de Nissan, acá el producto ya debe existir en ecom_products (sincronizado antes
// vía ProcessERPProduct a partir de EcomProducts): si el SKU no existe todavía, el registro se
// descarta (skipped=true) en vez de crear el producto. Por eso la búsqueda del producto se hace
// antes de abrir transacción: evita abrir/cerrar una transacción vacía por cada SKU que se
// termina saltando.
func (s *SyncService) ProcessERPStock(ctx context.Context, inv *domain.Inventory, cache *erpStockSyncCache, seen *stockSeen) (skipped bool, err error) {
	productRepo := mysqlRepo.NewProductRepository(s.db)

	product, err := productRepo.FindBySKU(ctx, inv.Code)
	if errors.Is(err, mysqlRepo.ErrProductNotFound) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("looking up product %s: %w", inv.Code, err)
	}

	if err := mysqlRepo.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		return newErpStockSyncTx(tx, seen).run(ctx, product.ID, inv, cache)
	}); err != nil {
		return false, err
	}

	return false, nil
}

// erpStockSyncTx agrupa los repos MySQL "scoped" a una única *sql.Tx, mismo criterio que
// nissanSyncTx: cada instancia se usa para un solo ProcessERPStock y se descarta al terminar.
type erpStockSyncTx struct {
	branchesRepo       *mysqlRepo.BranchesRepository
	warehousesRepo     *mysqlRepo.WarehousesRepository
	productStockRepo   *mysqlRepo.ProductStockRepository
	stockMovementsRepo *mysqlRepo.StockMovementsRepository
	productPricesRepo  *mysqlRepo.ProductPricesRepository
	priceHistoryRepo   *mysqlRepo.PriceHistoryRepository

	// seen: ver el campo homónimo en nissanSyncTx.
	seen *stockSeen
}

func newErpStockSyncTx(tx *sql.Tx, seen *stockSeen) *erpStockSyncTx {
	return &erpStockSyncTx{
		branchesRepo:       mysqlRepo.NewBranchesRepository(tx),
		warehousesRepo:     mysqlRepo.NewWarehousesRepository(tx),
		productStockRepo:   mysqlRepo.NewProductStockRepository(tx),
		stockMovementsRepo: mysqlRepo.NewStockMovementsRepository(tx),
		productPricesRepo:  mysqlRepo.NewProductPricesRepository(tx),
		priceHistoryRepo:   mysqlRepo.NewPriceHistoryRepository(tx),
		seen:               seen,
	}
}

func (t *erpStockSyncTx) run(ctx context.Context, productID int64, inv *domain.Inventory, cache *erpStockSyncCache) error {
	branchID, err := t.resolveBranch(ctx, inv.BranchName, cache)
	if err != nil {
		return fmt.Errorf("resolving branch %q: %w", inv.BranchName, err)
	}

	warehouseID, err := t.resolveWarehouse(ctx, branchID, inv.Warehouse, cache)
	if err != nil {
		return fmt.Errorf("resolving warehouse %q: %w", inv.Warehouse, err)
	}

	// Se marca antes de escribir: la fuente reportó este artículo@almacén con
	// stock > 0, así que la reconciliación por ausencia no debe bajarlo a 0.
	t.seen.mark(productID, branchID, warehouseID)

	if err := t.syncProductStock(ctx, productID, branchID, warehouseID, int(inv.Stock)); err != nil {
		return fmt.Errorf("syncing stock for product %s: %w", inv.Code, err)
	}

	if err := t.syncErpPrice(ctx, productID, cache.mxnCurrencyID, cache.priceListID, inv.Cost); err != nil {
		return fmt.Errorf("syncing price for product %s: %w", inv.Code, err)
	}

	return nil
}

func (t *erpStockSyncTx) resolveBranch(ctx context.Context, name string, cache *erpStockSyncCache) (int64, error) {
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

// resolveWarehouse cachea por sucursal+almacén (a diferencia de nissanSyncTx.resolveWarehouse,
// que cachea solo por sucursal porque agencia y almacén son 1:1 en Nissan).
func (t *erpStockSyncTx) resolveWarehouse(ctx context.Context, branchID int64, name string, cache *erpStockSyncCache) (int64, error) {
	key := fmt.Sprintf("%d:%s", branchID, name)

	if id, ok := cache.warehouseIDs[key]; ok {
		return id, nil
	}

	warehouse, err := t.findOrCreateWarehouse(ctx, name, branchID)
	if err != nil {
		return 0, err
	}

	cache.warehouseIDs[key] = warehouse.ID
	return warehouse.ID, nil
}

func (t *erpStockSyncTx) findOrCreateBranch(ctx context.Context, name string) (*mysqlRepo.BranchDTO, error) {
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

func (t *erpStockSyncTx) findOrCreateWarehouse(ctx context.Context, name string, branchID int64) (*mysqlRepo.WarehouseDTO, error) {
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

func (t *erpStockSyncTx) syncProductStock(ctx context.Context, productID, branchID, warehouseID int64, newQty int) error {
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

func (t *erpStockSyncTx) syncErpPrice(ctx context.Context, productID, currencyID, priceListID int64, cost float64) error {
	newPrice := fmt.Sprintf("%.2f", cost)
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

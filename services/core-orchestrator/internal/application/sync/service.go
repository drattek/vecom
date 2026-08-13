package sync

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"core-orchestrator/internal/domain"
	redisRepo "core-orchestrator/internal/infrastructure/redis"
)

// Usuario del sistema para columnas de auditoría (created_by/updated_by) en escrituras
// que se originan por sincronización automática, no por una acción de un usuario real.
const systemUserID int64 = 1

type SyncService struct {
	products    *redisRepo.ProductRepository
	inventory   *redisRepo.InventoryRepository
	existencias *redisRepo.NissanRepository

	// db se usa para abrir transacciones SQL (ver sync_nissan.go): cada flujo de
	// escritura multi-tabla construye sus propios repos MySQL "scoped" a la
	// transacción en vez de guardar instancias de repo sueltas aquí.
	db *sql.DB
}

func NewSyncService(
	products *redisRepo.ProductRepository,
	inventory *redisRepo.InventoryRepository,
	existencias *redisRepo.NissanRepository,
	db *sql.DB,
) *SyncService {
	return &SyncService{
		products:    products,
		inventory:   inventory,
		existencias: existencias,
		db:          db,
	}
}

func (s *SyncService) ProcessERPCompleted(event domain.SyncCompletedEvent) error {
	ctx := context.Background()

	keys, err := s.products.ScanProducts(ctx)

	if err != nil {
		return err
	}

	log.Printf("ERP sync completed (source=%s, totalRecord=%d, pages=%d): found %d products to sync", event.Source, event.TotalRecords, event.Pages, len(keys))

	// Lee Redis en lotes (MGET) en vez de un GET por clave, mismo criterio que
	// ProcessNissanExistenciasSyncCompleted.
	products, err := s.products.FindByKeys(ctx, keys)
	if err != nil {
		return fmt.Errorf("reading ERP products from Redis: %w", err)
	}

	log.Printf("ERP sync: read %d products from Redis (product:*)", len(products))

	// Fuente (DYNAMICS) se resuelve una sola vez por corrida en vez de una vez por producto.
	cache, err := prepareErpSyncCache(s.db)
	if err != nil {
		return fmt.Errorf("preparing ERP sync cache: %w", err)
	}

	synced, failed := 0, 0

	for _, product := range products {
		if err := s.ProcessERPProduct(product, cache); err != nil {
			log.Printf("Error syncing product %s to MySQL: %v", product.Code, err)
			failed++
			continue
		}
		synced++
	}

	log.Printf("ERP sync finished (source=%s): %d ok, %d failed, %d total", event.Source, synced, failed, len(products))

	return nil
}

func (s *SyncService) ProcessStockSyncCompleted(event domain.SyncCompletedEvent) error {
	ctx := context.Background()

	keys, err := s.inventory.ScanInventory(ctx)

	if err != nil {
		return err
	}

	log.Printf("Stock sync completed (source=%s, totalRecord=%d, pages=%d): found %d stock records to sync", event.Source, event.TotalRecords, event.Pages, len(keys))

	for _, key := range keys {
		inventory, err := s.inventory.FindByKey(ctx, key)

		if err != nil {
			continue
		}

		log.Printf("Updating stock for product %s: %.2f units", inventory.Code, inventory.Stock)
	}

	return nil
}

func (s *SyncService) ProcessStockPageProcessed(event domain.PageProcessedEvent) error {
	log.Printf("Stock page processed (source=%s): page %d, offset %d, records %d", event.Source, event.Page, event.Offset, event.Records)
	return nil
}

func (s *SyncService) ProcessStockSyncFailed(event domain.SyncFailedEvent) error {
	log.Printf("Stock sync failed (source=%s) at offset %d (pageSize %d): %s (%s)", event.Source, event.Offset, event.PageSize, event.Error, event.Timestamp)
	return nil
}

func (s *SyncService) ProcessNissanExistenciasSyncCompleted(event domain.SyncCompletedEvent) error {
	ctx := context.Background()

	keys, err := s.existencias.ScanExistencias(ctx)

	if err != nil {
		return err
	}

	log.Printf("Nissan existencias sync started (source=%s, totalRecord=%d, pages=%d): found %d Nissan existencias to sync", event.Source, event.TotalRecords, event.Pages, len(keys))

	// Lee Redis en lotes (MGET) en vez de un GET por clave: con 50k+ SKUs esto por sí solo
	// evita decenas de miles de round-trips de red.
	existencias, err := s.existencias.FindByKeys(ctx, keys)
	if err != nil {
		return fmt.Errorf("reading Nissan existencias from Redis: %w", err)
	}

	// Fuente/moneda/lista de precios se resuelven una sola vez por corrida en vez de una
	// vez por SKU (ver nissanSyncCache en sync_nissan.go): eran ~4 SELECT/UPDATE repetidos
	// por cada uno de los 50k+ registros que, sin cambiar, no tenía sentido re-consultar.
	cache, err := prepareNissanSyncCache(s.db)
	if err != nil {
		return fmt.Errorf("preparing Nissan sync cache: %w", err)
	}

	synced, failed := 0, 0

	for _, existencia := range existencias {
		// El registro/actualización de PROD_SUPERSESION (con su propio chequeo de "no
		// escribir si no cambió") vive dentro de ProcessNissanExistencia -> nissanSyncTx.run,
		// en la misma transacción que producto/stock/precio (ver sync_nissan.go).
		if err := s.ProcessNissanExistencia(ctx, existencia, cache); err != nil {
			failed++
			continue
		}
		synced++
	}

	log.Printf("Nissan existencias sync finished (source=%s): %d ok, %d failed, %d total", event.Source, synced, failed, len(existencias))

	return nil
}

func (s *SyncService) ProcessNissanExistenciasPageProcessed(event domain.PageProcessedEvent) error {
	log.Printf("Nissan existencias page processed (source=%s): page %d, offset %d, records %d", event.Source, event.Page, event.Offset, event.Records)
	return nil
}

func (s *SyncService) ProcessNissanExistenciasSyncFailed(event domain.SyncFailedEvent) error {
	log.Printf("Nissan existencias sync failed (source=%s) at offset %d (pageSize %d): %s (%s)", event.Source, event.Offset, event.PageSize, event.Error, event.Timestamp)
	return nil
}

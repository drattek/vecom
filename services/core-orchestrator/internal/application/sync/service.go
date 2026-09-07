package sync

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	channelListingsApp "core-orchestrator/internal/application/channel_listings"
	"core-orchestrator/internal/domain"
	redisRepo "core-orchestrator/internal/infrastructure/redis"
)

// Usuario del sistema para columnas de auditoría (created_by/updated_by) en escrituras
// que se originan por sincronización automática, no por una acción de un usuario real.
const systemUserID int64 = 1

// hardcodedNissanRefreshConnectionIDs son los ecom_channel_connections.id a los que se les
// dispara un refresh de listados justo después de que el stock/precio leído de Nissan queda
// escrito en MySQL (ver refreshListingsAfterNissanSync). Temporal: hardcodear estos IDs es un
// atajo mientras no existe una forma configurable de asociar connections a la fuente Nissan;
// se piensa reemplazar por algo más robusto y eliminar esto.
var hardcodedNissanRefreshConnectionIDs = []int64{2, 3}

// hardcodedErpStockRefreshConnectionIDs son los ecom_channel_connections.id a los que se les
// dispara un refresh de listados justo después de que el stock leído del ERP (evento
// stock.sync.completed) queda escrito en MySQL (ver refreshListingsAfterErpStockSync). Mismo
// atajo temporal que hardcodedNissanRefreshConnectionIDs, mientras no exista una forma
// configurable de asociar connections a la fuente ERP.
var hardcodedErpStockRefreshConnectionIDs = []int64{1, 3}

type SyncService struct {
	products    *redisRepo.ProductRepository
	inventory   *redisRepo.InventoryRepository
	existencias *redisRepo.NissanRepository

	// db se usa para abrir transacciones SQL (ver sync_nissan.go): cada flujo de
	// escritura multi-tabla construye sus propios repos MySQL "scoped" a la
	// transacción en vez de guardar instancias de repo sueltas aquí.
	db *sql.DB

	// channelListings se usa solo para refreshListingsAfterNissanSync (ver comentario ahí y
	// en hardcodedNissanRefreshConnectionIDs) — temporal.
	channelListings *channelListingsApp.Service
}

func NewSyncService(
	products *redisRepo.ProductRepository,
	inventory *redisRepo.InventoryRepository,
	existencias *redisRepo.NissanRepository,
	db *sql.DB,
	channelListings *channelListingsApp.Service,
) *SyncService {
	return &SyncService{
		products:        products,
		inventory:       inventory,
		existencias:     existencias,
		db:              db,
		channelListings: channelListings,
	}
}

func (s *SyncService) ProcessERPCompleted(ctx context.Context, event domain.SyncCompletedEvent) error {

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
	cache, err := prepareErpSyncCache(ctx, s.db)
	if err != nil {
		return fmt.Errorf("preparing ERP sync cache: %w", err)
	}

	synced, failed := 0, 0

	for _, product := range products {
		if err := s.ProcessERPProduct(ctx, product, cache); err != nil {
			log.Printf("Error syncing product %s to MySQL: %v", product.Code, err)
			failed++
			continue
		}
		synced++
	}

	log.Printf("ERP sync finished (source=%s): %d ok, %d failed, %d total", event.Source, synced, failed, len(products))

	return nil
}

func (s *SyncService) ProcessStockSyncCompleted(ctx context.Context, event domain.SyncCompletedEvent) error {

	keys, err := s.inventory.ScanInventory(ctx)

	if err != nil {
		return err
	}

	log.Printf("Stock sync completed (source=%s, totalRecord=%d, pages=%d): found %d stock records to sync", event.Source, event.TotalRecords, event.Pages, len(keys))

	// Lee Redis en lotes (MGET) en vez de un GET por clave, mismo criterio que
	// ProcessNissanExistenciasSyncCompleted.
	inventories, err := s.inventory.FindByKeys(ctx, keys)
	if err != nil {
		return fmt.Errorf("reading ERP stock from Redis: %w", err)
	}

	// Moneda/lista de precios se resuelven una sola vez por corrida en vez de una vez por
	// registro (ver erpStockSyncCache en sync_erp_stock.go).
	cache, err := prepareErpStockSyncCache(ctx, s.db)
	if err != nil {
		return fmt.Errorf("preparing ERP stock sync cache: %w", err)
	}

	log.Printf("Stock sync: saving %d records to MySQL (source=%s)", len(inventories), event.Source)

	synced, skipped, failed := 0, 0, 0

	// seen acumula qué (producto, sucursal, almacén) trajo el feed, para la
	// reconciliación por ausencia de más abajo.
	seen := newStockSeen()

	for i, inventory := range inventories {
		skip, err := s.ProcessERPStock(ctx, inventory, cache, seen)
		if err != nil {
			log.Printf("Error syncing stock for product %s: %v", inventory.Code, err)
			failed++
			continue
		}
		if skip {
			skipped++
			continue
		}
		synced++

		if (i+1)%1000 == 0 {
			log.Printf("Stock sync: progress %d/%d (%d ok, %d skipped, %d failed)", i+1, len(inventories), synced, skipped, failed)
		}
	}

	log.Printf("Stock sync finished (source=%s): %d ok, %d skipped (SKU not found), %d failed, %d total", event.Source, synced, skipped, failed, len(inventories))

	// Baja a 0 el stock del ERP que quedó colgado en un valor > 0 porque su fila
	// dejó de venir en el feed (que la fuente filtra a Disponible > 0). Antes del
	// refresh de listings, para que los marketplaces vean también las bajas a 0.
	s.reconcileZeroedStock(ctx, cache.sourceID, seen)

	// Borra las claves ya consumidas para que las que dejaron de venir no queden
	// como fantasmas (ver InventoryRepository.DeleteKeys).
	if err := s.inventory.DeleteKeys(ctx, keys); err != nil {
		log.Printf("Stock sync: no se pudieron borrar %d claves de Redis: %v", len(keys), err)
	}

	s.refreshListingsAfterErpStockSync(ctx)

	return nil
}

// refreshListingsAfterErpStockSync empuja, para cada connectionId en
// hardcodedErpStockRefreshConnectionIDs, el mismo refresh de precio/stock que POST
// /api/channel-listings/refresh — justo después de que el stock leído del ERP ya quedó
// escrito en MySQL. Temporal (ver hardcodedErpStockRefreshConnectionIDs). Un error acá solo se
// loguea, nunca se propaga: si ProcessStockSyncCompleted devolviera error por esto, RabbitMQ
// reencolaría y reprocesaría todo el sync de stock (ver el Nack en rabbitmq.StartConsumer), no
// solo el refresh que falló.
func (s *SyncService) refreshListingsAfterErpStockSync(ctx context.Context) {
	if s.channelListings == nil {
		return
	}

	for _, connectionID := range hardcodedErpStockRefreshConnectionIDs {
		result, err := s.channelListings.RefreshListings(ctx, channelListingsApp.RefreshListingsInput{ConnectionID: connectionID})
		if err != nil {
			log.Printf("Stock sync: refresh de listings para connectionId=%d falló: %v", connectionID, err)
			continue
		}
		log.Printf("Stock sync: refresh de listings para connectionId=%d completado (%d resultados)", connectionID, len(result.Results))
	}
}

func (s *SyncService) ProcessStockPageProcessed(ctx context.Context, event domain.PageProcessedEvent) error {
	log.Printf("Stock page processed (source=%s): page %d, offset %d, records %d", event.Source, event.Page, event.Offset, event.Records)
	return nil
}

func (s *SyncService) ProcessStockSyncFailed(ctx context.Context, event domain.SyncFailedEvent) error {
	log.Printf("Stock sync failed (source=%s) at offset %d (pageSize %d): %s (%s)", event.Source, event.Offset, event.PageSize, event.Error, event.Timestamp)
	return nil
}

func (s *SyncService) ProcessNissanExistenciasSyncCompleted(ctx context.Context, event domain.SyncCompletedEvent) error {

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
	cache, err := prepareNissanSyncCache(ctx, s.db)
	if err != nil {
		return fmt.Errorf("preparing Nissan sync cache: %w", err)
	}

	synced, failed := 0, 0

	// seen acumula qué (producto, sucursal, almacén) trajo el feed, para la
	// reconciliación por ausencia de más abajo. Compartido por todos los lotes.
	seen := newStockSeen()

	// Se procesa en lotes de nissanSyncBatchSize, compartiendo una única transacción por
	// lote en vez de una por SKU (ver ProcessNissanExistenciaBatch en sync_nissan.go): con
	// miles de SKUs, una transacción por registro hacía que el COMMIT (con su fsync)
	// dominara el tiempo total del sync. El registro/actualización de PROD_SUPERSESION (con
	// su propio chequeo de "no escribir si no cambió") vive dentro de nissanSyncTx.run, en
	// la misma transacción que producto/stock/precio.
	for start := 0; start < len(existencias); start += nissanSyncBatchSize {
		end := min(start+nissanSyncBatchSize, len(existencias))

		batchSynced, batchFailed, err := s.ProcessNissanExistenciaBatch(ctx, existencias[start:end], cache, seen)
		if err != nil {
			log.Printf("Error syncing Nissan existencias batch [%d:%d]: %v", start, end, err)
			failed += end - start
			continue
		}

		synced += batchSynced
		failed += batchFailed

		log.Printf("Nissan existencias sync: progress %d/%d (%d ok, %d failed)", end, len(existencias), synced, failed)
	}

	log.Printf("Nissan existencias sync finished (source=%s): %d ok, %d failed, %d total", event.Source, synced, failed, len(existencias))

	// Baja a 0 el stock de Nissan que quedó colgado en un valor > 0 porque su fila
	// dejó de venir en el feed (que la fuente filtra a stock > 0). Va después de
	// escribir todo el feed y antes del refresh de listings, para que los
	// marketplaces vean también las bajas a 0.
	s.reconcileZeroedStock(ctx, cache.sourceID, seen)

	// Borra las claves ya consumidas para que las que dejaron de venir no queden
	// como fantasmas re-sincronizándose para siempre (ver NissanRepository.DeleteKeys).
	// Un error acá solo se loguea: la baja de stock ya la resolvió reconcileZeroedStock.
	if err := s.existencias.DeleteKeys(ctx, keys); err != nil {
		log.Printf("Nissan existencias sync: no se pudieron borrar %d claves de Redis: %v", len(keys), err)
	}

	s.refreshListingsAfterNissanSync(ctx)

	return nil
}

// refreshListingsAfterNissanSync empuja, para cada connectionId en hardcodedNissanRefreshConnectionIDs,
// el mismo refresh de precio/stock que POST /api/channel-listings/refresh — justo después de que el
// stock/precio leído de Nissan ya quedó escrito en MySQL. Temporal (ver hardcodedNissanRefreshConnectionIDs).
// Un error acá solo se loguea, nunca se propaga: si ProcessNissanExistenciasSyncCompleted devolviera
// error por esto, RabbitMQ reencolaría y reprocesaría todo el sync de existencias (ver el Nack en
// rabbitmq.StartConsumer), no solo el refresh que falló.
func (s *SyncService) refreshListingsAfterNissanSync(ctx context.Context) {
	if s.channelListings == nil {
		return
	}

	for _, connectionID := range hardcodedNissanRefreshConnectionIDs {
		result, err := s.channelListings.RefreshListings(ctx, channelListingsApp.RefreshListingsInput{ConnectionID: connectionID})
		if err != nil {
			log.Printf("Nissan existencias sync: refresh de listings para connectionId=%d falló: %v", connectionID, err)
			continue
		}
		log.Printf("Nissan existencias sync: refresh de listings para connectionId=%d completado (%d resultados)", connectionID, len(result.Results))
	}
}

func (s *SyncService) ProcessNissanExistenciasPageProcessed(ctx context.Context, event domain.PageProcessedEvent) error {
	log.Printf("Nissan existencias page processed (source=%s): page %d, offset %d, records %d", event.Source, event.Page, event.Offset, event.Records)
	return nil
}

func (s *SyncService) ProcessNissanExistenciasSyncFailed(ctx context.Context, event domain.SyncFailedEvent) error {
	log.Printf("Nissan existencias sync failed (source=%s) at offset %d (pageSize %d): %s (%s)", event.Source, event.Offset, event.PageSize, event.Error, event.Timestamp)
	return nil
}

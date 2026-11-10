// Package product_bulk_update actualiza productos en bloque desde un Excel (sku,
// marca, titulo_optimizado, descripciones, fotos, video y medidas). Cada fila se
// convierte en un ítem en cola (ecom_product_bulk_update_items) que un worker en
// segundo plano procesa, con el mismo esquema lote → ítems → reintento → reporte
// que la importación masiva de publicaciones de MercadoLibre (meli_bulk_import).
//
// Reglas de la actualización (ver ADR 0008):
//   - El SKU debe existir en ecom_products y pertenecer a la source elegida al
//     subir el archivo; si no, la fila queda "missing" o "failed" y no se toca.
//   - Una celda vacía significa "no tocar ese campo".
//   - Las fotos reemplazan TODAS las imágenes del producto (la primera es la
//     portada y foto_aplicacion siempre va al final de la galería).
//   - El video se AGREGA a los existentes; puede ser un archivo o un enlace de
//     YouTube.
package product_bulk_update

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	channelListingsApp "core-orchestrator/internal/application/channel_listings"
	productImageImportApp "core-orchestrator/internal/application/product_image_import"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidInput        = errors.New("sourceId, storageDiskId and an Excel file are required")
	ErrSourceNotFound      = mysqlInfra.ErrSourceNotFound
	ErrStorageDiskNotFound = mysqlInfra.ErrStorageDiskNotFound
	ErrItemNotRetryable    = errors.New("only failed or missing items can be retried")
	ErrStillMissing        = errors.New("the SKU still doesn't exist in the products table")
)

type Service struct {
	db              *sql.DB
	repo            *mysqlInfra.ProductBulkUpdateRepository
	productRepo     *mysqlInfra.ProductRepository
	sourcesRepo     *mysqlInfra.SourcesRepository
	storageDiskRepo *mysqlInfra.StorageDiskRepository
	brandsRepo      *mysqlInfra.BrandsRepository
	media           *productImageImportApp.Service
	listings        *channelListingsApp.Service

	// resyncMu serializa la resincronización hacia los marketplaces: las demás
	// etapas de una fila corren en paralelo, pero el presupuesto de las APIs
	// (MercadoLibre sobre todo) es compartido, igual que en la importación masiva.
	resyncMu sync.Mutex
}

func NewService(
	db *sql.DB,
	repo *mysqlInfra.ProductBulkUpdateRepository,
	productRepo *mysqlInfra.ProductRepository,
	sourcesRepo *mysqlInfra.SourcesRepository,
	storageDiskRepo *mysqlInfra.StorageDiskRepository,
	brandsRepo *mysqlInfra.BrandsRepository,
	media *productImageImportApp.Service,
	listings *channelListingsApp.Service,
) *Service {
	return &Service{
		db:              db,
		repo:            repo,
		productRepo:     productRepo,
		sourcesRepo:     sourcesRepo,
		storageDiskRepo: storageDiskRepo,
		brandsRepo:      brandsRepo,
		media:           media,
		listings:        listings,
	}
}

type UploadInput struct {
	SourceID      int64
	StorageDiskID int64
	// SyncConnectionIDs: conexiones donde resincronizar cada producto después de
	// actualizarlo en la base local. Vacío = solo se actualiza la base local.
	SyncConnectionIDs []int64
	FileName          string
	File              io.Reader
	ActorID           int64
}

// InvalidSyncConnectionError indica qué conexión elegida no se puede resincronizar.
type InvalidSyncConnectionError struct {
	ConnectionID int64
	Err          error
}

func (e *InvalidSyncConnectionError) Error() string {
	return fmt.Sprintf("connection %d cannot be resynced: %v", e.ConnectionID, e.Err)
}

func (e *InvalidSyncConnectionError) Unwrap() error { return e.Err }

// UploadResult cuenta cómo se clasificaron las filas del lote nuevo.
type UploadResult struct {
	BatchID   int64 `json:"batchId"`
	TotalRows int   `json:"totalRows"`
	Queued    int   `json:"queued"`
	Missing   int   `json:"missing"`
	Invalid   int   `json:"invalid"`
}

// Upload lee el Excel y registra el lote: las filas válidas cuyo SKU existe en
// la source elegida quedan en cola para el worker; las que no tienen SKU o
// traen datos inválidos quedan "failed"; las de SKU inexistente, "missing".
// Aquí no se modifica ningún producto: eso lo hace el worker después.
func (s *Service) Upload(ctx context.Context, in UploadInput) (*UploadResult, error) {
	if in.SourceID <= 0 || in.StorageDiskID <= 0 || in.ActorID <= 0 || in.File == nil {
		return nil, ErrInvalidInput
	}
	if _, err := s.sourcesRepo.FindByID(ctx, in.SourceID); err != nil {
		return nil, err
	}
	if _, err := s.storageDiskRepo.FindByID(ctx, in.StorageDiskID); err != nil {
		return nil, err
	}

	// Conexiones a resincronizar: sin repetidas, y todas deben existir y tener un
	// canal que sepa resincronizar publicaciones.
	syncIDs := make([]int64, 0, len(in.SyncConnectionIDs))
	seenConn := map[int64]bool{}
	for _, id := range in.SyncConnectionIDs {
		if seenConn[id] {
			continue
		}
		seenConn[id] = true
		if _, err := s.listings.ValidateResyncConnection(ctx, id); err != nil {
			if errors.Is(err, channelListingsApp.ErrInvalidChannelConnection) || errors.Is(err, channelListingsApp.ErrNoFullRefresherForChannel) {
				return nil, &InvalidSyncConnectionError{ConnectionID: id, Err: err}
			}
			return nil, err
		}
		syncIDs = append(syncIDs, id)
	}

	rows, err := parseSheet(in.File)
	if err != nil {
		return nil, err
	}

	type resolved struct {
		sheetRow
		sku       string
		productID *int64
		payload   json.RawMessage
		status    string
		message   string
	}
	items := make([]resolved, 0, len(rows))
	result := &UploadResult{TotalRows: len(rows)}
	seenSKU := map[string]int{}
	sourceNames := map[int64]string{}

	for _, row := range rows {
		item := resolved{sheetRow: row, sku: row.cells[colSKU], status: mysqlInfra.ProductBulkUpdatePending}

		switch {
		case item.sku == "":
			item.status, item.message = mysqlInfra.ProductBulkUpdateFailed, "La fila no tiene sku."
			result.Invalid++
		case seenSKU[item.sku] > 0:
			item.status = mysqlInfra.ProductBulkUpdateFailed
			item.message = fmt.Sprintf("El SKU está repetido: ya aparece en la fila %d.", seenSKU[item.sku])
			result.Invalid++
		default:
			seenSKU[item.sku] = row.number

			payload, err := buildPayload(row.cells)
			if err != nil {
				item.status, item.message = mysqlInfra.ProductBulkUpdateFailed, err.Error()
				result.Invalid++
				break
			}
			item.payload, _ = json.Marshal(payload)

			product, err := s.productRepo.FindBySKU(ctx, item.sku)
			switch {
			case err == nil && product.SourceID != in.SourceID:
				item.productID = &product.ID
				item.status = mysqlInfra.ProductBulkUpdateFailed
				item.message = fmt.Sprintf("El SKU pertenece a otra source (%s), no a la elegida.", s.sourceName(ctx, sourceNames, product.SourceID))
				result.Invalid++
			case err == nil:
				item.productID = &product.ID
				result.Queued++
			case errors.Is(err, mysqlInfra.ErrProductNotFound):
				item.status, item.message = mysqlInfra.ProductBulkUpdateMissing, "El SKU no existe en la tabla de productos."
				result.Missing++
			default:
				return nil, fmt.Errorf("error looking up sku %q: %w", item.sku, err)
			}
		}
		items = append(items, item)
	}

	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		repo := mysqlInfra.NewProductBulkUpdateRepository(tx)
		batchID, err := repo.CreateBatch(ctx, mysqlInfra.CreateProductBulkUpdateBatchInput{
			SourceID:          in.SourceID,
			StorageDiskID:     in.StorageDiskID,
			SyncConnectionIDs: syncIDs,
			FileName:          in.FileName,
			TotalRows:         len(items),
			ActorID:           in.ActorID,
		})
		if err != nil {
			return err
		}
		result.BatchID = batchID
		for _, item := range items {
			if err := repo.CreateItem(ctx, mysqlInfra.CreateProductBulkUpdateItemInput{
				BatchID:      batchID,
				SheetRow:     item.number,
				SKU:          item.sku,
				ProductID:    item.productID,
				Payload:      item.payload,
				Status:       item.status,
				ErrorMessage: item.message,
				ActorID:      in.ActorID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) sourceName(ctx context.Context, cache map[int64]string, id int64) string {
	if name, ok := cache[id]; ok {
		return name
	}
	name := fmt.Sprintf("id %d", id)
	if source, err := s.sourcesRepo.FindByID(ctx, id); err == nil {
		name = source.Name
	}
	cache[id] = name
	return name
}

func (s *Service) ListBatches(ctx context.Context) ([]mysqlInfra.ProductBulkUpdateBatchDTO, error) {
	return s.repo.ListBatches(ctx, 200)
}

func (s *Service) List(ctx context.Context, f mysqlInfra.ProductBulkUpdateItemFilter) (*mysqlInfra.ProductBulkUpdateItemsPage, error) {
	return s.repo.FindItems(ctx, f)
}

func (s *Service) Summary(ctx context.Context, batchID int64) (map[string]int, error) {
	return s.repo.CountByStatus(ctx, batchID)
}

// Retry vuelve a poner en cola una fila fallida o faltante. Una fila faltante
// vuelve a buscar su SKU: solo es reintentable cuando el producto ya existe.
func (s *Service) Retry(ctx context.Context, itemID, actorID int64) error {
	item, err := s.repo.FindItem(ctx, itemID)
	if err != nil {
		return err
	}
	if item.Status != mysqlInfra.ProductBulkUpdateFailed && item.Status != mysqlInfra.ProductBulkUpdateMissing {
		return ErrItemNotRetryable
	}
	return s.requeue(ctx, item, actorID)
}

// RetryAll reencola todas las filas en el estado dado (failed o missing),
// opcionalmente de un solo lote. Las filas inválidas del Excel (sin datos que
// reprocesar) y las faltantes cuyo SKU sigue sin existir se quedan como están;
// devuelve cuántas se reencolaron realmente.
func (s *Service) RetryAll(ctx context.Context, status string, batchID, actorID int64) (int, error) {
	if status != mysqlInfra.ProductBulkUpdateFailed && status != mysqlInfra.ProductBulkUpdateMissing {
		return 0, ErrItemNotRetryable
	}
	items, err := s.repo.ListByStatus(ctx, status, batchID)
	if err != nil {
		return 0, err
	}
	requeued := 0
	for i := range items {
		err := s.requeue(ctx, &items[i], actorID)
		switch {
		case err == nil:
			requeued++
		case errors.Is(err, ErrStillMissing), errors.Is(err, mysqlInfra.ErrProductBulkUpdateItemNotFound):
		default:
			return requeued, err
		}
	}
	return requeued, nil
}

func (s *Service) requeue(ctx context.Context, item *mysqlInfra.ProductBulkUpdateItemDTO, actorID int64) error {
	product, err := s.productRepo.FindBySKU(ctx, item.SKU)
	if errors.Is(err, mysqlInfra.ErrProductNotFound) {
		return ErrStillMissing
	}
	if err != nil {
		return err
	}
	return s.repo.Requeue(ctx, item.ID, &product.ID, actorID)
}

package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"core-orchestrator/internal/domain"
	mysqlRepo "core-orchestrator/internal/infrastructure/mysql"
)

const (
	dynamicsSourceCode = "DYNAMICS"
	dynamicsSourceName = "Dynamics"
)

// Mapeo de EcomProductDTO.group (campo "Group" en domain.Product) a ecom_products.product_type.
// Por ahora el ERP no reporta "consumable"; cualquier valor que no sea "ACC" cae en "part"
// (mismo default de la columna).
func erpProductType(group string) string {
	switch group {
	case "ACC":
		return "accessory"
	default:
		return "part"
	}
}

// erpSyncCache resuelve una sola vez, por corrida de sync, la fuente ("DYNAMICS") que
// de otra forma se volvería a buscar/crear en MySQL para cada producto de la página.
type erpSyncCache struct {
	sourceID int64
}

func prepareErpSyncCache(ctx context.Context, db *sql.DB) (*erpSyncCache, error) {
	sourcesRepo := mysqlRepo.NewSourcesRepository(db)

	source, err := findOrCreateDynamicsSource(ctx, sourcesRepo)
	if err != nil {
		return nil, fmt.Errorf("resolving Dynamics source: %w", err)
	}

	return &erpSyncCache{sourceID: source.ID}, nil
}

func findOrCreateDynamicsSource(ctx context.Context, repo *mysqlRepo.SourcesRepository) (*mysqlRepo.SourceDTO, error) {
	source, err := repo.FindByCode(ctx, dynamicsSourceCode)
	if err == nil {
		return source, nil
	}
	if !errors.Is(err, mysqlRepo.ErrSourceNotFound) {
		return nil, err
	}

	return repo.Create(ctx, mysqlRepo.CreateSourceInput{
		Code:      dynamicsSourceCode,
		Name:      dynamicsSourceName,
		CreatedBy: systemUserID,
	})
}

// ProcessERPProduct crea el producto en ecom_products a partir de un EcomProductDTO leído
// de Redis la primera vez que aparece un SKU. Si el SKU ya existe, esta fila nunca se toca
// (mismo criterio que el sync de Nissan, ver nissanSyncTx.run): no se pisa curación manual
// hecha desde el admin-dashboard (marca, categoría, status, descripción, e incluso
// name/part_number/product_type, que antes se resincronizaban desde el ERP en cada corrida).
// A diferencia del sync de Nissan, este evento no trae sucursal/almacén: no hay stock ni
// precio que sincronizar, solo el producto — el stock/precio de un SKU existente se
// sincroniza aparte, vía ProcessERPStock (stock.sync.completed).
func (s *SyncService) ProcessERPProduct(ctx context.Context, product *domain.Product, cache *erpSyncCache) error {
	productRepo := mysqlRepo.NewProductRepository(s.db)

	_, err := productRepo.FindBySKU(ctx, product.Code)
	if err == nil {
		return nil
	}
	if !errors.Is(err, mysqlRepo.ErrProductNotFound) {
		return fmt.Errorf("looking up product %s: %w", product.Code, err)
	}

	productType := erpProductType(product.Group)

	log.Printf("Creating product %s in MySQL (partNumber=%s, name=%q, type=%s, source=%s)", product.Code, product.PartNumber, product.Description, productType, dynamicsSourceCode)

	created, err := productRepo.Create(ctx, mysqlRepo.CreateProductInput{
		SKU:         product.Code,
		PartNumber:  product.PartNumber,
		Name:        product.Description,
		ProductType: productType,
		IsSellable:  true,
		IsStockable: true,
		SourceID:    cache.sourceID,
		CreatedBy:   systemUserID,
	})
	if err != nil {
		return fmt.Errorf("creating product %s: %w", product.Code, err)
	}

	log.Printf("Created product %s in MySQL (id=%d)", product.Code, created.ID)
	return nil
}

func (s *SyncService) ProcessERPPageProcessed(ctx context.Context, event domain.PageProcessedEvent) error {
	log.Printf("ERP page processed (source=%s): page %d, offset %d, records %d", event.Source, event.Page, event.Offset, event.Records)
	return nil
}

func (s *SyncService) ProcessERPSyncFailed(ctx context.Context, event domain.SyncFailedEvent) error {
	log.Printf("ERP sync failed (source=%s) at offset %d (pageSize %d): %s (%s)", event.Source, event.Offset, event.PageSize, event.Error, event.Timestamp)
	return nil
}

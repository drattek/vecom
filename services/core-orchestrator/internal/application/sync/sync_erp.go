package sync

import (
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

func prepareErpSyncCache(db *sql.DB) (*erpSyncCache, error) {
	sourcesRepo := mysqlRepo.NewSourcesRepository(db)

	source, err := findOrCreateDynamicsSource(sourcesRepo)
	if err != nil {
		return nil, fmt.Errorf("resolving Dynamics source: %w", err)
	}

	return &erpSyncCache{sourceID: source.ID}, nil
}

func findOrCreateDynamicsSource(repo *mysqlRepo.SourcesRepository) (*mysqlRepo.SourceDTO, error) {
	source, err := repo.FindByCode(dynamicsSourceCode)
	if err == nil {
		return source, nil
	}
	if !errors.Is(err, mysqlRepo.ErrSourceNotFound) {
		return nil, err
	}

	return repo.Create(mysqlRepo.CreateSourceInput{
		Code:      dynamicsSourceCode,
		Name:      dynamicsSourceName,
		CreatedBy: systemUserID,
	})
}

// ProcessERPProduct crea o actualiza el producto en ecom_products a partir de un
// EcomProductDTO leído de Redis. A diferencia del sync de Nissan, este evento no trae
// sucursal/almacén: no hay stock ni precio que sincronizar, solo el producto.
func (s *SyncService) ProcessERPProduct(product *domain.Product, cache *erpSyncCache) error {
	productRepo := mysqlRepo.NewProductRepository(s.db)
	productType := erpProductType(product.Group)

	existing, err := productRepo.FindBySKU(product.Code)
	if errors.Is(err, mysqlRepo.ErrProductNotFound) {
		log.Printf("Creating product %s in MySQL (partNumber=%s, name=%q, type=%s, source=%s)", product.Code, product.PartNumber, product.Description, productType, dynamicsSourceCode)

		created, err := productRepo.Create(mysqlRepo.CreateProductInput{
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
	if err != nil {
		return fmt.Errorf("looking up product %s: %w", product.Code, err)
	}

	log.Printf("Updating product %s in MySQL (id=%d, partNumber=%s, name=%q, type=%s, source=%s)", product.Code, existing.ID, product.PartNumber, product.Description, productType, dynamicsSourceCode)

	// Preserva campos que no pertenecen a este sync (marca, categoría, status, descripción
	// larga) para no pisar curación manual hecha desde el admin-dashboard.
	_, err = productRepo.Update(existing.ID, mysqlRepo.UpdateProductInput{
		SKU:              product.Code,
		PartNumber:       product.PartNumber,
		Name:             product.Description,
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
	if err != nil {
		return fmt.Errorf("updating product %s: %w", product.Code, err)
	}

	log.Printf("Updated product %s in MySQL (id=%d)", product.Code, existing.ID)
	return nil
}

func (s *SyncService) ProcessERPPageProcessed(event domain.PageProcessedEvent) error {
	log.Printf("ERP page processed (source=%s): page %d, offset %d, records %d", event.Source, event.Page, event.Offset, event.Records)
	return nil
}

func (s *SyncService) ProcessERPSyncFailed(event domain.SyncFailedEvent) error {
	log.Printf("ERP sync failed (source=%s) at offset %d (pageSize %d): %s (%s)", event.Source, event.Offset, event.PageSize, event.Error, event.Timestamp)
	return nil
}

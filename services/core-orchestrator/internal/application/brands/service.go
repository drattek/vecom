package brands

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidBrandPayload = errors.New("invalid brand payload")

type BrandService struct {
	db                *sql.DB
	repository        *mysqlInfra.BrandsRepository
	productRepository *mysqlInfra.ProductRepository
}

// NewBrandService recibe *sql.DB (para las operaciones multi-sentencia que se
// corren dentro de una transacción con mysqlInfra.WithinTx) además de los repos
// ya construidos sobre el pool (para lecturas y escrituras de una sola
// sentencia). Estructura común a todos los servicios — ver
// infrastructure/decisions/0001-persistencia-transacciones-y-context.md.
func NewBrandService(db *sql.DB, repository *mysqlInfra.BrandsRepository, productRepository *mysqlInfra.ProductRepository) *BrandService {
	return &BrandService{db: db, repository: repository, productRepository: productRepository}
}

func (s *BrandService) GetPaginatedBrands(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedBrands, error) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *BrandService) GetBrandByID(ctx context.Context, id int64) (*mysqlInfra.BrandDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidBrandPayload
	}
	return s.repository.FindByID(ctx, id)
}

// CreateBrand es una sola sentencia (INSERT + read-back), no necesita transacción.
func (s *BrandService) CreateBrand(ctx context.Context, input mysqlInfra.CreateBrandInput) (*mysqlInfra.BrandDTO, error) {
	if input.Name == "" {
		return nil, ErrInvalidBrandPayload
	}
	if input.CreatedBy <= 0 {
		return nil, ErrInvalidBrandPayload
	}

	return s.repository.Create(ctx, input)
}

func (s *BrandService) UpdateBrand(ctx context.Context, id int64, input mysqlInfra.UpdateBrandInput) (*mysqlInfra.BrandDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidBrandPayload
	}
	if input.Name == "" {
		return nil, ErrInvalidBrandPayload
	}
	if input.UpdatedBy <= 0 {
		return nil, ErrInvalidBrandPayload
	}

	return s.repository.Update(ctx, id, input)
}

func (s *BrandService) DeleteBrand(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidBrandPayload
	}
	return s.repository.SoftDelete(ctx, id)
}

type BulkBrandAssignmentItem struct {
	SKU   string
	Brand string
}

type BulkBrandAssignmentStatus string

const (
	BulkBrandAssignmentUpdated  BulkBrandAssignmentStatus = "updated"
	BulkBrandAssignmentNotFound BulkBrandAssignmentStatus = "not_found"
	BulkBrandAssignmentError    BulkBrandAssignmentStatus = "error"
)

type BulkBrandAssignmentResult struct {
	Index   int                       `json:"index"`
	SKU     string                    `json:"sku"`
	Brand   string                    `json:"brand"`
	Status  BulkBrandAssignmentStatus `json:"status"`
	BrandID int64                     `json:"brandId,omitempty"`
	Error   string                    `json:"error,omitempty"`
}

// BulkAssignBrands resolves each item's product by SKU and assigns it the
// named brand, creating the brand the first time it's seen. Brand names are
// always uppercased before lookup/creation so "Bobcat", "BOBCAT" and "bobcat"
// resolve to the same ecom_brands row instead of creating duplicates.
//
// Cada ítem es un caso multi-sentencia (resolver/crear marca + actualizar el
// producto), así que corre en su propia transacción: si el UPDATE del producto
// falla, la marca recién creada también se revierte. El endpoint conserva su
// semántica de éxito parcial — un ítem que falla no aborta a los demás — y el
// caché brandIDByName se puebla sólo tras el commit para no arrastrar un id de
// marca que quedó sin persistir.
func (s *BrandService) BulkAssignBrands(ctx context.Context, items []BulkBrandAssignmentItem, actorID int64) ([]BulkBrandAssignmentResult, error) {
	results := make([]BulkBrandAssignmentResult, 0, len(items))
	brandIDByName := make(map[string]int64)

	for i, item := range items {
		sku := strings.TrimSpace(item.SKU)
		brandName := strings.ToUpper(strings.TrimSpace(item.Brand))

		if sku == "" || brandName == "" {
			results = append(results, BulkBrandAssignmentResult{Index: i, SKU: sku, Brand: brandName, Status: BulkBrandAssignmentError, Error: "sku and brand are required"})
			continue
		}

		var assignedBrandID int64
		txErr := mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
			brands := mysqlInfra.NewBrandsRepository(tx)
			products := mysqlInfra.NewProductRepository(tx)

			product, err := products.FindBySKU(ctx, sku)
			if err != nil {
				return err
			}

			brandID, ok := brandIDByName[brandName]
			if !ok {
				resolvedID, resolveErr := resolveBrandID(ctx, brands, brandName, actorID)
				if resolveErr != nil {
					return resolveErr
				}
				brandID = resolvedID
			}

			if err := products.UpdateBrandID(ctx, product.ID, brandID, actorID); err != nil {
				return err
			}

			assignedBrandID = brandID
			return nil
		})
		if txErr != nil {
			if errors.Is(txErr, mysqlInfra.ErrProductNotFound) {
				results = append(results, BulkBrandAssignmentResult{Index: i, SKU: sku, Brand: brandName, Status: BulkBrandAssignmentNotFound, Error: "product not found for sku"})
				continue
			}
			results = append(results, BulkBrandAssignmentResult{Index: i, SKU: sku, Brand: brandName, Status: BulkBrandAssignmentError, Error: txErr.Error()})
			continue
		}

		brandIDByName[brandName] = assignedBrandID
		results = append(results, BulkBrandAssignmentResult{Index: i, SKU: sku, Brand: brandName, BrandID: assignedBrandID, Status: BulkBrandAssignmentUpdated})
	}

	return results, nil
}

// resolveBrandID finds the brand by its (already uppercased) name, creating
// it if it doesn't exist yet. repo puede ser el repo sobre el pool o uno
// escopeado a una *sql.Tx.
func resolveBrandID(ctx context.Context, repo *mysqlInfra.BrandsRepository, brandName string, actorID int64) (int64, error) {
	brand, err := repo.FindByName(ctx, brandName)
	if err == nil {
		return brand.ID, nil
	}
	if !errors.Is(err, mysqlInfra.ErrBrandNotFound) {
		return 0, err
	}

	created, err := repo.Create(ctx, mysqlInfra.CreateBrandInput{Name: brandName, CreatedBy: actorID})
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

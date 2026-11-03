package product_details

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidProductID       = errors.New("invalid product id")
	ErrProductNotFound        = errors.New("product not found")
	ErrInvalidGeneralPatch    = errors.New("invalid general patch")
	ErrInvalidDimensionsPatch = errors.New("invalid dimensions patch")
	ErrInvalidSEOPatch        = errors.New("invalid seo patch")
	// ErrInvalidStockAdjustment envuelve (con %w) el motivo concreto; el handler
	// lo expone al usuario porque son errores de validación, no internos.
	ErrInvalidStockAdjustment = errors.New("invalid stock adjustment")
)

// maxStockAdjustmentQty protege la columna int de available_qty y evita
// typos absurdos en un movimiento manual.
const maxStockAdjustmentQty = 10_000_000

// maxNameLen / maxShortDescriptionLen espejan varchar(255) de ecom_products.
const (
	maxNameLen             = 255
	maxShortDescriptionLen = 255
	// maxDimensionValue es el tope de decimal(10,2) de ecom_product_dimensions.
	maxDimensionValue = 99999999.99
	// ecom_product_seo: meta_title/meta_description son varchar(255), keywords es text.
	maxSEOMetaLen  = 255
	maxSEOKeywords = 5000
)

// Service backs the product detail page. Reads are one query per section;
// UpdateGeneral is a single dynamic UPDATE. Nothing here needs a transaction,
// so db is kept only for consistency with the rest of the application layer
// (ver ADR 0001).
type Service struct {
	db                   *sql.DB
	repository           *mysqlInfra.ProductDetailsRepository
	brandsRepository     *mysqlInfra.BrandsRepository
	categoriesRepository *mysqlInfra.CategoriesRepository
	dimensionsRepository *mysqlInfra.ProductDimensionsRepository
	seoRepository        *mysqlInfra.ProductSEORepository
}

func NewService(
	db *sql.DB,
	repository *mysqlInfra.ProductDetailsRepository,
	brandsRepository *mysqlInfra.BrandsRepository,
	categoriesRepository *mysqlInfra.CategoriesRepository,
	dimensionsRepository *mysqlInfra.ProductDimensionsRepository,
	seoRepository *mysqlInfra.ProductSEORepository,
) *Service {
	return &Service{
		db:                   db,
		repository:           repository,
		brandsRepository:     brandsRepository,
		categoriesRepository: categoriesRepository,
		dimensionsRepository: dimensionsRepository,
		seoRepository:        seoRepository,
	}
}

func (s *Service) GetGeneral(ctx context.Context, productID int64) (*mysqlInfra.ProductGeneralDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}

	general, err := s.repository.FindGeneral(ctx, productID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	return general, nil
}

// UpdateGeneralInput son los campos editables de la sección General del detalle.
// Puntero nil = "no enviado"; el PATCH solo toca las columnas presentes. Para
// BrandID / CategoryID (FK anulable) un puntero no-nil con valor <= 0 limpia la
// relación (marca/categoría "ninguna").
type UpdateGeneralInput struct {
	Name             *string
	Description      *string
	ShortDescription *string
	BrandID          *int64
	CategoryID       *int64
}

// UpdateGeneral aplica una edición inline sobre la sección General y devuelve el
// DTO ya refrescado (mismo shape que GetGeneral, para que el front actualice su
// caché con la respuesta).
func (s *Service) UpdateGeneral(ctx context.Context, productID, actorID int64, in UpdateGeneralInput) (*mysqlInfra.ProductGeneralDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}
	if actorID <= 0 {
		return nil, ErrInvalidGeneralPatch
	}

	// Confirma que el producto existe (y no está borrado) antes de escribir:
	// el repo no distingue "no existe" de "sin cambios".
	if _, err := s.repository.FindGeneral(ctx, productID); err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	fields := mysqlInfra.UpdateGeneralFields{}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || len(name) > maxNameLen {
			return nil, ErrInvalidGeneralPatch
		}
		fields.Name = &name
	}

	if in.ShortDescription != nil {
		shortDescription := strings.TrimSpace(*in.ShortDescription)
		if len(shortDescription) > maxShortDescriptionLen {
			return nil, ErrInvalidGeneralPatch
		}
		fields.ShortDescription = &shortDescription
	}

	if in.Description != nil {
		description := strings.TrimSpace(*in.Description)
		fields.Description = &description
	}

	// Solo validamos existencia cuando se asigna una marca/categoría concreta;
	// valor <= 0 significa "ninguna" y se limpia sin más.
	if in.BrandID != nil {
		if *in.BrandID > 0 {
			if _, err := s.brandsRepository.FindByID(ctx, *in.BrandID); err != nil {
				if errors.Is(err, mysqlInfra.ErrBrandNotFound) {
					return nil, ErrInvalidGeneralPatch
				}
				return nil, fmt.Errorf("error validating brand: %w", err)
			}
		}
		fields.BrandID = in.BrandID
	}

	if in.CategoryID != nil {
		if *in.CategoryID > 0 {
			if _, err := s.categoriesRepository.FindByID(ctx, *in.CategoryID); err != nil {
				if errors.Is(err, mysqlInfra.ErrCategoryNotFound) {
					return nil, ErrInvalidGeneralPatch
				}
				return nil, fmt.Errorf("error validating category: %w", err)
			}
		}
		fields.CategoryID = in.CategoryID
	}

	if _, err := s.repository.UpdateGeneral(ctx, productID, actorID, fields); err != nil {
		return nil, fmt.Errorf("error updating product general details: %w", err)
	}

	return s.repository.FindGeneral(ctx, productID)
}

func (s *Service) GetMedia(ctx context.Context, productID int64) (*mysqlInfra.ProductMediaSectionDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}

	return s.repository.FindMedia(ctx, productID)
}

func (s *Service) GetPricing(ctx context.Context, productID int64) (*mysqlInfra.ProductPricingSectionDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}

	return s.repository.FindPricing(ctx, productID)
}

func (s *Service) GetInventory(ctx context.Context, productID int64) (*mysqlInfra.ProductInventorySectionDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}

	return s.repository.FindInventory(ctx, productID)
}

// defaultDetailPageSize / maxDetailPageSize acotan el tamaño de página de las
// tablas paginadas del detalle (movimientos de stock, historial de precio): un
// producto con años de sync acumula miles de filas y el default debe traer un
// JSON chico de inicio.
const (
	defaultDetailPageSize = 10
	maxDetailPageSize     = 100
)

func normalizeDetailPage(offset, pageSize int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	switch {
	case pageSize <= 0:
		pageSize = defaultDetailPageSize
	case pageSize > maxDetailPageSize:
		pageSize = maxDetailPageSize
	}
	return offset, pageSize
}

func (s *Service) GetStockMovements(ctx context.Context, productID int64, offset, pageSize int) (*mysqlInfra.PaginatedProductStockMovements, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}

	offset, pageSize = normalizeDetailPage(offset, pageSize)
	return s.repository.FindStockMovements(ctx, productID, offset, pageSize)
}

// AdjustStockInput describe un movimiento manual de inventario sobre una
// ubicación (sucursal + almacén) del producto.
//
// Quantity significa según MovementType:
//   - "in":         piezas que entran (> 0), suma al stock.
//   - "out":        piezas que salen (> 0), resta; no puede dejar el stock < 0.
//   - "adjustment": cantidad final deseada (>= 0); el cambio es final - actual.
type AdjustStockInput struct {
	BranchID     int64
	WarehouseID  int64
	MovementType string
	Quantity     int
}

// AdjustStock aplica un movimiento manual: bloquea la fila de stock, actualiza
// ecom_product_stock e inserta el movimiento en ecom_stock_movements, todo-o-nada
// (ver ADR 0001). Si la ubicación aún no tiene fila de stock la crea.
func (s *Service) AdjustStock(ctx context.Context, productID, userID int64, input AdjustStockInput) (*mysqlInfra.StockMovementDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}
	if input.BranchID <= 0 || input.WarehouseID <= 0 {
		return nil, fmt.Errorf("%w: branchId and warehouseId are required", ErrInvalidStockAdjustment)
	}
	if input.Quantity > maxStockAdjustmentQty {
		return nil, fmt.Errorf("%w: quantity exceeds %d", ErrInvalidStockAdjustment, maxStockAdjustmentQty)
	}

	switch input.MovementType {
	case "in", "out":
		if input.Quantity <= 0 {
			return nil, fmt.Errorf("%w: quantity must be greater than 0", ErrInvalidStockAdjustment)
		}
	case "adjustment":
		if input.Quantity < 0 {
			return nil, fmt.Errorf("%w: quantity cannot be negative", ErrInvalidStockAdjustment)
		}
	default:
		return nil, fmt.Errorf("%w: movementType must be in, out or adjustment", ErrInvalidStockAdjustment)
	}

	var movement *mysqlInfra.StockMovementDTO
	err := mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		stockRepo := mysqlInfra.NewProductStockRepository(tx)
		movementsRepo := mysqlInfra.NewStockMovementsRepository(tx)

		exists, err := stockRepo.ProductExists(ctx, productID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrProductNotFound
		}

		belongs, err := stockRepo.WarehouseBelongsToBranch(ctx, input.WarehouseID, input.BranchID)
		if err != nil {
			return err
		}
		if !belongs {
			return fmt.Errorf("%w: warehouse does not belong to the branch", ErrInvalidStockAdjustment)
		}

		current, err := stockRepo.FindByProductBranchWarehouseForUpdate(ctx, productID, input.BranchID, input.WarehouseID)
		if errors.Is(err, mysqlInfra.ErrProductStockNotFound) {
			current, err = stockRepo.Create(ctx, mysqlInfra.CreateProductStockInput{
				ProductID:    productID,
				BranchID:     input.BranchID,
				WarehouseID:  input.WarehouseID,
				AvailableQty: 0,
			})
		}
		if err != nil {
			return err
		}

		before := current.AvailableQty
		var after int
		switch input.MovementType {
		case "in":
			after = before + input.Quantity
		case "out":
			after = before - input.Quantity
		default:
			after = input.Quantity
		}

		if after < 0 {
			return fmt.Errorf("%w: insufficient stock (available %d)", ErrInvalidStockAdjustment, before)
		}
		if after > maxStockAdjustmentQty {
			return fmt.Errorf("%w: resulting stock exceeds %d", ErrInvalidStockAdjustment, maxStockAdjustmentQty)
		}
		if after == before {
			return fmt.Errorf("%w: the movement does not change the stock", ErrInvalidStockAdjustment)
		}

		if err := stockRepo.SetManualQty(ctx, current.ID, after, userID); err != nil {
			return err
		}

		movement, err = movementsRepo.Create(ctx, mysqlInfra.CreateStockMovementInput{
			ProductID:      productID,
			BranchID:       input.BranchID,
			WarehouseID:    input.WarehouseID,
			MovementType:   input.MovementType,
			QuantityBefore: before,
			QuantityChange: after - before,
			QuantityAfter:  after,
			UpdatedBy:      userID,
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	return movement, nil
}

func (s *Service) GetPriceHistory(ctx context.Context, productID int64, offset, pageSize int) (*mysqlInfra.PaginatedProductPriceHistory, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}

	offset, pageSize = normalizeDetailPage(offset, pageSize)
	return s.repository.FindPriceHistory(ctx, productID, offset, pageSize)
}

func (s *Service) GetPartNumbers(ctx context.Context, productID int64) (*mysqlInfra.ProductPartNumbersSectionDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}

	partNumbers, err := s.repository.FindPartNumbers(ctx, productID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	return partNumbers, nil
}

func (s *Service) GetAttributes(ctx context.Context, productID int64) (*mysqlInfra.ProductAttributesSectionDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}

	return s.repository.FindAttributes(ctx, productID)
}

// GetCompatibilities devuelve, en solo lectura, las compatibilidades de
// vehículo del producto (fitment resuelto a marca/modelo/años) junto con los
// calificadores motor/posición/lado de la tabla de enlace.
func (s *Service) GetCompatibilities(ctx context.Context, productID int64) (*mysqlInfra.ProductCompatibilitiesSectionDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}

	return s.repository.FindCompatibilities(ctx, productID)
}

// UpdateDimensionsInput lleva las medidas editables de la sección Dimensiones,
// tal como llegan del formulario (texto). Todas son obligatorias: la edición
// manda la sección completa. El volumen no se recibe: lo calcula el repositorio
// como largo*ancho*alto (cm³).
type UpdateDimensionsInput struct {
	Weight   string
	Length   string
	Width    string
	Height   string
	Diameter string
}

// UpdateDimensions hace un upsert de ecom_product_dimensions (crea la fila si el
// producto aún no tiene dimensiones) y devuelve la sección Atributos refrescada,
// para que el front actualice su caché con la respuesta.
func (s *Service) UpdateDimensions(ctx context.Context, productID, actorID int64, in UpdateDimensionsInput) (*mysqlInfra.ProductAttributesSectionDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}
	if actorID <= 0 {
		return nil, ErrInvalidDimensionsPatch
	}

	if _, err := s.repository.FindGeneral(ctx, productID); err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	weight, err := normalizeDimensionValue(in.Weight)
	if err != nil {
		return nil, err
	}
	length, err := normalizeDimensionValue(in.Length)
	if err != nil {
		return nil, err
	}
	width, err := normalizeDimensionValue(in.Width)
	if err != nil {
		return nil, err
	}
	height, err := normalizeDimensionValue(in.Height)
	if err != nil {
		return nil, err
	}
	diameter, err := normalizeDimensionValue(in.Diameter)
	if err != nil {
		return nil, err
	}

	_, findErr := s.dimensionsRepository.FindByProductID(ctx, productID)
	switch {
	case findErr == nil:
		_, err = s.dimensionsRepository.Update(ctx, productID, mysqlInfra.UpdateProductDimensionsInput{
			Weight: weight, Length: length, Width: width, Height: height, Diameter: diameter,
			UpdatedBy: actorID,
		})
	case errors.Is(findErr, mysqlInfra.ErrProductDimensionsNotFound):
		_, err = s.dimensionsRepository.Create(ctx, mysqlInfra.CreateProductDimensionsInput{
			ProductID: productID,
			Weight:    weight, Length: length, Width: width, Height: height, Diameter: diameter,
			CreatedBy: actorID,
		})
	default:
		return nil, fmt.Errorf("error loading product dimensions: %w", findErr)
	}
	if err != nil {
		return nil, fmt.Errorf("error saving product dimensions: %w", err)
	}

	return s.repository.FindAttributes(ctx, productID)
}

// normalizeDimensionValue valida una medida (número >= 0, dentro de
// decimal(10,2)) y la devuelve con dos decimales para casar con la columna.
func normalizeDimensionValue(raw string) (string, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > maxDimensionValue {
		return "", ErrInvalidDimensionsPatch
	}
	return strconv.FormatFloat(value, 'f', 2, 64), nil
}

// UpdateSEOInput lleva los metadatos SEO editables tal como llegan del
// formulario. Vacío = limpiar ese campo (se guarda NULL).
type UpdateSEOInput struct {
	MetaTitle       string
	MetaDescription string
	Keywords        string
}

// UpdateSEO hace un upsert de ecom_product_seo (crea la fila si el producto aún
// no tiene SEO) y devuelve la sección Atributos refrescada.
func (s *Service) UpdateSEO(ctx context.Context, productID, actorID int64, in UpdateSEOInput) (*mysqlInfra.ProductAttributesSectionDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductID
	}
	if actorID <= 0 {
		return nil, ErrInvalidSEOPatch
	}

	if _, err := s.repository.FindGeneral(ctx, productID); err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	metaTitle := strings.TrimSpace(in.MetaTitle)
	metaDescription := strings.TrimSpace(in.MetaDescription)
	keywords := strings.TrimSpace(in.Keywords)
	if len(metaTitle) > maxSEOMetaLen || len(metaDescription) > maxSEOMetaLen || len(keywords) > maxSEOKeywords {
		return nil, ErrInvalidSEOPatch
	}

	_, findErr := s.seoRepository.FindByProductID(ctx, productID)
	var err error
	switch {
	case findErr == nil:
		_, err = s.seoRepository.Update(ctx, productID, mysqlInfra.UpdateProductSEOInput{
			MetaTitle:       emptyToNil(metaTitle),
			MetaDescription: emptyToNil(metaDescription),
			Keywords:        emptyToNil(keywords),
			UpdatedBy:       actorID,
		})
	case errors.Is(findErr, mysqlInfra.ErrProductSEONotFound):
		_, err = s.seoRepository.Create(ctx, mysqlInfra.CreateProductSEOInput{
			ProductID:       productID,
			MetaTitle:       emptyToNil(metaTitle),
			MetaDescription: emptyToNil(metaDescription),
			Keywords:        emptyToNil(keywords),
			CreatedBy:       actorID,
		})
	default:
		return nil, fmt.Errorf("error loading product seo: %w", findErr)
	}
	if err != nil {
		return nil, fmt.Errorf("error saving product seo: %w", err)
	}

	return s.repository.FindAttributes(ctx, productID)
}

// emptyToNil convierte "" en nil para que el campo se guarde como NULL.
func emptyToNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

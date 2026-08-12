package products

import (
	"errors"
	"log"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidProductPayload = errors.New("invalid product payload")

var validProductTypes = map[string]bool{
	"part":       true,
	"consumable": true,
	"accessory":  true,
}

var validProductStatuses = map[string]bool{
	"active":       true,
	"discontinued": true,
	"hidden":       true,
}

type ProductService struct {
	repository                            *mysqlInfra.ProductRepository
	pendingVehicleFitmentsRepository      *mysqlInfra.PendingProductVehicleFitmentsRepository
	productVehicleCompatibilityRepository *mysqlInfra.ProductVehicleCompatibilityRepository
	partNumberSupersessionsRepository     *mysqlInfra.PartNumberSupersessionsRepository
}

func NewProductService(
	repository *mysqlInfra.ProductRepository,
	pendingVehicleFitmentsRepository *mysqlInfra.PendingProductVehicleFitmentsRepository,
	productVehicleCompatibilityRepository *mysqlInfra.ProductVehicleCompatibilityRepository,
	partNumberSupersessionsRepository *mysqlInfra.PartNumberSupersessionsRepository,
) *ProductService {
	return &ProductService{
		repository:                            repository,
		pendingVehicleFitmentsRepository:      pendingVehicleFitmentsRepository,
		productVehicleCompatibilityRepository: productVehicleCompatibilityRepository,
		partNumberSupersessionsRepository:     partNumberSupersessionsRepository,
	}
}

func (s *ProductService) GetPaginatedProducts(offset, pageSize int) (*mysqlInfra.PaginatedProducts, error) {
	if offset < 0 {
		offset = 0
	}

	if pageSize <= 0 {
		pageSize = 10
	}

	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindPaginated(offset, pageSize)
}

func (s *ProductService) GetProductByID(id int64) (*mysqlInfra.ProductDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidProductPayload
	}
	return s.repository.FindByID(id)
}

func (s *ProductService) CreateProduct(input mysqlInfra.CreateProductInput) (*mysqlInfra.ProductDTO, error) {
	if err := validateProductInput(input.SKU, input.PartNumber, input.Name, input.ProductType, input.Status, input.SourceID); err != nil {
		return nil, err
	}
	if input.CreatedBy <= 0 {
		return nil, ErrInvalidProductPayload
	}

	product, err := s.repository.Create(input)
	if err != nil {
		return nil, err
	}

	s.resolvePendingVehicleFitments(product)
	s.resolvePartNumberSupersessions(product)

	return product, nil
}

// resolvePendingVehicleFitments turns any staged compatibilities waiting on
// this product's SKU (created by a bulk vehicle-fitment import that ran
// before this product existed) into real ecom_product_vehicle_compatibility
// rows. It never fails product creation: a resolution issue here is logged
// and left pending rather than rolling back the product that was just made.
func (s *ProductService) resolvePendingVehicleFitments(product *mysqlInfra.ProductDTO) {
	pending, err := s.pendingVehicleFitmentsRepository.FindUnresolvedBySKU(product.SKU)
	if err != nil {
		log.Printf("error looking up pending vehicle fitments for sku %s: %v", product.SKU, err)
		return
	}

	for _, item := range pending {
		_, err := s.productVehicleCompatibilityRepository.Create(mysqlInfra.CreateProductVehicleCompatibilityInput{
			ProductID:        product.ID,
			VehicleFitmentID: item.VehicleFitmentID,
			Motor:            item.Motor,
			Position:         item.Position,
			Side:             item.Side,
			CreatedBy:        product.CreatedBy,
		})
		if err != nil && !errors.Is(err, mysqlInfra.ErrProductVehicleCompatibilityAlreadyExists) {
			log.Printf("error resolving pending vehicle fitment %d for sku %s: %v", item.ID, product.SKU, err)
			continue
		}

		if markErr := s.pendingVehicleFitmentsRepository.MarkResolved(item.ID, product.ID, product.CreatedBy); markErr != nil {
			log.Printf("error marking pending vehicle fitment %d resolved for sku %s: %v", item.ID, product.SKU, markErr)
		}
	}
}

// resolvePartNumberSupersessions vincula sucesiones de número de parte (ver
// ecom_part_number_supersessions) que estaban esperando este producto — ya sea como el lado
// "viejo" (pieza descontinuada) o el lado "nuevo" (pieza que reemplaza), pues cualquiera de
// los dos pudo no existir todavía en ecom_products cuando el ERP reportó la sucesión. Nunca
// hace fallar la creación del producto: un problema de resolución aquí se loguea y la fila
// queda pendiente en vez de revertir el producto que se acaba de crear.
func (s *ProductService) resolvePartNumberSupersessions(product *mysqlInfra.ProductDTO) {
	if err := s.partNumberSupersessionsRepository.ResolveForProduct(product.SourceID, product.ID, product.PartNumber, product.CreatedBy); err != nil {
		log.Printf("error resolving part number supersessions for part number %s: %v", product.PartNumber, err)
	}
}

func (s *ProductService) UpdateProduct(id int64, input mysqlInfra.UpdateProductInput) (*mysqlInfra.ProductDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidProductPayload
	}
	if err := validateProductInput(input.SKU, input.PartNumber, input.Name, input.ProductType, input.Status, input.SourceID); err != nil {
		return nil, err
	}
	if input.UpdatedBy <= 0 {
		return nil, ErrInvalidProductPayload
	}

	return s.repository.Update(id, input)
}

func (s *ProductService) DeleteProduct(id int64) error {
	if id <= 0 {
		return ErrInvalidProductPayload
	}
	return s.repository.SoftDelete(id)
}

func validateProductInput(sku, partNumber, name, productType string, status *string, sourceID int64) error {
	if sku == "" || partNumber == "" || name == "" {
		return ErrInvalidProductPayload
	}
	if productType != "" && !validProductTypes[productType] {
		return ErrInvalidProductPayload
	}
	if status != nil && *status != "" && !validProductStatuses[*status] {
		return ErrInvalidProductPayload
	}
	if sourceID <= 0 {
		return ErrInvalidProductPayload
	}
	return nil
}

package attributes

import (
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidAttributePayload        = errors.New("invalid attribute payload")
	ErrInvalidAttributeOptionPayload  = errors.New("invalid attribute option payload")
	ErrInvalidProductAttributePayload = errors.New("invalid product attribute payload")
	// ErrProductAttributeValueMismatch is returned when the value supplied for
	// a product attribute doesn't match the attribute's own data_type (e.g.
	// value_number set on a 'text' attribute, or no option_id given for an
	// 'enum' one).
	ErrProductAttributeValueMismatch = errors.New("product attribute value does not match the attribute's data type")
)

func isValidDataType(dataType string) bool {
	for _, allowed := range mysqlInfra.AttributeDataTypes {
		if dataType == allowed {
			return true
		}
	}
	return false
}

// AttributeService manages ecom_attributes, the canonical catalog of
// product attributes (e.g. COLOR, VOLTAGE, MATERIAL) that
// ecom_product_attributes and ecom_channel_attribute_map both reference.
type AttributeService struct {
	repository *mysqlInfra.AttributesRepository
}

func NewAttributeService(repository *mysqlInfra.AttributesRepository) *AttributeService {
	return &AttributeService{repository: repository}
}

func (s *AttributeService) GetPaginatedAttributes(offset, pageSize int) (*mysqlInfra.PaginatedAttributes, error) {
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

func (s *AttributeService) GetAttributeByID(id int64) (*mysqlInfra.AttributeDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidAttributePayload
	}
	return s.repository.FindByID(id)
}

func (s *AttributeService) GetAttributeByCode(code string) (*mysqlInfra.AttributeDTO, error) {
	if code == "" {
		return nil, ErrInvalidAttributePayload
	}
	return s.repository.FindByCode(code)
}

func (s *AttributeService) CreateAttribute(input mysqlInfra.CreateAttributeInput) (*mysqlInfra.AttributeDTO, error) {
	if input.Code == "" || input.Name == "" || input.CreatedBy <= 0 {
		return nil, ErrInvalidAttributePayload
	}
	if !isValidDataType(input.DataType) {
		return nil, ErrInvalidAttributePayload
	}

	return s.repository.Create(input)
}

func (s *AttributeService) UpdateAttribute(id int64, input mysqlInfra.UpdateAttributeInput) (*mysqlInfra.AttributeDTO, error) {
	if id <= 0 || input.Name == "" || input.UpdatedBy <= 0 {
		return nil, ErrInvalidAttributePayload
	}
	if !isValidDataType(input.DataType) {
		return nil, ErrInvalidAttributePayload
	}

	return s.repository.Update(id, input)
}

func (s *AttributeService) DeleteAttribute(id int64) error {
	if id <= 0 {
		return ErrInvalidAttributePayload
	}
	return s.repository.SoftDelete(id)
}

// AttributeOptionService manages ecom_attribute_options, the predefined
// value lists for attributes whose data_type is 'enum' (e.g. COLOR ->
// "Rojo"/"Azul"/"Negro"), so the same value is never stored with
// inconsistent casing/spelling across products.
type AttributeOptionService struct {
	repository *mysqlInfra.AttributeOptionsRepository
}

func NewAttributeOptionService(repository *mysqlInfra.AttributeOptionsRepository) *AttributeOptionService {
	return &AttributeOptionService{repository: repository}
}

func (s *AttributeOptionService) GetOptionsByAttribute(attributeID int64) ([]mysqlInfra.AttributeOptionDTO, error) {
	if attributeID <= 0 {
		return nil, ErrInvalidAttributeOptionPayload
	}
	return s.repository.FindByAttributeID(attributeID)
}

func (s *AttributeOptionService) GetOptionByID(id int64) (*mysqlInfra.AttributeOptionDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidAttributeOptionPayload
	}
	return s.repository.FindByID(id)
}

func (s *AttributeOptionService) CreateOption(input mysqlInfra.CreateAttributeOptionInput) (*mysqlInfra.AttributeOptionDTO, error) {
	if input.AttributeID <= 0 || input.Value == "" || input.CreatedBy <= 0 {
		return nil, ErrInvalidAttributeOptionPayload
	}
	return s.repository.Create(input)
}

func (s *AttributeOptionService) UpdateOption(id int64, input mysqlInfra.UpdateAttributeOptionInput) (*mysqlInfra.AttributeOptionDTO, error) {
	if id <= 0 || input.Value == "" || input.UpdatedBy <= 0 {
		return nil, ErrInvalidAttributeOptionPayload
	}
	return s.repository.Update(id, input)
}

func (s *AttributeOptionService) DeleteOption(id int64) error {
	if id <= 0 {
		return ErrInvalidAttributeOptionPayload
	}
	return s.repository.SoftDelete(id)
}

// ProductAttributeService manages ecom_product_attributes: the actual value
// of a catalog attribute on a specific product. It validates that the value
// supplied matches the attribute's own data_type before writing, since
// ecom_product_attributes stores the value in one of several typed columns
// depending on data_type.
type ProductAttributeService struct {
	repository           *mysqlInfra.ProductAttributesRepository
	attributesRepository *mysqlInfra.AttributesRepository
}

func NewProductAttributeService(repository *mysqlInfra.ProductAttributesRepository, attributesRepository *mysqlInfra.AttributesRepository) *ProductAttributeService {
	return &ProductAttributeService{repository: repository, attributesRepository: attributesRepository}
}

func (s *ProductAttributeService) GetAttributesByProduct(productID int64) ([]mysqlInfra.ProductAttributeDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidProductAttributePayload
	}
	return s.repository.FindByProductID(productID)
}

// SetValue creates or replaces the value of one attribute on one product.
// It looks up the attribute's data_type first and rejects a payload whose
// value doesn't correspond to it (e.g. ValueNumber set for a 'text'
// attribute), so ecom_product_attributes never ends up with a value in the
// wrong typed column.
func (s *ProductAttributeService) SetValue(input mysqlInfra.UpsertProductAttributeInput) (*mysqlInfra.ProductAttributeDTO, error) {
	if input.ProductID <= 0 || input.AttributeID <= 0 || input.ActorID <= 0 {
		return nil, ErrInvalidProductAttributePayload
	}

	attribute, err := s.attributesRepository.FindByID(input.AttributeID)
	if err != nil {
		return nil, err
	}

	if !valueMatchesDataType(attribute.DataType, input) {
		return nil, ErrProductAttributeValueMismatch
	}

	return s.repository.Upsert(input)
}

func (s *ProductAttributeService) DeleteValue(id int64) error {
	if id <= 0 {
		return ErrInvalidProductAttributePayload
	}
	return s.repository.SoftDelete(id)
}

func valueMatchesDataType(dataType string, input mysqlInfra.UpsertProductAttributeInput) bool {
	switch dataType {
	case "text":
		return input.ValueText != nil
	case "number":
		return input.ValueNumber != nil
	case "boolean":
		return input.ValueBool != nil
	case "date":
		return input.ValueDate != nil
	case "enum":
		return input.OptionID != nil
	default:
		return false
	}
}

package channel_attributes

import (
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidChannelAttributePayload    = errors.New("invalid channel attribute payload")
	ErrInvalidChannelAttributeMapPayload = errors.New("invalid channel attribute map payload")
)

// AllowedSystemFields whitelists the ecom_channel_attribute_map.system_field
// values a 'system_field' map row may point at: existing domain fields (not
// custom attributes) that can fill a channel attribute slot, mirroring what
// sync_mercadolibre_products.go hardcodes today for BRAND/PART_NUMBER/
// SELLER_SKU. Extend this list — and the resolver that will eventually read
// it — together whenever a new system field needs to be mappable.
var AllowedSystemFields = []string{
	"brand.name",
	"part_number",
	"sku",
	"dimensions.weight",
	"dimensions.length",
	"dimensions.width",
	"dimensions.height",
}

func isValidTargetStrategy(strategy string) bool {
	for _, allowed := range mysqlInfra.ChannelAttributeTargetStrategies {
		if strategy == allowed {
			return true
		}
	}
	return false
}

func isValidValueMode(mode string) bool {
	for _, allowed := range mysqlInfra.ChannelAttributeValueModes {
		if mode == allowed {
			return true
		}
	}
	return false
}

func isValidSourceType(sourceType string) bool {
	for _, allowed := range mysqlInfra.ChannelAttributeMapSourceTypes {
		if sourceType == allowed {
			return true
		}
	}
	return false
}

func isAllowedSystemField(field string) bool {
	for _, allowed := range AllowedSystemFields {
		if field == allowed {
			return true
		}
	}
	return false
}

// ChannelAttributeService manages ecom_channel_attributes: the catalog of
// attribute "slots" a channel/marketplace exposes on a listing — either a
// fixed key it already knows about (MercadoLibre's BRAND, PART_NUMBER,
// MODEL...) or a dynamic field with no fixed key (Odoo's attribute_line_ids)
// — optionally scoped to a single local category for channels whose
// required attributes vary by category.
type ChannelAttributeService struct {
	repository *mysqlInfra.ChannelAttributesRepository
}

func NewChannelAttributeService(repository *mysqlInfra.ChannelAttributesRepository) *ChannelAttributeService {
	return &ChannelAttributeService{repository: repository}
}

func (s *ChannelAttributeService) GetPaginatedAttributes(offset, pageSize int) (*mysqlInfra.PaginatedChannelAttributes, error) {
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

func (s *ChannelAttributeService) GetAttributeByID(id int64) (*mysqlInfra.ChannelAttributeDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidChannelAttributePayload
	}
	return s.repository.FindByID(id)
}

func (s *ChannelAttributeService) GetAttributesByChannel(channelID int64) ([]mysqlInfra.ChannelAttributeDTO, error) {
	if channelID <= 0 {
		return nil, ErrInvalidChannelAttributePayload
	}
	return s.repository.FindByChannelID(channelID)
}

// GetApplicableAttributes returns every slot a listing under categoryID
// should fill on channelID (category-specific rows plus channel-wide ones).
// Exposed for admin/preview use; the marketplace sync flows don't call it
// yet — see FindApplicable's own comment.
func (s *ChannelAttributeService) GetApplicableAttributes(channelID int64, categoryID *int64) ([]mysqlInfra.ChannelAttributeDTO, error) {
	if channelID <= 0 {
		return nil, ErrInvalidChannelAttributePayload
	}
	return s.repository.FindApplicable(channelID, categoryID)
}

func (s *ChannelAttributeService) CreateAttribute(input mysqlInfra.CreateChannelAttributeInput) (*mysqlInfra.ChannelAttributeDTO, error) {
	if err := validateChannelAttribute(input.ChannelID, input.TargetStrategy, input.ExternalKey, input.TargetField, input.ValueMode, input.CreatedBy); err != nil {
		return nil, err
	}

	return s.repository.Create(input)
}

func (s *ChannelAttributeService) UpdateAttribute(id int64, input mysqlInfra.UpdateChannelAttributeInput) (*mysqlInfra.ChannelAttributeDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidChannelAttributePayload
	}
	if err := validateChannelAttribute(1, input.TargetStrategy, input.ExternalKey, input.TargetField, input.ValueMode, input.UpdatedBy); err != nil {
		return nil, err
	}

	return s.repository.Update(id, input)
}

func (s *ChannelAttributeService) DeleteAttribute(id int64) error {
	if id <= 0 {
		return ErrInvalidChannelAttributePayload
	}
	return s.repository.SoftDelete(id)
}

// validateChannelAttribute checks the invariants a channel attribute slot
// must satisfy regardless of whether it's being created or updated: a valid
// target_strategy/value_mode, and — depending on target_strategy — either
// external_key (fixed_key) or target_field (dynamic_field) present.
// channelID/actorID only need to be positive; the placeholder value 1 is
// passed from UpdateAttribute since that input has no channel_id of its own.
func validateChannelAttribute(channelID int64, targetStrategy string, externalKey, targetField *string, valueMode string, actorID int64) error {
	if channelID <= 0 || actorID <= 0 {
		return ErrInvalidChannelAttributePayload
	}
	if !isValidTargetStrategy(targetStrategy) || !isValidValueMode(valueMode) {
		return ErrInvalidChannelAttributePayload
	}

	switch targetStrategy {
	case "fixed_key":
		if externalKey == nil || *externalKey == "" {
			return ErrInvalidChannelAttributePayload
		}
	case "dynamic_field":
		if targetField == nil || *targetField == "" {
			return ErrInvalidChannelAttributePayload
		}
	}

	return nil
}

// ChannelAttributeMapService manages ecom_channel_attribute_map: which
// internal source (a custom ecom_attributes value, an existing domain field,
// or a fixed literal) fills a given ecom_channel_attributes slot, optionally
// overridden per connection.
type ChannelAttributeMapService struct {
	repository *mysqlInfra.ChannelAttributeMapRepository
}

func NewChannelAttributeMapService(repository *mysqlInfra.ChannelAttributeMapRepository) *ChannelAttributeMapService {
	return &ChannelAttributeMapService{repository: repository}
}

func (s *ChannelAttributeMapService) GetMapsByChannelAttribute(channelAttributeID int64) ([]mysqlInfra.ChannelAttributeMapDTO, error) {
	if channelAttributeID <= 0 {
		return nil, ErrInvalidChannelAttributeMapPayload
	}
	return s.repository.FindByChannelAttributeID(channelAttributeID)
}

func (s *ChannelAttributeMapService) GetMapByID(id int64) (*mysqlInfra.ChannelAttributeMapDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidChannelAttributeMapPayload
	}
	return s.repository.FindByID(id)
}

// ResolveForConnection returns the map row that applies to channelAttributeID
// on connectionID (exact connection override, falling back to the
// channel-wide row). Exposed for admin/preview use; not called by the
// marketplace sync flows yet.
func (s *ChannelAttributeMapService) ResolveForConnection(channelAttributeID, connectionID int64) (*mysqlInfra.ChannelAttributeMapDTO, error) {
	if channelAttributeID <= 0 || connectionID <= 0 {
		return nil, ErrInvalidChannelAttributeMapPayload
	}
	return s.repository.FindByChannelAttributeAndConnection(channelAttributeID, connectionID)
}

func (s *ChannelAttributeMapService) CreateMap(input mysqlInfra.CreateChannelAttributeMapInput) (*mysqlInfra.ChannelAttributeMapDTO, error) {
	if input.ChannelAttributeID <= 0 || input.CreatedBy <= 0 {
		return nil, ErrInvalidChannelAttributeMapPayload
	}
	if err := validateChannelAttributeMapSource(input.SourceType, input.AttributeID, input.SystemField, input.StaticValue); err != nil {
		return nil, err
	}

	return s.repository.Create(input)
}

func (s *ChannelAttributeMapService) UpdateMap(id int64, input mysqlInfra.UpdateChannelAttributeMapInput) (*mysqlInfra.ChannelAttributeMapDTO, error) {
	if id <= 0 || input.UpdatedBy <= 0 {
		return nil, ErrInvalidChannelAttributeMapPayload
	}
	if err := validateChannelAttributeMapSource(input.SourceType, input.AttributeID, input.SystemField, input.StaticValue); err != nil {
		return nil, err
	}

	return s.repository.Update(id, input)
}

func (s *ChannelAttributeMapService) DeleteMap(id int64) error {
	if id <= 0 {
		return ErrInvalidChannelAttributeMapPayload
	}
	return s.repository.SoftDelete(id)
}

// validateChannelAttributeMapSource enforces that exactly the field matching
// source_type is populated: attribute_id for custom_attribute, a whitelisted
// system_field for system_field, static_value for static_value.
func validateChannelAttributeMapSource(sourceType string, attributeID *int64, systemField, staticValue *string) error {
	if !isValidSourceType(sourceType) {
		return ErrInvalidChannelAttributeMapPayload
	}

	switch sourceType {
	case "custom_attribute":
		if attributeID == nil || *attributeID <= 0 {
			return ErrInvalidChannelAttributeMapPayload
		}
	case "system_field":
		if systemField == nil || !isAllowedSystemField(*systemField) {
			return ErrInvalidChannelAttributeMapPayload
		}
	case "static_value":
		if staticValue == nil || *staticValue == "" {
			return ErrInvalidChannelAttributeMapPayload
		}
	}

	return nil
}

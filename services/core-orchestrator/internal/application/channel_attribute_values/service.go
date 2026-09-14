// Package channel_attribute_values sets the value of a custom product
// attribute for a channel's own attribute vocabulary (e.g. MercadoLibre's
// external_key "MATERIAL"), resolving a product by SKU and reusing — or, on
// first use, creating — every row the existing ecom_attributes/
// ecom_channel_attributes/ecom_channel_attribute_map/ecom_product_attributes
// chain requires (see channel_attributes and attributes packages for the CRUD
// operations this composes). It exists so a caller only has to know
// {sku, externalKey, value} instead of manually wiring the four tables
// together through their individual CRUD endpoints. MercadoLibre-only for
// now (see mercadoLibreChannelCode) — extending to other channels means
// accepting a channel code instead of hardcoding one.
package channel_attribute_values

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	attributesApp "core-orchestrator/internal/application/attributes"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// LocalCategoryResolver is satisfied by *sync.MercadoLibreCategoryPredictorService
// (duck-typed — no import of the sync package here). sync_mercadolibre_products.go
// needs to import *this* package too, to reuse ProvisionCategoryAttributes
// when building an item's required attributes at publish time; importing
// sync back here would create an import cycle.
type LocalCategoryResolver interface {
	EnsureLocalCategory(ctx context.Context, connectionID int64, externalCategoryID string, actorID int64) (int64, error)
}

// mercadoLibreChannelCode is ecom_channels.code for MercadoLibre — the channel
// SetValue defaults to when no ConnectionID is given (mirrors the literal used
// to register channel_listings.Publisher/Refresher and the token refresher in
// main.go).
const mercadoLibreChannelCode = "MERCADOLIBRE"

// odooChannelCode / odooProductInfoTargetField: a channel whose
// ecom_channels.attribute_scope is "product" (Odoo) stores custom attributes in
// a per-product key/value model rather than per-category fixed keys, so SetValue
// creates its ecom_channel_attributes slots as target_strategy 'dynamic_field'
// pointing at that model, with category_id NULL. The literal is duplicated from
// odooInfra.ProductInfoModel to avoid an application→marketplace-infra import.
// See ADR 0004.
const (
	odooChannelCode            = "ODOO"
	odooProductInfoTargetField = "website.sale.product.info"
)

// Defaults applied to a newly-created ecom_channel_attributes slot: MercadoLibre
// exposes a fixed, known vocabulary of external keys (target_strategy
// 'fixed_key'). Most custom attributes are set by value_name; value_id is
// used for MercadoLibre-managed fixed attributes like ITEM_CONDITION (which
// this endpoint never touches) and, since options are now provisioned from
// MercadoLibre's own closed lists, for any custom_attribute whose
// value_type is "list"/"string_list" (see
// CategoryRequiredAttribute.IsListType) — ProvisionCategoryAttributes decides
// which mode applies per attribute; SetValue always uses value_name since it
// has no MercadoLibre category payload to read value_type from.
const (
	channelAttributeDefaultTargetStrategy = "fixed_key"
	channelAttributeDynamicTargetStrategy = "dynamic_field"
	channelAttributeDefaultValueMode      = "value_name"
	channelAttributeValueModeValueID      = "value_id"
)

var (
	ErrInvalidSetValueInput = errors.New("sku, externalKey, a valid dataType and a matching value are required")
	// ErrAttributeDataTypeConflict is returned when externalKey already
	// resolved to an ecom_attributes row (by code) on an earlier call, but
	// this call's dataType doesn't match it — the existing attribute's
	// data_type is treated as the stable, authoritative one rather than
	// silently overwritten.
	ErrAttributeDataTypeConflict = errors.New("an attribute with this external key already exists with a different dataType")
	// ErrChannelAttributeMapConflict is returned when externalKey's channel
	// attribute slot already has a channel-wide (connection_id IS NULL) map
	// row pointing somewhere other than the attribute this call resolved —
	// e.g. an admin already wired it to a system_field or static_value
	// through /api/channel-attribute-map. This endpoint never overwrites an
	// existing mapping; it must be repointed through that endpoint instead.
	ErrChannelAttributeMapConflict = errors.New("this external key is already mapped to a different source; edit it via /api/channel-attribute-map")
	ErrInvalidProvisionInput       = errors.New("sku and categoryId are required")
	// ErrAttributeOptionNotProvisioned is returned by SetValue when the
	// target attribute is value_id-mode (its ecom_channel_attributes slot was
	// provisioned from a MercadoLibre closed list — see
	// ProvisionCategoryAttributes) and the given value doesn't match any
	// option already provisioned for it. Unlike a value_name attribute,
	// SetValue never invents a new option here: MercadoLibre would reject
	// (or silently mismatch) a value_id it doesn't recognize, so the caller
	// must pick from GET /api/attributes/{attributeId}/options instead.
	ErrAttributeOptionNotProvisioned = errors.New("value is not one of the options provisioned for this attribute; see GET /api/attributes/{attributeId}/options for the allowed values")
)

// systemFieldByExternalKey lists the MercadoLibre required-attribute ids
// ProvisionCategoryAttributes maps to an existing domain field (via
// ecom_channel_attribute_map's source_type 'system_field') instead of a
// custom_attribute: their values already come from elsewhere —
// ecom_product_dimensions for the SELLER_PACKAGE_* ones, and the product/
// brand tables for BRAND/PART_NUMBER/SELLER_SKU/MODEL — mirroring exactly
// what sync_mercadolibre_products.go's createNewItem/resolvePackageAttributes
// already hardcode. Values must be members of channel_attributes.AllowedSystemFields.
var systemFieldByExternalKey = map[string]string{
	"BRAND":                 "brand.name",
	"PART_NUMBER":           "part_number",
	"MODEL":                 "part_number", // createNewItem sends the same PartNumber for both
	"SELLER_SKU":            "sku",
	"SELLER_PACKAGE_HEIGHT": "dimensions.height",
	"SELLER_PACKAGE_LENGTH": "dimensions.length",
	"SELLER_PACKAGE_WIDTH":  "dimensions.width",
	"SELLER_PACKAGE_WEIGHT": "dimensions.weight",
}

// skippedExternalKeys are required attributes ProvisionCategoryAttributes
// leaves completely unmanaged: ITEM_CONDITION is sent by createNewItem as a
// fixed, non-per-product value_id (mercadoLibreItemConditionValue), not
// something sourced from product data or worth a channel_attribute_map row.
var skippedExternalKeys = map[string]bool{
	"ITEM_CONDITION": true,
}

// SetValueInput carries everything needed to resolve/create the
// attribute/channel-attribute/map chain and write the value: exactly one of
// ValueText/ValueNumber/ValueBool/ValueDate/EnumValue must be set, matching
// DataType.
type SetValueInput struct {
	SKU string
	// ConnectionID picks the channel: nil resolves to MercadoLibre (the
	// historical default), otherwise the connection's channel is used — which is
	// how an Odoo (attribute_scope "product") slot gets created as a
	// dynamic_field targeting website.sale.product.info. See ADR 0004.
	ConnectionID *int64
	ExternalKey  string
	DataType     string
	ValueText    *string
	ValueNumber  *float64
	ValueBool    *bool
	ValueDate    *time.Time
	// EnumValue is the raw option text for DataType "enum" — the option row
	// itself (ecom_attribute_options) is resolved/created inside SetValue,
	// once the attribute it belongs to is known.
	EnumValue *string
	ActorID   int64
}

// SetValueResult reports every row SetValue resolved or created, so a caller
// can see exactly what got wired up on a first call for a new externalKey.
type SetValueResult struct {
	Attribute           *mysqlInfra.AttributeDTO           `json:"attribute"`
	ChannelAttribute    *mysqlInfra.ChannelAttributeDTO    `json:"channelAttribute"`
	ChannelAttributeMap *mysqlInfra.ChannelAttributeMapDTO `json:"channelAttributeMap"`
	ProductAttribute    *mysqlInfra.ProductAttributeDTO    `json:"productAttribute"`
}

type Service struct {
	db                            *sql.DB
	productRepository             *mysqlInfra.ProductRepository
	channelRepository             *mysqlInfra.ChannelRepository
	channelConnectionRepository   *mysqlInfra.ChannelConnectionRepository
	attributesRepository          *mysqlInfra.AttributesRepository
	attributeOptionsRepository    *mysqlInfra.AttributeOptionsRepository
	channelAttributesRepository   *mysqlInfra.ChannelAttributesRepository
	channelAttributeMapRepository *mysqlInfra.ChannelAttributeMapRepository
	productAttributeService       *attributesApp.ProductAttributeService
	categoriesHandler             *mercadoLibreInfra.CategoriesHandler
	categoryPredictorService      LocalCategoryResolver
}

func NewService(
	db *sql.DB,
	productRepository *mysqlInfra.ProductRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	attributesRepository *mysqlInfra.AttributesRepository,
	attributeOptionsRepository *mysqlInfra.AttributeOptionsRepository,
	channelAttributesRepository *mysqlInfra.ChannelAttributesRepository,
	channelAttributeMapRepository *mysqlInfra.ChannelAttributeMapRepository,
	productAttributeService *attributesApp.ProductAttributeService,
	categoriesHandler *mercadoLibreInfra.CategoriesHandler,
	categoryPredictorService LocalCategoryResolver,
) *Service {
	return &Service{
		db:                            db,
		productRepository:             productRepository,
		channelRepository:             channelRepository,
		channelConnectionRepository:   channelConnectionRepository,
		attributesRepository:          attributesRepository,
		attributeOptionsRepository:    attributeOptionsRepository,
		channelAttributesRepository:   channelAttributesRepository,
		channelAttributeMapRepository: channelAttributeMapRepository,
		productAttributeService:       productAttributeService,
		categoriesHandler:             categoriesHandler,
		categoryPredictorService:      categoryPredictorService,
	}
}

// SetValue resolves product by input.SKU, then walks/creates the chain a
// MercadoLibre custom attribute needs:
//
//  1. ecom_channel_attributes — the "MATERIAL slot" on the MercadoLibre
//     channel. Reused if one already applies to product's own category (or a
//     channel-wide one with category_id NULL — see resolveOrCreateChannelAttribute),
//     scoped to product's category on first creation (NULL if product has no
//     category yet), since MercadoLibre often requires different attributes
//     per category.
//  2. ecom_attributes — the generic attribute catalog entry, found/created by
//     code = input.ExternalKey (MercadoLibre's own keys already double as a
//     stable, human-legible code).
//  3. ecom_channel_attribute_map — links the slot to the catalog entry,
//     channel-wide (connection_id NULL) so it applies to every MercadoLibre
//     connection.
//  4. ecom_product_attributes — the actual value on product, written through
//     attributes.ProductAttributeService.SetValue (reusing its data_type
//     validation and upsert).
//
// A (product, externalKey) pair that already has a value simply gets it
// replaced, mirroring ecom_product_attributes' own upsert-on-write semantics.
func (s *Service) SetValue(ctx context.Context, input SetValueInput) (*SetValueResult, error) {
	if err := validateSetValueInput(input); err != nil {
		return nil, err
	}

	product, err := s.productRepository.FindBySKU(ctx, strings.TrimSpace(input.SKU))
	if err != nil {
		return nil, err
	}

	channel, err := s.resolveChannel(ctx, input.ConnectionID)
	if err != nil {
		return nil, err
	}

	externalKey := strings.TrimSpace(input.ExternalKey)

	// A "product"-scope channel (Odoo) keeps its slots category-agnostic
	// (category_id NULL) and targets a per-product key/value model via
	// dynamic_field; a "category"-scope channel (MercadoLibre) scopes the slot
	// to the product's own category with a fixed external key.
	slotCategoryID := product.CategoryID
	targetStrategy := channelAttributeDefaultTargetStrategy
	var targetField *string
	if channel.AttributeScope == mysqlInfra.ChannelAttributeScopeProduct {
		slotCategoryID = nil
		targetStrategy = channelAttributeDynamicTargetStrategy
		field := odooProductInfoTargetField
		targetField = &field
	}

	// isRequired/valueMode are always false/value_name here: SetValue has no
	// MercadoLibre category payload to read Tags.Required/value_type from
	// (unlike ProvisionCategoryAttributes), so a slot it creates can't claim
	// to know either. If the slot already exists — e.g. created earlier by
	// ProvisionCategoryAttributes — its existing is_required/value_mode are
	// left untouched (resolveOrCreateChannelAttribute never overwrites an
	// existing row's flags), which is what makes the value_id strictness
	// below (resolveOrCreateOption) actually take effect for a
	// list-provisioned attribute even though this call site itself never
	// requests value_id mode.
	channelAttribute, err := s.resolveOrCreateChannelAttribute(ctx, channel.ID, slotCategoryID, externalKey, targetStrategy, targetField, false, channelAttributeDefaultValueMode, input.ActorID)
	if err != nil {
		return nil, err
	}

	attribute, err := s.resolveOrCreateAttribute(ctx, externalKey, externalKey, input.DataType, false, input.ActorID)
	if err != nil {
		return nil, err
	}

	channelAttributeMap, err := s.resolveOrCreateChannelAttributeMap(ctx, channelAttribute.ID, "custom_attribute", &attribute.ID, nil, input.ActorID)
	if err != nil {
		return nil, err
	}

	upsertInput := mysqlInfra.UpsertProductAttributeInput{
		ProductID:   product.ID,
		AttributeID: attribute.ID,
		ValueText:   input.ValueText,
		ValueNumber: input.ValueNumber,
		ValueBool:   input.ValueBool,
		ValueDate:   input.ValueDate,
		ActorID:     input.ActorID,
	}

	if input.DataType == "enum" {
		// A slot provisioned as value_id (channelAttribute.ValueMode) is tied
		// to MercadoLibre's own closed catalogue: only an option already
		// provisioned from that catalogue (ecom_attribute_options.external_value_id
		// set — see ProvisionCategoryAttributes) may be selected here.
		requireExisting := channelAttribute.ValueMode == channelAttributeValueModeValueID
		option, err := s.resolveOrCreateOption(ctx, attribute.ID, *input.EnumValue, requireExisting, input.ActorID)
		if err != nil {
			return nil, err
		}
		upsertInput.OptionID = &option.ID
	}

	productAttribute, err := s.productAttributeService.SetValue(ctx, upsertInput)
	if err != nil {
		return nil, err
	}

	return &SetValueResult{
		Attribute:           attribute,
		ChannelAttribute:    channelAttribute,
		ChannelAttributeMap: channelAttributeMap,
		ProductAttribute:    productAttribute,
	}, nil
}

// ProvisionCategoryAttributesInput carries what's needed to seed the
// required-attribute slots for a product's MercadoLibre category: SKU
// resolves the product (its own local category scopes any new
// ecom_channel_attributes row — see resolveOrCreateChannelAttribute),
// CategoryID is MercadoLibre's own category id, used to call MELI's
// GET /categories/{id}/attributes (a public endpoint — no connection/access
// token needed) and, when the product has no local category yet, to build
// one. ConnectionID is optional and only used for that second part: without
// it, a product with no category keeps ecom_channel_attributes slots scoped
// channel-wide (category_id NULL) instead of to a specific category — see
// ProvisionCategoryAttributes.
type ProvisionCategoryAttributesInput struct {
	SKU          string
	CategoryID   string
	ConnectionID *int64
	ActorID      int64
}

// ProvisionedAttributeOutcome reports what happened for one required
// attribute MercadoLibre's category-attributes response listed.
type ProvisionedAttributeOutcome struct {
	ExternalKey string `json:"externalKey"`
	Name        string `json:"name"`
	// IsRequired mirrors MercadoLibre's own Tags.Required for this attribute
	// (see ecom_channel_attributes.is_required) — false doesn't mean
	// unimportant, just optional: ProvisionCategoryAttributes provisions both,
	// so a caller (e.g. resolveCustomAttributes) can still tell them apart.
	IsRequired bool `json:"isRequired"`
	// IsListType is true when MercadoLibre restricts this attribute to a
	// fixed set of values (value_type "list"/"string_list") — its allowed
	// options were provisioned into ecom_attribute_options (with their real
	// external_value_id) as part of this same call, and are retrievable via
	// GET /api/attributes/{attributeId}/options. A caller building a value
	// for this attribute (e.g. before calling SetValue) should pick one of
	// those instead of free-typing text.
	IsListType bool `json:"isListType,omitempty"`
	// SourceType is "system_field" or "custom_attribute" — never set when
	// Skipped is true.
	SourceType  string `json:"sourceType,omitempty"`
	SystemField string `json:"systemField,omitempty"`
	AttributeID *int64 `json:"attributeId,omitempty"`
	// DataType is the mysqlInfra.AttributeDataTypes value this attribute's
	// ecom_attributes row was created/reused with (see
	// mapMercadoLibreValueType) — only set when SourceType is
	// "custom_attribute". Lets a caller that already has this attribute's
	// live value in hand (e.g. sync.MercadoLibreListingsAuditService reading
	// it straight off a MercadoLibre item) call SetValue with the exact
	// dataType it needs, without re-deriving the mapping itself.
	DataType string `json:"dataType,omitempty"`
	// Skipped is true for an attribute this endpoint deliberately doesn't
	// manage (see skippedExternalKeys) or one whose tags mark it read_only
	// (MercadoLibre computes or fixes those itself — a seller-facing flow
	// shouldn't try to set them). Tags.Hidden alone does NOT skip an
	// attribute — MercadoLibre uses it to mean "not shown in the simplified
	// publish form", not "not seller-settable" (confirmed against a real
	// category: SELLER_PACKAGE_* come back hidden=true, read_only=false, and
	// this integration does set them, via the system_field path below).
	Skipped bool   `json:"skipped,omitempty"`
	Error   string `json:"error,omitempty"`
}

type ProvisionCategoryAttributesResult struct {
	CategoryID string `json:"categoryId"`
	// LocalCategoryID is whichever ecom_categories.id every created slot in
	// Results was scoped to — product's own pre-existing category, the one
	// just built/assigned from MercadoLibre's path_from_root (see
	// ProvisionCategoryAttributes), or nil if neither applied (channel-wide
	// slots).
	LocalCategoryID *int64                        `json:"localCategoryId,omitempty"`
	Results         []ProvisionedAttributeOutcome `json:"results"`
}

// ProvisionCategoryAttributes reads every attribute MercadoLibre's category
// input.CategoryID exposes — required and optional alike, so slots exist to
// fill in optional attributes too when the data is available (better catalog
// completeness/exposure on the listing) — and, for each one not already
// deliberately left alone (skippedExternalKeys, or read_only),
// resolves or creates the slot it needs. Each outcome's IsRequired mirrors
// MercadoLibre's own Tags.Required for that attribute, so a caller can still
// tell a genuinely mandatory attribute apart from an optional one:
//
//   - systemFieldByExternalKey members (BRAND, PART_NUMBER, MODEL, SELLER_SKU,
//     SELLER_PACKAGE_*) get an ecom_channel_attributes slot mapped via
//     ecom_channel_attribute_map's source_type 'system_field' — never an
//     ecom_attributes row or a product_attributes value, since their values
//     already come from the product/brand/ecom_product_dimensions tables (see
//     sync_mercadolibre_products.go's createNewItem/resolvePackageAttributes).
//   - Everything else gets the same custom_attribute chain SetValue builds
//     (ecom_attributes + ecom_channel_attributes + ecom_channel_attribute_map),
//     named after MercadoLibre's own attribute name and typed via
//     mapMercadoLibreValueType — but with no ecom_product_attributes value
//     written, since this call has no per-attribute value to give it; use
//     SetValue afterwards for that.
//
// If product has no local category yet (ecom_products.category_id IS NULL)
// and input.ConnectionID is given, ProvisionCategoryAttributes first
// resolves one: it reuses sync.MercadoLibreCategoryPredictorService.
// EnsureLocalCategory — the same recursive path_from_root replication the
// category predictor flow already performs — to build (or reuse) the local
// ecom_categories chain for input.CategoryID under that connection, records
// it in ecom_channel_category_map, and assigns the resulting leaf category to
// the product via ProductRepository.UpdateCategoryID. Every slot this call
// creates is then scoped to that category. Without a ConnectionID, a
// category-less product keeps getting channel-wide (category_id NULL) slots,
// same as before.
//
// One attribute failing (e.g. an ErrAttributeDataTypeConflict from an earlier
// SetValue call) never stops the rest of the batch.
func (s *Service) ProvisionCategoryAttributes(ctx context.Context, input ProvisionCategoryAttributesInput) (*ProvisionCategoryAttributesResult, error) {
	categoryID := strings.TrimSpace(input.CategoryID)
	if strings.TrimSpace(input.SKU) == "" || categoryID == "" || input.ActorID <= 0 {
		return nil, ErrInvalidProvisionInput
	}
	if input.ConnectionID != nil && *input.ConnectionID <= 0 {
		return nil, ErrInvalidProvisionInput
	}

	product, err := s.productRepository.FindBySKU(ctx, strings.TrimSpace(input.SKU))
	if err != nil {
		return nil, err
	}

	channel, err := s.channelRepository.FindByCode(ctx, mercadoLibreChannelCode)
	if err != nil {
		return nil, fmt.Errorf("error loading %s channel: %w", mercadoLibreChannelCode, err)
	}

	localCategoryID := product.CategoryID
	if localCategoryID == nil && input.ConnectionID != nil {
		leafCategoryID, err := s.categoryPredictorService.EnsureLocalCategory(ctx, *input.ConnectionID, categoryID, input.ActorID)
		if err != nil {
			return nil, fmt.Errorf("error resolving local category for mercadolibre category %s: %w", categoryID, err)
		}
		if err := s.productRepository.UpdateCategoryID(ctx, product.ID, leafCategoryID, input.ActorID); err != nil {
			return nil, fmt.Errorf("error assigning category %d to product %d: %w", leafCategoryID, product.ID, err)
		}
		localCategoryID = &leafCategoryID
	}

	categoryAttributes, err := s.categoriesHandler.GetCategoryAttributes(ctx, categoryID)
	if err != nil {
		return nil, fmt.Errorf("error loading mercadolibre category %s attributes: %w", categoryID, err)
	}

	results := make([]ProvisionedAttributeOutcome, 0, len(categoryAttributes))
	for _, attr := range categoryAttributes {
		outcome := ProvisionedAttributeOutcome{ExternalKey: attr.ID, Name: attr.Name, IsRequired: attr.Tags.Required, IsListType: attr.IsListType()}

		if skippedExternalKeys[attr.ID] {
			outcome.Skipped = true
			outcome.Error = "already handled as a fixed value elsewhere in the mercadolibre item-creation flow"
			results = append(results, outcome)
			continue
		}
		if attr.Tags.ReadOnly {
			outcome.Skipped = true
			outcome.Error = "attribute is read-only; mercadolibre computes/fixes it itself"
			results = append(results, outcome)
			continue
		}

		if systemField, ok := systemFieldByExternalKey[attr.ID]; ok {
			// system_field slots are always value_name: their value comes
			// straight from product/brand/dimensions data, never from a
			// MercadoLibre closed list.
			channelAttribute, err := s.resolveOrCreateChannelAttribute(ctx, channel.ID, localCategoryID, attr.ID, channelAttributeDefaultTargetStrategy, nil, attr.Tags.Required, channelAttributeDefaultValueMode, input.ActorID)
			if err != nil {
				outcome.Error = err.Error()
				results = append(results, outcome)
				continue
			}
			if _, err := s.resolveOrCreateChannelAttributeMap(ctx, channelAttribute.ID, "system_field", nil, &systemField, input.ActorID); err != nil {
				outcome.Error = err.Error()
				results = append(results, outcome)
				continue
			}
			outcome.SourceType = "system_field"
			outcome.SystemField = systemField
			results = append(results, outcome)
			continue
		}

		valueMode := channelAttributeDefaultValueMode
		if attr.IsListType() {
			valueMode = channelAttributeValueModeValueID
		}
		channelAttribute, err := s.resolveOrCreateChannelAttribute(ctx, channel.ID, localCategoryID, attr.ID, channelAttributeDefaultTargetStrategy, nil, attr.Tags.Required, valueMode, input.ActorID)
		if err != nil {
			outcome.Error = err.Error()
			results = append(results, outcome)
			continue
		}

		dataType := mapMercadoLibreDataType(attr)
		// reconcileToEnum: MercadoLibre is the source of truth for whether this
		// attribute is a closed list. If an earlier provision (before "string
		// with values" counted as a list) or a SetValue call created the local
		// ecom_attributes row as free "text", widen it to "enum" in place
		// rather than failing every publish with ErrAttributeDataTypeConflict.
		attribute, err := s.resolveOrCreateAttribute(ctx, attr.ID, attr.Name, dataType, dataType == "enum", input.ActorID)
		if err != nil {
			outcome.Error = err.Error()
			results = append(results, outcome)
			continue
		}

		if _, err := s.resolveOrCreateChannelAttributeMap(ctx, channelAttribute.ID, "custom_attribute", &attribute.ID, nil, input.ActorID); err != nil {
			outcome.Error = err.Error()
			results = append(results, outcome)
			continue
		}

		if attr.IsListType() {
			if err := s.provisionAttributeOptions(ctx, attribute.ID, attr.Values, input.ActorID); err != nil {
				outcome.Error = err.Error()
				results = append(results, outcome)
				continue
			}
		}

		outcome.SourceType = "custom_attribute"
		outcome.AttributeID = &attribute.ID
		outcome.DataType = dataType
		results = append(results, outcome)
	}

	return &ProvisionCategoryAttributesResult{CategoryID: categoryID, LocalCategoryID: localCategoryID, Results: results}, nil
}

// mapMercadoLibreDataType maps a MercadoLibre category attribute to one of
// mysqlInfra.AttributeDataTypes: any closed-catalogue attribute (see
// CategoryRequiredAttribute.IsListType — "list"/"string_list", or a "string"
// that ships a values list) becomes "enum", since its allowed values are
// provisioned into ecom_attribute_options (see provisionAttributeOptions);
// "number" and "boolean" map directly; everything else (free "string",
// "number_unit", ...) becomes "text". Keyed off the whole attribute rather
// than just value_type so the "string-with-values" case lands on "enum" too.
func mapMercadoLibreDataType(attr mercadoLibreInfra.CategoryRequiredAttribute) string {
	if attr.IsListType() {
		return "enum"
	}
	switch attr.ValueType {
	case "number":
		return "number"
	case "boolean":
		return "boolean"
	default:
		return "text"
	}
}

// provisionAttributeOptions resolves-or-creates an ecom_attribute_options row
// for each of MercadoLibre's own allowed values under attributeID, keyed by
// external_value_id (MercadoLibre's stable id for that option) rather than
// its display name — the name is what could in principle change on
// MercadoLibre's side without the id changing. An existing option's Value
// (label) is never updated to match a later call's name, mirroring
// resolveOrCreateChannelAttribute's own "never overwrite on reuse" rule. One
// value failing never stops the rest of the batch — the first error is
// returned once every value has been attempted, same shape as
// channel_listings.updateExistingListings.
func (s *Service) provisionAttributeOptions(ctx context.Context, attributeID int64, values []mercadoLibreInfra.CategoryAttributeValue, actorID int64) error {
	var firstErr error

	for _, value := range values {
		externalValueID := strings.TrimSpace(value.ID)
		if externalValueID == "" {
			continue
		}

		existing, err := s.attributeOptionsRepository.FindByAttributeAndExternalValueID(ctx, attributeID, externalValueID)
		if err != nil && !errors.Is(err, mysqlInfra.ErrAttributeOptionNotFound) {
			if firstErr == nil {
				firstErr = fmt.Errorf("error loading option %s for attribute %d: %w", externalValueID, attributeID, err)
			}
			continue
		}
		if existing != nil {
			continue
		}

		if _, err := s.attributeOptionsRepository.Create(ctx, mysqlInfra.CreateAttributeOptionInput{
			AttributeID:     attributeID,
			Value:           value.Name,
			ExternalValueID: &externalValueID,
			CreatedBy:       actorID,
		}); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("error creating option %s for attribute %d: %w", externalValueID, attributeID, err)
			}
			continue
		}
	}

	return firstErr
}

// resolveChannel picks the channel a SetValue call targets: nil connectionID
// resolves to MercadoLibre (the historical default), otherwise the given
// connection's channel is loaded. Used so an Odoo (attribute_scope "product")
// connection routes SetValue to a dynamic_field slot. See ADR 0004.
func (s *Service) resolveChannel(ctx context.Context, connectionID *int64) (*mysqlInfra.ChannelDTO, error) {
	if connectionID == nil {
		channel, err := s.channelRepository.FindByCode(ctx, mercadoLibreChannelCode)
		if err != nil {
			return nil, fmt.Errorf("error loading %s channel: %w", mercadoLibreChannelCode, err)
		}
		return channel, nil
	}

	connection, err := s.channelConnectionRepository.FindByID(ctx, *connectionID)
	if err != nil {
		return nil, fmt.Errorf("error loading connection %d: %w", *connectionID, err)
	}
	channel, err := s.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}
	return channel, nil
}

// resolveOrCreateChannelAttribute finds the ecom_channel_attributes row for
// (channelID, externalKey) that applies to productCategoryID — a row scoped
// to that exact category taking precedence over a channel-wide one (see
// ChannelAttributesRepository.FindApplicable) — creating one scoped to
// productCategoryID (NULL if the product has no category yet, or always for a
// product-scope channel) if neither exists, as targetStrategy/targetField with
// is_required/value_mode set to isRequired/valueMode. An existing row's
// is_required/value_mode/target_* are never updated to match a later call's
// values — if MercadoLibre changes whether an attribute is mandatory, or
// switches it between a closed list and free text, for a category, the slot
// needs a direct edit (or deletion so it gets re-created) to pick that up.
func (s *Service) resolveOrCreateChannelAttribute(ctx context.Context, channelID int64, productCategoryID *int64, externalKey, targetStrategy string, targetField *string, isRequired bool, valueMode string, actorID int64) (*mysqlInfra.ChannelAttributeDTO, error) {
	applicable, err := s.channelAttributesRepository.FindApplicable(ctx, channelID, productCategoryID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel attributes for external key %s: %w", externalKey, err)
	}
	for i := range applicable {
		if applicable[i].ExternalKey != nil && *applicable[i].ExternalKey == externalKey {
			return &applicable[i], nil
		}
	}

	created, err := s.channelAttributesRepository.Create(ctx, mysqlInfra.CreateChannelAttributeInput{
		ChannelID:      channelID,
		TargetStrategy: targetStrategy,
		ExternalKey:    &externalKey,
		TargetField:    targetField,
		ValueMode:      valueMode,
		CategoryID:     productCategoryID,
		IsRequired:     isRequired,
		CreatedBy:      actorID,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating channel attribute slot for external key %s: %w", externalKey, err)
	}
	return created, nil
}

// resolveOrCreateAttribute finds the ecom_attributes row whose code equals
// externalKey, reusing it only if its data_type already matches dataType
// (ErrAttributeDataTypeConflict otherwise — an attribute's data_type is meant
// to stay stable once other rows reference it, same rationale as
// UpdateAttributeInput excluding Code). Creates one with the given name when
// none exists yet (SetValue has no separate display label to offer, so it
// passes externalKey as name too; ProvisionCategoryAttributes passes
// MercadoLibre's own attribute name).
//
// widenTextToEnum lets the one safe reconciliation through: a row created as
// free "text" being promoted to "enum" because MercadoLibre now exposes the
// attribute as a closed value list. Only ProvisionCategoryAttributes passes
// true (MercadoLibre's category payload is authoritative about this); it never
// widens the other direction or across unrelated types, and existing
// ecom_product_attributes.value_text rows keep rendering unchanged (see
// formatProductAttributeValue / renderAttributeValue).
func (s *Service) resolveOrCreateAttribute(ctx context.Context, externalKey, name, dataType string, widenTextToEnum bool, actorID int64) (*mysqlInfra.AttributeDTO, error) {
	existing, err := s.attributesRepository.FindByCode(ctx, externalKey)
	if err != nil && !errors.Is(err, mysqlInfra.ErrAttributeNotFound) {
		return nil, fmt.Errorf("error loading attribute %s: %w", externalKey, err)
	}
	if existing != nil {
		if existing.DataType == dataType {
			return existing, nil
		}
		if widenTextToEnum && existing.DataType == "text" && dataType == "enum" {
			updated, updErr := s.attributesRepository.Update(ctx, existing.ID, mysqlInfra.UpdateAttributeInput{
				Name:      existing.Name,
				DataType:  "enum",
				Unit:      existing.Unit,
				UpdatedBy: actorID,
			})
			if updErr != nil {
				return nil, fmt.Errorf("error widening attribute %s from text to enum: %w", externalKey, updErr)
			}
			return updated, nil
		}
		return nil, fmt.Errorf("%w: %s is %s, got %s", ErrAttributeDataTypeConflict, externalKey, existing.DataType, dataType)
	}

	created, err := s.attributesRepository.Create(ctx, mysqlInfra.CreateAttributeInput{
		Code:      externalKey,
		Name:      name,
		DataType:  dataType,
		CreatedBy: actorID,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating attribute %s: %w", externalKey, err)
	}
	return created, nil
}

// resolveOrCreateChannelAttributeMap finds channelAttributeID's channel-wide
// (connection_id IS NULL) map row. If one exists it must already match
// sourceType/attributeID/systemField (exactly one of attributeID/systemField
// is meaningful, matching sourceType) — anything else is a conflict this
// endpoint refuses to silently overwrite (see channelAttributeMapMatches).
// Creates a channel-wide row with the given source if none exists yet.
func (s *Service) resolveOrCreateChannelAttributeMap(ctx context.Context, channelAttributeID int64, sourceType string, attributeID *int64, systemField *string, actorID int64) (*mysqlInfra.ChannelAttributeMapDTO, error) {
	maps, err := s.channelAttributeMapRepository.FindByChannelAttributeID(ctx, channelAttributeID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel attribute map for slot %d: %w", channelAttributeID, err)
	}

	for i := range maps {
		if maps[i].ConnectionID != nil {
			continue
		}
		if !channelAttributeMapMatches(&maps[i], sourceType, attributeID, systemField) {
			return nil, fmt.Errorf("%w (channel attribute slot %d)", ErrChannelAttributeMapConflict, channelAttributeID)
		}
		return &maps[i], nil
	}

	created, err := s.channelAttributeMapRepository.Create(ctx, mysqlInfra.CreateChannelAttributeMapInput{
		ChannelAttributeID: channelAttributeID,
		SourceType:         sourceType,
		AttributeID:        attributeID,
		SystemField:        systemField,
		CreatedBy:          actorID,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating channel attribute map for slot %d: %w", channelAttributeID, err)
	}
	return created, nil
}

// channelAttributeMapMatches reports whether m already represents
// (sourceType, attributeID, systemField) — used to decide whether an
// existing channel-wide map row can be reused as-is or is a genuine conflict.
func channelAttributeMapMatches(m *mysqlInfra.ChannelAttributeMapDTO, sourceType string, attributeID *int64, systemField *string) bool {
	if m.SourceType != sourceType {
		return false
	}
	switch sourceType {
	case "custom_attribute":
		return m.AttributeID != nil && attributeID != nil && *m.AttributeID == *attributeID
	case "system_field":
		return m.SystemField != nil && systemField != nil && *m.SystemField == *systemField
	default:
		return false
	}
}

// resolveOrCreateOption finds attributeID's existing option matching value
// exactly (trimmed), creating one if none exists — keeps the same enum value
// typed differently across calls from fragmenting into duplicate options.
// requireExisting is true for a value_id-mode attribute (see SetValue): the
// option must already have been provisioned from MercadoLibre's own closed
// list (provisionAttributeOptions) — ErrAttributeOptionNotProvisioned is
// returned instead of inventing a new, external_value_id-less option that
// MercadoLibre wouldn't recognize.
func (s *Service) resolveOrCreateOption(ctx context.Context, attributeID int64, value string, requireExisting bool, actorID int64) (*mysqlInfra.AttributeOptionDTO, error) {
	value = strings.TrimSpace(value)

	existing, err := s.attributeOptionsRepository.FindByAttributeAndValue(ctx, attributeID, value)
	if err != nil && !errors.Is(err, mysqlInfra.ErrAttributeOptionNotFound) {
		return nil, fmt.Errorf("error loading attribute option %q: %w", value, err)
	}
	if existing != nil {
		return existing, nil
	}

	if requireExisting {
		return nil, fmt.Errorf("%w: %q", ErrAttributeOptionNotProvisioned, value)
	}

	created, err := s.attributeOptionsRepository.Create(ctx, mysqlInfra.CreateAttributeOptionInput{
		AttributeID: attributeID,
		Value:       value,
		CreatedBy:   actorID,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating attribute option %q: %w", value, err)
	}
	return created, nil
}

func validateSetValueInput(input SetValueInput) error {
	if strings.TrimSpace(input.SKU) == "" || strings.TrimSpace(input.ExternalKey) == "" || input.ActorID <= 0 {
		return ErrInvalidSetValueInput
	}
	if !isValidDataType(input.DataType) {
		return ErrInvalidSetValueInput
	}

	switch input.DataType {
	case "text":
		if input.ValueText == nil {
			return ErrInvalidSetValueInput
		}
	case "number":
		if input.ValueNumber == nil {
			return ErrInvalidSetValueInput
		}
	case "boolean":
		if input.ValueBool == nil {
			return ErrInvalidSetValueInput
		}
	case "date":
		if input.ValueDate == nil {
			return ErrInvalidSetValueInput
		}
	case "enum":
		if input.EnumValue == nil || strings.TrimSpace(*input.EnumValue) == "" {
			return ErrInvalidSetValueInput
		}
	}

	return nil
}

// isValidDataType duplicates attributes.isValidDataType (unexported there) —
// small enough (5 constants) that importing just for this wasn't worth
// coupling the two packages' validation together.
func isValidDataType(dataType string) bool {
	for _, allowed := range mysqlInfra.AttributeDataTypes {
		if dataType == allowed {
			return true
		}
	}
	return false
}

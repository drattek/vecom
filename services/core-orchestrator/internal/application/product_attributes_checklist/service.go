// Package product_attributes_checklist builds, for a product + channel
// connection, the list of custom attributes that connection's channel expects
// for the product's local category (ecom_channel_attributes), marking which are
// required and which the product is still missing. It's a pure read across the
// channel-attribute, category-map and product-attribute tables — nothing here
// writes (the UI edits values through the existing /api/products/{id}/attributes
// endpoints).
package product_attributes_checklist

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidInput       = errors.New("invalid checklist input")
	ErrProductNotFound    = errors.New("product not found")
	ErrConnectionNotFound = errors.New("connection not found")
)

// Checklist states.
const (
	StateNoCategory        = "no_category"
	StateCategoryNotMapped = "category_not_mapped"
	StateOK                = "ok"
)

type Service struct {
	productDetails      *mysqlInfra.ProductDetailsRepository
	channelConnections  *mysqlInfra.ChannelConnectionRepository
	channelCategoryMap  *mysqlInfra.ChannelCategoryMapRepository
	channelAttributes   *mysqlInfra.ChannelAttributesRepository
	channelAttributeMap *mysqlInfra.ChannelAttributeMapRepository
	attributes          *mysqlInfra.AttributesRepository
	attributeOptions    *mysqlInfra.AttributeOptionsRepository
	productAttributes   *mysqlInfra.ProductAttributesRepository
}

func NewService(
	productDetails *mysqlInfra.ProductDetailsRepository,
	channelConnections *mysqlInfra.ChannelConnectionRepository,
	channelCategoryMap *mysqlInfra.ChannelCategoryMapRepository,
	channelAttributes *mysqlInfra.ChannelAttributesRepository,
	channelAttributeMap *mysqlInfra.ChannelAttributeMapRepository,
	attributes *mysqlInfra.AttributesRepository,
	attributeOptions *mysqlInfra.AttributeOptionsRepository,
	productAttributes *mysqlInfra.ProductAttributesRepository,
) *Service {
	return &Service{
		productDetails:      productDetails,
		channelConnections:  channelConnections,
		channelCategoryMap:  channelCategoryMap,
		channelAttributes:   channelAttributes,
		channelAttributeMap: channelAttributeMap,
		attributes:          attributes,
		attributeOptions:    attributeOptions,
		productAttributes:   productAttributes,
	}
}

type OptionDTO struct {
	ID    int64  `json:"id"`
	Value string `json:"value"`
}

type ItemDTO struct {
	AttributeID        int64       `json:"attributeId"`
	Code               string      `json:"code"`
	Name               string      `json:"name"`
	DataType           string      `json:"dataType"`
	Unit               *string     `json:"unit,omitempty"`
	ExternalLabel      *string     `json:"externalLabel,omitempty"`
	IsRequired         bool        `json:"isRequired"`
	Value              string      `json:"value"`
	OptionID           *int64      `json:"optionId,omitempty"`
	ProductAttributeID *int64      `json:"productAttributeId,omitempty"`
	Options            []OptionDTO `json:"options,omitempty"`
}

type AutoCoveredDTO struct {
	Label       string `json:"label"`
	SystemField string `json:"systemField"`
}

type ChecklistDTO struct {
	ConnectionID         int64            `json:"connectionId"`
	ChannelName          string           `json:"channelName"`
	ConnectionName       string           `json:"connectionName"`
	CategoryID           *int64           `json:"categoryId,omitempty"`
	CategoryName         *string          `json:"categoryName,omitempty"`
	State                string           `json:"state"`
	ExternalCategoryID   *string          `json:"externalCategoryId,omitempty"`
	ExternalCategoryName *string          `json:"externalCategoryName,omitempty"`
	RequiredMissing      int              `json:"requiredMissing"`
	Items                []ItemDTO        `json:"items"`
	AutoCovered          []AutoCoveredDTO `json:"autoCovered"`
}

func (s *Service) GetChecklist(ctx context.Context, productID, connectionID int64) (*ChecklistDTO, error) {
	if productID <= 0 || connectionID <= 0 {
		return nil, ErrInvalidInput
	}

	connection, err := s.channelConnections.FindByID(ctx, connectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return nil, ErrConnectionNotFound
		}
		return nil, fmt.Errorf("error loading connection: %w", err)
	}

	general, err := s.productDetails.FindGeneral(ctx, productID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("error loading product: %w", err)
	}

	out := &ChecklistDTO{
		ConnectionID:   connectionID,
		ChannelName:    connection.ChannelName,
		ConnectionName: connection.Name,
		CategoryID:     general.CategoryID,
		CategoryName:   general.CategoryName,
		Items:          make([]ItemDTO, 0),
		AutoCovered:    make([]AutoCoveredDTO, 0),
	}

	if general.CategoryID == nil {
		out.State = StateNoCategory
		return out, nil
	}
	categoryID := *general.CategoryID

	categoryMap, err := s.channelCategoryMap.FindByCategoryAndConnection(ctx, categoryID, connectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelCategoryMapNotFound) {
			out.State = StateCategoryNotMapped
			return out, nil
		}
		return nil, fmt.Errorf("error loading category map: %w", err)
	}
	out.State = StateOK
	out.ExternalCategoryID = &categoryMap.ExternalCategoryID
	out.ExternalCategoryName = categoryMap.ExternalCategoryName

	slots, err := s.channelAttributes.FindApplicable(ctx, connection.ChannelID, &categoryID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel attributes: %w", err)
	}

	// Valores actuales del producto, indexados por attribute_id.
	productAttrs, err := s.productAttributes.FindByProductID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("error loading product attributes: %w", err)
	}
	currentByAttr := make(map[int64]mysqlInfra.ProductAttributeDTO, len(productAttrs))
	for _, pa := range productAttrs {
		currentByAttr[pa.AttributeID] = pa
	}

	// FindApplicable devuelve primero las filas específicas de categoría; al
	// deduplicar por attribute_id nos quedamos con la primera y hacemos OR de
	// is_required entre duplicados.
	itemsByAttr := make(map[int64]*ItemDTO)
	order := make([]int64, 0)
	autoCoveredSeen := make(map[string]bool)

	for _, slot := range slots {
		m, mapErr := s.channelAttributeMap.FindByChannelAttributeAndConnection(ctx, slot.ID, connectionID)
		if mapErr != nil {
			if errors.Is(mapErr, mysqlInfra.ErrChannelAttributeMapNotFound) {
				continue
			}
			return nil, fmt.Errorf("error resolving channel attribute map: %w", mapErr)
		}

		switch m.SourceType {
		case "system_field":
			if m.SystemField != nil && !autoCoveredSeen[*m.SystemField] {
				autoCoveredSeen[*m.SystemField] = true
				out.AutoCovered = append(out.AutoCovered, AutoCoveredDTO{
					Label:       labelForSlot(slot),
					SystemField: *m.SystemField,
				})
			}
			continue
		case "custom_attribute":
			if m.AttributeID == nil {
				continue
			}
		default: // static_value u otros: nada que completar
			continue
		}

		attrID := *m.AttributeID
		if existing, ok := itemsByAttr[attrID]; ok {
			existing.IsRequired = existing.IsRequired || slot.IsRequired
			if existing.ExternalLabel == nil {
				existing.ExternalLabel = slot.ExternalLabel
			}
			continue
		}

		attr, attrErr := s.attributes.FindByID(ctx, attrID)
		if attrErr != nil {
			if errors.Is(attrErr, mysqlInfra.ErrAttributeNotFound) {
				continue
			}
			return nil, fmt.Errorf("error loading attribute %d: %w", attrID, attrErr)
		}

		item := ItemDTO{
			AttributeID:   attr.ID,
			Code:          attr.Code,
			Name:          attr.Name,
			DataType:      attr.DataType,
			Unit:          attr.Unit,
			ExternalLabel: slot.ExternalLabel,
			IsRequired:    slot.IsRequired,
		}

		optionValueByID := map[int64]string{}
		if attr.DataType == "enum" {
			options, optErr := s.attributeOptions.FindByAttributeID(ctx, attr.ID)
			if optErr != nil {
				return nil, fmt.Errorf("error loading options for attribute %d: %w", attr.ID, optErr)
			}
			item.Options = make([]OptionDTO, 0, len(options))
			for _, opt := range options {
				item.Options = append(item.Options, OptionDTO{ID: opt.ID, Value: opt.Value})
				optionValueByID[opt.ID] = opt.Value
			}
		}

		if pa, ok := currentByAttr[attrID]; ok {
			paID := pa.ID
			item.ProductAttributeID = &paID
			item.OptionID = pa.OptionID
			item.Value = renderAttributeValue(pa, optionValueByID)
		}

		itemsByAttr[attrID] = &item
		order = append(order, attrID)
	}

	for _, attrID := range order {
		out.Items = append(out.Items, *itemsByAttr[attrID])
	}

	// Orden final: requeridos sin valor → requeridos → opcionales; alfabético dentro.
	sort.SliceStable(out.Items, func(i, j int) bool {
		a, b := out.Items[i], out.Items[j]
		ra, rb := rank(a), rank(b)
		if ra != rb {
			return ra < rb
		}
		return a.Name < b.Name
	})

	for _, item := range out.Items {
		if item.IsRequired && !hasValue(item) {
			out.RequiredMissing++
		}
	}

	return out, nil
}

func hasValue(item ItemDTO) bool {
	return item.Value != "" || item.OptionID != nil
}

func rank(item ItemDTO) int {
	switch {
	case item.IsRequired && !hasValue(item):
		return 0
	case item.IsRequired:
		return 1
	default:
		return 2
	}
}

func labelForSlot(slot mysqlInfra.ChannelAttributeDTO) string {
	if slot.ExternalLabel != nil && *slot.ExternalLabel != "" {
		return *slot.ExternalLabel
	}
	if slot.ExternalKey != nil {
		return *slot.ExternalKey
	}
	return ""
}

// renderAttributeValue produce el string de display de un valor, igual que
// ProductDetailsRepository.FindAttributes.
func renderAttributeValue(pa mysqlInfra.ProductAttributeDTO, optionValueByID map[int64]string) string {
	switch {
	case pa.OptionID != nil:
		return optionValueByID[*pa.OptionID]
	case pa.ValueText != nil:
		return *pa.ValueText
	case pa.ValueNumber != nil:
		return strconv.FormatFloat(*pa.ValueNumber, 'g', -1, 64)
	case pa.ValueBool != nil:
		if *pa.ValueBool {
			return "Sí"
		}
		return "No"
	case pa.ValueDate != nil:
		return pa.ValueDate.Format("2006-01-02")
	}
	return ""
}

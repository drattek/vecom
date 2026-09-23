// Package product_attributes_checklist builds, for a product + channel
// connection, the list of custom attributes that connection's channel expects
// for the product's local category (ecom_channel_attributes), marking which are
// required and which the product is still missing. It's a pure read across the
// channel-attribute, category-map and product-attribute tables — nothing here
// writes (the UI edits values through the existing /api/products/{id}/attributes
// endpoints).
//
// A channel whose ecom_channels.attribute_scope is "product" (Odoo — see ADR
// 0004) has attributes that don't depend on the category, so the applicable
// slots (category_id IS NULL) are listed regardless of the product's
// category. It still needs_selection like any other channel when it has no
// resolved external category, though: Odoo publishing requires one (see
// OdooProductSyncService.resolveConnectionCategoryID) just as much as
// MercadoLibre's does.
//
// For every channel, the category slots — and, for a product-scoped
// channel, whether an external category is resolved at all — are checked
// against ecom_channel_product_category_selection first — the per-connection
// choice made in the "Sincronización" tab (see ADR 0005).
// ecom_products.category_id / ecom_channel_category_map (the product's own
// catalog category and its category-level mapping) are deliberately never
// consulted to resolve this — see resolvePublishedCategory's doc comment for
// why. The only other source is an already-published listing's own recorded
// external category (ecom_channel_product_map), for connections that were
// published before this feature existed.
package product_attributes_checklist

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidInput       = errors.New("invalid checklist input")
	ErrProductNotFound    = errors.New("product not found")
	ErrConnectionNotFound = errors.New("connection not found")
)

// Checklist states.
const (
	// StateNeedsSelection means the channel is category-scoped and neither a
	// per-connection selection (ADR 0005) nor a legacy category-level mapping
	// resolves an external category yet — the UI should show the category
	// picker (predictor/tree) instead of the attribute list.
	StateNeedsSelection = "needs_selection"
	StateOK             = "ok"
)

type Service struct {
	productDetails      *mysqlInfra.ProductDetailsRepository
	channels            *mysqlInfra.ChannelRepository
	channelConnections  *mysqlInfra.ChannelConnectionRepository
	channelCategoryMap  *mysqlInfra.ChannelCategoryMapRepository
	channelProductMap   *mysqlInfra.ChannelProductMapRepository
	channelAttributes   *mysqlInfra.ChannelAttributesRepository
	channelAttributeMap *mysqlInfra.ChannelAttributeMapRepository
	attributes          *mysqlInfra.AttributesRepository
	attributeOptions    *mysqlInfra.AttributeOptionsRepository
	productAttributes   *mysqlInfra.ProductAttributesRepository
	categorySelection   *mysqlInfra.ChannelProductCategorySelectionRepository
}

func NewService(
	productDetails *mysqlInfra.ProductDetailsRepository,
	channels *mysqlInfra.ChannelRepository,
	channelConnections *mysqlInfra.ChannelConnectionRepository,
	channelCategoryMap *mysqlInfra.ChannelCategoryMapRepository,
	channelProductMap *mysqlInfra.ChannelProductMapRepository,
	channelAttributes *mysqlInfra.ChannelAttributesRepository,
	channelAttributeMap *mysqlInfra.ChannelAttributeMapRepository,
	attributes *mysqlInfra.AttributesRepository,
	attributeOptions *mysqlInfra.AttributeOptionsRepository,
	productAttributes *mysqlInfra.ProductAttributesRepository,
	categorySelection *mysqlInfra.ChannelProductCategorySelectionRepository,
) *Service {
	return &Service{
		productDetails:      productDetails,
		channels:            channels,
		channelConnections:  channelConnections,
		channelCategoryMap:  channelCategoryMap,
		channelProductMap:   channelProductMap,
		channelAttributes:   channelAttributes,
		channelAttributeMap: channelAttributeMap,
		attributes:          attributes,
		attributeOptions:    attributeOptions,
		productAttributes:   productAttributes,
		categorySelection:   categorySelection,
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
	ConnectionID   int64  `json:"connectionId"`
	ChannelName    string `json:"channelName"`
	ConnectionName string `json:"connectionName"`
	// AttributeScope mirrors ecom_channels.attribute_scope ("category" | "product").
	// "product" (Odoo) means the UI can offer a free-form "add attribute" form
	// since attributes aren't tied to the category. See ADR 0004.
	AttributeScope       string           `json:"attributeScope"`
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

	channel, err := s.channels.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}
	productScope := channel.AttributeScope == mysqlInfra.ChannelAttributeScopeProduct

	out := &ChecklistDTO{
		ConnectionID:   connectionID,
		ChannelName:    connection.ChannelName,
		ConnectionName: connection.Name,
		AttributeScope: channel.AttributeScope,
		CategoryID:     general.CategoryID,
		CategoryName:   general.CategoryName,
		Items:          make([]ItemDTO, 0),
		AutoCovered:    make([]AutoCoveredDTO, 0),
	}

	// slotCategoryID scopes FindApplicable below. A per-connection selection
	// (ADR 0005 — picked in the "Sincronización" tab before this product has
	// any listing on connectionID) takes priority over everything else: it's
	// what lets two products sharing a local category (e.g. "Frenos")
	// resolve to different external categories — and different required
	// attributes — per connection. general.CategoryID (the product's own
	// catalog category) is deliberately never used to derive this — see the
	// package doc comment and resolvePublishedCategory below.
	var slotCategoryID *int64
	selection, selErr := s.categorySelection.FindByProductAndConnection(ctx, productID, connectionID)
	if selErr != nil && !errors.Is(selErr, mysqlInfra.ErrChannelProductCategorySelectionNotFound) {
		return nil, fmt.Errorf("error loading category selection: %w", selErr)
	}

	switch {
	case selErr == nil:
		categoryID := selection.CategoryID
		slotCategoryID = &categoryID
		out.ExternalCategoryID = &selection.ExternalCategoryID
		out.ExternalCategoryName = selection.ExternalCategoryName
	default:
		publishedCategoryID, externalCategoryID, externalCategoryName, pubErr := s.resolvePublishedCategory(ctx, productID, connectionID)
		if pubErr != nil {
			return nil, pubErr
		}
		switch {
		case publishedCategoryID != nil:
			slotCategoryID = publishedCategoryID
			out.ExternalCategoryID = externalCategoryID
			out.ExternalCategoryName = externalCategoryName
		default:
			// No per-connection selection and no already-published listing to
			// read a category from — every channel needs one picked before
			// attributes are shown, product-scoped (Odoo) included: it's what
			// lets sync resolve/create the external category at publish time.
			out.State = StateNeedsSelection
			return out, nil
		}
	}
	out.State = StateOK

	slots, err := s.channelAttributes.FindApplicable(ctx, connection.ChannelID, slotCategoryID)
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

		// Product-scope (Odoo): the ecom_channel_attributes slots are
		// channel-wide, not per product, so the full list would be the union of
		// every attribute ever added on the channel. Only surface the ones this
		// product actually has a value for — new ones are added through the
		// "add attribute" form, not by completing a shared list. See ADR 0004.
		if productScope {
			if _, hasValue := currentByAttr[attrID]; !hasValue {
				continue
			}
		}

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

// resolvePublishedCategory is the fallback used when (productID, connectionID)
// has no per-connection selection yet (ADR 0005): it looks at whatever
// category an already-published listing for this exact product actually used
// (ecom_channel_product_map.external_category_id — written once at publish
// time, per product, never ambiguous) and reverse-maps it to its local
// ecom_categories leaf via ecom_channel_category_map.FindByExternalCategoryAndConnection.
// This is safe where resolving forward from the product's own catalog
// category never was: here the starting point is one product's own real,
// already-resolved listing, not a local category shared by every product
// under it — the exact ambiguity ADR 0005 exists to avoid (see the package
// doc comment). Returns all nils when the product has no listing on
// connectionID yet, or that listing has no external category recorded — the
// caller then needs a fresh selection.
func (s *Service) resolvePublishedCategory(ctx context.Context, productID, connectionID int64) (categoryID *int64, externalCategoryID, externalCategoryName *string, err error) {
	rows, err := s.channelProductMap.FindAllByProductAndConnection(ctx, productID, connectionID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error loading channel product map: %w", err)
	}

	for _, row := range rows {
		if row.ExternalCategoryID == nil || strings.TrimSpace(*row.ExternalCategoryID) == "" {
			continue
		}

		categoryMap, mapErr := s.channelCategoryMap.FindByExternalCategoryAndConnection(ctx, *row.ExternalCategoryID, connectionID)
		if mapErr != nil {
			if errors.Is(mapErr, mysqlInfra.ErrChannelCategoryMapNotFound) {
				continue
			}
			return nil, nil, nil, fmt.Errorf("error loading channel category map for external category %q: %w", *row.ExternalCategoryID, mapErr)
		}

		id := categoryMap.CategoryID
		return &id, row.ExternalCategoryID, categoryMap.ExternalCategoryName, nil
	}

	return nil, nil, nil, nil
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

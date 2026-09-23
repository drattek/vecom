// Package product_category_selection backs the "Sincronización" tab's
// category picker: before a product has any listing on a connection, the
// user picks the external category it should publish under there (see ADR
// 0005) — either the MercadoLibre predictor's suggestion or a leaf chosen
// from the channel's external category tree (ExternalCategoryTree.tsx,
// backed by category_import.Service.BrowseTree, reused unchanged here).
// Select ensures that category exists locally, seeds MercadoLibre's required
// attribute slots for it, and records the choice in
// ecom_channel_product_category_selection — a table separate from
// ecom_channel_product_map precisely so recording "the user picked X" before
// publishing never makes channel_listings.publishOne treat the product as
// already published.
package product_category_selection

import (
	"context"
	"errors"
	"fmt"
	"strings"

	categoryImportApp "core-orchestrator/internal/application/category_import"
	channelAttributeValuesApp "core-orchestrator/internal/application/channel_attribute_values"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// mercadoLibreChannelCode mirrors channel_attribute_values.mercadoLibreChannelCode
// (unexported there) — only MercadoLibre's required-attribute slots need
// seeding per external category; Odoo's attributes are per-product key/value
// (ADR 0004) and don't depend on category at all.
const mercadoLibreChannelCode = "MERCADOLIBRE"

var (
	ErrInvalidInput       = errors.New("productId, connectionId and externalCategoryId are required")
	ErrProductNotFound    = mysqlInfra.ErrProductNotFound
	ErrConnectionNotFound = mysqlInfra.ErrChannelConnectionNotFound
)

type Service struct {
	productRepository             *mysqlInfra.ProductRepository
	channelRepository             *mysqlInfra.ChannelRepository
	channelConnectionRepository   *mysqlInfra.ChannelConnectionRepository
	selectionRepository           *mysqlInfra.ChannelProductCategorySelectionRepository
	categoryImportService         *categoryImportApp.Service
	channelAttributeValuesService *channelAttributeValuesApp.Service
}

func NewService(
	productRepository *mysqlInfra.ProductRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	selectionRepository *mysqlInfra.ChannelProductCategorySelectionRepository,
	categoryImportService *categoryImportApp.Service,
	channelAttributeValuesService *channelAttributeValuesApp.Service,
) *Service {
	return &Service{
		productRepository:             productRepository,
		channelRepository:             channelRepository,
		channelConnectionRepository:   channelConnectionRepository,
		selectionRepository:           selectionRepository,
		categoryImportService:         categoryImportService,
		channelAttributeValuesService: channelAttributeValuesService,
	}
}

// SelectionDTO reports the outcome of Select: the local leaf category
// category_import.Service.Import resolved/created (never written to
// ecom_products.category_id) alongside the external category it corresponds
// to.
type SelectionDTO struct {
	ProductID            int64  `json:"productId"`
	ConnectionID         int64  `json:"connectionId"`
	CategoryID           int64  `json:"categoryId"`
	ExternalCategoryID   string `json:"externalCategoryId"`
	ExternalCategoryName string `json:"externalCategoryName"`
}

// Select ensures a local category exists for externalCategoryID on
// connectionID (via category_import.Service.Import, reused unchanged), seeds
// MercadoLibre's required-attribute slots for it when connectionID's channel
// is category-scoped, and records the choice for (productID, connectionID)
// in ecom_channel_product_category_selection. Calling it again for the same
// (productID, connectionID) before publishing replaces the previous choice.
func (s *Service) Select(ctx context.Context, productID, connectionID int64, externalCategoryID string, actorID int64) (*SelectionDTO, error) {
	externalCategoryID = strings.TrimSpace(externalCategoryID)
	if productID <= 0 || connectionID <= 0 || externalCategoryID == "" || actorID <= 0 {
		return nil, ErrInvalidInput
	}

	product, err := s.productRepository.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}

	connection, err := s.channelConnectionRepository.FindByID(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	channel, err := s.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}

	imported, err := s.categoryImportService.Import(ctx, connectionID, externalCategoryID, actorID, true)
	if err != nil {
		return nil, fmt.Errorf("error resolving local category for external category %q: %w", externalCategoryID, err)
	}

	if strings.EqualFold(strings.TrimSpace(channel.Code), mercadoLibreChannelCode) {
		localCategoryID := imported.CategoryID
		if _, err := s.channelAttributeValuesService.ProvisionCategoryAttributes(ctx, channelAttributeValuesApp.ProvisionCategoryAttributesInput{
			SKU:                     product.SKU,
			CategoryID:              externalCategoryID,
			ConnectionID:            &connectionID,
			ActorID:                 actorID,
			LocalCategoryIDOverride: &localCategoryID,
		}); err != nil {
			return nil, fmt.Errorf("error provisioning attributes for category %q: %w", externalCategoryID, err)
		}
	}

	var namePtr *string
	if name := strings.TrimSpace(imported.Name); name != "" {
		namePtr = &name
	}

	if _, err := s.selectionRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductCategorySelectionInput{
		ProductID:            productID,
		ConnectionID:         connectionID,
		CategoryID:           imported.CategoryID,
		ExternalCategoryID:   externalCategoryID,
		ExternalCategoryName: namePtr,
		ActorID:              actorID,
	}); err != nil {
		return nil, fmt.Errorf("error saving category selection: %w", err)
	}

	return &SelectionDTO{
		ProductID:            productID,
		ConnectionID:         connectionID,
		CategoryID:           imported.CategoryID,
		ExternalCategoryID:   externalCategoryID,
		ExternalCategoryName: imported.Name,
	}, nil
}

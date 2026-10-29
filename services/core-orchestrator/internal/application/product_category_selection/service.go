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
	categoriesRepository          *mysqlInfra.CategoriesRepository
	channelCategoryMapRepository  *mysqlInfra.ChannelCategoryMapRepository
	categoryImportService         *categoryImportApp.Service
	channelAttributeValuesService *channelAttributeValuesApp.Service
}

func NewService(
	productRepository *mysqlInfra.ProductRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	selectionRepository *mysqlInfra.ChannelProductCategorySelectionRepository,
	categoriesRepository *mysqlInfra.CategoriesRepository,
	channelCategoryMapRepository *mysqlInfra.ChannelCategoryMapRepository,
	categoryImportService *categoryImportApp.Service,
	channelAttributeValuesService *channelAttributeValuesApp.Service,
) *Service {
	return &Service{
		productRepository:             productRepository,
		channelRepository:             channelRepository,
		channelConnectionRepository:   channelConnectionRepository,
		selectionRepository:           selectionRepository,
		categoriesRepository:          categoriesRepository,
		channelCategoryMapRepository:  channelCategoryMapRepository,
		categoryImportService:         categoryImportService,
		channelAttributeValuesService: channelAttributeValuesService,
	}
}

// resolveLocalCategory returns the local category externalCategoryID belongs to.
// For channels without MercadoLibre's per-category attribute slots (Odoo), when
// the product already has a local category it is re-pointed at the chosen
// external category — replacing its previous mapping on this connection — rather
// than forking a new local category named after the external one. Otherwise
// (MercadoLibre, or a product with no local category yet) the external category
// is imported, which creates/reuses the local one.
func (s *Service) resolveLocalCategory(ctx context.Context, product *mysqlInfra.ProductDTO, channelCode string, connectionID int64, externalCategoryID string, actorID int64) (*categoryImportApp.ImportResultDTO, error) {
	if !strings.EqualFold(strings.TrimSpace(channelCode), mercadoLibreChannelCode) {
		selections, err := s.selectionRepository.FindByProduct(ctx, product.ID)
		if err != nil {
			return nil, err
		}
		if candidates := s.candidateCategoryIDs(product, selections, connectionID); len(candidates) > 0 {
			mapped, err := s.categoryImportService.SetMappingAllowingParents(ctx, candidates[0], connectionID, externalCategoryID, actorID)
			if err != nil {
				return nil, err
			}
			return &categoryImportApp.ImportResultDTO{
				CategoryID:          mapped.CategoryID,
				Name:                mapped.ExternalCategoryName,
				AlreadyLinked:       mapped.Replaced,
				MappedConnectionIDs: []int64{connectionID},
			}, nil
		}
	}

	return s.categoryImportService.ImportAllowingParents(ctx, connectionID, externalCategoryID, actorID, true)
}

// SuggestionDTO is a recommended external category for a (product, connection):
// the one the connection's ecom_channel_category_map already links to a local
// category the product is known under.
type SuggestionDTO struct {
	CategoryID           int64    `json:"categoryId"`
	CategoryPath         []string `json:"categoryPath"`
	ExternalCategoryID   string   `json:"externalCategoryId"`
	ExternalCategoryName string   `json:"externalCategoryName"`
}

// Suggestions recommends external categories on connectionID for a product by
// reusing the local categories it already has — the one on ecom_products and
// the ones picked (or auto-created, e.g. from a MercadoLibre listing) on other
// connections — and looking up where each is mapped on connectionID (linked
// from Settings → Categorías). Always a slice, empty when nothing is mapped.
func (s *Service) Suggestions(ctx context.Context, productID, connectionID int64) ([]SuggestionDTO, error) {
	if productID <= 0 || connectionID <= 0 {
		return nil, ErrInvalidInput
	}

	product, err := s.productRepository.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}

	if _, err := s.channelConnectionRepository.FindByID(ctx, connectionID); err != nil {
		return nil, err
	}

	selections, err := s.selectionRepository.FindByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}

	candidates := s.candidateCategoryIDs(product, selections, connectionID)

	suggestions := make([]SuggestionDTO, 0, len(candidates))
	for _, categoryID := range candidates {
		mapping, err := s.channelCategoryMapRepository.FindByCategoryAndConnection(ctx, categoryID, connectionID)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrChannelCategoryMapNotFound) {
				continue
			}
			return nil, fmt.Errorf("error loading category mapping for category %d: %w", categoryID, err)
		}
		path, err := s.localCategoryPath(ctx, categoryID)
		if err != nil {
			return nil, err
		}

		name := ""
		if mapping.ExternalCategoryName != nil {
			name = strings.TrimSpace(*mapping.ExternalCategoryName)
		}
		suggestions = append(suggestions, SuggestionDTO{
			CategoryID:           categoryID,
			CategoryPath:         path,
			ExternalCategoryID:   mapping.ExternalCategoryID,
			ExternalCategoryName: name,
		})
		// One recommendation per connection: the first local category in
		// priority order that is mapped here.
		break
	}

	return suggestions, nil
}

// candidateCategoryIDs lists, in priority order, the local categories a product
// is known under for connectionID: the one on ecom_products first, then the ones
// picked (or auto-created, e.g. from a MercadoLibre listing) on other connections.
func (s *Service) candidateCategoryIDs(product *mysqlInfra.ProductDTO, selections []mysqlInfra.ChannelProductCategorySelectionDTO, connectionID int64) []int64 {
	candidates := make([]int64, 0, len(selections)+1)
	seen := make(map[int64]bool)
	add := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			candidates = append(candidates, id)
		}
	}
	if product.CategoryID != nil {
		add(*product.CategoryID)
	}
	for _, selection := range selections {
		if selection.ConnectionID != connectionID {
			add(selection.CategoryID)
		}
	}
	return candidates
}

// localCategoryPath returns the root→category names of a local category.
func (s *Service) localCategoryPath(ctx context.Context, categoryID int64) ([]string, error) {
	path := make([]string, 0, 4)
	visited := make(map[int64]bool)
	for cursor := &categoryID; cursor != nil && !visited[*cursor]; {
		visited[*cursor] = true
		category, err := s.categoriesRepository.FindByID(ctx, *cursor)
		if err != nil {
			return nil, fmt.Errorf("error loading local category %d: %w", *cursor, err)
		}
		path = append([]string{category.Name}, path...)
		cursor = category.ParentID
	}
	return path, nil
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

	imported, err := s.resolveLocalCategory(ctx, product, channel.Code, connectionID, externalCategoryID, actorID)
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

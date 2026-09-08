// Package category_import backs the Settings → Categorías "Importar categoría"
// flow: it lists the channels whose active connections can be browsed for
// categories, walks a channel's *external* category tree one level at a time,
// and imports a chosen leaf — replicating its full ancestry into
// ecom_categories and recording ecom_channel_category_map rows for it.
//
// Nothing here publishes or talks to a marketplace beyond reading its category
// tree. MercadoLibre reuses sync.MercadoLibreCategoryPredictorService
// (BrowseCategory / EnsureLocalCategory); Odoo reads product.public.category
// directly. Amazon and any other channel are listed but not importable.
package category_import

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	odooInfra "core-orchestrator/internal/infrastructure/marketplace/odoo"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"

	syncApp "core-orchestrator/internal/application/sync"
)

const (
	channelCodeMercadoLibre = "MERCADOLIBRE"
	channelCodeOdoo         = "ODOO"
)

var (
	ErrInvalidInput             = errors.New("invalid category import input")
	ErrConnectionNotFound       = errors.New("channel connection not found")
	ErrChannelNotSupported      = errors.New("channel does not support category import")
	ErrExternalCategoryNotFound = errors.New("external category not found")
	ErrLocalCategoryNotFound    = errors.New("local category not found")
	ErrNotLeaf                  = errors.New("only leaf categories can be imported")
)

// mercadoLibreCategoryBrowser is the slice of
// sync.MercadoLibreCategoryPredictorService this package needs, declared as an
// interface so main.go injects the concrete service without this package
// depending on its constructor.
type mercadoLibreCategoryBrowser interface {
	BrowseRootCategoriesForConnection(ctx context.Context, connectionID int64) (json.RawMessage, error)
	BrowseCategory(ctx context.Context, connectionID int64, categoryID string) (json.RawMessage, error)
	EnsureLocalCategory(ctx context.Context, connectionID int64, externalCategoryID string, actorID int64) (int64, error)
}

type Service struct {
	channelRepository         *mysqlInfra.ChannelRepository
	channelConnectionRepo     *mysqlInfra.ChannelConnectionRepository
	categoriesRepository      *mysqlInfra.CategoriesRepository
	channelCategoryMapRepo    *mysqlInfra.ChannelCategoryMapRepository
	connectionCredentialsRepo *mysqlInfra.ConnectionCredentialsRepository
	connectionSettingsRepo    *mysqlInfra.ConnectionSettingsRepository
	mercadoLibre              mercadoLibreCategoryBrowser
	odooRateLimiter           *odooInfra.RateLimiter
}

func NewService(
	channelRepository *mysqlInfra.ChannelRepository,
	channelConnectionRepo *mysqlInfra.ChannelConnectionRepository,
	categoriesRepository *mysqlInfra.CategoriesRepository,
	channelCategoryMapRepo *mysqlInfra.ChannelCategoryMapRepository,
	connectionCredentialsRepo *mysqlInfra.ConnectionCredentialsRepository,
	connectionSettingsRepo *mysqlInfra.ConnectionSettingsRepository,
	mercadoLibre mercadoLibreCategoryBrowser,
	odooRateLimiter *odooInfra.RateLimiter,
) *Service {
	return &Service{
		channelRepository:         channelRepository,
		channelConnectionRepo:     channelConnectionRepo,
		categoriesRepository:      categoriesRepository,
		channelCategoryMapRepo:    channelCategoryMapRepo,
		connectionCredentialsRepo: connectionCredentialsRepo,
		connectionSettingsRepo:    connectionSettingsRepo,
		mercadoLibre:              mercadoLibre,
		odooRateLimiter:           odooRateLimiter,
	}
}

// --- ListSources ---------------------------------------------------------

type SourceConnectionDTO struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

type SourceDTO struct {
	ChannelID   int64                 `json:"channelId"`
	ChannelCode string                `json:"channelCode"`
	ChannelName string                `json:"channelName"`
	Supported   bool                  `json:"supported"`
	Connections []SourceConnectionDTO `json:"connections"`
}

// ListSources returns one entry per channel that has at least one active
// connection, in channel-id order, marking whether category import is
// implemented for that channel's code.
func (s *Service) ListSources(ctx context.Context) ([]SourceDTO, error) {
	channels, err := s.channelRepository.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("error loading channels: %w", err)
	}

	connections, err := s.channelConnectionRepo.FindAllActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("error loading active channel connections: %w", err)
	}

	connectionsByChannel := make(map[int64][]SourceConnectionDTO)
	for _, connection := range connections {
		connectionsByChannel[connection.ChannelID] = append(connectionsByChannel[connection.ChannelID], SourceConnectionDTO{
			ID:          connection.ID,
			Name:        connection.Name,
			Environment: connection.Environment,
		})
	}

	sources := make([]SourceDTO, 0, len(connectionsByChannel))
	for _, channel := range channels {
		channelConnections, ok := connectionsByChannel[channel.ID]
		if !ok {
			continue
		}
		sources = append(sources, SourceDTO{
			ChannelID:   channel.ID,
			ChannelCode: channel.Code,
			ChannelName: channel.Name,
			Supported:   isSupportedChannelCode(channel.Code),
			Connections: channelConnections,
		})
	}

	return sources, nil
}

// --- BrowseTree --------------------------------------------------------

// ExternalNodeDTO is one node of a channel's external category tree. Leaf is
// nil when the channel can't tell without another call (MercadoLibre): the
// caller expands it and treats an empty child list as "leaf".
type ExternalNodeDTO struct {
	ExternalID string `json:"externalId"`
	Name       string `json:"name"`
	Leaf       *bool  `json:"leaf,omitempty"`
}

// BrowseTree returns the children of externalCategoryID in the channel's own
// category tree — or its root categories when externalCategoryID is empty.
func (s *Service) BrowseTree(ctx context.Context, connectionID int64, externalCategoryID string) ([]ExternalNodeDTO, error) {
	if connectionID <= 0 {
		return nil, ErrInvalidInput
	}

	code, _, err := s.resolveChannelCode(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	switch code {
	case channelCodeMercadoLibre:
		return s.browseMercadoLibre(ctx, connectionID, strings.TrimSpace(externalCategoryID))
	case channelCodeOdoo:
		return s.browseOdoo(ctx, connectionID, strings.TrimSpace(externalCategoryID))
	default:
		return nil, ErrChannelNotSupported
	}
}

func (s *Service) browseMercadoLibre(ctx context.Context, connectionID int64, externalCategoryID string) ([]ExternalNodeDTO, error) {
	if externalCategoryID == "" {
		raw, err := s.mercadoLibre.BrowseRootCategoriesForConnection(ctx, connectionID)
		if err != nil {
			return nil, err
		}
		var roots []mlIDName
		if err := json.Unmarshal(raw, &roots); err != nil {
			return nil, fmt.Errorf("error decoding mercadolibre root categories: %w", err)
		}
		return mlNodes(roots), nil
	}

	children, err := s.mercadoLibreChildren(ctx, connectionID, externalCategoryID)
	if err != nil {
		return nil, err
	}
	return mlNodes(children), nil
}

type mlIDName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type mlCategory struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	ChildrenCategories []mlIDName `json:"children_categories"`
}

func (s *Service) mercadoLibreCategory(ctx context.Context, connectionID int64, externalCategoryID string) (mlCategory, error) {
	raw, err := s.mercadoLibre.BrowseCategory(ctx, connectionID, externalCategoryID)
	if err != nil {
		return mlCategory{}, err
	}
	var category mlCategory
	if err := json.Unmarshal(raw, &category); err != nil {
		return mlCategory{}, fmt.Errorf("error decoding mercadolibre category %q: %w", externalCategoryID, err)
	}
	return category, nil
}

func (s *Service) mercadoLibreChildren(ctx context.Context, connectionID int64, externalCategoryID string) ([]mlIDName, error) {
	category, err := s.mercadoLibreCategory(ctx, connectionID, externalCategoryID)
	if err != nil {
		return nil, err
	}
	return category.ChildrenCategories, nil
}

func mlNodes(items []mlIDName) []ExternalNodeDTO {
	nodes := make([]ExternalNodeDTO, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		nodes = append(nodes, ExternalNodeDTO{ExternalID: id, Name: name})
	}
	return nodes
}

func (s *Service) browseOdoo(ctx context.Context, connectionID int64, externalCategoryID string) ([]ExternalNodeDTO, error) {
	categories, err := s.loadOdooCategories(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	childrenByParent, hasChildren := indexOdooCategories(categories)

	var parentID int64
	if externalCategoryID != "" {
		parsed, err := strconv.ParseInt(externalCategoryID, 10, 64)
		if err != nil {
			return nil, ErrInvalidInput
		}
		parentID = parsed
	}

	children := childrenByParent[parentID]
	nodes := make([]ExternalNodeDTO, 0, len(children))
	for _, category := range children {
		leaf := !hasChildren[category.ID]
		nodes = append(nodes, ExternalNodeDTO{
			ExternalID: strconv.FormatInt(category.ID, 10),
			Name:       strings.TrimSpace(category.Name),
			Leaf:       &leaf,
		})
	}
	return nodes, nil
}

// indexOdooCategories groups Odoo categories by parent id (0 = root) and marks
// which ids are a parent of at least one other category.
func indexOdooCategories(categories []odooInfra.PublicCategory) (map[int64][]odooInfra.PublicCategory, map[int64]bool) {
	childrenByParent := make(map[int64][]odooInfra.PublicCategory, len(categories))
	hasChildren := make(map[int64]bool, len(categories))
	for _, category := range categories {
		var parentID int64
		if category.ParentID.Valid {
			parentID = category.ParentID.ID
		}
		childrenByParent[parentID] = append(childrenByParent[parentID], category)
		if category.ParentID.Valid {
			hasChildren[category.ParentID.ID] = true
		}
	}
	return childrenByParent, hasChildren
}

// --- Import -----------------------------------------------------------

type ImportResultDTO struct {
	CategoryID          int64   `json:"categoryId"`
	Name                string  `json:"name"`
	CreatedCount        int     `json:"createdCount"`
	AlreadyLinked       bool    `json:"alreadyLinked"`
	MappedConnectionIDs []int64 `json:"mappedConnectionIds"`
}

// Import replicates the external leaf category externalCategoryID (and its
// missing ancestors) into ecom_categories and records an
// ecom_channel_category_map row for the leaf on every target connection —
// which is the single passed connection when onlySelectedConnection is true,
// or every active connection of the same channel otherwise.
func (s *Service) Import(ctx context.Context, connectionID int64, externalCategoryID string, actorID int64, onlySelectedConnection bool) (*ImportResultDTO, error) {
	externalCategoryID = strings.TrimSpace(externalCategoryID)
	if connectionID <= 0 || externalCategoryID == "" || actorID <= 0 {
		return nil, ErrInvalidInput
	}

	code, channelCode, err := s.resolveChannelCode(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	targets, err := s.resolveTargetConnections(ctx, connectionID, channelCode, onlySelectedConnection)
	if err != nil {
		return nil, err
	}

	switch code {
	case channelCodeMercadoLibre:
		return s.importMercadoLibre(ctx, connectionID, externalCategoryID, actorID, targets)
	case channelCodeOdoo:
		return s.importOdoo(ctx, connectionID, externalCategoryID, actorID, targets)
	default:
		return nil, ErrChannelNotSupported
	}
}

func (s *Service) resolveTargetConnections(ctx context.Context, connectionID int64, channelCode string, onlySelectedConnection bool) ([]int64, error) {
	if onlySelectedConnection {
		return []int64{connectionID}, nil
	}

	connections, err := s.channelConnectionRepo.FindActiveByChannelCode(ctx, channelCode)
	if err != nil {
		return nil, fmt.Errorf("error loading active connections for channel %q: %w", channelCode, err)
	}

	ids := make([]int64, 0, len(connections)+1)
	seen := make(map[int64]bool)
	for _, connection := range connections {
		if !seen[connection.ID] {
			ids = append(ids, connection.ID)
			seen[connection.ID] = true
		}
	}
	if !seen[connectionID] {
		ids = append(ids, connectionID)
	}
	return ids, nil
}

func (s *Service) importMercadoLibre(ctx context.Context, connectionID int64, externalCategoryID string, actorID int64, targets []int64) (*ImportResultDTO, error) {
	children, err := s.mercadoLibreChildren(ctx, connectionID, externalCategoryID)
	if err != nil {
		return nil, err
	}
	if len(children) > 0 {
		return nil, ErrNotLeaf
	}

	alreadyLinked := s.mappingExists(ctx, externalCategoryID, connectionID)
	categoriesBefore, err := s.countCategories(ctx)
	if err != nil {
		return nil, err
	}

	categoryID, err := s.mercadoLibre.EnsureLocalCategory(ctx, connectionID, externalCategoryID, actorID)
	if err != nil {
		return nil, fmt.Errorf("error importing mercadolibre category %q: %w", externalCategoryID, err)
	}

	categoriesAfter, err := s.countCategories(ctx)
	if err != nil {
		return nil, err
	}

	category, err := s.categoriesRepository.FindByID(ctx, categoryID)
	if err != nil {
		return nil, fmt.Errorf("error loading imported category %d: %w", categoryID, err)
	}

	externalName := category.Name
	if mapping, mapErr := s.channelCategoryMapRepo.FindByCategoryAndConnection(ctx, categoryID, connectionID); mapErr == nil && mapping.ExternalCategoryName != nil {
		externalName = *mapping.ExternalCategoryName
	}

	mapped, err := s.upsertMappings(ctx, categoryID, externalCategoryID, externalName, actorID, targets)
	if err != nil {
		return nil, err
	}

	return &ImportResultDTO{
		CategoryID:          categoryID,
		Name:                category.Name,
		CreatedCount:        categoriesAfter - categoriesBefore,
		AlreadyLinked:       alreadyLinked,
		MappedConnectionIDs: mapped,
	}, nil
}

func (s *Service) importOdoo(ctx context.Context, connectionID int64, externalCategoryID string, actorID int64, targets []int64) (*ImportResultDTO, error) {
	odooID, err := strconv.ParseInt(externalCategoryID, 10, 64)
	if err != nil {
		return nil, ErrInvalidInput
	}

	categories, err := s.loadOdooCategories(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]odooInfra.PublicCategory, len(categories))
	for _, category := range categories {
		byID[category.ID] = category
	}
	if _, ok := byID[odooID]; !ok {
		return nil, ErrExternalCategoryNotFound
	}

	_, hasChildren := indexOdooCategories(categories)
	if hasChildren[odooID] {
		return nil, ErrNotLeaf
	}

	// Build the root→leaf path.
	path := make([]odooInfra.PublicCategory, 0)
	for cursor := odooID; ; {
		category, ok := byID[cursor]
		if !ok {
			return nil, fmt.Errorf("odoo category %d in the path of %d is missing from the fetched set", cursor, odooID)
		}
		path = append([]odooInfra.PublicCategory{category}, path...)
		if !category.ParentID.Valid {
			break
		}
		cursor = category.ParentID.ID
	}

	alreadyLinked := s.mappingExists(ctx, externalCategoryID, connectionID)

	var parentLocalID *int64
	var leafLocalID int64
	created := 0
	for _, node := range path {
		name := strings.TrimSpace(node.Name)
		if name == "" {
			return nil, fmt.Errorf("odoo category %d has an empty name", node.ID)
		}

		existing, err := s.categoriesRepository.FindByNameAndParentID(ctx, name, parentLocalID)
		if err != nil && !errors.Is(err, mysqlInfra.ErrCategoryNotFound) {
			return nil, fmt.Errorf("error looking up category %q: %w", name, err)
		}

		var localID int64
		if existing != nil {
			localID = existing.ID
		} else {
			row, err := s.categoriesRepository.Create(ctx, mysqlInfra.CreateCategoryInput{
				Name:      name,
				ParentID:  parentLocalID,
				CreatedBy: actorID,
			})
			if err != nil {
				return nil, fmt.Errorf("error creating category %q: %w", name, err)
			}
			localID = row.ID
			created++
		}

		parentLocalID = &localID
		leafLocalID = localID
	}

	leafName := strings.TrimSpace(path[len(path)-1].Name)
	mapped, err := s.upsertMappings(ctx, leafLocalID, externalCategoryID, leafName, actorID, targets)
	if err != nil {
		return nil, err
	}

	return &ImportResultDTO{
		CategoryID:          leafLocalID,
		Name:                leafName,
		CreatedCount:        created,
		AlreadyLinked:       alreadyLinked,
		MappedConnectionIDs: mapped,
	}, nil
}

// --- SetMapping (link/replace from the category detail view) ------------

type SetMappingResultDTO struct {
	CategoryID           int64  `json:"categoryId"`
	ConnectionID         int64  `json:"connectionId"`
	ExternalCategoryID   string `json:"externalCategoryId"`
	ExternalCategoryName string `json:"externalCategoryName"`
	Replaced             bool   `json:"replaced"`
}

// SetMapping links an already-existing local category to an external leaf
// category on one connection (or replaces its current mapping). Unlike Import
// it never creates local categories — the local side is the category the user
// is looking at; it only writes the ecom_channel_category_map row.
func (s *Service) SetMapping(ctx context.Context, categoryID, connectionID int64, externalCategoryID string, actorID int64) (*SetMappingResultDTO, error) {
	externalCategoryID = strings.TrimSpace(externalCategoryID)
	if categoryID <= 0 || connectionID <= 0 || externalCategoryID == "" || actorID <= 0 {
		return nil, ErrInvalidInput
	}

	if _, err := s.categoriesRepository.FindByID(ctx, categoryID); err != nil {
		if errors.Is(err, mysqlInfra.ErrCategoryNotFound) {
			return nil, ErrLocalCategoryNotFound
		}
		return nil, fmt.Errorf("error loading local category %d: %w", categoryID, err)
	}

	normalized, _, err := s.resolveChannelCode(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	name, err := s.externalLeafName(ctx, normalized, connectionID, externalCategoryID)
	if err != nil {
		return nil, err
	}

	existing, err := s.channelCategoryMapRepo.FindByCategoryAndConnection(ctx, categoryID, connectionID)
	if err != nil && !errors.Is(err, mysqlInfra.ErrChannelCategoryMapNotFound) {
		return nil, fmt.Errorf("error loading current mapping: %w", err)
	}

	var namePtr *string
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		namePtr = &trimmed
	}
	if _, err := s.channelCategoryMapRepo.Upsert(ctx, mysqlInfra.UpsertChannelCategoryMapInput{
		CategoryID:           categoryID,
		ConnectionID:         connectionID,
		ExternalCategoryID:   externalCategoryID,
		ExternalCategoryName: namePtr,
		ActorID:              actorID,
	}); err != nil {
		return nil, fmt.Errorf("error saving category mapping: %w", err)
	}

	return &SetMappingResultDTO{
		CategoryID:           categoryID,
		ConnectionID:         connectionID,
		ExternalCategoryID:   externalCategoryID,
		ExternalCategoryName: name,
		Replaced:             existing != nil,
	}, nil
}

// externalLeafName returns the external category's own name, or ErrNotLeaf if
// it still has children (and ErrExternalCategoryNotFound / ErrChannelNotSupported
// as applicable).
func (s *Service) externalLeafName(ctx context.Context, normalizedCode string, connectionID int64, externalCategoryID string) (string, error) {
	switch normalizedCode {
	case channelCodeMercadoLibre:
		category, err := s.mercadoLibreCategory(ctx, connectionID, externalCategoryID)
		if err != nil {
			return "", err
		}
		if len(category.ChildrenCategories) > 0 {
			return "", ErrNotLeaf
		}
		return strings.TrimSpace(category.Name), nil
	case channelCodeOdoo:
		odooID, err := strconv.ParseInt(externalCategoryID, 10, 64)
		if err != nil {
			return "", ErrInvalidInput
		}
		categories, err := s.loadOdooCategories(ctx, connectionID)
		if err != nil {
			return "", err
		}
		var node *odooInfra.PublicCategory
		for i := range categories {
			if categories[i].ID == odooID {
				node = &categories[i]
				break
			}
		}
		if node == nil {
			return "", ErrExternalCategoryNotFound
		}
		if _, hasChildren := indexOdooCategories(categories); hasChildren[odooID] {
			return "", ErrNotLeaf
		}
		return strings.TrimSpace(node.Name), nil
	default:
		return "", ErrChannelNotSupported
	}
}

// upsertMappings records (or refreshes) the ecom_channel_category_map row
// linking categoryID to externalCategoryID on every target connection.
func (s *Service) upsertMappings(ctx context.Context, categoryID int64, externalCategoryID, externalName string, actorID int64, targets []int64) ([]int64, error) {
	name := strings.TrimSpace(externalName)
	var namePtr *string
	if name != "" {
		namePtr = &name
	}

	mapped := make([]int64, 0, len(targets))
	for _, target := range targets {
		if _, err := s.channelCategoryMapRepo.Upsert(ctx, mysqlInfra.UpsertChannelCategoryMapInput{
			CategoryID:           categoryID,
			ConnectionID:         target,
			ExternalCategoryID:   externalCategoryID,
			ExternalCategoryName: namePtr,
			ActorID:              actorID,
		}); err != nil {
			return nil, fmt.Errorf("error recording category mapping on connection %d: %w", target, err)
		}
		mapped = append(mapped, target)
	}
	return mapped, nil
}

func (s *Service) mappingExists(ctx context.Context, externalCategoryID string, connectionID int64) bool {
	_, err := s.channelCategoryMapRepo.FindByExternalCategoryAndConnection(ctx, externalCategoryID, connectionID)
	return err == nil
}

func (s *Service) countCategories(ctx context.Context) (int, error) {
	tree, err := s.categoriesRepository.FindTree(ctx)
	if err != nil {
		return 0, fmt.Errorf("error counting categories: %w", err)
	}
	return countTree(tree), nil
}

func countTree(nodes []mysqlInfra.CategoryTreeDTO) int {
	total := 0
	for _, node := range nodes {
		total += 1 + countTree(node.Children)
	}
	return total
}

// --- shared helpers --------------------------------------------------

// resolveChannelCode returns the connection's channel code both normalized
// (upper/trimmed, for switching) and raw (as stored, for FindActiveByChannelCode
// which normalizes internally).
func (s *Service) resolveChannelCode(ctx context.Context, connectionID int64) (normalized string, raw string, err error) {
	connection, err := s.channelConnectionRepo.FindByID(ctx, connectionID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrChannelConnectionNotFound) {
			return "", "", ErrConnectionNotFound
		}
		return "", "", fmt.Errorf("error loading connection %d: %w", connectionID, err)
	}

	channel, err := s.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return "", "", fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}

	return strings.ToUpper(strings.TrimSpace(channel.Code)), channel.Code, nil
}

func (s *Service) loadOdooCategories(ctx context.Context, connectionID int64) ([]odooInfra.PublicCategory, error) {
	values, err := syncApp.LoadOdooConnectionValues(ctx, s.connectionCredentialsRepo, s.connectionSettingsRepo, connectionID)
	if err != nil {
		return nil, err
	}

	odooURL := values["odoo_url"]
	apiKey := values["apikey"]
	if odooURL == "" || apiKey == "" {
		return nil, fmt.Errorf("%w: missing odoo_url/apikey for connection %d", ErrInvalidInput, connectionID)
	}

	client := odooInfra.NewClient(nil, odooURL, s.odooRateLimiter)
	handler := odooInfra.NewCategoriesHandler(client)
	categories, err := handler.SearchReadPublicCategories(ctx, odooInfra.SearchReadPublicCategoriesRequest{
		Credentials: odooInfra.Credentials{APIKey: apiKey, Database: values["x-odoo-database"]},
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching odoo categories: %w", err)
	}
	return categories, nil
}

func isSupportedChannelCode(code string) bool {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case channelCodeMercadoLibre, channelCodeOdoo:
		return true
	default:
		return false
	}
}

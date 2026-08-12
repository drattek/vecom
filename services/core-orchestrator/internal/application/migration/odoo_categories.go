package migration

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	syncApp "core-orchestrator/internal/application/sync"
	odooInfra "core-orchestrator/internal/infrastructure/marketplace/odoo"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidOdooCategoryConnection  = errors.New("invalid odoo connection")
	ErrMissingOdooCategoryActor       = errors.New("missing requesting user")
	ErrMissingOdooCategorySettings    = errors.New("missing required odoo connection settings")
	ErrMissingOdooCategoryCredentials = errors.New("missing required odoo connection credentials")
)

// OdooCategoryMapping describes how a single Odoo product.public.category
// record was resolved against ecom_categories.
type OdooCategoryMapping struct {
	OdooID     int64  `json:"odooId"`
	CategoryID int64  `json:"categoryId"`
	Name       string `json:"name"`
	Created    bool   `json:"created"`
}

type OdooCategoryMigrationResult struct {
	ConnectionID int64                 `json:"connectionId"`
	Fetched      int                   `json:"fetched"`
	Created      int                   `json:"created"`
	Matched      int                   `json:"matched"`
	Categories   []OdooCategoryMapping `json:"categories"`
}

// OdooCategoryMigrationService is a one-off data-completion helper: it
// downloads Odoo's ecommerce category tree (product.public.category) for a
// connection and mirrors it into ecom_categories, matching existing rows by
// name (scoped to the correct parent) and creating whatever is missing
// using the same hierarchy Odoo has.
type OdooCategoryMigrationService struct {
	credentialsRepository *mysqlInfra.ConnectionCredentialsRepository
	settingsRepository    *mysqlInfra.ConnectionSettingsRepository
	categoriesRepository  *mysqlInfra.CategoriesRepository
	categoryMapRepository *mysqlInfra.ChannelCategoryMapRepository
	rateLimiter           *odooInfra.RateLimiter
}

func NewOdooCategoryMigrationService(
	credentialsRepository *mysqlInfra.ConnectionCredentialsRepository,
	settingsRepository *mysqlInfra.ConnectionSettingsRepository,
	categoriesRepository *mysqlInfra.CategoriesRepository,
	categoryMapRepository *mysqlInfra.ChannelCategoryMapRepository,
	rateLimiter *odooInfra.RateLimiter,
) *OdooCategoryMigrationService {
	return &OdooCategoryMigrationService{
		credentialsRepository: credentialsRepository,
		settingsRepository:    settingsRepository,
		categoriesRepository:  categoriesRepository,
		categoryMapRepository: categoryMapRepository,
		rateLimiter:           rateLimiter,
	}
}

func (s *OdooCategoryMigrationService) MigrateCategories(ctx context.Context, connectionID int64, requestedBy int64) (*OdooCategoryMigrationResult, error) {
	if connectionID <= 0 {
		return nil, ErrInvalidOdooCategoryConnection
	}
	if requestedBy <= 0 {
		return nil, ErrMissingOdooCategoryActor
	}

	values, err := syncApp.LoadOdooConnectionValues(s.credentialsRepository, s.settingsRepository, connectionID)
	if err != nil {
		return nil, err
	}

	odooURL, ok := values["odoo_url"]
	if !ok || odooURL == "" {
		return nil, fmt.Errorf("%w: odoo_url", ErrMissingOdooCategorySettings)
	}

	apiKey, ok := values["apikey"]
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("%w: ApiKey", ErrMissingOdooCategoryCredentials)
	}

	database := values["x-odoo-database"]

	client := odooInfra.NewClient(nil, odooURL, s.rateLimiter)
	categoriesHandler := odooInfra.NewCategoriesHandler(client)

	odooCategories, err := categoriesHandler.SearchReadPublicCategories(ctx, odooInfra.SearchReadPublicCategoriesRequest{
		Credentials: odooInfra.Credentials{APIKey: apiKey, Database: database},
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching odoo categories: %w", err)
	}

	result, err := s.upsertHierarchy(odooCategories, connectionID, requestedBy)
	if err != nil {
		return nil, err
	}

	result.ConnectionID = connectionID
	return result, nil
}

type categoryResolution struct {
	localID int64
	created bool
}

// upsertHierarchy walks each Odoo category up through its parent chain
// (resolving/creating parents first, since ecom_categories.parent_id is a
// self-referencing FK) and memoizes results so a parent shared by several
// children is only matched/created once. Every resolved category (whether
// newly created or matched by name) gets its ecom_channel_category_map row
// upserted, so the local category stays traceable to the Odoo
// product.public.category it came from even after this migration endpoint
// is retired.
func (s *OdooCategoryMigrationService) upsertHierarchy(odooCategories []odooInfra.PublicCategory, connectionID int64, requestedBy int64) (*OdooCategoryMigrationResult, error) {
	byOdooID := make(map[int64]odooInfra.PublicCategory, len(odooCategories))
	for _, category := range odooCategories {
		byOdooID[category.ID] = category
	}

	resolved := make(map[int64]categoryResolution, len(odooCategories))

	var resolve func(odooID int64, visiting map[int64]bool) (categoryResolution, error)
	resolve = func(odooID int64, visiting map[int64]bool) (categoryResolution, error) {
		if res, ok := resolved[odooID]; ok {
			return res, nil
		}
		if visiting[odooID] {
			return categoryResolution{}, fmt.Errorf("cyclical odoo category hierarchy detected at category id %d", odooID)
		}
		visiting[odooID] = true
		defer delete(visiting, odooID)

		odooCategory, ok := byOdooID[odooID]
		if !ok {
			return categoryResolution{}, fmt.Errorf("odoo category %d referenced as parent but not present in the fetched set", odooID)
		}

		name := strings.TrimSpace(odooCategory.Name)
		if name == "" {
			return categoryResolution{}, fmt.Errorf("odoo category %d has an empty name", odooID)
		}

		var parentLocalID *int64
		if odooCategory.ParentID.Valid {
			parentRes, err := resolve(odooCategory.ParentID.ID, visiting)
			if err != nil {
				return categoryResolution{}, err
			}
			parentLocalID = &parentRes.localID
		}

		existing, err := s.categoriesRepository.FindByNameAndParentID(name, parentLocalID)
		if err != nil && !errors.Is(err, mysqlInfra.ErrCategoryNotFound) {
			return categoryResolution{}, fmt.Errorf("error looking up category %q: %w", name, err)
		}

		var res categoryResolution
		if existing != nil {
			res = categoryResolution{localID: existing.ID}
		} else {
			created, err := s.categoriesRepository.Create(mysqlInfra.CreateCategoryInput{
				Name:      name,
				ParentID:  parentLocalID,
				CreatedBy: requestedBy,
			})
			if err != nil {
				return categoryResolution{}, fmt.Errorf("error creating category %q: %w", name, err)
			}
			res = categoryResolution{localID: created.ID, created: true}
		}

		resolved[odooID] = res
		return res, nil
	}

	mappings := make([]OdooCategoryMapping, 0, len(odooCategories))
	createdCount := 0
	matchedCount := 0

	for _, odooCategory := range odooCategories {
		res, err := resolve(odooCategory.ID, map[int64]bool{})
		if err != nil {
			return nil, err
		}

		if res.created {
			createdCount++
		} else {
			matchedCount++
		}

		name := strings.TrimSpace(odooCategory.Name)
		if _, err := s.categoryMapRepository.Upsert(mysqlInfra.UpsertChannelCategoryMapInput{
			CategoryID:           res.localID,
			ConnectionID:         connectionID,
			ExternalCategoryID:   strconv.FormatInt(odooCategory.ID, 10),
			ExternalCategoryName: &name,
			ActorID:              requestedBy,
		}); err != nil {
			return nil, fmt.Errorf("error recording channel category map for category %q: %w", name, err)
		}

		mappings = append(mappings, OdooCategoryMapping{
			OdooID:     odooCategory.ID,
			CategoryID: res.localID,
			Name:       name,
			Created:    res.created,
		})
	}

	return &OdooCategoryMigrationResult{
		Fetched:    len(odooCategories),
		Created:    createdCount,
		Matched:    matchedCount,
		Categories: mappings,
	}, nil
}

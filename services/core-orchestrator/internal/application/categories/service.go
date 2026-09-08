package categories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidCategoryPayload = errors.New("invalid category payload")

type CategoryService struct {
	db                 *sql.DB
	repository         *mysqlInfra.CategoriesRepository
	channels           *mysqlInfra.ChannelRepository
	channelConnections *mysqlInfra.ChannelConnectionRepository
	channelCategoryMap *mysqlInfra.ChannelCategoryMapRepository
}

// NewCategoryService recibe *sql.DB (estructura común a todos los servicios, para
// operaciones multi-sentencia vía mysqlInfra.WithinTx) además del repo sobre el
// pool. Hoy todas las operaciones de categorías son de una sola sentencia, así
// que no se abre transacción — ver
// infrastructure/decisions/0001-persistencia-transacciones-y-context.md.
func NewCategoryService(
	db *sql.DB,
	repository *mysqlInfra.CategoriesRepository,
	channels *mysqlInfra.ChannelRepository,
	channelConnections *mysqlInfra.ChannelConnectionRepository,
	channelCategoryMap *mysqlInfra.ChannelCategoryMapRepository,
) *CategoryService {
	return &CategoryService{
		db:                 db,
		repository:         repository,
		channels:           channels,
		channelConnections: channelConnections,
		channelCategoryMap: channelCategoryMap,
	}
}

// GetCategories returns every category as a nested tree built from
// parent_id, with each category's children under its Children key.
func (s *CategoryService) GetCategories(ctx context.Context) ([]mysqlInfra.CategoryTreeDTO, error) {
	return s.repository.FindTree(ctx)
}

func (s *CategoryService) GetCategoryByID(ctx context.Context, id int64) (*mysqlInfra.CategoryDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidCategoryPayload
	}
	return s.repository.FindByID(ctx, id)
}

func (s *CategoryService) GetCategoryChildren(ctx context.Context, id int64) ([]mysqlInfra.CategoryDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidCategoryPayload
	}
	return s.repository.FindByParentID(ctx, id)
}

// CategoryChannelMappingDTO is one channel connection's view of a local
// category: the external category it maps to on that connection, or nulls when
// there's no mapping yet. The mapping itself is written by the sync flows or by
// category_import.Service (link/replace from the category detail view).
type CategoryChannelMappingDTO struct {
	ConnectionID         int64   `json:"connectionId"`
	ConnectionName       string  `json:"connectionName"`
	ChannelID            int64   `json:"channelId"`
	ChannelName          string  `json:"channelName"`
	ChannelCode          string  `json:"channelCode"`
	Environment          string  `json:"environment"`
	ExternalCategoryID   *string `json:"externalCategoryId,omitempty"`
	ExternalCategoryName *string `json:"externalCategoryName,omitempty"`
	MappedAt             *string `json:"mappedAt,omitempty"`
}

type CategoryChannelMappingsDTO struct {
	Mappings []CategoryChannelMappingDTO `json:"mappings"`
}

// GetChannelMappings returns, for a local category, one entry per active
// channel connection with whatever ecom_channel_category_map already links to
// that category on that connection (nil external fields when unmapped).
func (s *CategoryService) GetChannelMappings(ctx context.Context, id int64) (*CategoryChannelMappingsDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidCategoryPayload
	}

	if _, err := s.repository.FindByID(ctx, id); err != nil {
		return nil, err
	}

	connections, err := s.channelConnections.FindAllActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("error loading active channel connections: %w", err)
	}

	maps, err := s.channelCategoryMap.FindByCategory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("error loading category mappings: %w", err)
	}

	byConnection := make(map[int64]mysqlInfra.ChannelCategoryMapDTO, len(maps))
	for _, m := range maps {
		byConnection[m.ConnectionID] = m
	}

	channelCodeByID := make(map[int64]string)

	out := &CategoryChannelMappingsDTO{Mappings: make([]CategoryChannelMappingDTO, 0, len(connections))}
	for _, connection := range connections {
		code, ok := channelCodeByID[connection.ChannelID]
		if !ok {
			channel, err := s.channels.FindByID(ctx, connection.ChannelID)
			if err != nil && !errors.Is(err, mysqlInfra.ErrChannelNotFound) {
				return nil, fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
			}
			if channel != nil {
				code = channel.Code
			}
			channelCodeByID[connection.ChannelID] = code
		}

		entry := CategoryChannelMappingDTO{
			ConnectionID:   connection.ID,
			ConnectionName: connection.Name,
			ChannelID:      connection.ChannelID,
			ChannelName:    connection.ChannelName,
			ChannelCode:    code,
			Environment:    connection.Environment,
		}
		if m, ok := byConnection[connection.ID]; ok {
			externalID := m.ExternalCategoryID
			entry.ExternalCategoryID = &externalID
			entry.ExternalCategoryName = m.ExternalCategoryName
			mappedAt := m.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
			entry.MappedAt = &mappedAt
		}
		out.Mappings = append(out.Mappings, entry)
	}

	return out, nil
}

func (s *CategoryService) CreateCategory(ctx context.Context, input mysqlInfra.CreateCategoryInput) (*mysqlInfra.CategoryDTO, error) {
	if input.Name == "" {
		return nil, ErrInvalidCategoryPayload
	}
	if input.CreatedBy <= 0 {
		return nil, ErrInvalidCategoryPayload
	}

	return s.repository.Create(ctx, input)
}

func (s *CategoryService) UpdateCategory(ctx context.Context, id int64, input mysqlInfra.UpdateCategoryInput) (*mysqlInfra.CategoryDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidCategoryPayload
	}
	if input.Name == "" {
		return nil, ErrInvalidCategoryPayload
	}
	if input.UpdatedBy <= 0 {
		return nil, ErrInvalidCategoryPayload
	}

	return s.repository.Update(ctx, id, input)
}

func (s *CategoryService) DeleteCategory(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidCategoryPayload
	}
	return s.repository.SoftDelete(ctx, id)
}

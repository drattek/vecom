// Package dashboard expone las lecturas de la vista Dashboard. Cada bloque de
// la pantalla es una operación independiente para que una consulta lenta no
// retrase a las demás.
package dashboard

import (
	"context"
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

const (
	defaultPageSize = 10
	maxPageSize     = 100
)

var (
	ErrInvalidState       = errors.New("invalid listing state")
	ErrInvalidFilter      = errors.New("invalid catalog filter")
	ErrConnectionRequired = errors.New("connectionId is required for this filter")
)

// attentionStates son los estados de publicación que requieren atención.
var attentionStates = map[string]bool{
	mysqlInfra.ListingStateError:           true,
	mysqlInfra.ListingStateNotPublished:    true,
	mysqlInfra.ListingStateUnderReview:     true,
	mysqlInfra.ListingStatePausedManual:    true,
	mysqlInfra.ListingStatePausedZeroStock: true,
}

var catalogFilters = map[string]bool{
	mysqlInfra.CatalogFilterNoImage:          true,
	mysqlInfra.CatalogFilterNoPrice:          true,
	mysqlInfra.CatalogFilterNoBrand:          true,
	mysqlInfra.CatalogFilterNoCategory:       true,
	mysqlInfra.CatalogFilterNoStock:          true,
	mysqlInfra.CatalogFilterCategoryUnmapped: true,
	mysqlInfra.CatalogFilterReadyUnpublished: true,
}

type Service struct {
	repository *mysqlInfra.DashboardRepository
}

func NewService(repository *mysqlInfra.DashboardRepository) *Service {
	return &Service{repository: repository}
}

type AttentionPage struct {
	Total    int64                              `json:"total"`
	Offset   int                                `json:"offset"`
	PageSize int                                `json:"pageSize"`
	Rows     []mysqlInfra.DashboardAttentionRow `json:"rows"`
}

type CatalogPage struct {
	Offset   int                              `json:"offset"`
	PageSize int                              `json:"pageSize"`
	Rows     []mysqlInfra.DashboardCatalogRow `json:"rows"`
}

func normalizePage(offset, pageSize int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	return offset, min(pageSize, maxPageSize)
}

func validateCatalogFilter(filter string, connectionID int64) error {
	if !catalogFilters[filter] {
		return ErrInvalidFilter
	}
	if filter == mysqlInfra.CatalogFilterCategoryUnmapped && connectionID <= 0 {
		return ErrConnectionRequired
	}
	return nil
}

// Attention devuelve una página de las publicaciones en un estado que requiere
// atención.
func (s *Service) Attention(ctx context.Context, state string, offset, pageSize int) (*AttentionPage, error) {
	if !attentionStates[state] {
		return nil, ErrInvalidState
	}
	offset, pageSize = normalizePage(offset, pageSize)

	rows, total, err := s.repository.FindAttentionPage(ctx, state, offset, pageSize)
	if err != nil {
		return nil, err
	}
	return &AttentionPage{Total: total, Offset: offset, PageSize: pageSize, Rows: rows}, nil
}

func (s *Service) CatalogCount(ctx context.Context, filter string, connectionID int64) (int64, error) {
	if err := validateCatalogFilter(filter, connectionID); err != nil {
		return 0, err
	}
	return s.repository.CountCatalogProducts(ctx, filter, connectionID)
}

func (s *Service) CatalogProducts(ctx context.Context, filter string, connectionID int64, offset, pageSize int) (*CatalogPage, error) {
	if err := validateCatalogFilter(filter, connectionID); err != nil {
		return nil, err
	}
	offset, pageSize = normalizePage(offset, pageSize)

	rows, err := s.repository.FindCatalogProducts(ctx, filter, connectionID, offset, pageSize)
	if err != nil {
		return nil, err
	}
	return &CatalogPage{Offset: offset, PageSize: pageSize, Rows: rows}, nil
}

func (s *Service) ConnectionsHealth(ctx context.Context) ([]mysqlInfra.DashboardConnectionHealth, error) {
	return s.repository.FindConnectionsHealth(ctx)
}

func (s *Service) InventoryHealth(ctx context.Context) ([]mysqlInfra.DashboardInventoryHealth, error) {
	return s.repository.FindInventoryHealth(ctx)
}

func (s *Service) QueueHealth(ctx context.Context) ([]mysqlInfra.DashboardQueueHealth, error) {
	return s.repository.FindQueueHealth(ctx)
}

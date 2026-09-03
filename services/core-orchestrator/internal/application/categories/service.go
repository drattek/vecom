package categories

import (
	"context"
	"database/sql"
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidCategoryPayload = errors.New("invalid category payload")

type CategoryService struct {
	db         *sql.DB
	repository *mysqlInfra.CategoriesRepository
}

// NewCategoryService recibe *sql.DB (estructura común a todos los servicios, para
// operaciones multi-sentencia vía mysqlInfra.WithinTx) además del repo sobre el
// pool. Hoy todas las operaciones de categorías son de una sola sentencia, así
// que no se abre transacción — ver
// infrastructure/decisions/0001-persistencia-transacciones-y-context.md.
func NewCategoryService(db *sql.DB, repository *mysqlInfra.CategoriesRepository) *CategoryService {
	return &CategoryService{db: db, repository: repository}
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

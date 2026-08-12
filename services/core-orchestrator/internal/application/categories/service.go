package categories

import (
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidCategoryPayload = errors.New("invalid category payload")

type CategoryService struct {
	repository *mysqlInfra.CategoriesRepository
}

func NewCategoryService(repository *mysqlInfra.CategoriesRepository) *CategoryService {
	return &CategoryService{repository: repository}
}

// GetCategories returns every category as a nested tree built from
// parent_id, with each category's children under its Children key.
func (s *CategoryService) GetCategories() ([]mysqlInfra.CategoryTreeDTO, error) {
	return s.repository.FindTree()
}

func (s *CategoryService) GetCategoryByID(id int64) (*mysqlInfra.CategoryDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidCategoryPayload
	}
	return s.repository.FindByID(id)
}

func (s *CategoryService) GetCategoryChildren(id int64) ([]mysqlInfra.CategoryDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidCategoryPayload
	}
	return s.repository.FindByParentID(id)
}

func (s *CategoryService) CreateCategory(input mysqlInfra.CreateCategoryInput) (*mysqlInfra.CategoryDTO, error) {
	if input.Name == "" {
		return nil, ErrInvalidCategoryPayload
	}
	if input.CreatedBy <= 0 {
		return nil, ErrInvalidCategoryPayload
	}

	return s.repository.Create(input)
}

func (s *CategoryService) UpdateCategory(id int64, input mysqlInfra.UpdateCategoryInput) (*mysqlInfra.CategoryDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidCategoryPayload
	}
	if input.Name == "" {
		return nil, ErrInvalidCategoryPayload
	}
	if input.UpdatedBy <= 0 {
		return nil, ErrInvalidCategoryPayload
	}

	return s.repository.Update(id, input)
}

func (s *CategoryService) DeleteCategory(id int64) error {
	if id <= 0 {
		return ErrInvalidCategoryPayload
	}
	return s.repository.SoftDelete(id)
}

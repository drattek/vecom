package brands

import (
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidBrandPayload = errors.New("invalid brand payload")

type BrandService struct {
	repository *mysqlInfra.BrandsRepository
}

func NewBrandService(repository *mysqlInfra.BrandsRepository) *BrandService {
	return &BrandService{repository: repository}
}

func (s *BrandService) GetPaginatedBrands(offset, pageSize int) (*mysqlInfra.PaginatedBrands, error) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindPaginated(offset, pageSize)
}

func (s *BrandService) GetBrandByID(id int64) (*mysqlInfra.BrandDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidBrandPayload
	}
	return s.repository.FindByID(id)
}

func (s *BrandService) CreateBrand(input mysqlInfra.CreateBrandInput) (*mysqlInfra.BrandDTO, error) {
	if input.Name == "" {
		return nil, ErrInvalidBrandPayload
	}
	if input.CreatedBy <= 0 {
		return nil, ErrInvalidBrandPayload
	}

	return s.repository.Create(input)
}

func (s *BrandService) UpdateBrand(id int64, input mysqlInfra.UpdateBrandInput) (*mysqlInfra.BrandDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidBrandPayload
	}
	if input.Name == "" {
		return nil, ErrInvalidBrandPayload
	}
	if input.UpdatedBy <= 0 {
		return nil, ErrInvalidBrandPayload
	}

	return s.repository.Update(id, input)
}

func (s *BrandService) DeleteBrand(id int64) error {
	if id <= 0 {
		return ErrInvalidBrandPayload
	}
	return s.repository.SoftDelete(id)
}

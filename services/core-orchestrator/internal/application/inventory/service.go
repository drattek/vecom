package inventory

import (
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidInventoryPayload = errors.New("invalid inventory payload")

// BranchService
type BranchService struct {
	repository *mysqlInfra.BranchesRepository
}

func NewBranchService(repository *mysqlInfra.BranchesRepository) *BranchService {
	return &BranchService{repository: repository}
}

func (s *BranchService) GetPaginatedBranches(offset, pageSize int) (*mysqlInfra.PaginatedBranches, error) {
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

func (s *BranchService) GetBranchByID(id int64) (*mysqlInfra.BranchDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.FindByID(id)
}

func (s *BranchService) CreateBranch(input mysqlInfra.CreateBranchInput) (*mysqlInfra.BranchDTO, error) {
	if input.Name == "" || input.CreatedBy <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Create(input)
}

func (s *BranchService) UpdateBranch(id int64, input mysqlInfra.UpdateBranchInput) (*mysqlInfra.BranchDTO, error) {
	if id <= 0 || input.Name == "" || input.UpdatedBy <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Update(id, input)
}

func (s *BranchService) DeleteBranch(id int64) error {
	if id <= 0 {
		return ErrInvalidInventoryPayload
	}
	return s.repository.SoftDelete(id)
}

// WarehouseService
type WarehouseService struct {
	repository *mysqlInfra.WarehousesRepository
}

func NewWarehouseService(repository *mysqlInfra.WarehousesRepository) *WarehouseService {
	return &WarehouseService{repository: repository}
}

func (s *WarehouseService) GetPaginatedWarehouses(offset, pageSize int) (*mysqlInfra.PaginatedWarehouses, error) {
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

func (s *WarehouseService) GetWarehouseByID(id int64) (*mysqlInfra.WarehouseDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.FindByID(id)
}

func (s *WarehouseService) GetWarehousesByBranch(branchID int64) ([]mysqlInfra.WarehouseDTO, error) {
	if branchID <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.FindByBranchID(branchID)
}

func (s *WarehouseService) CreateWarehouse(input mysqlInfra.CreateWarehouseInput) (*mysqlInfra.WarehouseDTO, error) {
	if input.Name == "" || input.BranchID <= 0 || input.CreatedBy <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Create(input)
}

func (s *WarehouseService) UpdateWarehouse(id int64, input mysqlInfra.UpdateWarehouseInput) (*mysqlInfra.WarehouseDTO, error) {
	if id <= 0 || input.Name == "" || input.BranchID <= 0 || input.UpdatedBy <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Update(id, input)
}

func (s *WarehouseService) DeleteWarehouse(id int64) error {
	if id <= 0 {
		return ErrInvalidInventoryPayload
	}
	return s.repository.SoftDelete(id)
}

// ProductStockService
type ProductStockService struct {
	repository *mysqlInfra.ProductStockRepository
}

func NewProductStockService(repository *mysqlInfra.ProductStockRepository) *ProductStockService {
	return &ProductStockService{repository: repository}
}

func (s *ProductStockService) GetPaginatedProductStock(offset, pageSize int) (*mysqlInfra.PaginatedProductStock, error) {
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

func (s *ProductStockService) GetProductStockByID(id int64) (*mysqlInfra.ProductStockDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.FindByID(id)
}

func (s *ProductStockService) GetProductStockByProduct(productID int64) ([]mysqlInfra.ProductStockDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.FindByProductID(productID)
}

func (s *ProductStockService) CreateProductStock(input mysqlInfra.CreateProductStockInput) (*mysqlInfra.ProductStockDTO, error) {
	if input.ProductID <= 0 || input.BranchID <= 0 || input.WarehouseID <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Create(input)
}

func (s *ProductStockService) UpdateProductStock(id int64, input mysqlInfra.UpdateProductStockInput) (*mysqlInfra.ProductStockDTO, error) {
	if id <= 0 || input.UpdatedBy <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Update(id, input)
}

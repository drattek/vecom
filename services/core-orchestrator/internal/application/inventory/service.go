package inventory

import (
	"context"
	"database/sql"
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidInventoryPayload = errors.New("invalid inventory payload")

// BranchService
type BranchService struct {
	db         *sql.DB
	repository *mysqlInfra.BranchesRepository
}

func NewBranchService(db *sql.DB, repository *mysqlInfra.BranchesRepository) *BranchService {
	return &BranchService{db: db, repository: repository}
}

func (s *BranchService) GetPaginatedBranches(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedBranches, error) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *BranchService) GetBranchByID(ctx context.Context, id int64) (*mysqlInfra.BranchDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.FindByID(ctx, id)
}

func (s *BranchService) CreateBranch(ctx context.Context, input mysqlInfra.CreateBranchInput) (*mysqlInfra.BranchDTO, error) {
	if input.Name == "" || input.CreatedBy <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Create(ctx, input)
}

func (s *BranchService) UpdateBranch(ctx context.Context, id int64, input mysqlInfra.UpdateBranchInput) (*mysqlInfra.BranchDTO, error) {
	if id <= 0 || input.Name == "" || input.UpdatedBy <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Update(ctx, id, input)
}

func (s *BranchService) DeleteBranch(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidInventoryPayload
	}
	return s.repository.SoftDelete(ctx, id)
}

// WarehouseService
type WarehouseService struct {
	db         *sql.DB
	repository *mysqlInfra.WarehousesRepository
}

func NewWarehouseService(db *sql.DB, repository *mysqlInfra.WarehousesRepository) *WarehouseService {
	return &WarehouseService{db: db, repository: repository}
}

func (s *WarehouseService) GetPaginatedWarehouses(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedWarehouses, error) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *WarehouseService) GetWarehouseByID(ctx context.Context, id int64) (*mysqlInfra.WarehouseDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.FindByID(ctx, id)
}

func (s *WarehouseService) GetWarehousesByBranch(ctx context.Context, branchID int64) ([]mysqlInfra.WarehouseDTO, error) {
	if branchID <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.FindByBranchID(ctx, branchID)
}

func (s *WarehouseService) CreateWarehouse(ctx context.Context, input mysqlInfra.CreateWarehouseInput) (*mysqlInfra.WarehouseDTO, error) {
	if input.Name == "" || input.BranchID <= 0 || input.CreatedBy <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Create(ctx, input)
}

func (s *WarehouseService) UpdateWarehouse(ctx context.Context, id int64, input mysqlInfra.UpdateWarehouseInput) (*mysqlInfra.WarehouseDTO, error) {
	if id <= 0 || input.Name == "" || input.BranchID <= 0 || input.UpdatedBy <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Update(ctx, id, input)
}

func (s *WarehouseService) DeleteWarehouse(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidInventoryPayload
	}
	return s.repository.SoftDelete(ctx, id)
}

// ProductStockService
type ProductStockService struct {
	db         *sql.DB
	repository *mysqlInfra.ProductStockRepository
}

func NewProductStockService(db *sql.DB, repository *mysqlInfra.ProductStockRepository) *ProductStockService {
	return &ProductStockService{db: db, repository: repository}
}

func (s *ProductStockService) GetPaginatedProductStock(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedProductStock, error) {
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *ProductStockService) GetProductStockByID(ctx context.Context, id int64) (*mysqlInfra.ProductStockDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.FindByID(ctx, id)
}

func (s *ProductStockService) GetProductStockByProduct(ctx context.Context, productID int64) ([]mysqlInfra.ProductStockDTO, error) {
	if productID <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.FindByProductID(ctx, productID)
}

func (s *ProductStockService) CreateProductStock(ctx context.Context, input mysqlInfra.CreateProductStockInput) (*mysqlInfra.ProductStockDTO, error) {
	if input.ProductID <= 0 || input.BranchID <= 0 || input.WarehouseID <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Create(ctx, input)
}

func (s *ProductStockService) UpdateProductStock(ctx context.Context, id int64, input mysqlInfra.UpdateProductStockInput) (*mysqlInfra.ProductStockDTO, error) {
	if id <= 0 || input.UpdatedBy <= 0 {
		return nil, ErrInvalidInventoryPayload
	}
	return s.repository.Update(ctx, id, input)
}

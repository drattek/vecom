package stock_movements

import (
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidStockMovement = errors.New("invalid stock movement")
)

type StockMovementsService struct {
	repository *mysqlInfra.StockMovementsRepository
}

func NewStockMovementsService(repository *mysqlInfra.StockMovementsRepository) *StockMovementsService {
	return &StockMovementsService{repository: repository}
}

func (s *StockMovementsService) GetPaginatedMovements(offset, pageSize int) (*mysqlInfra.PaginatedStockMovements, error) {
	return s.repository.FindPaginated(offset, pageSize)
}

func (s *StockMovementsService) GetMovementByID(id int64) (*mysqlInfra.StockMovementDTO, error) {
	return s.repository.FindByID(id)
}

func (s *StockMovementsService) GetMovementsByProduct(productID int64, offset, pageSize int) (*mysqlInfra.PaginatedStockMovements, error) {
	return s.repository.FindByProductID(productID, offset, pageSize)
}

func (s *StockMovementsService) CreateMovement(input mysqlInfra.CreateStockMovementInput) (*mysqlInfra.StockMovementDTO, error) {
	if input.ProductID == 0 || input.BranchID == 0 || input.WarehouseID == 0 {
		return nil, ErrInvalidStockMovement
	}

	return s.repository.Create(input)
}

func (s *StockMovementsService) DeleteMovement(id int64) error {
	return s.repository.SoftDelete(id)
}

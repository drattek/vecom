package stock_movements

import (
	"context"
	"database/sql"
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidStockMovement = errors.New("invalid stock movement")
)

type StockMovementsService struct {
	db         *sql.DB
	repository *mysqlInfra.StockMovementsRepository
}

func NewStockMovementsService(db *sql.DB, repository *mysqlInfra.StockMovementsRepository) *StockMovementsService {
	return &StockMovementsService{db: db, repository: repository}
}

func (s *StockMovementsService) GetPaginatedMovements(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedStockMovements, error) {
	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *StockMovementsService) GetMovementByID(ctx context.Context, id int64) (*mysqlInfra.StockMovementDTO, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *StockMovementsService) GetMovementsByProduct(ctx context.Context, productID int64, offset, pageSize int) (*mysqlInfra.PaginatedStockMovements, error) {
	return s.repository.FindByProductID(ctx, productID, offset, pageSize)
}

func (s *StockMovementsService) CreateMovement(ctx context.Context, input mysqlInfra.CreateStockMovementInput) (*mysqlInfra.StockMovementDTO, error) {
	if input.ProductID == 0 || input.BranchID == 0 || input.WarehouseID == 0 {
		return nil, ErrInvalidStockMovement
	}

	return s.repository.Create(ctx, input)
}

func (s *StockMovementsService) DeleteMovement(ctx context.Context, id int64) error {
	return s.repository.SoftDelete(ctx, id)
}

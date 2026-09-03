package pricing

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrEmptyPricingFormulaExpression = errors.New("expression is required")
	ErrPricingFormulaSlotTaken       = errors.New("a pricing formula already exists for this brand/connection/price list combination")
)

type PricingFormulaService struct {
	db         *sql.DB
	repository *mysqlInfra.PricingFormulaRepository
}

func NewPricingFormulaService(db *sql.DB, repository *mysqlInfra.PricingFormulaRepository) *PricingFormulaService {
	return &PricingFormulaService{db: db, repository: repository}
}

func (s *PricingFormulaService) GetPaginatedPricingFormulas(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedPricingFormulas, error) {
	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *PricingFormulaService) GetPricingFormulaByID(ctx context.Context, id int64) (*mysqlInfra.PricingFormulaDTO, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *PricingFormulaService) CreatePricingFormula(ctx context.Context, input mysqlInfra.CreatePricingFormulaInput) (*mysqlInfra.PricingFormulaDTO, error) {
	if strings.TrimSpace(input.Expression) == "" {
		return nil, ErrEmptyPricingFormulaExpression
	}
	if _, err := compilePricingFormula(input.Expression); err != nil {
		return nil, err
	}

	if err := s.ensureNoExistingFormula(ctx, input.BrandID, input.ConnectionID, input.PriceListID); err != nil {
		return nil, err
	}

	return s.repository.Create(ctx, input)
}

func (s *PricingFormulaService) UpdatePricingFormula(ctx context.Context, id int64, input mysqlInfra.UpdatePricingFormulaInput) (*mysqlInfra.PricingFormulaDTO, error) {
	if strings.TrimSpace(input.Expression) == "" {
		return nil, ErrEmptyPricingFormulaExpression
	}
	if _, err := compilePricingFormula(input.Expression); err != nil {
		return nil, err
	}

	return s.repository.Update(ctx, id, input)
}

func (s *PricingFormulaService) DeletePricingFormula(ctx context.Context, id int64) error {
	return s.repository.SoftDelete(ctx, id)
}

// ensureNoExistingFormula rejects creating a second formula for the same
// (brandID, connectionID, priceListID) slot — MySQL treats every NULL as
// distinct inside a unique key (even a composite one), so this slot's
// uniqueness, including the all-NULL universal-default slot, only exists
// at this layer.
func (s *PricingFormulaService) ensureNoExistingFormula(ctx context.Context, brandID *int64, connectionID *int64, priceListID *int64) error {
	if _, err := s.repository.FindBySlot(ctx, brandID, connectionID, priceListID); err == nil {
		return ErrPricingFormulaSlotTaken
	} else if !errors.Is(err, mysqlInfra.ErrPricingFormulaNotFound) {
		return err
	}
	return nil
}

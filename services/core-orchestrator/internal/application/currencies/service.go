package currencies

import (
	"context"
	"database/sql"
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidCurrencyPayload = errors.New("invalid currency payload")

type CurrencyService struct {
	db         *sql.DB
	repository *mysqlInfra.CurrenciesRepository
}

func NewCurrencyService(db *sql.DB, repository *mysqlInfra.CurrenciesRepository) *CurrencyService {
	return &CurrencyService{db: db, repository: repository}
}

func (s *CurrencyService) GetPaginatedCurrencies(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedCurrencies, error) {
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

func (s *CurrencyService) GetCurrencyByID(ctx context.Context, id int64) (*mysqlInfra.CurrencyDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidCurrencyPayload
	}
	return s.repository.FindByID(ctx, id)
}

func (s *CurrencyService) GetCurrencyByCode(ctx context.Context, code string) (*mysqlInfra.CurrencyDTO, error) {
	if code == "" {
		return nil, ErrInvalidCurrencyPayload
	}
	return s.repository.FindByCode(ctx, code)
}

func (s *CurrencyService) CreateCurrency(ctx context.Context, input mysqlInfra.CreateCurrencyInput) (*mysqlInfra.CurrencyDTO, error) {
	if input.Name == "" || input.Code == "" || input.Symbol == "" {
		return nil, ErrInvalidCurrencyPayload
	}
	if input.DecimalPlaces < 0 || input.DecimalPlaces > 8 {
		return nil, ErrInvalidCurrencyPayload
	}
	if input.CreatedBy <= 0 {
		return nil, ErrInvalidCurrencyPayload
	}

	return s.repository.Create(ctx, input)
}

func (s *CurrencyService) UpdateCurrency(ctx context.Context, id int64, input mysqlInfra.UpdateCurrencyInput) (*mysqlInfra.CurrencyDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidCurrencyPayload
	}
	if input.Name == "" || input.Code == "" || input.Symbol == "" {
		return nil, ErrInvalidCurrencyPayload
	}
	if input.DecimalPlaces < 0 || input.DecimalPlaces > 8 {
		return nil, ErrInvalidCurrencyPayload
	}
	if input.UpdatedBy <= 0 {
		return nil, ErrInvalidCurrencyPayload
	}

	return s.repository.Update(ctx, id, input)
}

func (s *CurrencyService) DeleteCurrency(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidCurrencyPayload
	}
	return s.repository.SoftDelete(ctx, id)
}

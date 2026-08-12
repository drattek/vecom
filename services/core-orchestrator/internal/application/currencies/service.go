package currencies

import (
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidCurrencyPayload = errors.New("invalid currency payload")

type CurrencyService struct {
	repository *mysqlInfra.CurrenciesRepository
}

func NewCurrencyService(repository *mysqlInfra.CurrenciesRepository) *CurrencyService {
	return &CurrencyService{repository: repository}
}

func (s *CurrencyService) GetPaginatedCurrencies(offset, pageSize int) (*mysqlInfra.PaginatedCurrencies, error) {
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

func (s *CurrencyService) GetCurrencyByID(id int64) (*mysqlInfra.CurrencyDTO, error) {
	if id <= 0 {
		return nil, ErrInvalidCurrencyPayload
	}
	return s.repository.FindByID(id)
}

func (s *CurrencyService) GetCurrencyByCode(code string) (*mysqlInfra.CurrencyDTO, error) {
	if code == "" {
		return nil, ErrInvalidCurrencyPayload
	}
	return s.repository.FindByCode(code)
}

func (s *CurrencyService) CreateCurrency(input mysqlInfra.CreateCurrencyInput) (*mysqlInfra.CurrencyDTO, error) {
	if input.Name == "" || input.Code == "" || input.Symbol == "" {
		return nil, ErrInvalidCurrencyPayload
	}
	if input.DecimalPlaces < 0 || input.DecimalPlaces > 8 {
		return nil, ErrInvalidCurrencyPayload
	}
	if input.CreatedBy <= 0 {
		return nil, ErrInvalidCurrencyPayload
	}

	return s.repository.Create(input)
}

func (s *CurrencyService) UpdateCurrency(id int64, input mysqlInfra.UpdateCurrencyInput) (*mysqlInfra.CurrencyDTO, error) {
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

	return s.repository.Update(id, input)
}

func (s *CurrencyService) DeleteCurrency(id int64) error {
	if id <= 0 {
		return ErrInvalidCurrencyPayload
	}
	return s.repository.SoftDelete(id)
}

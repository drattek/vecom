package pricing

import (
	"errors"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidPricingPayload = errors.New("invalid pricing payload")
)

type PriceListService struct {
	repository *mysqlInfra.PriceListRepository
}

func NewPriceListService(repository *mysqlInfra.PriceListRepository) *PriceListService {
	return &PriceListService{repository: repository}
}

func (s *PriceListService) GetPaginatedPriceLists(offset, pageSize int) (*mysqlInfra.PaginatedPriceLists, error) {
	return s.repository.FindPaginated(offset, pageSize)
}

func (s *PriceListService) GetPriceListByID(id int64) (*mysqlInfra.PriceListDTO, error) {
	return s.repository.FindByID(id)
}

func (s *PriceListService) CreatePriceList(input mysqlInfra.CreatePriceListInput) (*mysqlInfra.PriceListDTO, error) {
	if input.Name == "" {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Create(input)
}

func (s *PriceListService) UpdatePriceList(id int64, input mysqlInfra.UpdatePriceListInput) (*mysqlInfra.PriceListDTO, error) {
	if input.Name == "" {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Update(id, input)
}

func (s *PriceListService) DeletePriceList(id int64) error {
	return s.repository.SoftDelete(id)
}

type ProductPricesService struct {
	repository *mysqlInfra.ProductPricesRepository
}

func NewProductPricesService(repository *mysqlInfra.ProductPricesRepository) *ProductPricesService {
	return &ProductPricesService{repository: repository}
}

func (s *ProductPricesService) GetPaginatedProductPrices(offset, pageSize int) (*mysqlInfra.PaginatedProductPrices, error) {
	return s.repository.FindPaginated(offset, pageSize)
}

func (s *ProductPricesService) GetProductPriceByID(id int64) (*mysqlInfra.ProductPriceDTO, error) {
	return s.repository.FindByID(id)
}

func (s *ProductPricesService) GetPricesByProduct(productID int64) ([]mysqlInfra.ProductPriceDTO, error) {
	return s.repository.FindByProductID(productID)
}

func (s *ProductPricesService) CreateProductPrice(input mysqlInfra.CreateProductPriceInput) (*mysqlInfra.ProductPriceDTO, error) {
	if input.ProductID == 0 || input.PriceListID == 0 {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Create(input)
}

func (s *ProductPricesService) UpdateProductPrice(id int64, input mysqlInfra.UpdateProductPriceInput) (*mysqlInfra.ProductPriceDTO, error) {
	if input.Price == "" {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Update(id, input)
}

func (s *ProductPricesService) DeleteProductPrice(id int64) error {
	return s.repository.Delete(id)
}

type ExchangeRatesService struct {
	repository *mysqlInfra.ExchangeRatesRepository
}

func NewExchangeRatesService(repository *mysqlInfra.ExchangeRatesRepository) *ExchangeRatesService {
	return &ExchangeRatesService{repository: repository}
}

func (s *ExchangeRatesService) GetPaginatedExchangeRates(offset, pageSize int) (*mysqlInfra.PaginatedExchangeRates, error) {
	return s.repository.FindPaginated(offset, pageSize)
}

func (s *ExchangeRatesService) GetExchangeRateByID(id int64) (*mysqlInfra.ExchangeRateDTO, error) {
	return s.repository.FindByID(id)
}

func (s *ExchangeRatesService) CreateExchangeRate(input mysqlInfra.CreateExchangeRateInput) (*mysqlInfra.ExchangeRateDTO, error) {
	if input.FromCurrencyID == 0 || input.ToCurrencyID == 0 || input.Rate == "" {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Create(input)
}

func (s *ExchangeRatesService) UpdateExchangeRate(id int64, input mysqlInfra.UpdateExchangeRateInput) (*mysqlInfra.ExchangeRateDTO, error) {
	if input.Rate == "" {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Update(id, input)
}

func (s *ExchangeRatesService) DeleteExchangeRate(id int64) error {
	return s.repository.SoftDelete(id)
}

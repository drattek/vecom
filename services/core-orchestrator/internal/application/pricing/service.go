package pricing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidPricingPayload = errors.New("invalid pricing payload")
)

type PriceListService struct {
	db         *sql.DB
	repository *mysqlInfra.PriceListRepository
}

func NewPriceListService(db *sql.DB, repository *mysqlInfra.PriceListRepository) *PriceListService {
	return &PriceListService{db: db, repository: repository}
}

func (s *PriceListService) GetPaginatedPriceLists(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedPriceLists, error) {
	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *PriceListService) GetPriceListByID(ctx context.Context, id int64) (*mysqlInfra.PriceListDTO, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *PriceListService) CreatePriceList(ctx context.Context, input mysqlInfra.CreatePriceListInput) (*mysqlInfra.PriceListDTO, error) {
	if input.Name == "" {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Create(ctx, input)
}

func (s *PriceListService) UpdatePriceList(ctx context.Context, id int64, input mysqlInfra.UpdatePriceListInput) (*mysqlInfra.PriceListDTO, error) {
	if input.Name == "" {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Update(ctx, id, input)
}

func (s *PriceListService) DeletePriceList(ctx context.Context, id int64) error {
	return s.repository.SoftDelete(ctx, id)
}

type ProductPricesService struct {
	db                  *sql.DB
	repository          *mysqlInfra.ProductPricesRepository
	productRepository   *mysqlInfra.ProductRepository
	priceListRepository *mysqlInfra.PriceListRepository
}

func NewProductPricesService(
	db *sql.DB,
	repository *mysqlInfra.ProductPricesRepository,
	productRepository *mysqlInfra.ProductRepository,
	priceListRepository *mysqlInfra.PriceListRepository,
) *ProductPricesService {
	return &ProductPricesService{
		db:                  db,
		repository:          repository,
		productRepository:   productRepository,
		priceListRepository: priceListRepository,
	}
}

func (s *ProductPricesService) GetPaginatedProductPrices(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedProductPrices, error) {
	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *ProductPricesService) GetProductPriceByID(ctx context.Context, id int64) (*mysqlInfra.ProductPriceDTO, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *ProductPricesService) GetPricesByProduct(ctx context.Context, productID int64) ([]mysqlInfra.ProductPriceDTO, error) {
	return s.repository.FindByProductID(ctx, productID)
}

func (s *ProductPricesService) CreateProductPrice(ctx context.Context, input mysqlInfra.CreateProductPriceInput) (*mysqlInfra.ProductPriceDTO, error) {
	if input.ProductID == 0 || input.PriceListID == 0 {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Create(ctx, input)
}

func (s *ProductPricesService) UpdateProductPrice(ctx context.Context, id int64, input mysqlInfra.UpdateProductPriceInput) (*mysqlInfra.ProductPriceDTO, error) {
	if input.Price == "" {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Update(ctx, id, input)
}

func (s *ProductPricesService) DeleteProductPrice(ctx context.Context, id int64) error {
	return s.repository.Delete(ctx, id)
}

type BulkPriceItem struct {
	SKU         string
	Price       string
	TaxIncluded bool
	Margin      *float64
}

type BulkPriceStatus string

const (
	BulkPriceCreated  BulkPriceStatus = "created"
	BulkPriceUpdated  BulkPriceStatus = "updated"
	BulkPriceNotFound BulkPriceStatus = "not_found"
	BulkPriceError    BulkPriceStatus = "error"
)

type BulkPriceResult struct {
	Index     int                         `json:"index"`
	SKU       string                      `json:"sku"`
	Status    BulkPriceStatus             `json:"status"`
	ProductID int64                       `json:"productId,omitempty"`
	Price     *mysqlInfra.ProductPriceDTO `json:"price,omitempty"`
	Error     string                      `json:"error,omitempty"`
}

// BulkUpsertPrices resolves each item's product by SKU and creates or
// updates its price row for priceListID. Currency always comes from the
// price list itself, never from the item — every price in a list shares one
// currency. Margin defaults to 0 and taxIncluded to false when omitted.
func (s *ProductPricesService) BulkUpsertPrices(ctx context.Context, priceListID int64, items []BulkPriceItem, actorID int64) ([]BulkPriceResult, error) {
	priceList, err := s.priceListRepository.FindByID(ctx, priceListID)
	if err != nil {
		return nil, err
	}

	results := make([]BulkPriceResult, 0, len(items))

	for i, item := range items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" {
			results = append(results, BulkPriceResult{Index: i, SKU: sku, Status: BulkPriceError, Error: "sku is required"})
			continue
		}
		if strings.TrimSpace(item.Price) == "" {
			results = append(results, BulkPriceResult{Index: i, SKU: sku, Status: BulkPriceError, Error: "price is required"})
			continue
		}

		product, err := s.productRepository.FindBySKU(ctx, sku)
		if err != nil {
			if errors.Is(err, mysqlInfra.ErrProductNotFound) {
				results = append(results, BulkPriceResult{Index: i, SKU: sku, Status: BulkPriceNotFound, Error: "product not found for sku"})
				continue
			}
			results = append(results, BulkPriceResult{Index: i, SKU: sku, Status: BulkPriceError, Error: err.Error()})
			continue
		}

		margin := 0.0
		if item.Margin != nil {
			margin = *item.Margin
		}
		marginStr := fmt.Sprintf("%.2f", margin)

		existing, err := s.repository.FindByProductAndPriceList(ctx, product.ID, priceListID)
		if err != nil {
			if !errors.Is(err, mysqlInfra.ErrProductPriceNotFound) {
				results = append(results, BulkPriceResult{Index: i, SKU: sku, ProductID: product.ID, Status: BulkPriceError, Error: err.Error()})
				continue
			}

			created, createErr := s.repository.Create(ctx, mysqlInfra.CreateProductPriceInput{
				ProductID:   product.ID,
				PriceListID: priceListID,
				Price:       item.Price,
				Currency:    priceList.Currency,
				Margin:      marginStr,
				TaxIncluded: item.TaxIncluded,
				UpdatedBy:   actorID,
			})
			if createErr != nil {
				results = append(results, BulkPriceResult{Index: i, SKU: sku, ProductID: product.ID, Status: BulkPriceError, Error: createErr.Error()})
				continue
			}
			results = append(results, BulkPriceResult{Index: i, SKU: sku, ProductID: product.ID, Status: BulkPriceCreated, Price: created})
			continue
		}

		updated, updateErr := s.repository.Update(ctx, existing.ID, mysqlInfra.UpdateProductPriceInput{
			Price:       item.Price,
			Margin:      marginStr,
			TaxIncluded: item.TaxIncluded,
			UpdatedBy:   actorID,
		})
		if updateErr != nil {
			results = append(results, BulkPriceResult{Index: i, SKU: sku, ProductID: product.ID, Status: BulkPriceError, Error: updateErr.Error()})
			continue
		}
		results = append(results, BulkPriceResult{Index: i, SKU: sku, ProductID: product.ID, Status: BulkPriceUpdated, Price: updated})
	}

	return results, nil
}

type ExchangeRatesService struct {
	db         *sql.DB
	repository *mysqlInfra.ExchangeRatesRepository
}

func NewExchangeRatesService(db *sql.DB, repository *mysqlInfra.ExchangeRatesRepository) *ExchangeRatesService {
	return &ExchangeRatesService{db: db, repository: repository}
}

func (s *ExchangeRatesService) GetPaginatedExchangeRates(ctx context.Context, offset, pageSize int) (*mysqlInfra.PaginatedExchangeRates, error) {
	return s.repository.FindPaginated(ctx, offset, pageSize)
}

func (s *ExchangeRatesService) GetExchangeRateByID(ctx context.Context, id int64) (*mysqlInfra.ExchangeRateDTO, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *ExchangeRatesService) CreateExchangeRate(ctx context.Context, input mysqlInfra.CreateExchangeRateInput) (*mysqlInfra.ExchangeRateDTO, error) {
	if input.FromCurrencyID == 0 || input.ToCurrencyID == 0 || input.Rate == "" {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Create(ctx, input)
}

func (s *ExchangeRatesService) UpdateExchangeRate(ctx context.Context, id int64, input mysqlInfra.UpdateExchangeRateInput) (*mysqlInfra.ExchangeRateDTO, error) {
	if input.Rate == "" {
		return nil, ErrInvalidPricingPayload
	}

	return s.repository.Update(ctx, id, input)
}

func (s *ExchangeRatesService) DeleteExchangeRate(ctx context.Context, id int64) error {
	return s.repository.SoftDelete(ctx, id)
}

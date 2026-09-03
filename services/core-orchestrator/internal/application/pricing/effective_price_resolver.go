package pricing

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrNoExchangeRateConfigured = errors.New("no exchange rate configured for currency conversion")

// EffectivePriceResolver resolves a product's effective price (highest-
// priority active/valid price list, any currency — see
// ProductPricesRepository.FindEffectivePrice) and converts it into a target
// currency via ecom_exchange_rates when the winning list isn't already in
// that currency. It is the single place both MercadoLibreProductSyncService
// and OdooProductSyncService go through to get the MXN base price they feed
// into PricingFormulaCalculator — every marketplace channel receives prices
// in MXN, but price lists themselves may be denominated in any currency
// (e.g. USD).
type EffectivePriceResolver struct {
	pricesRepository        *mysqlInfra.ProductPricesRepository
	exchangeRatesRepository *mysqlInfra.ExchangeRatesRepository
}

func NewEffectivePriceResolver(
	pricesRepository *mysqlInfra.ProductPricesRepository,
	exchangeRatesRepository *mysqlInfra.ExchangeRatesRepository,
) *EffectivePriceResolver {
	return &EffectivePriceResolver{
		pricesRepository:        pricesRepository,
		exchangeRatesRepository: exchangeRatesRepository,
	}
}

// ResolveInCurrency returns productID's effective price converted into
// targetCurrencyID, along with the UpdatedAt of the underlying price row
// (callers use it, unconverted, to detect whether the price changed since a
// prior sync — see priceOrStockChangedSince in the sync package) and the id
// of the price list that won (callers feed it into
// PricingFormulaCalculator.CalculatePrice, letting a formula be scoped to a
// specific price list).
func (r *EffectivePriceResolver) ResolveInCurrency(ctx context.Context, productID, targetCurrencyID int64) (basePrice float64, updatedAt time.Time, priceListID int64, err error) {
	price, err := r.pricesRepository.FindEffectivePrice(ctx, productID)
	if err != nil {
		return 0, time.Time{}, 0, err
	}

	amount, err := strconv.ParseFloat(price.Price, 64)
	if err != nil {
		return 0, time.Time{}, 0, fmt.Errorf("error parsing price %q for product %d: %w", price.Price, productID, err)
	}

	if price.Currency == targetCurrencyID {
		return amount, price.UpdatedAt, price.PriceListID, nil
	}

	rate, err := r.exchangeRatesRepository.FindByCurrencies(ctx, price.Currency, targetCurrencyID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrExchangeRateNotFound) {
			return 0, time.Time{}, 0, fmt.Errorf("%w: from currency %d to %d for product %d", ErrNoExchangeRateConfigured, price.Currency, targetCurrencyID, productID)
		}
		return 0, time.Time{}, 0, fmt.Errorf("error loading exchange rate from currency %d to %d: %w", price.Currency, targetCurrencyID, err)
	}

	rateValue, err := strconv.ParseFloat(rate.Rate, 64)
	if err != nil {
		return 0, time.Time{}, 0, fmt.Errorf("error parsing exchange rate %q (currency %d to %d): %w", rate.Rate, price.Currency, targetCurrencyID, err)
	}

	return amount * rateValue, price.UpdatedAt, price.PriceListID, nil
}

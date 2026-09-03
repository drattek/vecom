package pricing

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	banxicoInfra "core-orchestrator/internal/infrastructure/banxico"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// systemSIEExchangeRateActorID is recorded as updated_by for the exchange
// rate row this updater writes, since the daily scheduler isn't threaded
// through an authenticated user (mirrors application/sync's
// systemMercadoLibreSyncActorID / systemOdooSyncActorID).
const systemSIEExchangeRateActorID int64 = 1

// sieUSDMXNSeriesID is Banxico's "tipo de cambio FIX" series — the
// mid-market reference rate Banxico publishes each business day around
// 12:00-13:00 hrs CDMX, dated the same day. See
// https://www.banxico.org.mx/SieAPIRest/service/v1/doc/index.html
const sieUSDMXNSeriesID = "SF43718"

// SIEExchangeRateUpdater keeps ecom_exchange_rates' USD→MXN row in sync
// with Banco de México's published FIX rate. It writes only that one row —
// there is exactly one USD→MXN rate, applied by
// EffectivePriceResolver.ResolveInCurrency (multiplied straight through,
// never inverted) whenever a product's effective price comes from a
// USD-denominated price list but the marketplace sync target needs MXN.
type SIEExchangeRateUpdater struct {
	sieClient               *banxicoInfra.Client
	currenciesRepository    *mysqlInfra.CurrenciesRepository
	exchangeRatesRepository *mysqlInfra.ExchangeRatesRepository
}

func NewSIEExchangeRateUpdater(
	sieClient *banxicoInfra.Client,
	currenciesRepository *mysqlInfra.CurrenciesRepository,
	exchangeRatesRepository *mysqlInfra.ExchangeRatesRepository,
) *SIEExchangeRateUpdater {
	return &SIEExchangeRateUpdater{
		sieClient:               sieClient,
		currenciesRepository:    currenciesRepository,
		exchangeRatesRepository: exchangeRatesRepository,
	}
}

// UpdateUSDToMXN fetches Banxico's latest published FIX rate and
// creates/updates ecom_exchange_rates' USD→MXN row to match, returning the
// applied rate and the date Banxico reports it as of (so the caller can log
// staleness — e.g. Banxico hasn't published yet for today).
func (u *SIEExchangeRateUpdater) UpdateUSDToMXN(ctx context.Context) (rate float64, asOf time.Time, err error) {
	usd, err := u.currenciesRepository.FindByCode(ctx, "USD")
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("error loading USD currency: %w", err)
	}
	mxn, err := u.currenciesRepository.FindByCode(ctx, "MXN")
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("error loading MXN currency: %w", err)
	}

	rate, asOf, err = u.sieClient.GetLatestValue(ctx, sieUSDMXNSeriesID)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("error fetching SIE USD/MXN rate: %w", err)
	}

	rateStr := strconv.FormatFloat(rate, 'f', 4, 64)

	existing, err := u.exchangeRatesRepository.FindByCurrencies(ctx, usd.ID, mxn.ID)
	if err != nil {
		if !errors.Is(err, mysqlInfra.ErrExchangeRateNotFound) {
			return 0, time.Time{}, fmt.Errorf("error loading existing USD/MXN exchange rate: %w", err)
		}

		if _, createErr := u.exchangeRatesRepository.Create(ctx, mysqlInfra.CreateExchangeRateInput{
			FromCurrencyID: usd.ID,
			ToCurrencyID:   mxn.ID,
			Rate:           rateStr,
			UpdatedBy:      systemSIEExchangeRateActorID,
		}); createErr != nil {
			return 0, time.Time{}, fmt.Errorf("error creating USD/MXN exchange rate: %w", createErr)
		}
		return rate, asOf, nil
	}

	if _, updateErr := u.exchangeRatesRepository.Update(ctx, existing.ID, mysqlInfra.UpdateExchangeRateInput{
		Rate:      rateStr,
		UpdatedBy: systemSIEExchangeRateActorID,
	}); updateErr != nil {
		return 0, time.Time{}, fmt.Errorf("error updating USD/MXN exchange rate: %w", updateErr)
	}

	return rate, asOf, nil
}

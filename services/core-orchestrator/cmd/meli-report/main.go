// Comando meli-report: compara las publicaciones de MercadoLibre con la base
// local y deja un CSV con las diferencias. Es de solo lectura:
//
//   - la conexión a MySQL se abre con transaction_read_only, así que el servidor
//     rechaza cualquier escritura;
//   - el cliente HTTP solo permite GET;
//   - usa el token guardado y nunca lo renueva.
//
// Uso, desde services/core-orchestrator:
//
//	go run ./cmd/meli-report [-connection N] [-rate 60] [-out archivo.csv] [-env ../../.env]
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/joho/godotenv"

	meliReportApp "core-orchestrator/internal/application/meli_report"
	pricingApp "core-orchestrator/internal/application/pricing"
	syncApp "core-orchestrator/internal/application/sync"
	"core-orchestrator/internal/config"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// getOnly rechaza cualquier petición que pudiera modificar algo.
type getOnly struct{ next http.RoundTripper }

func (t getOnly) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Method != http.MethodGet {
		return nil, fmt.Errorf("read-only report: %s requests are not allowed", request.Method)
	}
	return t.next.RoundTrip(request)
}

func main() {
	envFile := flag.String("env", "../../.env", "archivo .env")
	connectionID := flag.Int64("connection", 0, "id de una sola conexión (0 = todas las activas de MercadoLibre)")
	// Por debajo del límite del servidor: ambos comparten la cuota de la app.
	rate := flag.Int("rate", 60, "consultas por minuto a MercadoLibre")
	output := flag.String("out", "meli-diferencias-"+time.Now().Format("20060102-1504")+".csv", "archivo CSV de salida")
	flag.Parse()

	if err := run(*envFile, *connectionID, *rate, *output); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(envFile string, connectionID int64, rate int, output string) error {
	if rate <= 0 {
		return errors.New("-rate debe ser mayor que 0")
	}
	if err := godotenv.Load(envFile); err != nil {
		fmt.Fprintf(os.Stderr, "Aviso: no se cargó %s (%v); se usan las variables de entorno.\n", envFile, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg := config.Load()
	// El cliente de MercadoLibre registra cada petición; aquí solo se muestra el
	// avance.
	log.SetOutput(io.Discard)

	db, err := mysqlInfra.OpenReadOnlyConnection(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	currencies := mysqlInfra.NewCurrenciesRepository(db)
	mxn, err := currencies.FindByCode(ctx, "MXN")
	if err != nil {
		return fmt.Errorf("no se encontró la divisa MXN: %w", err)
	}
	resolver := pricingApp.NewEffectivePriceResolver(
		mysqlInfra.NewProductPricesRepository(db), mysqlInfra.NewExchangeRatesRepository(db))
	calculator := pricingApp.NewPricingFormulaCalculator(mysqlInfra.NewPricingFormulaRepository(db))

	fmt.Fprintln(os.Stderr, "Leyendo la base (puede tardar con muchas publicaciones)...")
	listings, err := meliReportApp.LoadListings(ctx, db, connectionID,
		func(ctx context.Context, listing meliReportApp.Listing) (float64, sql.NullTime, error) {
			base, changedAt, priceListID, err := resolver.ResolveInCurrency(ctx, listing.ProductID, mxn.ID)
			if err != nil {
				return 0, sql.NullTime{}, err
			}
			if base <= 0 {
				return 0, sql.NullTime{}, errors.New("sin precio base")
			}
			price, err := calculator.CalculatePrice(ctx, listing.BrandID, listing.ConnectionID, priceListID, base)
			if err != nil {
				return 0, sql.NullTime{}, err
			}
			return syncApp.MercadoLibreListingPrice(listing.SKU, price), sql.NullTime{Time: changedAt, Valid: true}, nil
		})
	if err != nil {
		return err
	}
	if len(listings) == 0 {
		return errors.New("no hay publicaciones de MercadoLibre que comparar")
	}
	fmt.Fprintf(os.Stderr, "%d publicaciones por comparar.\n", len(listings))

	httpClient := &http.Client{Timeout: 30 * time.Second, Transport: getOnly{http.DefaultTransport}}
	items := mercadoLibreInfra.NewItemsHandler(
		mercadoLibreInfra.NewClient(httpClient, "", mercadoLibreInfra.NewRateLimiter(rate, time.Minute)))
	settings := mysqlInfra.NewConnectionSettingsRepository(db)

	runner := &meliReportApp.Runner{
		Items: items,
		Token: func(ctx context.Context, id int64) (string, time.Time, error) {
			return syncApp.StoredMercadoLibreAccessToken(ctx, settings, id)
		},
		Fold:       syncApp.FoldMercadoLibreItemStatus,
		Progress:   func(message string) { fmt.Fprintln(os.Stderr, message) },
		RetryDelay: 5 * time.Second,
		Now:        time.Now,
	}
	result, err := runner.Run(ctx, listings)
	if err != nil {
		return err
	}

	// O_EXCL: no se sobrescribe un reporte anterior.
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("no se pudo crear %s: %w", output, err)
	}
	if err := meliReportApp.WriteCSV(file, result.Rows); err != nil {
		_ = file.Close()
		return fmt.Errorf("escribiendo %s: %w", output, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("escribiendo %s: %w", output, err)
	}

	counts := map[meliReportApp.Severity]int{}
	for _, row := range result.Rows {
		counts[row.Findings[0].Severity]++
	}
	fmt.Printf("\nComparadas: %d · Con diferencias: %d\n", result.Compared, len(result.Rows))
	for severity := meliReportApp.SeverityUrgent; severity <= meliReportApp.SeverityInfo; severity++ {
		fmt.Printf("  %s: %d\n", severity, counts[severity])
	}
	fmt.Println("Reporte:", output)

	if result.NotQueried > 0 {
		for _, problem := range result.Problems {
			fmt.Println("  ·", problem)
		}
		return fmt.Errorf("REPORTE INCOMPLETO: %d publicaciones no se pudieron consultar", result.NotQueried)
	}
	return nil
}

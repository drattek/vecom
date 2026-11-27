package meli_report

import (
	"bytes"
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	"core-orchestrator/internal/testsupport/sqlscript"
)

type fakeItems struct {
	batches [][]string
	// failing: ids que hacen fallar su lote.
	failing map[string]bool
}

func (f *fakeItems) GetItemsStatus(_ context.Context, _ string, ids []string) ([]mercadoLibreInfra.ItemStatus, error) {
	f.batches = append(f.batches, ids)
	var statuses []mercadoLibreInfra.ItemStatus
	for _, id := range ids {
		if f.failing[id] {
			return nil, errors.New("mercadolibre 500")
		}
		quantity := 5
		statuses = append(statuses, mercadoLibreInfra.ItemStatus{ID: id, Status: "active", AvailableQuantity: &quantity})
	}
	return statuses, nil
}

func newRunner(items ItemSource, expiresAt time.Time) *Runner {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	return &Runner{
		Items: items,
		Token: func(context.Context, int64) (string, time.Time, error) { return "token", expiresAt, nil },
		Fold:  func(string, []string) string { return "synced" },
		Now:   func() time.Time { return now },
	}
}

var validUntil = time.Date(2026, 11, 26, 18, 0, 0, 0, time.UTC)

func manyListings(connectionID int64, prefix string, count, stock int) []Listing {
	listings := make([]Listing, 0, count)
	for i := range count {
		listings = append(listings, Listing{
			ConnectionID: connectionID, ConnectionName: prefix, SKU: fmt.Sprintf("%s-%03d", prefix, i),
			ExternalID: fmt.Sprintf("%s%d", prefix, i), Status: "synced", Stock: stock,
		})
	}
	return listings
}

func TestRunQueriesInBatchesAndSortsByUrgency(t *testing.T) {
	items := &fakeItems{}
	listings := append(manyListings(1, "MLA", 25, 9), manyListings(2, "MLB", 1, 0)...)
	// Dos filas locales de la misma publicación se consultan una sola vez.
	listings = append(listings, listings[len(listings)-1])

	result, err := newRunner(items, validUntil).Run(context.Background(), listings)
	if err != nil {
		t.Fatal(err)
	}

	if len(items.batches) != 3 || len(items.batches[0]) != 20 || len(items.batches[2]) != 1 {
		t.Fatalf("batches = %d", len(items.batches))
	}
	if result.Compared != 27 || result.NotQueried != 0 || len(result.Rows) != 27 {
		t.Fatalf("compared=%d notQueried=%d rows=%d", result.Compared, result.NotQueried, len(result.Rows))
	}
	if result.Rows[0].Findings[0].Severity != SeverityUrgent || result.Rows[0].Listing.ConnectionID != 2 {
		t.Errorf("the urgent rows must come first: %+v", result.Rows[0])
	}
}

func TestRunReportsWhatCouldNotBeQueried(t *testing.T) {
	items := &fakeItems{failing: map[string]bool{"MLA0": true}}

	result, err := newRunner(items, validUntil).Run(context.Background(), manyListings(1, "MLA", 25, 5))
	if err != nil {
		t.Fatal(err)
	}

	// El primer lote falla (con reintentos); el segundo se consulta.
	if result.NotQueried != 20 || len(items.batches) != attempts+1 {
		t.Fatalf("notQueried=%d batches=%d", result.NotQueried, len(items.batches))
	}
	if len(result.Problems) != 1 || len(result.Rows) != 20 {
		t.Errorf("problems=%v rows=%d: an unqueried listing is not a match", result.Problems, len(result.Rows))
	}
}

func TestRunDoesNotUseAnExpiredToken(t *testing.T) {
	items := &fakeItems{}
	expired := time.Date(2026, 11, 26, 12, 0, 30, 0, time.UTC)

	result, err := newRunner(items, expired).Run(context.Background(), manyListings(1, "MLA", 3, 5))
	if err != nil {
		t.Fatal(err)
	}

	if len(items.batches) != 0 || result.NotQueried != 3 {
		t.Fatalf("batches=%d notQueried=%d", len(items.batches), result.NotQueried)
	}
	if !strings.Contains(result.Problems[0], "no lo renueva") {
		t.Errorf("problem = %q", result.Problems[0])
	}
}

func TestWriteCSVNeutralizesFormulas(t *testing.T) {
	var out bytes.Buffer
	err := WriteCSV(&out, []Row{{
		Listing:  Listing{SKU: "=HYPERLINK(1)", ConnectionName: "ML 1"},
		Findings: []Finding{{SeverityUrgent, "problema", "acción"}, {SeverityHigh, "otro", ""}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"\xEF\xBB\xBF", "'=HYPERLINK(1)", "1-URGENTE", "otro"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %q", want, text)
		}
	}
}

func TestLoadListingsOnlyReads(t *testing.T) {
	now := time.Now()
	database := &sqlscript.DB{Rows: map[string][][]driver.Value{
		"FROM ecom_channel_product_map m": {{
			int64(7), int64(1), "ML 1", int64(10), nil, "A", "Filtro", "MLM1", "paused", nil, now, int64(4), now,
		}},
		"FROM ecom_channel_listing_events": {{int64(7)}},
	}}

	listings, err := LoadListings(context.Background(), database.Open(t), 0, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(listings) != 1 || !listings[0].PausedForStockByUs || listings[0].Stock != 4 || listings[0].PausedReason != "" {
		t.Fatalf("listings = %+v", listings)
	}
	if writes := database.Writes(""); len(writes) != 0 {
		t.Errorf("the report must not write: %+v", writes)
	}
}

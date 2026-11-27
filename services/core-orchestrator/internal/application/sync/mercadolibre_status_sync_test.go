package sync

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/testsupport/sqlscript"
)

const mapTable = "ecom_channel_product_map"

type localListing struct {
	status       string
	pausedReason string
	category     string
}

func mapRow(listing localListing) []driver.Value {
	nullable := func(v string) driver.Value {
		if v == "" {
			return nil
		}
		return v
	}
	longAgo := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// En el orden de channelProductMapColumns.
	return []driver.Value{
		int64(1), "Filtro", int64(10), int64(3), nil, int64(1), "MLM1", nullable(listing.category),
		listing.status, nullable(listing.pausedReason), longAgo, "error anterior",
		int64(1), nil, longAgo, longAgo, nil,
	}
}

// newStatusService arma el servicio con una publicación local y un
// MercadoLibre simulado que responde response a todo.
func newStatusService(t *testing.T, local localListing, response string) (*MercadoLibreProductSyncService, *sqlscript.DB) {
	t.Helper()
	now := time.Now().UTC()
	setting := func(id int64, key, value string) []driver.Value {
		return []driver.Value{id, int64(3), key, value, int64(0), int64(1), nil, now, now}
	}
	database := &sqlscript.DB{Rows: map[string][][]driver.Value{
		"FROM ecom_channel_product_map": {mapRow(local)},
		"FROM ecom_connection_settings": {
			setting(1, "access_token", "token"),
			setting(2, "refresh_token", "refresh"),
			setting(3, "expiration_time", now.Add(time.Hour).Format(time.RFC3339)),
			setting(4, "app_id", "1"),
			setting(5, "client_secret", "s"),
		},
	}}
	db := database.Open(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)

	return &MercadoLibreProductSyncService{
		channelProductMapRepository: mysqlInfra.NewChannelProductMapRepository(db),
		productRepository:           mysqlInfra.NewProductRepository(db),
		tokenService:                &MercadoLibreTokenService{settingsRepository: mysqlInfra.NewConnectionSettingsRepository(db)},
		itemsHandler:                mercadoLibreInfra.NewItemsHandler(mercadoLibreInfra.NewClient(server.Client(), server.URL, nil)),
	}, database
}

func multiget(item string) string { return fmt.Sprintf(`[{"code":200,"body":%s}]`, item) }

// runStatusSync ejecuta la sincronización de estado y categoría real.
func runStatusSync(t *testing.T, local localListing, item string) *sqlscript.DB {
	t.Helper()
	service, database := newStatusService(t, local, multiget(item))
	result, err := service.RefreshChannelProductMapStatusAndCategory(context.Background(), 3)
	if err != nil {
		t.Fatalf("status sync: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].Error != "" {
		t.Fatalf("unexpected result: %+v", result.Results)
	}
	return database
}

func singleMapWrite(t *testing.T, database *sqlscript.DB) sqlscript.Write {
	t.Helper()
	writes := database.Writes(mapTable)
	if len(writes) != 1 {
		t.Fatalf("expected one write on the map, got %d", len(writes))
	}
	return writes[0]
}

// assertNothingPushed: last_synced_at es "cuándo se envió precio/stock" y
// Refresh omite lo que no cambió después de esa fecha.
func assertNothingPushed(t *testing.T, write sqlscript.Write) {
	t.Helper()
	for _, column := range []string{"last_synced_at", "last_error"} {
		if write.Sets(column) {
			t.Errorf("must not touch %s: %s", column, write.Query)
		}
	}
}

func TestStatusSyncKeepsPendingPriceAndStockPush(t *testing.T) {
	cases := map[string]struct {
		local localListing
		item  string
	}{
		"listing comes back active": {
			local: localListing{status: "paused", pausedReason: "user"},
			item:  `{"id":"MLM1","status":"active"}`,
		},
		"under review cleared": {
			local: localListing{status: "under_review"},
			item:  `{"id":"MLM1","status":"active"}`,
		},
		"paused on mercadolibre": {
			local: localListing{status: "synced"},
			item:  `{"id":"MLM1","status":"paused","sub_status":["paused_by_seller"]}`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assertNothingPushed(t, singleMapWrite(t, runStatusSync(t, tc.local, tc.item)))
		})
	}
}

func TestStatusOnlySyncKeepsPendingPush(t *testing.T) {
	service, database := newStatusService(t, localListing{status: "under_review", pausedReason: "zero_stock"},
		multiget(`{"id":"MLM1","status":"paused","sub_status":["paused_by_seller"]}`))

	if _, err := service.RefreshChannelProductMapStatus(context.Background(), 3); err != nil {
		t.Fatal(err)
	}

	write := singleMapWrite(t, database)
	assertNothingPushed(t, write)
	if got := write.Column(t, "paused_reason"); got != "zero_stock" {
		t.Errorf("paused_reason = %v", got)
	}
}

// Un envío rechazado no se envió: no debe quedar como sincronizado.
func TestFailedUpdateReconciliationKeepsPendingPush(t *testing.T) {
	service, database := newStatusService(t, localListing{status: "paused", pausedReason: "zero_stock"},
		`{"id":"MLM1","status":"under_review","sub_status":["waiting_for_patch"]}`)
	externalID, reason := "MLM1", "zero_stock"

	service.reconcileStatusAfterFailedUpdate(context.Background(), "token", 10, 3, &mysqlInfra.ChannelProductMapDTO{
		ID: 1, ProductID: 10, ConnectionID: 3, ExternalID: &externalID, Status: "paused", PausedReason: &reason,
	}, "")

	write := singleMapWrite(t, database)
	assertNothingPushed(t, write)
	if got := write.Column(t, "paused_reason"); got != "zero_stock" {
		t.Errorf("paused_reason = %v", got)
	}
}

func TestStatusSyncPauseReason(t *testing.T) {
	cases := []struct {
		name       string
		local      localListing
		item       string
		wantStatus string
		wantReason driver.Value
	}{
		{
			name:  "zero stock pause survives a trip to under review",
			local: localListing{status: "paused", pausedReason: "zero_stock"},
			item:  `{"id":"MLM1","status":"under_review","sub_status":["waiting_for_patch"]}`, wantStatus: "under_review", wantReason: "zero_stock",
		},
		{
			name:  "and back to paused",
			local: localListing{status: "under_review", pausedReason: "zero_stock"},
			item:  `{"id":"MLM1","status":"paused","sub_status":["paused_by_seller"]}`, wantStatus: "paused", wantReason: "zero_stock",
		},
		{
			name:  "seller pause",
			local: localListing{status: "synced"},
			item:  `{"id":"MLM1","status":"paused","sub_status":["paused_by_seller"]}`, wantStatus: "paused", wantReason: "user",
		},
		{
			// MercadoLibre la reactiva solo al recibir stock: sin motivo propio.
			name:  "out of stock pause by mercadolibre",
			local: localListing{status: "synced"},
			item:  `{"id":"MLM1","status":"paused","sub_status":["out_of_stock"]}`, wantStatus: "paused", wantReason: nil,
		},
		{
			name:  "under review without a zero stock pause",
			local: localListing{status: "paused", pausedReason: "user"},
			item:  `{"id":"MLM1","status":"under_review"}`, wantStatus: "under_review", wantReason: nil,
		},
		{
			name:  "active again clears the reason",
			local: localListing{status: "paused", pausedReason: "zero_stock"},
			item:  `{"id":"MLM1","status":"active"}`, wantStatus: "synced", wantReason: nil,
		},
		{
			name:  "closed clears the reason",
			local: localListing{status: "paused", pausedReason: "zero_stock"},
			item:  `{"id":"MLM1","status":"closed"}`, wantStatus: "closed", wantReason: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			write := singleMapWrite(t, runStatusSync(t, tc.local, tc.item))
			if got := write.Column(t, "status"); got != tc.wantStatus {
				t.Errorf("status = %v, want %v", got, tc.wantStatus)
			}
			if got := write.Column(t, "paused_reason"); got != tc.wantReason {
				t.Errorf("paused_reason = %v, want %v", got, tc.wantReason)
			}
		})
	}
}

// Tras enviar stock 0 sin pausar por cuenta propia, MercadoLibre pausa solo.
func TestStateAfterZeroStockPush(t *testing.T) {
	cases := []struct {
		name, response, wantStatus, wantReason string
	}{
		{"paused by mercadolibre", `{"id":"MLM1","status":"paused","sub_status":["out_of_stock"]}`, "paused", ""},
		{"still active", `{"id":"MLM1","status":"active"}`, "synced", ""},
		{"paused by the seller", `{"id":"MLM1","status":"paused","sub_status":["paused_by_seller"]}`, "paused", "user"},
		{"unreadable", `not json`, "synced", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := newStatusService(t, localListing{status: "synced"}, tc.response)
			status, reason := service.stateAfterZeroStockPush(context.Background(), "token", "MLM1")
			if status != tc.wantStatus || reason != tc.wantReason {
				t.Errorf("got (%q, %q), want (%q, %q)", status, reason, tc.wantStatus, tc.wantReason)
			}
		})
	}
}

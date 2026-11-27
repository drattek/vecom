package main

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	syncApp "core-orchestrator/internal/application/sync"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/testsupport/sqlscript"
)

func TestClientOnlyAllowsGet(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		_, _ = w.Write([]byte(`[{"code":200,"body":{"id":"MLM1","status":"active","available_quantity":3,"price":99.5,
			"shipping":{"logistic_type":"fulfillment"}}}]`))
	}))
	defer server.Close()
	client := &http.Client{Transport: getOnly{http.DefaultTransport}}
	items := mercadoLibreInfra.NewItemsHandler(mercadoLibreInfra.NewClient(client, server.URL, nil))

	statuses, err := items.GetItemsStatus(context.Background(), "token", []string{"MLM1"})
	if err != nil || len(statuses) != 1 {
		t.Fatalf("statuses=%v err=%v", statuses, err)
	}
	got := statuses[0]
	if got.AvailableQuantity == nil || *got.AvailableQuantity != 3 || got.Price == nil || *got.Price != 99.5 || got.LogisticType != "fulfillment" {
		t.Errorf("unexpected status: %+v", got)
	}

	if err := items.UpdateItemStatus(context.Background(), mercadoLibreInfra.UpdateItemStatusRequest{
		AccessToken: "token", ExternalID: "MLM1", Status: "closed",
	}); err == nil {
		t.Fatal("a write must be rejected")
	}
	if len(methods) != 1 || methods[0] != http.MethodGet {
		t.Errorf("requests that reached the server: %v", methods)
	}
}

// Aunque esté vencido, leer el token no lo renueva ni escribe.
func TestStoredTokenIsNeverRefreshed(t *testing.T) {
	now := time.Now().UTC()
	setting := func(id int64, key, value string) []driver.Value {
		return []driver.Value{id, int64(1), key, value, int64(0), int64(1), nil, now, now}
	}
	database := &sqlscript.DB{Rows: map[string][][]driver.Value{
		"FROM ecom_connection_settings": {
			setting(1, "access_token", "guardado"),
			setting(2, "refresh_token", "refresh"),
			setting(3, "expiration_time", now.Add(-time.Hour).Format(time.RFC3339)),
		},
	}}

	token, expiresAt, err := syncApp.StoredMercadoLibreAccessToken(
		context.Background(), mysqlInfra.NewConnectionSettingsRepository(database.Open(t)), 1)

	if err != nil || token != "guardado" || !expiresAt.Before(now) {
		t.Fatalf("token=%q expiresAt=%v err=%v", token, expiresAt, err)
	}
	if writes := database.Writes(""); len(writes) != 0 {
		t.Errorf("reading the token must not write: %+v", writes)
	}
}

package odoo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSyncProductFitmentsReusesExistingModelOnUniqueViolation(t *testing.T) {
	var syncCalls int32
	var secondPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/product.template/ecom_sync_fitments"):
			n := atomic.AddInt32(&syncCalls, 1)
			if n == 1 {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"name":"odoo.exceptions.ValidationError","message":"The machine model name must be unique for the selected brand, machine type and years."}`))
				return
			}
			_ = json.NewDecoder(r.Body).Decode(&secondPayload)
			_, _ = w.Write([]byte(`{"linked":1,"unlinked":0}`))
		case strings.HasSuffix(r.URL.Path, "/machine.model/fields_get"):
			_, _ = w.Write([]byte(`{"name":{"type":"char"},"brand_id":{"type":"many2one","relation":"machine.brand"},"type_id":{"type":"many2one","relation":"machine.type"}}`))
		case strings.HasSuffix(r.URL.Path, "/machine.model/search_read"):
			_, _ = w.Write([]byte(`[{"id":9,"ecom_ref":"vehicle_fitment:11"}]`))
		default:
			t.Errorf("unexpected call %s", r.URL.Path)
		}
	}))
	defer server.Close()

	handler := NewFitmentsHandler(NewClient(server.Client(), server.URL, nil))
	fitments := []Fitment{{
		EcomRef: "vehicle_fitment:10", Name: "Versa", YearStart: 2016, YearEnd: 2016,
		Brand: FitmentBrand{EcomRef: "brand:1", Name: "Nissan"},
		Type:  FitmentType{Name: "Vehículo", IsVehicle: true},
	}}

	result, err := handler.SyncProductFitments(context.Background(), Credentials{}, 24445, fitments, nil)
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	if result.Linked != 1 {
		t.Fatalf("unexpected result %+v", result)
	}
	if got := atomic.LoadInt32(&syncCalls); got != 2 {
		t.Fatalf("expected 2 sync attempts, got %d", got)
	}

	sent, _ := secondPayload["fitments"].([]any)
	if len(sent) != 1 || sent[0].(map[string]any)["ecom_ref"] != "vehicle_fitment:11" {
		t.Fatalf("retry must carry the existing ecom_ref, got %v", secondPayload["fitments"])
	}
}

func TestSyncProductFitmentsSkipsSingleModelWhenNothingToReuse(t *testing.T) {
	var syncCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/ecom_sync_fitments"):
			atomic.AddInt32(&syncCalls, 1)
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"The machine model name must be unique for the selected brand, machine type and years."}`))
		case strings.HasSuffix(r.URL.Path, "/fields_get"):
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/search_read"):
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()

	handler := NewFitmentsHandler(NewClient(server.Client(), server.URL, nil))
	result, err := handler.SyncProductFitments(context.Background(), Credentials{}, 1, []Fitment{{EcomRef: "vehicle_fitment:10", Name: "Versa"}}, nil)
	if err != nil {
		t.Fatalf("an irreducible conflict must be skipped, not fail the sync: %v", err)
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("expected the model to be reported as skipped, got %v", result.Skipped)
	}
	if got := atomic.LoadInt32(&syncCalls); got != 1 {
		t.Fatalf("must not retry when no existing model was found, got %d attempts", got)
	}
}

func TestSyncProductFitmentsSkipsAllWithoutExtraWritesWhenStillConflicting(t *testing.T) {
	var syncCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/ecom_sync_fitments"):
			atomic.AddInt32(&syncCalls, 1)
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"The machine model name must be unique for the selected brand, machine type and years."}`))
		case strings.HasSuffix(r.URL.Path, "/fields_get"):
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/search_read"):
			_, _ = w.Write([]byte(`[{"id":9,"name":"Other","ecom_ref":"vehicle_fitment:99"}]`))
		default:
			t.Errorf("unexpected call %s", r.URL.Path)
		}
	}))
	defer server.Close()

	handler := NewFitmentsHandler(NewClient(server.Client(), server.URL, nil))
	fitments := []Fitment{
		{EcomRef: "vehicle_fitment:1", Name: "A"},
		{EcomRef: "vehicle_fitment:2", Name: "B"},
	}

	result, err := handler.SyncProductFitments(context.Background(), Credentials{}, 1, fitments, nil)
	if err != nil {
		t.Fatalf("a conflict that cannot be resolved must not fail the sync: %v", err)
	}
	if len(result.Skipped) != 2 {
		t.Fatalf("expected both fitments reported as skipped, got %v", result.Skipped)
	}
	// 1 original attempt + 1 retry with the reused refs; no probing of subsets.
	if got := atomic.LoadInt32(&syncCalls); got != 2 {
		t.Fatalf("expected exactly 2 ecom_sync_fitments calls, got %d", got)
	}
}

func TestSyncProductFitmentsDropsEquivalentFitmentsBeforeRetry(t *testing.T) {
	var syncCalls int32
	var secondPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/ecom_sync_fitments"):
			if atomic.AddInt32(&syncCalls, 1) == 1 {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"message":"The machine model name must be unique for the selected brand, machine type and years."}`))
				return
			}
			_ = json.NewDecoder(r.Body).Decode(&secondPayload)
			_, _ = w.Write([]byte(`{"linked":1,"unlinked":0}`))
		case strings.HasSuffix(r.URL.Path, "/fields_get"):
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/search_read"):
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()

	handler := NewFitmentsHandler(NewClient(server.Client(), server.URL, nil))
	fitments := []Fitment{
		{EcomRef: "vehicle_fitment:1", Name: "Versa", YearStart: 2017, YearEnd: 2017, Brand: FitmentBrand{Name: "Nissan"}, Type: FitmentType{Name: "Vehículo"}},
		{EcomRef: "vehicle_fitment:2:2017", Name: "versa ", YearStart: 2017, YearEnd: 2017, Brand: FitmentBrand{Name: "NISSAN"}, Type: FitmentType{Name: "Vehículo"}},
	}

	if _, err := handler.SyncProductFitments(context.Background(), Credentials{}, 1, fitments, nil); err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	sent, _ := secondPayload["fitments"].([]any)
	if len(sent) != 1 {
		t.Fatalf("equivalent fitments must be sent once, got %v", secondPayload["fitments"])
	}
}

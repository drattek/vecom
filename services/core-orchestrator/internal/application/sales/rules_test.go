package sales

import (
	"testing"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

func TestSyncWindow(t *testing.T) {
	floor := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	cursor := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

	initial := syncWindow(&mysqlInfra.SaleSyncStateDTO{FloorAt: floor}, time.Hour)
	if !initial.Initial || !initial.From.Equal(floor) {
		t.Errorf("carga inicial = %+v", initial)
	}
	incremental := syncWindow(&mysqlInfra.SaleSyncStateDTO{FloorAt: floor, CursorAt: &cursor}, time.Hour)
	if incremental.Initial || !incremental.From.Equal(cursor.Add(-time.Hour)) {
		t.Errorf("incremental = %+v", incremental)
	}
}

func TestIsDue(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { value := now.Add(-d); return &value }
	cases := []struct {
		name                     string
		lastSuccess, lastStarted *time.Time
		want                     bool
	}{
		{"nunca corrió", nil, nil, true},
		{"éxito reciente", ago(time.Hour), ago(time.Hour), false},
		{"éxito vencido", ago(5 * time.Hour), ago(5 * time.Hour), true},
		{"falló hace poco", ago(9 * time.Hour), ago(5 * time.Minute), false},
		{"falló hace rato", ago(9 * time.Hour), ago(time.Hour), true},
	}
	for _, tc := range cases {
		if got := isDue(tc.lastSuccess, tc.lastStarted, now, 4*time.Hour); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}

func TestIsUnchanged(t *testing.T) {
	stored := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	remote := stored.Add(300 * time.Millisecond)
	later := stored.Add(time.Minute)

	if !isUnchanged(&mysqlInfra.SaleKnownState{ExternalUpdatedAt: &stored}, &remote) {
		t.Error("las fracciones de segundo no cuentan como cambio")
	}
	if isUnchanged(&mysqlInfra.SaleKnownState{ExternalUpdatedAt: &stored}, &later) {
		t.Error("una fecha posterior es un cambio")
	}
	if isUnchanged(&mysqlInfra.SaleKnownState{ExternalUpdatedAt: &stored, Incomplete: true}, &stored) {
		t.Error("una venta incompleta se vuelve a consultar")
	}
	if isUnchanged(nil, &stored) {
		t.Error("una venta nueva se consulta")
	}
}

func TestEstimateNet(t *testing.T) {
	sale := &mysqlInfra.SaleDetailDTO{
		SaleListItemDTO:      mysqlInfra.SaleListItemDTO{ID: 10, OrderStatus: OrderConfirmed, PaymentStatus: PaymentPaid, ShippingStatus: ShippingShipped, TotalAmount: floatPtr(100)},
		MarketplaceFeeAmount: floatPtr(15),
	}
	if net, _ := estimateNet(sale, nil); net != nil {
		t.Errorf("sin costo de envío conocido no hay neto: %v", *net)
	}
	sale.ShippingCostSeller = floatPtr(20)
	if net, shared := estimateNet(sale, nil); net == nil || !near(*net, 65) || shared {
		t.Errorf("neto = %v", net)
	}
	// En un paquete, el envío se resta solo en la orden guardada primero.
	if net, shared := estimateNet(sale, []mysqlInfra.RelatedSaleDTO{{ID: 9}}); net == nil || !near(*net, 85) || !shared {
		t.Errorf("neto compartido = %v, shared = %v", net, shared)
	}
	if net, shared := estimateNet(sale, []mysqlInfra.RelatedSaleDTO{{ID: 11}}); net == nil || !near(*net, 65) || shared {
		t.Errorf("neto de la primera orden = %v", net)
	}
	sale.OrderStatus = OrderCancelled
	if net, _ := estimateNet(sale, nil); net != nil {
		t.Error("una venta cancelada no tiene neto estimado")
	}
}

func TestBuildFilter(t *testing.T) {
	filter, err := buildFilter(ListInput{From: "2026-11-01", To: "2026-11-01", PageSize: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if filter.Until.Sub(*filter.From) != 24*time.Hour || filter.PageSize != maxPageSize {
		t.Errorf("filtro = %+v", filter)
	}
	if _, err := buildFilter(ListInput{OrderStatus: "x' OR 1=1"}); err == nil {
		t.Error("un estado desconocido debe rechazarse")
	}
}

package sales

// Prueba contra un MySQL real DESECHABLE (se omite sin la variable). Requiere
// una base cargada con eco-architecture.sql y 2026-11-25_sales_sync.sql;
// inserta datos y vacía las tablas ecom_sale*. Nunca apuntarla a una base real.
//
//	SALES_TEST_MYSQL_DSN='root:pw@tcp(127.0.0.1:33307)/ecommerce?parseTime=true&loc=Local' \
//	  go test ./internal/application/sales/ -run Integration -v

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type fakeReader struct{ session *fakeSession }

func (fakeReader) ChannelCode() string                            { return mercadoLibreChannelCode }
func (r fakeReader) Open(context.Context, int64) (Session, error) { return r.session, nil }

type fakeSession struct {
	records  []*mysqlInfra.SaleRecord
	failing  map[string]bool
	windows  []Window
	lookups  []string
	loadedBy map[string]int
}

func (s *fakeSession) Warnings() []string { return nil }

func (s *fakeSession) candidate(record *mysqlInfra.SaleRecord) Candidate {
	return Candidate{
		ExternalID:        record.ExternalID,
		ExternalUpdatedAt: record.ExternalUpdatedAt,
		Load: func(context.Context, *mysqlInfra.SaleKnownState) (*mysqlInfra.SaleRecord, error) {
			if s.failing[record.ExternalID] {
				return nil, errors.New("fallo simulado")
			}
			s.loadedBy[record.ExternalID]++
			copied := *record
			copied.Items = append([]mysqlInfra.SaleItemRecord(nil), record.Items...)
			return &copied, nil
		},
	}
}

func (s *fakeSession) Search(ctx context.Context, window Window, handle HandleFunc) error {
	s.windows = append(s.windows, window)
	for _, record := range s.records {
		if err := handle(ctx, s.candidate(record)); err != nil {
			return err
		}
	}
	return nil
}

func (s *fakeSession) Lookup(ctx context.Context, ids []string, handle HandleFunc) error {
	s.lookups = append(s.lookups, ids...)
	return nil
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	result, err := db.ExecContext(context.Background(), query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	id, _ := result.LastInsertId()
	return id
}

func TestIntegrationSalesSync(t *testing.T) {
	dsn := os.Getenv("SALES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("SALES_TEST_MYSQL_DSN no está definido")
	}
	ctx := context.Background()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, table := range []string{"ecom_sale_items", "ecom_sale_charges", "ecom_sale_payments", "ecom_sale_shipments", "ecom_sales", "ecom_sale_sync_state"} {
		mustExec(t, db, "DELETE FROM "+table)
	}

	suffix := time.Now().Format("150405.000000")
	user := mustExec(t, db, "INSERT INTO ecom_api_user (username, password_hash) VALUES (?, 'x')", "u"+suffix)
	currency := mustExec(t, db, "INSERT INTO ecom_currencies (name, code, symbol, created_by) VALUES ('Peso', ?, '$', ?)", "C"+suffix, user)
	var channel int64
	if err := db.QueryRowContext(ctx, "SELECT id FROM ecom_channels WHERE code = 'MERCADOLIBRE'").Scan(&channel); err != nil {
		channel = mustExec(t, db, "INSERT INTO ecom_channels (name, code, created_by) VALUES ('MercadoLibre', 'MERCADOLIBRE', ?)", user)
	}
	mustExec(t, db, "UPDATE ecom_channel_connections SET status = 'hidden'")
	connection := mustExec(t, db, "INSERT INTO ecom_channel_connections (channel_id, name, status, currency_id, created_by) VALUES (?, 'ML prueba', 'active', ?, ?)", channel, currency, user)
	source := mustExec(t, db, "INSERT INTO ecom_sources (code, name, created_by) VALUES (?, 'Fuente', ?)", "S"+suffix, user)
	byListing := mustExec(t, db, "INSERT INTO ecom_products (sku, part_number, name, status, source_id, created_by) VALUES (?, 'P1', 'Por publicación', 'active', ?, ?)", "A"+suffix, source, user)
	bySKU := mustExec(t, db, "INSERT INTO ecom_products (sku, part_number, name, status, source_id, created_by) VALUES (?, 'P2', 'Por SKU', 'active', ?, ?)", "B"+suffix, source, user)
	mustExec(t, db, "INSERT INTO ecom_channel_product_map (product_id, connection_id, external_id, status, created_by) VALUES (?, ?, 'MLM1', 'synced', ?)", byListing, connection, user)

	orderedAt := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	updatedAt := orderedAt.Add(time.Hour)
	sale := func(id string) *mysqlInfra.SaleRecord {
		return &mysqlInfra.SaleRecord{
			ExternalID: id, OrderNumber: strPtr(id), PackID: strPtr("PACK-1"),
			OrderStatus: OrderConfirmed, PaymentStatus: strPtr(PaymentPaid), ShippingStatus: strPtr(ShippingShipped),
			OrderedAt: orderedAt, ExternalUpdatedAt: &updatedAt,
			CustomerName: strPtr("Cliente de prueba"), CustomerEmail: strPtr("cliente@example.com"), TaxID: strPtr("XAXX010101000"),
			CurrencyCode: strPtr("MXN"), TotalAmount: floatPtr(200), MarketplaceFeeAmount: floatPtr(30), ShippingCostSeller: floatPtr(50),
			RawPayload: strPtr(`{"id":1}`),
			Items: []mysqlInfra.SaleItemRecord{
				{ExternalID: "l1", ListingExternalID: "MLM1", SKU: strPtr("otro"), Title: strPtr("Uno"), Quantity: 1, UnitPrice: floatPtr(100)},
				{ExternalID: "l2", ListingExternalID: "MLM2", SKU: strPtr("B" + suffix), Title: strPtr("Dos"), Quantity: 2, UnitPrice: floatPtr(50)},
				{ExternalID: "l3", ListingExternalID: "MLM3", Title: strPtr("Sin vínculo"), Quantity: 1},
			},
			Payments:  []mysqlInfra.SalePaymentRecord{{ExternalID: "p1", Method: strPtr("visa"), Amount: floatPtr(200), ApprovedAt: &orderedAt}},
			Shipments: []mysqlInfra.SaleShipmentRecord{{ExternalID: "s1", TrackingNumber: strPtr("GUIA1"), DispatchDeadline: &updatedAt, SellerCost: floatPtr(50)}},
			Charges: []mysqlInfra.SaleChargeRecord{
				{ExternalID: "fee:sale", Source: chargeSourceOrder, Type: chargeFee, PaidBy: strPtr(paidBySeller), Amount: 30},
				{ExternalID: "shipping:seller:s1", Source: chargeSourceShipping, Type: chargeShipping, PaidBy: strPtr(paidBySeller), Amount: 50},
			},
			ChargeSources: []string{chargeSourceOrder, chargeSourceShipping},
		}
	}

	session := &fakeSession{records: []*mysqlInfra.SaleRecord{sale("1001"), sale("1002")}, failing: map[string]bool{"1002": true}, loadedBy: map[string]int{}}
	// Una venta anterior al piso no se importa.
	old := sale("0900")
	old.OrderedAt = time.Now().AddDate(0, 0, -60)
	session.records = append(session.records, old)

	service := NewSyncService(db, mysqlInfra.NewSalesRepository(db), mysqlInfra.NewSaleSyncStateRepository(db), Config{})
	service.Register(fakeReader{session: session})
	queries := NewQueryService(mysqlInfra.NewSalesViewRepository(db))
	run := func() mysqlInfra.SaleSyncStatusDTO {
		t.Helper()
		if err := service.syncConnection(ctx, connection, nil); err != nil {
			t.Fatalf("syncConnection: %v", err)
		}
		statuses, err := service.Status(ctx)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		for _, status := range statuses {
			if status.ConnectionID == connection {
				return status
			}
		}
		t.Fatal("la conexión no aparece en el estado")
		return mysqlInfra.SaleSyncStatusDTO{}
	}

	// 1) Con un fallo, lo demás se guarda y el cursor no avanza.
	status := run()
	if status.InitialLoadDone || status.LastSuccessAt != nil || status.LastError == nil || status.LastSavedCount != 1 || status.LastFailedCount != 1 || status.Running {
		t.Fatalf("estado tras el fallo: %+v", status)
	}

	// 2) Al recuperarse, el cursor avanza y la venta ya guardada no se vuelve a pedir.
	session.failing = nil
	status = run()
	if !status.InitialLoadDone || status.LastSuccessAt == nil || status.LastError != nil {
		t.Fatalf("estado tras recuperarse: %+v", status)
	}
	if !session.windows[1].Initial || session.loadedBy["1001"] != 1 {
		t.Fatalf("ventana %+v, cargas de 1001 = %d", session.windows[1], session.loadedBy["1001"])
	}

	list, err := queries.List(ctx, ListInput{ConnectionID: connection, ChannelCode: "mercadolibre", Search: "B" + suffix, From: orderedAt.Format("2006-01-02")})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list.Total != 2 || list.Sales[0].ItemCount != 3 || list.Sales[0].UnlinkedItems != 1 || list.Sales[0].DispatchDeadline == nil {
		t.Fatalf("lista = %+v", list)
	}

	var firstID, secondID int64
	for _, item := range list.Sales {
		if item.ExternalID == "1001" {
			firstID = item.ID
		} else {
			secondID = item.ID
		}
	}
	detail, err := queries.Detail(ctx, firstID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if detail.Items[0].ProductID == nil || *detail.Items[0].ProductID != byListing || *detail.Items[0].MatchMethod != "listing" {
		t.Errorf("vínculo por publicación: %+v", detail.Items[0])
	}
	if detail.Items[1].ProductID == nil || *detail.Items[1].ProductID != bySKU || *detail.Items[1].MatchMethod != "sku" {
		t.Errorf("vínculo por SKU: %+v", detail.Items[1])
	}
	if len(detail.RelatedSales) != 1 || detail.EstimatedNetAmount == nil || !near(*detail.EstimatedNetAmount, 120) {
		t.Errorf("paquete %+v, neto %v", detail.RelatedSales, detail.EstimatedNetAmount)
	}
	second, err := queries.Detail(ctx, secondID)
	if err != nil {
		t.Fatalf("Detail 2: %v", err)
	}
	if !second.ShippingCostOnRelatedSale || !near(*second.EstimatedNetAmount, 170) {
		t.Errorf("envío compartido: %v, neto %v", second.ShippingCostOnRelatedSale, *second.EstimatedNetAmount)
	}

	// 3) Respuesta parcial: lo ausente se conserva y lo desaparecido se da de baja.
	newer := updatedAt.Add(time.Hour)
	partial := sale("1001")
	partial.ExternalUpdatedAt = &newer
	partial.CustomerName, partial.CustomerEmail, partial.TaxID, partial.RawPayload = nil, nil, nil, nil
	partial.PaymentStatus, partial.ShippingStatus, partial.ShippingCostSeller = nil, nil, nil
	partial.Items = partial.Items[:2]
	partial.Payments, partial.Shipments = nil, nil
	partial.Charges = nil
	partial.ChargeSources = []string{chargeSourceOrder}
	partial.IncompleteReasons = []string{"envío: fallo"}
	session.records = []*mysqlInfra.SaleRecord{partial}
	run()
	if window := session.windows[2]; window.Initial {
		t.Errorf("la tercera corrida debe ser incremental: %+v", window)
	}

	detail, err = queries.Detail(ctx, firstID)
	if err != nil {
		t.Fatalf("Detail parcial: %v", err)
	}
	if detail.CustomerEmail == nil || detail.TaxID == nil || detail.PaymentStatus != PaymentPaid || detail.ShippingStatus != ShippingShipped || detail.ShippingCostSeller == nil {
		t.Errorf("un dato ausente sobrescribió lo guardado: %+v", detail.SaleDetailDTO)
	}
	if len(detail.Items) != 2 || len(detail.Payments) != 1 || len(detail.Shipments) != 1 {
		t.Errorf("hijos: %d items, %d pagos, %d envíos", len(detail.Items), len(detail.Payments), len(detail.Shipments))
	}
	if len(detail.Charges) != 1 || detail.Charges[0].Type != chargeShipping {
		t.Errorf("cargos = %+v", detail.Charges)
	}
	if !detail.IsIncomplete {
		t.Error("la venta debe quedar incompleta")
	}
	var removed int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ecom_sale_items WHERE sale_id = ? AND deleted_at IS NOT NULL", firstID).Scan(&removed); err != nil || removed != 1 {
		t.Errorf("bajas lógicas = %d (%v)", removed, err)
	}

	// 4) Solo las ventas abiertas se vuelven a consultar: la incompleta sí, la
	// entregada no.
	mustExec(t, db, "UPDATE ecom_sales SET shipping_status = ? WHERE id = ?", ShippingDelivered, secondID)
	session.records, session.lookups = nil, nil
	run()
	if len(session.lookups) != 1 || session.lookups[0] != "1001" {
		t.Errorf("abiertas consultadas = %v", session.lookups)
	}

	// 5) El bloqueo impide una segunda corrida simultánea.
	state := mysqlInfra.NewSaleSyncStateRepository(db)
	now := time.Now()
	first, err := state.AcquireLock(ctx, connection, "a", now, now.Add(time.Minute))
	if err != nil || !first {
		t.Fatalf("primer bloqueo: %v %v", first, err)
	}
	if again, err := state.AcquireLock(ctx, connection, "b", now, now.Add(time.Minute)); err != nil || again {
		t.Errorf("segundo bloqueo: %v %v", again, err)
	}
	if owned, err := state.RenewLock(ctx, connection, "a", now.Add(time.Minute)); err != nil || !owned {
		t.Errorf("renovación: %v %v", owned, err)
	}
	if owned, err := state.RenewLock(ctx, connection, "b", now.Add(time.Minute)); err != nil || owned {
		t.Errorf("renovación ajena: %v %v", owned, err)
	}

	if _, err := queries.Detail(ctx, 999999999); !errors.Is(err, mysqlInfra.ErrSaleNotFound) {
		t.Errorf("venta inexistente: %v", err)
	}
}

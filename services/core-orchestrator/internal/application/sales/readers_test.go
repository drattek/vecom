package sales

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// Recorte del ejemplo oficial de GET /orders/{id}: pack_id es numérico. Se
// agrega una variation_id numérica y un billing_info.id.
const sampleOrder = `{
  "id": 2000009713473608,
  "date_created": "2024-11-01T09:34:59.000-04:00",
  "last_updated": "2024-11-01T09:37:20.000-04:00",
  "date_closed": "2024-11-01T09:35:43.000-04:00",
  "pack_id": 2000006556183755,
  "total_amount": 125.92,
  "paid_amount": 125.92,
  "order_items": [{
    "item": {"id": "MLM3737188005", "title": "Espaciador", "variation_id": 174997747012, "seller_sku": "ESP-500"},
    "quantity": 2, "unit_price": 62.96, "full_unit_price": 72.37, "sale_fee": 11.07
  }],
  "currency_id": "MXN",
  "payments": [{"id": 91776699099, "status": "approved", "payment_method_id": "account_money",
    "transaction_amount": 125.92, "transaction_amount_refunded": 0.0, "installments": 1}],
  "shipping": {"id": 44028792542},
  "status": "paid",
  "status_detail": null,
  "tags": ["pack_order", "paid"],
  "context": {"site": "MLM"},
  "buyer": {"id": 6427433, "nickname": "COMPRADOR1", "billing_info": {"id": 677487519924852462}}
}`

type fakeTokens struct{}

func (fakeTokens) EnsureValidAccessToken(context.Context, int64) (string, error) { return "token", nil }

func newMercadoLibreTestSession(t *testing.T, routes map[string]func(w http.ResponseWriter)) Session {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route, ok := routes[r.URL.Path]; ok {
			route(w)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found","error":"not_found"}`))
	}))
	t.Cleanup(server.Close)

	client := mercadoLibreInfra.NewClient(server.Client(), server.URL, nil)
	routes["/users/me"] = jsonRoute(http.StatusOK, `{"id": 123}`)
	reader := NewMercadoLibreReader(mercadoLibreInfra.NewSalesHandler(client), mercadoLibreInfra.NewUsersHandler(client), fakeTokens{})
	session, err := reader.Open(context.Background(), 1)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return session
}

func jsonRoute(status int, body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func searchAll(t *testing.T, session Session) []*mysqlInfra.SaleRecord {
	t.Helper()
	var records []*mysqlInfra.SaleRecord
	err := session.Search(context.Background(), Window{Initial: true}, func(ctx context.Context, c Candidate) error {
		record, err := c.Load(ctx, nil)
		if err != nil {
			return err
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	return records
}

func TestMercadoLibreReaderMapsOrderWithNumericIDs(t *testing.T) {
	routes := map[string]func(http.ResponseWriter){
		"/orders/search":           jsonRoute(http.StatusOK, `{"results":[`+sampleOrder+`],"paging":{"total":1}}`),
		"/orders/2000009713473608": jsonRoute(http.StatusPartialContent, sampleOrder),
		"/shipments/44028792542": jsonRoute(http.StatusOK, `{"id":44028792542,"status":"ready_to_ship",
			"logistic":{"mode":"me2","type":"xd_drop_off"},
			"destination":{"receiver_name":"Persona Prueba","shipping_address":{"street_name":"Calle 1","street_number":"5","zip_code":"01040","city":{"name":"CDMX"}}}}`),
		"/shipments/44028792542/costs":                jsonRoute(http.StatusOK, `{"receiver":{"cost":0},"senders":[{"cost":45.5}]}`),
		"/shipments/44028792542/sla":                  jsonRoute(http.StatusOK, `{"expected_date":"2024-11-02T23:59:59-06:00"}`),
		"/orders/billing-info/MLM/677487519924852462": jsonRoute(http.StatusOK, `{"buyer":{"billing_info":{"name":"Persona","last_name":"Prueba","identification":{"type":"RFC","number":"XAXX010101000"},"taxes":{"cfdi":{"id":"G03","description":"Gastos en general"}}}}}`),
	}
	records := searchAll(t, newMercadoLibreTestSession(t, routes))
	if len(records) != 1 {
		t.Fatalf("records = %d", len(records))
	}
	record := records[0]

	if deref(record.PackID) != "2000006556183755" {
		t.Errorf("pack id = %q", deref(record.PackID))
	}
	if got := deref(record.Items[0].ExternalVariationID); got != "174997747012" {
		t.Errorf("variation id = %q", got)
	}
	if deref(record.PaymentStatus) != PaymentPaid || deref(record.ShippingStatus) != ShippingReadyToShip {
		t.Errorf("estados = %q / %q", deref(record.PaymentStatus), deref(record.ShippingStatus))
	}
	// sale_fee es por unidad: 2 × 11.07, igual que marketplace_fee del ejemplo.
	if record.MarketplaceFeeAmount == nil || !near(*record.MarketplaceFeeAmount, 22.14) {
		t.Errorf("comisión = %v", record.MarketplaceFeeAmount)
	}
	if record.ShippingCostSeller == nil || !near(*record.ShippingCostSeller, 45.5) {
		t.Errorf("envío vendedor = %v", record.ShippingCostSeller)
	}
	if record.Shipments[0].DispatchDeadline == nil || deref(record.TaxID) != "XAXX010101000" {
		t.Errorf("fecha límite o RFC ausentes")
	}
	if len(record.IncompleteReasons) != 0 {
		t.Errorf("incompleta: %v", record.IncompleteReasons)
	}
}

func TestMercadoLibreReaderKeepsSaleWhenSecondaryDataFails(t *testing.T) {
	routes := map[string]func(http.ResponseWriter){
		"/orders/search":                              jsonRoute(http.StatusOK, `{"results":[`+sampleOrder+`],"paging":{"total":1}}`),
		"/orders/2000009713473608":                    jsonRoute(http.StatusOK, sampleOrder),
		"/shipments/44028792542":                      jsonRoute(http.StatusInternalServerError, `{"message":"boom","buyer_email":"privado@example.com"}`),
		"/orders/billing-info/MLM/677487519924852462": jsonRoute(http.StatusNotFound, `{}`),
	}
	record := searchAll(t, newMercadoLibreTestSession(t, routes))[0]

	if len(record.IncompleteReasons) != 1 {
		t.Fatalf("razones = %v", record.IncompleteReasons)
	}
	if strings.Contains(record.IncompleteReasons[0], "privado@example.com") {
		t.Error("el error expone el cuerpo de la respuesta")
	}
	// nil = no se obtuvo: no debe sobrescribir el envío guardado.
	if record.Shipments != nil || record.ShippingStatus != nil || record.ShippingCostSeller != nil {
		t.Error("un envío no obtenido debe quedar en nil")
	}
}

func TestMercadoLibrePaymentStatus(t *testing.T) {
	cases := map[string]string{
		`{"status":"paid","payments":[{"status":"approved","transaction_amount":100,"transaction_amount_refunded":30}]}`: PaymentPartiallyRefunded,
		`{"status":"cancelled","payments":[{"status":"refunded","transaction_amount":100}]}`:                             PaymentRefunded,
		`{"status":"cancelled","payments":[{"status":"rejected","transaction_amount":100}]}`:                             PaymentRejected,
		`{"status":"paid","payments":[{"status":"rejected"},{"status":"approved","transaction_amount":100}]}`:            PaymentPaid,
		`{"status":"payment_in_process","payments":[{"status":"in_process","transaction_amount":100}]}`:                  PaymentPending,
		`{"status":"paid","payments":[{"status":"algo_nuevo"}]}`:                                                         StatusUnknown,
	}
	for body, want := range cases {
		var order mercadoLibreInfra.SalesOrder
		if err := json.Unmarshal([]byte(body), &order); err != nil {
			t.Fatal(err)
		}
		if got := deref(mercadoLibrePaymentStatus(&order)); got != want {
			t.Errorf("%s: got %q, want %q", body, got, want)
		}
	}
}

func TestOdooReaderImportsOnlyEcommerceOrders(t *testing.T) {
	var orderDomains []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/json/2/"), "/")
		model, method := parts[0], parts[1]
		if method == "fields_get" {
			if model == "account.move" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			fields := map[string]any{}
			for _, name := range odooWantedFields[model] {
				fields[name] = map[string]string{"type": "char"}
			}
			_ = json.NewEncoder(w).Encode(fields)
			return
		}
		responses := map[string]string{
			"sale.order": `[{"id":7,"name":"S00007","state":"sale","date_order":"2026-11-20 18:00:00","write_date":"2026-11-20 18:05:00",
				"website_id":[1,"Tienda"],"partner_id":[3,"Cliente"],"partner_invoice_id":[3,"Cliente"],"partner_shipping_id":[3,"Cliente"],
				"order_line":[11,12],"amount_tax":16,"amount_total":126,"currency_id":[33,"MXN"],
				"picking_ids":[21],"transaction_ids":[31],"invoice_ids":[]}]`,
			"sale.order.line": `[{"id":11,"product_id":[5,"[SKU-1] Balata"],"product_uom_qty":2,"price_unit":50,"discount":10,"price_subtotal":90,"price_tax":14.4,"price_total":104.4,"is_delivery":false,"display_type":false},
				{"id":12,"product_id":[6,"Envío"],"name":"Envío estándar","price_total":11.6,"is_delivery":true,"display_type":false}]`,
			"product.product":     `[{"id":5,"default_code":"SKU-1","product_tmpl_id":[900,"Balata"]}]`,
			"res.partner":         `[{"id":3,"name":"Cliente","email":"cliente@example.com","phone":false,"mobile":"5550000000","vat":"XAXX010101000","street":"Calle 1","state_id":[1,"CDMX"]}]`,
			"stock.picking":       `[{"id":21,"state":"assigned","picking_type_code":"outgoing","carrier_tracking_ref":false,"scheduled_date":"2026-11-21 18:00:00"}]`,
			"payment.transaction": `[{"id":31,"state":"done","amount":126,"provider_id":[2,"Mercado Pago"]}]`,
		}
		if model == "sale.order" {
			orderDomains = append(orderDomains, string(body))
		}
		_, _ = w.Write([]byte(responses[model]))
	}))
	t.Cleanup(server.Close)

	reader := NewOdooReader(func(context.Context, int64) (map[string]string, error) {
		return map[string]string{"odoo_url": server.URL, "apikey": "k", "sales_website_id": "1"}, nil
	}, nil)
	session, err := reader.Open(context.Background(), 2)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	records := searchAll(t, session)

	if len(records) != 1 || len(orderDomains) != 1 {
		t.Fatalf("records = %d, consultas = %d", len(records), len(orderDomains))
	}
	for _, expected := range []string{`["website_id","!=",false]`, `["website_id","=",1]`, `["state","in",["sent","sale","cancel"]]`, `date_order`} {
		if !strings.Contains(orderDomains[0], expected) {
			t.Errorf("el dominio no contiene %s: %s", expected, orderDomains[0])
		}
	}

	record := records[0]
	if len(record.Items) != 1 || deref(record.Items[0].SKU) != "SKU-1" || record.Items[0].ListingExternalID != "900" {
		t.Fatalf("items = %+v", record.Items)
	}
	if !near(*record.ShippingAmount, 11.6) || !near(*record.DiscountAmount, 10) || !near(*record.SubtotalAmount, 90) {
		t.Errorf("importes: envío %v, descuento %v, subtotal %v", *record.ShippingAmount, *record.DiscountAmount, *record.SubtotalAmount)
	}
	if deref(record.PaymentStatus) != PaymentPaid || deref(record.ShippingStatus) != ShippingReadyToShip {
		t.Errorf("estados = %q / %q", deref(record.PaymentStatus), deref(record.ShippingStatus))
	}
	if deref(record.CustomerPhone) != "5550000000" || deref(record.CurrencyCode) != "MXN" {
		t.Errorf("teléfono o moneda incorrectos")
	}
	// Un modelo sin permiso se avisa, sin marcar la venta como incompleta.
	if len(session.Warnings()) != 1 || len(record.IncompleteReasons) != 0 {
		t.Errorf("avisos = %v, incompleta = %v", session.Warnings(), record.IncompleteReasons)
	}
}

func near(a, b float64) bool { return a-b < 0.001 && b-a < 0.001 }

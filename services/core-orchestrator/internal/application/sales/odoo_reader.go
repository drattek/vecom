package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	odooInfra "core-orchestrator/internal/infrastructure/marketplace/odoo"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

const (
	odooChannelCode = "ODOO"
	odooPageSize    = 50
	// odooWebsiteSetting (ecom_connection_settings) limita la conexión a un
	// sitio web; necesario si dos conexiones comparten instancia.
	odooWebsiteSetting = "sales_website_id"

	odooModelOrder   = "sale.order"
	odooModelLine    = "sale.order.line"
	odooModelProduct = "product.product"
	odooModelPartner = "res.partner"
	odooModelPicking = "stock.picking"
	odooModelPayment = "payment.transaction"
	odooModelInvoice = "account.move"
)

// odooImportedStates excluye draft: en el ecommerce los carritos sin terminar
// son órdenes en borrador.
var odooImportedStates = []string{"sent", "sale", "cancel"}

// odooWantedFields son los campos que se piden si la instancia los tiene.
var odooWantedFields = map[string][]string{
	odooModelOrder: {
		"name", "state", "date_order", "write_date", "website_id",
		"partner_id", "partner_invoice_id", "partner_shipping_id", "order_line",
		"amount_untaxed", "amount_tax", "amount_total", "currency_id",
		"picking_ids", "transaction_ids", "invoice_ids", "delivery_status", "carrier_id",
	},
	odooModelLine: {
		"product_id", "name", "product_uom_qty", "price_unit", "discount",
		"price_subtotal", "price_tax", "price_total", "is_delivery", "display_type",
	},
	odooModelProduct: {"default_code", "product_tmpl_id"},
	odooModelPartner: {"name", "email", "phone", "mobile", "vat", "street", "street2", "city", "state_id", "zip", "country_id"},
	odooModelPicking: {"state", "picking_type_code", "carrier_id", "carrier_tracking_ref", "scheduled_date", "date_done"},
	odooModelPayment: {"reference", "state", "amount", "provider_id", "payment_method_id", "last_state_change"},
	odooModelInvoice: {"move_type", "state", "payment_state", "amount_total"},
}

// OdooConnectionValuesLoader entrega ajustes y credenciales de la conexión
// (claves en minúsculas).
type OdooConnectionValuesLoader func(ctx context.Context, connectionID int64) (map[string]string, error)

type OdooReader struct {
	loadValues OdooConnectionValuesLoader
	httpClient *http.Client
	limiter    *odooInfra.RateLimiter
}

func NewOdooReader(loadValues OdooConnectionValuesLoader, limiter *odooInfra.RateLimiter) *OdooReader {
	return &OdooReader{loadValues: loadValues, httpClient: &http.Client{Timeout: 60 * time.Second}, limiter: limiter}
}

func (r *OdooReader) ChannelCode() string { return odooChannelCode }

func (r *OdooReader) Open(ctx context.Context, connectionID int64) (Session, error) {
	values, err := r.loadValues(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if values["odoo_url"] == "" {
		return nil, errors.New("falta el ajuste odoo_url en la conexión")
	}
	if values["apikey"] == "" {
		return nil, errors.New("falta la credencial ApiKey en la conexión")
	}
	websiteID := int64(0)
	if raw := values[odooWebsiteSetting]; raw != "" {
		websiteID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || websiteID <= 0 {
			return nil, fmt.Errorf("el ajuste %s debe ser el id numérico del sitio web", odooWebsiteSetting)
		}
	}

	session := &odooSession{
		handler:     odooInfra.NewSalesHandler(odooInfra.NewClient(r.httpClient, values["odoo_url"], r.limiter)),
		credentials: odooInfra.Credentials{APIKey: values["apikey"], Database: values["x-odoo-database"]},
		websiteID:   websiteID,
		fields:      make(map[string][]string, len(odooWantedFields)),
	}
	if err := session.discoverFields(ctx); err != nil {
		return nil, err
	}
	return session, nil
}

type odooSession struct {
	handler     *odooInfra.SalesHandler
	credentials odooInfra.Credentials
	websiteID   int64
	// fields: por modelo, los campos disponibles. Un modelo ausente no se
	// puede leer.
	fields   map[string][]string
	warnings []string
}

func (s *odooSession) Warnings() []string { return s.warnings }

func (s *odooSession) discoverFields(ctx context.Context) error {
	models := make([]string, 0, len(odooWantedFields))
	for model := range odooWantedFields {
		models = append(models, model)
	}
	sort.Strings(models)

	for _, model := range models {
		available, err := s.handler.FieldNames(ctx, s.credentials, model)
		if err != nil {
			// Sin permiso o sin el módulo se sigue sin esos datos; cualquier
			// otro error es temporal y aborta la corrida.
			var statusErr *odooInfra.StatusError
			missing := errors.As(err, &statusErr) &&
				(statusErr.Status == http.StatusForbidden || statusErr.Status == http.StatusNotFound)
			if !missing || model == odooModelOrder {
				return fmt.Errorf("consultando los campos de %s: %w", model, err)
			}
			s.warnings = append(s.warnings, fmt.Sprintf("sin acceso a %s: esos datos no se importan", model))
			continue
		}
		fields := make([]string, 0, len(odooWantedFields[model]))
		for _, field := range odooWantedFields[model] {
			if available[field] {
				fields = append(fields, field)
			}
		}
		s.fields[model] = fields
	}

	// Sin website_id no se puede separar el ecommerce de CRM y mostrador.
	for _, required := range []string{"website_id", "date_order", "state", "write_date"} {
		if !slices.Contains(s.fields[odooModelOrder], required) {
			return fmt.Errorf("sale.order no tiene el campo %s (¿está instalado el módulo de ecommerce?)", required)
		}
	}
	return nil
}

func (s *odooSession) ecommerceDomain() []any {
	domain := []any{
		[]any{"website_id", "!=", false},
	}
	if s.websiteID > 0 {
		domain = append(domain, []any{"website_id", "=", s.websiteID})
	}
	return domain
}

func (s *odooSession) Search(ctx context.Context, window Window, handle HandleFunc) error {
	dateField := "write_date"
	if window.Initial {
		dateField = "date_order"
	}
	domain := append(s.ecommerceDomain(),
		[]any{"state", "in", odooImportedStates},
		[]any{dateField, ">=", window.From.UTC().Format(odooTimeLayout)},
	)

	// Paginación por id: un cambio durante la corrida no hace saltar filas.
	for lastID := int64(0); ; {
		pageDomain := append(slices.Clone(domain), []any{"id", ">", lastID})
		orders, err := s.handler.SearchRead(ctx, s.credentials, odooModelOrder, pageDomain, s.fields[odooModelOrder], 0, odooPageSize)
		if err != nil {
			return fmt.Errorf("buscando órdenes de Odoo: %w", err)
		}
		if len(orders) == 0 {
			return nil
		}
		if err := s.handlePage(ctx, orders, handle); err != nil {
			return err
		}
		next := odooID(orders[len(orders)-1])
		if len(orders) < odooPageSize || next <= lastID {
			return nil
		}
		lastID = next
	}
}

// Lookup no filtra por estado, para detectar una venta que volvió a borrador.
func (s *odooSession) Lookup(ctx context.Context, externalIDs []string, handle HandleFunc) error {
	ids := make([]int64, 0, len(externalIDs))
	for _, raw := range externalIDs {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	for chunk := range slices.Chunk(ids, odooPageSize) {
		domain := append(s.ecommerceDomain(), []any{"id", "in", chunk})
		orders, err := s.handler.SearchRead(ctx, s.credentials, odooModelOrder, domain, s.fields[odooModelOrder], 0, len(chunk))
		if err != nil {
			return fmt.Errorf("consultando órdenes abiertas de Odoo: %w", err)
		}
		if err := s.handlePage(ctx, orders, handle); err != nil {
			return err
		}
	}
	return nil
}

// odooRelated son los registros relacionados de una página, leídos en bloque
// para no hacer varias llamadas por venta.
type odooRelated struct {
	records map[string]map[int64]odooInfra.Record
	// failed: modelos cuya lectura falló en esta página.
	failed map[string]string
}

func (r odooRelated) loaded(model string) bool {
	_, ok := r.records[model]
	return ok
}

func (r odooRelated) pick(model string, ids []int64) []odooInfra.Record {
	result := make([]odooInfra.Record, 0, len(ids))
	for _, id := range ids {
		if record, ok := r.records[model][id]; ok {
			result = append(result, record)
		}
	}
	return result
}

func (s *odooSession) readByID(ctx context.Context, related odooRelated, model string, ids []int64, includeArchived bool) {
	if _, available := s.fields[model]; !available {
		return
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	ids = slices.DeleteFunc(ids, func(id int64) bool { return id <= 0 })

	byID := make(map[int64]odooInfra.Record, len(ids))
	if len(ids) > 0 {
		domain := []any{[]any{"id", "in", ids}}
		if includeArchived {
			domain = append(domain, []any{"active", "in", []bool{true, false}})
		}
		records, err := s.handler.SearchRead(ctx, s.credentials, model, domain, s.fields[model], 0, len(ids))
		if err != nil {
			related.failed[model] = err.Error()
			return
		}
		for _, record := range records {
			byID[odooID(record)] = record
		}
	}
	related.records[model] = byID
}

func (s *odooSession) handlePage(ctx context.Context, orders []odooInfra.Record, handle HandleFunc) error {
	related := odooRelated{records: map[string]map[int64]odooInfra.Record{}, failed: map[string]string{}}

	var lineIDs, partnerIDs, pickingIDs, paymentIDs, invoiceIDs, productIDs []int64
	for _, order := range orders {
		lineIDs = append(lineIDs, odooIDs(order, "order_line")...)
		pickingIDs = append(pickingIDs, odooIDs(order, "picking_ids")...)
		paymentIDs = append(paymentIDs, odooIDs(order, "transaction_ids")...)
		invoiceIDs = append(invoiceIDs, odooIDs(order, "invoice_ids")...)
		for _, field := range []string{"partner_id", "partner_invoice_id", "partner_shipping_id"} {
			id, _ := odooRef(order, field)
			partnerIDs = append(partnerIDs, id)
		}
	}
	s.readByID(ctx, related, odooModelLine, lineIDs, false)
	for _, line := range related.records[odooModelLine] {
		id, _ := odooRef(line, "product_id")
		productIDs = append(productIDs, id)
	}
	s.readByID(ctx, related, odooModelProduct, productIDs, true)
	s.readByID(ctx, related, odooModelPartner, partnerIDs, true)
	s.readByID(ctx, related, odooModelPicking, pickingIDs, false)
	s.readByID(ctx, related, odooModelPayment, paymentIDs, false)
	s.readByID(ctx, related, odooModelInvoice, invoiceIDs, false)
	if err := ctx.Err(); err != nil {
		return err
	}

	for _, order := range orders {
		candidate := Candidate{
			ExternalID:        odooIDString(odooID(order)),
			ExternalUpdatedAt: odooTime(order, "write_date"),
			Load: func(context.Context, *mysqlInfra.SaleKnownState) (*mysqlInfra.SaleRecord, error) {
				return mapOdooOrder(order, related)
			},
		}
		if err := handle(ctx, candidate); err != nil {
			return err
		}
	}
	return nil
}

func mapOdooOrder(order odooInfra.Record, related odooRelated) (*mysqlInfra.SaleRecord, error) {
	id := odooID(order)
	orderedAt := odooTime(order, "date_order")
	if id <= 0 || orderedAt == nil {
		return nil, fmt.Errorf("orden de Odoo %d sin id o sin date_order", id)
	}
	state := odooString(order, "state")
	_, currency := odooRef(order, "currency_id")

	record := &mysqlInfra.SaleRecord{
		ExternalID:        odooIDString(id),
		OrderNumber:       strPtr(odooString(order, "name")),
		OrderStatus:       odooOrderStatus(state),
		ExternalStatus:    strPtr(state),
		OrderedAt:         *orderedAt,
		ExternalUpdatedAt: odooTime(order, "write_date"),
		CurrencyCode:      strPtr(currency),
		ChargeSources:     []string{},
	}
	if _, ok := order["amount_total"].(float64); ok {
		record.TotalAmount = floatPtr(odooFloat(order, "amount_total"))
	}
	if _, ok := order["amount_tax"].(float64); ok {
		record.TaxAmount = floatPtr(odooFloat(order, "amount_tax"))
	}
	if raw, err := json.Marshal(order); err == nil {
		record.RawPayload = strPtr(string(raw))
	}

	mapOdooCustomer(order, related, record)
	mapOdooLines(order, related, record)
	mapOdooShipping(order, related, record, state)
	mapOdooPayments(order, related, record)

	failedModels := make([]string, 0, len(related.failed))
	for model := range related.failed {
		failedModels = append(failedModels, model)
	}
	sort.Strings(failedModels)
	for _, model := range failedModels {
		record.IncompleteReasons = append(record.IncompleteReasons, model+": "+related.failed[model])
	}
	return record, nil
}

func odooOrderStatus(state string) string {
	switch state {
	case "draft", "sent":
		return OrderPending
	case "sale", "done":
		return OrderConfirmed
	case "cancel":
		return OrderCancelled
	}
	return StatusUnknown
}

func odooAddress(partner odooInfra.Record) string {
	_, state := odooRef(partner, "state_id")
	_, country := odooRef(partner, "country_id")
	zip := odooString(partner, "zip")
	if zip != "" {
		zip = "C.P. " + zip
	}
	return joinNonEmpty(", ",
		odooString(partner, "street"), odooString(partner, "street2"),
		odooString(partner, "city"), state, zip, country)
}

func odooPartnerByField(order odooInfra.Record, related odooRelated, field string) (odooInfra.Record, bool) {
	id, _ := odooRef(order, field)
	partner, ok := related.records[odooModelPartner][id]
	return partner, ok
}

func odooPhone(partner odooInfra.Record) string {
	if phone := odooString(partner, "phone"); phone != "" {
		return phone
	}
	return odooString(partner, "mobile")
}

func mapOdooCustomer(order odooInfra.Record, related odooRelated, record *mysqlInfra.SaleRecord) {
	partnerID, partnerName := odooRef(order, "partner_id")
	record.CustomerExternalID = strPtr(odooIDString(partnerID))
	record.CustomerName = strPtr(partnerName)

	if partner, ok := odooPartnerByField(order, related, "partner_id"); ok {
		if name := odooString(partner, "name"); name != "" {
			record.CustomerName = strPtr(name)
		}
		record.CustomerEmail = strPtr(odooString(partner, "email"))
		record.CustomerPhone = strPtr(odooPhone(partner))
	}
	if invoice, ok := odooPartnerByField(order, related, "partner_invoice_id"); ok {
		record.TaxID = strPtr(odooString(invoice, "vat"))
		if record.TaxID != nil {
			record.TaxName = strPtr(odooString(invoice, "name"))
		}
		record.BillingAddress = strPtr(odooAddress(invoice))
	}
}

func mapOdooLines(order odooInfra.Record, related odooRelated, record *mysqlInfra.SaleRecord) {
	if !related.loaded(odooModelLine) {
		return
	}
	lines := related.pick(odooModelLine, odooIDs(order, "order_line"))
	items := make([]mysqlInfra.SaleItemRecord, 0, len(lines))
	var subtotal, discounts, shipping float64

	for _, line := range lines {
		// Secciones y notas no son ventas.
		if odooString(line, "display_type") != "" {
			continue
		}
		lineID := odooIDString(odooID(line))
		title := odooString(line, "name")
		total := odooFloat(line, "price_total")

		if odooBool(line, "is_delivery") {
			shipping += total
			if total != 0 {
				record.Charges = append(record.Charges, mysqlInfra.SaleChargeRecord{
					ExternalID: "shipping:" + lineID, Source: chargeSourceOrder, Type: chargeShipping,
					PaidBy: strPtr(paidByBuyer), Amount: total, Description: strPtr(title),
				})
			}
			continue
		}
		// Cupones y promociones llegan como líneas negativas.
		if total < 0 {
			discounts += -total
			record.Charges = append(record.Charges, mysqlInfra.SaleChargeRecord{
				ExternalID: "discount:" + lineID, Source: chargeSourceOrder, Type: chargeDiscount,
				Amount: -total, Description: strPtr(title),
			})
			continue
		}

		quantity := odooFloat(line, "product_uom_qty")
		listPrice := odooFloat(line, "price_unit")
		percent := odooFloat(line, "discount")
		productID, productName := odooRef(line, "product_id")
		if productName != "" {
			title = productName
		}

		item := mysqlInfra.SaleItemRecord{
			ExternalID:        lineID,
			ExternalProductID: strPtr(odooIDString(productID)),
			Title:             strPtr(title),
			Quantity:          quantity,
			UnitPrice:         floatPtr(listPrice * (1 - percent/100)),
			ListUnitPrice:     floatPtr(listPrice),
			TaxAmount:         floatPtr(odooFloat(line, "price_tax")),
			TotalAmount:       floatPtr(total),
		}
		if percent > 0 {
			discount := listPrice * quantity * percent / 100
			item.DiscountAmount = floatPtr(discount)
			discounts += discount
			record.Charges = append(record.Charges, mysqlInfra.SaleChargeRecord{
				ExternalID: "discount:" + lineID, Source: chargeSourceOrder, Type: chargeDiscount,
				PaidBy: strPtr(paidBySeller), Amount: discount, Description: strPtr("Descuento: " + title),
			})
		}
		if product, ok := related.records[odooModelProduct][productID]; ok {
			item.SKU = strPtr(odooString(product, "default_code"))
			// ecom_channel_product_map guarda el id del product.template.
			templateID, _ := odooRef(product, "product_tmpl_id")
			item.ListingExternalID = odooIDString(templateID)
		}
		subtotal += odooFloat(line, "price_subtotal")
		items = append(items, item)
	}

	record.Items = items
	record.SubtotalAmount = floatPtr(subtotal)
	record.DiscountAmount = floatPtr(discounts)
	record.ShippingAmount = floatPtr(shipping)
	if record.TaxAmount != nil && *record.TaxAmount > 0 {
		record.Charges = append(record.Charges, mysqlInfra.SaleChargeRecord{
			ExternalID: "tax:order", Source: chargeSourceOrder, Type: chargeTax,
			Amount: *record.TaxAmount, Description: strPtr("Impuestos"),
		})
	}
	record.ChargeSources = append(record.ChargeSources, chargeSourceOrder)
}

// outgoingPickings separa las entregas al cliente (las devoluciones no).
func outgoingPickings(pickings []odooInfra.Record) []odooInfra.Record {
	result := make([]odooInfra.Record, 0, len(pickings))
	for _, picking := range pickings {
		if code := odooString(picking, "picking_type_code"); code == "" || code == "outgoing" {
			result = append(result, picking)
		}
	}
	return result
}

// odooShippingStatus: "done" en Odoo significa que salió del almacén, no que
// el cliente lo recibió.
func odooShippingStatus(orderState string, pickings []odooInfra.Record) string {
	if orderState == "cancel" {
		return ShippingCancelled
	}
	var active, done, assigned int
	for _, picking := range pickings {
		switch odooString(picking, "state") {
		case "cancel":
			continue
		case "done":
			done++
		case "assigned":
			assigned++
		}
		active++
	}
	switch {
	case active == 0 && len(pickings) > 0:
		return ShippingCancelled
	case active == 0 && orderState == "sale":
		return ShippingNotApplicable
	case active == 0:
		return ShippingPending
	case done == active:
		return ShippingShipped
	case assigned > 0:
		return ShippingReadyToShip
	}
	return ShippingPending
}

func mapOdooShipping(order odooInfra.Record, related odooRelated, record *mysqlInfra.SaleRecord, state string) {
	// Si la lectura falló no se toca lo guardado.
	if _, failed := related.failed[odooModelPicking]; failed {
		return
	}
	var receiverName, receiverPhone, receiverAddress string
	if partner, ok := odooPartnerByField(order, related, "partner_shipping_id"); ok {
		receiverName = odooString(partner, "name")
		receiverPhone = odooPhone(partner)
		receiverAddress = odooAddress(partner)
	}
	_, orderCarrier := odooRef(order, "carrier_id")

	_, linksPickings := order["picking_ids"]
	var pickings []odooInfra.Record
	if linksPickings && related.loaded(odooModelPicking) {
		pickings = outgoingPickings(related.pick(odooModelPicking, odooIDs(order, "picking_ids")))
		record.ShippingStatus = strPtr(odooShippingStatus(state, pickings))
	} else {
		switch odooString(order, "delivery_status") {
		case "pending", "started":
			record.ShippingStatus = strPtr(ShippingPending)
		case "partial", "full":
			record.ShippingStatus = strPtr(ShippingShipped)
		}
	}

	shipments := make([]mysqlInfra.SaleShipmentRecord, 0, len(pickings))
	for _, picking := range pickings {
		_, carrier := odooRef(picking, "carrier_id")
		if carrier == "" {
			carrier = orderCarrier
		}
		pickingState := odooString(picking, "state")
		shipment := mysqlInfra.SaleShipmentRecord{
			ExternalID:      odooIDString(odooID(picking)),
			ExternalStatus:  strPtr(pickingState),
			Carrier:         strPtr(carrier),
			TrackingNumber:  strPtr(odooString(picking, "carrier_tracking_ref")),
			ReceiverName:    strPtr(receiverName),
			ReceiverPhone:   strPtr(receiverPhone),
			ReceiverAddress: strPtr(receiverAddress),
		}
		switch pickingState {
		case "done":
			shipment.ShippedAt = odooTime(picking, "date_done")
		case "cancel":
		default:
			shipment.DispatchDeadline = odooTime(picking, "scheduled_date")
		}
		shipments = append(shipments, shipment)
	}
	// Sin entregas, se guarda al menos a dónde va el pedido.
	if len(shipments) == 0 && joinNonEmpty("", receiverAddress, orderCarrier) != "" {
		shipments = append(shipments, mysqlInfra.SaleShipmentRecord{
			ExternalID:      "order-" + record.ExternalID,
			Carrier:         strPtr(orderCarrier),
			ReceiverName:    strPtr(receiverName),
			ReceiverPhone:   strPtr(receiverPhone),
			ReceiverAddress: strPtr(receiverAddress),
		})
	}
	record.Shipments = shipments
}

func mapOdooPayments(order odooInfra.Record, related odooRelated, record *mysqlInfra.SaleRecord) {
	_, paymentsFailed := related.failed[odooModelPayment]
	_, invoicesFailed := related.failed[odooModelInvoice]

	transactions := related.pick(odooModelPayment, odooIDs(order, "transaction_ids"))
	invoices := related.pick(odooModelInvoice, odooIDs(order, "invoice_ids"))
	_, linksTransactions := order["transaction_ids"]
	_, linksInvoices := order["invoice_ids"]
	hasTransactions := !paymentsFailed && linksTransactions && related.loaded(odooModelPayment)
	hasInvoices := !invoicesFailed && linksInvoices && related.loaded(odooModelInvoice)

	if hasTransactions {
		payments := make([]mysqlInfra.SalePaymentRecord, 0, len(transactions))
		paid := 0.0
		for _, transaction := range transactions {
			_, method := odooRef(transaction, "payment_method_id")
			_, provider := odooRef(transaction, "provider_id")
			state := odooString(transaction, "state")
			amount := odooFloat(transaction, "amount")
			payment := mysqlInfra.SalePaymentRecord{
				ExternalID:     odooIDString(odooID(transaction)),
				Method:         strPtr(joinNonEmpty(" / ", provider, method)),
				ExternalStatus: strPtr(state),
				Amount:         floatPtr(amount),
			}
			if state == "done" {
				paid += amount
				payment.ApprovedAt = odooTime(transaction, "last_state_change")
			}
			payments = append(payments, payment)
		}
		record.Payments = payments
		if len(transactions) > 0 {
			record.PaidAmount = floatPtr(paid)
		}
	}

	// Un estado calculado con datos a medias podría ser falso.
	if paymentsFailed || invoicesFailed || (!hasTransactions && !hasInvoices) {
		return
	}
	record.PaymentStatus = strPtr(odooPaymentStatus(odooFloat(order, "amount_total"), transactions, invoices))
}

func odooPaymentStatus(total float64, transactions, invoices []odooInfra.Record) string {
	const tolerance = 0.01
	var refunded, collected float64
	var invoicePaid, invoiceReversed bool
	var open, failed int

	for _, invoice := range invoices {
		if odooString(invoice, "state") != "posted" {
			continue
		}
		switch odooString(invoice, "move_type") {
		case "out_refund":
			refunded += odooFloat(invoice, "amount_total")
		case "out_invoice":
			switch odooString(invoice, "payment_state") {
			case "paid", "in_payment":
				invoicePaid = true
			case "reversed":
				invoiceReversed = true
			}
		}
	}
	for _, transaction := range transactions {
		switch strings.ToLower(odooString(transaction, "state")) {
		case "done":
			if amount := odooFloat(transaction, "amount"); amount > 0 {
				collected += amount
			}
		case "draft", "pending", "authorized":
			open++
		case "error", "cancel":
			failed++
		}
	}

	switch {
	case invoiceReversed || (refunded > tolerance && refunded >= total-tolerance):
		return PaymentRefunded
	case refunded > tolerance:
		return PaymentPartiallyRefunded
	case invoicePaid || (collected > tolerance && collected >= total-tolerance):
		return PaymentPaid
	case collected == 0 && open == 0 && failed > 0:
		return PaymentRejected
	}
	return PaymentPending
}

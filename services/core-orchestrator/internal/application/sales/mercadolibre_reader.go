package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

const (
	mercadoLibreChannelCode = "MERCADOLIBRE"
	mercadoLibrePageSize    = 50
)

// MercadoLibreTokenEnsurer entrega un access token vigente de la conexión.
type MercadoLibreTokenEnsurer interface {
	EnsureValidAccessToken(ctx context.Context, connectionID int64) (string, error)
}

type MercadoLibreReader struct {
	sales  *mercadoLibreInfra.SalesHandler
	users  *mercadoLibreInfra.UsersHandler
	tokens MercadoLibreTokenEnsurer
}

func NewMercadoLibreReader(sales *mercadoLibreInfra.SalesHandler, users *mercadoLibreInfra.UsersHandler, tokens MercadoLibreTokenEnsurer) *MercadoLibreReader {
	return &MercadoLibreReader{sales: sales, users: users, tokens: tokens}
}

func (r *MercadoLibreReader) ChannelCode() string { return mercadoLibreChannelCode }

func (r *MercadoLibreReader) Open(ctx context.Context, connectionID int64) (Session, error) {
	token, err := r.tokens.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("obteniendo el access token: %w", err)
	}
	me, err := r.users.GetMe(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("identificando al vendedor: %w", err)
	}
	if me.ID <= 0 {
		return nil, errors.New("MercadoLibre no devolvió el id del vendedor")
	}
	return &mercadoLibreSession{reader: r, connectionID: connectionID, sellerID: me.ID}, nil
}

type mercadoLibreSession struct {
	reader       *MercadoLibreReader
	connectionID int64
	sellerID     int64
}

func (s *mercadoLibreSession) Warnings() []string { return nil }

// token se pide en cada consulta: una carga larga puede sobrevivir al token.
func (s *mercadoLibreSession) token(ctx context.Context) (string, error) {
	return s.reader.tokens.EnsureValidAccessToken(ctx, s.connectionID)
}

// mercadoLibreSearchDate trunca a la hora, como indica MercadoLibre. Truncar
// solo amplía la ventana.
func mercadoLibreSearchDate(value time.Time) string {
	return value.UTC().Truncate(time.Hour).Format("2006-01-02T15:04:05.000-00:00")
}

func (s *mercadoLibreSession) Search(ctx context.Context, window Window, handle HandleFunc) error {
	dateField := "date_last_updated"
	if window.Initial {
		dateField = "date_created"
	}
	from := mercadoLibreSearchDate(window.From)

	for offset := 0; ; {
		token, err := s.token(ctx)
		if err != nil {
			return fmt.Errorf("obteniendo el access token: %w", err)
		}
		page, err := s.reader.sales.SearchOrders(ctx, token, mercadoLibreInfra.SearchOrdersParams{
			SellerID:  s.sellerID,
			DateField: dateField,
			From:      from,
			Offset:    offset,
			Limit:     mercadoLibrePageSize,
		})
		if err != nil {
			return fmt.Errorf("buscando órdenes (offset %d): %w", offset, err)
		}

		for _, result := range page.Orders {
			if err := handle(ctx, s.searchCandidate(result)); err != nil {
				return err
			}
		}

		offset += len(page.Orders)
		if len(page.Orders) == 0 || offset >= page.Total {
			return nil
		}
	}
}

func (s *mercadoLibreSession) searchCandidate(result mercadoLibreInfra.SalesSearchResult) Candidate {
	orderID := string(result.Order.ID)
	if orderID == "" {
		var minimal struct {
			ID mercadoLibreInfra.FlexString `json:"id"`
		}
		_ = json.Unmarshal(result.Raw, &minimal)
		orderID = string(minimal.ID)
	}
	candidate := s.candidate(orderID)
	if result.Err != nil {
		// Con una decodificación parcial no se puede confiar en la fecha: se
		// consulta el detalle, que reportará el error si persiste.
		return candidate
	}
	candidate.ExternalUpdatedAt = orderUpdatedAt(result.Order)
	return candidate
}

func (s *mercadoLibreSession) candidate(orderID string) Candidate {
	return Candidate{
		ExternalID: orderID,
		Load: func(ctx context.Context, known *mysqlInfra.SaleKnownState) (*mysqlInfra.SaleRecord, error) {
			return s.load(ctx, orderID, known)
		},
	}
}

func (s *mercadoLibreSession) Lookup(ctx context.Context, externalIDs []string, handle HandleFunc) error {
	for _, id := range externalIDs {
		if err := handle(ctx, s.candidate(id)); err != nil {
			return err
		}
	}
	return nil
}

func (s *mercadoLibreSession) load(ctx context.Context, orderID string, known *mysqlInfra.SaleKnownState) (*mysqlInfra.SaleRecord, error) {
	if orderID == "" {
		return nil, errors.New("la búsqueda devolvió una orden sin id")
	}
	token, err := s.token(ctx)
	if err != nil {
		return nil, fmt.Errorf("obteniendo el access token: %w", err)
	}
	order, raw, err := s.reader.sales.GetOrder(ctx, token, orderID)
	if err != nil {
		return nil, err
	}
	record, err := mapMercadoLibreOrder(order)
	if err != nil {
		return nil, err
	}
	record.RawPayload = strPtr(string(raw))

	s.loadShipment(ctx, token, order, record)
	if known == nil || !known.HasBilling {
		s.loadBilling(ctx, token, order, record)
	}
	// Una corrida cancelada no debe guardar la venta como incompleta.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *mercadoLibreSession) loadShipment(ctx context.Context, token string, order *mercadoLibreInfra.SalesOrder, record *mysqlInfra.SaleRecord) {
	shipmentID := string(order.Shipping.ID)
	if shipmentID == "" {
		// Sin la etiqueta explícita no se asume que no hay envío: una
		// respuesta parcial no debe borrar el que ya estaba guardado.
		if hasTag(order.Tags, "no_shipping") {
			record.ShippingStatus = strPtr(ShippingNotApplicable)
			record.Shipments = []mysqlInfra.SaleShipmentRecord{}
		}
		return
	}

	shipment, raw, err := s.reader.sales.GetShipment(ctx, token, shipmentID)
	if err != nil {
		record.IncompleteReasons = append(record.IncompleteReasons, "envío: "+err.Error())
		return
	}
	mapped := mapMercadoLibreShipment(shipmentID, shipment)
	mapped.RawPayload = strPtr(string(raw))
	record.ShippingStatus = strPtr(mercadoLibreShippingStatus(shipment.Status))

	costs, err := s.reader.sales.GetShipmentCosts(ctx, token, shipmentID)
	switch {
	case err == nil:
		applyMercadoLibreShipmentCosts(record, &mapped, shipmentID, costs)
	case !mercadoLibreInfra.IsSalesNotFound(err):
		record.IncompleteReasons = append(record.IncompleteReasons, "costos de envío: "+err.Error())
	}

	if needsDispatchDeadline(shipment.Status, deref(mapped.LogisticType)) {
		deadline, err := s.reader.sales.GetShipmentDispatchDeadline(ctx, token, shipmentID)
		switch {
		case err == nil:
			if parsed := parseMercadoLibreTime(deadline); parsed != nil {
				mapped.DispatchDeadline = parsed
			}
		// MercadoLibre responde 404 cuando no hay SLA para el envío.
		case !mercadoLibreInfra.IsSalesNotFound(err):
			record.IncompleteReasons = append(record.IncompleteReasons, "fecha límite de despacho: "+err.Error())
		}
	}

	record.Shipments = []mysqlInfra.SaleShipmentRecord{mapped}
}

// needsDispatchDeadline: solo antes de despachar y nunca en Fulfillment.
func needsDispatchDeadline(status, logisticType string) bool {
	if strings.EqualFold(logisticType, "fulfillment") {
		return false
	}
	switch status {
	case "pending", "handling", "ready_to_ship":
		return true
	}
	return false
}

func (s *mercadoLibreSession) loadBilling(ctx context.Context, token string, order *mercadoLibreInfra.SalesOrder, record *mysqlInfra.SaleRecord) {
	if order.Buyer.BillingInfo == nil || order.Buyer.BillingInfo.ID == "" {
		return
	}
	site := mercadoLibreSite(order)
	if site == "" {
		return
	}
	info, raw, err := s.reader.sales.GetBillingInfo(ctx, token, site, string(order.Buyer.BillingInfo.ID))
	if err != nil {
		if !mercadoLibreInfra.IsSalesNotFound(err) {
			record.IncompleteReasons = append(record.IncompleteReasons, "facturación: "+err.Error())
		}
		return
	}
	applyMercadoLibreBilling(record, info)
	record.BillingRawPayload = strPtr(string(raw))
}

// mercadoLibreSite toma el sitio (MLM) de la orden o, si no viene, del prefijo
// de la publicación.
func mercadoLibreSite(order *mercadoLibreInfra.SalesOrder) string {
	if order.Context.Site != "" {
		return order.Context.Site
	}
	if len(order.Items) > 0 {
		if id := string(order.Items[0].Item.ID); len(id) > 3 {
			return id[:3]
		}
	}
	return ""
}

func orderUpdatedAt(order mercadoLibreInfra.SalesOrder) *time.Time {
	if parsed := parseMercadoLibreTime(order.LastUpdated); parsed != nil {
		return parsed
	}
	return parseMercadoLibreTime(order.DateLastUpdated)
}

func parseMercadoLibreTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &parsed
}

func mapMercadoLibreOrder(order *mercadoLibreInfra.SalesOrder) (*mysqlInfra.SaleRecord, error) {
	orderID := string(order.ID)
	createdAt := parseMercadoLibreTime(order.DateCreated)
	if orderID == "" || createdAt == nil {
		return nil, fmt.Errorf("orden %q sin id o sin fecha de creación", orderID)
	}

	record := &mysqlInfra.SaleRecord{
		ExternalID:           orderID,
		OrderNumber:          strPtr(orderID),
		PackID:               strPtr(string(order.PackID)),
		OrderStatus:          mercadoLibreOrderStatus(order.Status),
		ExternalStatus:       strPtr(order.Status),
		ExternalStatusDetail: strPtr(mercadoLibreStatusDetail(order.StatusDetail)),
		OrderedAt:            *createdAt,
		ClosedAt:             parseMercadoLibreTime(order.DateClosed),
		ExternalUpdatedAt:    orderUpdatedAt(*order),
		CustomerExternalID:   strPtr(string(order.Buyer.ID)),
		CustomerName:         strPtr(joinNonEmpty(" ", order.Buyer.FirstName, order.Buyer.LastName)),
		CustomerNickname:     strPtr(order.Buyer.Nickname),
		CustomerEmail:        strPtr(order.Buyer.Email),
		CurrencyCode:         strPtr(order.CurrencyID),
		TotalAmount:          order.TotalAmount,
		PaidAmount:           order.PaidAmount,
		ChargeSources:        []string{},
	}
	if phone := order.Buyer.Phone; phone != nil && phone.Number != "" {
		record.CustomerPhone = strPtr(joinNonEmpty(" ", string(phone.AreaCode), string(phone.Number)))
	}
	if order.Coupon != nil {
		record.DiscountAmount = order.Coupon.Amount
	}
	if order.Taxes != nil {
		record.TaxAmount = order.Taxes.Amount
	}

	mapMercadoLibreItems(order, record)
	mapMercadoLibrePayments(order, record)
	record.PaymentStatus = mercadoLibrePaymentStatus(order)
	return record, nil
}

func mapMercadoLibreItems(order *mercadoLibreInfra.SalesOrder, record *mysqlInfra.SaleRecord) {
	// Sin líneas = respuesta parcial: se conservan las guardadas.
	if len(order.Items) == 0 {
		return
	}
	items := make([]mysqlInfra.SaleItemRecord, 0, len(order.Items))
	seen := make(map[string]int, len(order.Items))
	subtotal, fees := 0.0, 0.0
	hasAllPrices, hasAllFees := true, true

	for _, line := range order.Items {
		listingID := string(line.Item.ID)
		key := listingID + ":" + string(line.Item.VariationID)
		seen[key]++
		if seen[key] > 1 {
			key = fmt.Sprintf("%s#%d", key, seen[key])
		}
		sku := string(line.Item.SellerSKU)
		if sku == "" {
			sku = string(line.Item.SellerCustomField)
		}

		item := mysqlInfra.SaleItemRecord{
			ExternalID:          key,
			ExternalProductID:   strPtr(listingID),
			ExternalVariationID: strPtr(string(line.Item.VariationID)),
			ListingExternalID:   listingID,
			SKU:                 strPtr(sku),
			Title:               strPtr(line.Item.Title),
			Quantity:            line.Quantity,
			UnitPrice:           line.UnitPrice,
			ListUnitPrice:       line.FullUnitPrice,
		}
		if line.UnitPrice != nil {
			total := *line.UnitPrice * line.Quantity
			item.TotalAmount = floatPtr(total)
			subtotal += total
			if line.FullUnitPrice != nil && *line.FullUnitPrice > *line.UnitPrice {
				item.DiscountAmount = floatPtr((*line.FullUnitPrice - *line.UnitPrice) * line.Quantity)
			}
		} else {
			hasAllPrices = false
		}
		if line.SaleFee != nil {
			fee := *line.SaleFee * line.Quantity
			item.FeeAmount = floatPtr(fee)
			fees += fee
		} else {
			hasAllFees = false
		}

		for index, discount := range line.Discounts {
			sellerAmount := discount.Amounts.Seller
			parts := []struct {
				paidBy string
				amount float64
			}{
				{paidBySeller, sellerAmount},
				{paidByMarketplace, discount.Amounts.Full - sellerAmount},
			}
			for _, part := range parts {
				if part.amount <= 0.004 {
					continue
				}
				record.Charges = append(record.Charges, mysqlInfra.SaleChargeRecord{
					ExternalID:  fmt.Sprintf("discount:%s:%d:%s", key, index, part.paidBy),
					Source:      chargeSourceOrder,
					Type:        chargeDiscount,
					PaidBy:      strPtr(part.paidBy),
					Amount:      part.amount,
					Description: strPtr("Descuento aplicado al producto"),
				})
			}
		}
		items = append(items, item)
	}

	record.Items = items
	if hasAllPrices {
		record.SubtotalAmount = floatPtr(subtotal)
	}
	if hasAllFees {
		record.MarketplaceFeeAmount = floatPtr(fees)
		if fees > 0 {
			record.Charges = append(record.Charges, mysqlInfra.SaleChargeRecord{
				ExternalID: "fee:sale", Source: chargeSourceOrder, Type: chargeFee,
				PaidBy: strPtr(paidBySeller), Amount: fees, Description: strPtr("Comisión por venta"),
			})
		}
	}
	if record.DiscountAmount != nil && *record.DiscountAmount > 0 {
		record.Charges = append(record.Charges, mysqlInfra.SaleChargeRecord{
			ExternalID: "discount:coupon", Source: chargeSourceOrder, Type: chargeDiscount,
			Amount: *record.DiscountAmount, Description: strPtr("Cupón"),
		})
	}
	if record.TaxAmount != nil && *record.TaxAmount > 0 {
		record.Charges = append(record.Charges, mysqlInfra.SaleChargeRecord{
			ExternalID: "tax:order", Source: chargeSourceOrder, Type: chargeTax,
			Amount: *record.TaxAmount, Description: strPtr("Impuestos"),
		})
	}
	record.ChargeSources = append(record.ChargeSources, chargeSourceOrder)
}

func mapMercadoLibrePayments(order *mercadoLibreInfra.SalesOrder, record *mysqlInfra.SaleRecord) {
	if len(order.Payments) == 0 {
		return
	}
	payments := make([]mysqlInfra.SalePaymentRecord, 0, len(order.Payments))
	for _, payment := range order.Payments {
		if payment.ID == "" {
			continue
		}
		payments = append(payments, mysqlInfra.SalePaymentRecord{
			ExternalID:     string(payment.ID),
			Method:         strPtr(payment.PaymentMethodID),
			PaymentType:    strPtr(payment.PaymentType),
			Installments:   payment.Installments,
			ExternalStatus: strPtr(payment.Status),
			Amount:         payment.TransactionAmount,
			RefundedAmount: payment.TransactionAmountRefunded,
			ApprovedAt:     parseMercadoLibreTime(payment.DateApproved),
		})
	}
	record.Payments = payments
}

func mercadoLibreOrderStatus(status string) string {
	switch status {
	case "paid", "partially_refunded":
		return OrderConfirmed
	case "confirmed", "payment_required", "payment_in_process", "partially_paid", "pending_cancel":
		return OrderPending
	case "cancelled", "invalid":
		return OrderCancelled
	}
	return StatusUnknown
}

// mercadoLibrePaymentStatus resume los pagos de la orden. nil = la respuesta
// no trae información suficiente.
func mercadoLibrePaymentStatus(order *mercadoLibreInfra.SalesOrder) *string {
	if len(order.Payments) == 0 {
		switch order.Status {
		case "paid":
			return strPtr(PaymentPaid)
		case "confirmed", "payment_required", "payment_in_process", "partially_paid":
			return strPtr(PaymentPending)
		}
		return nil
	}

	var charged, refunded float64
	var approved, pending, rejected, refundedPayments, unknown int
	for _, payment := range order.Payments {
		amount, back := 0.0, 0.0
		if payment.TransactionAmount != nil {
			amount = *payment.TransactionAmount
		}
		if payment.TransactionAmountRefunded != nil {
			back = *payment.TransactionAmountRefunded
		}
		switch payment.Status {
		case "approved":
			approved++
			charged += amount
			refunded += back
		case "refunded", "charged_back":
			refundedPayments++
			charged += amount
			refunded += amount
		case "pending", "in_process", "authorized", "in_mediation":
			pending++
		case "rejected", "cancelled":
			rejected++
		default:
			unknown++
		}
	}

	switch {
	case refunded > 0.004 && refunded >= charged-0.004:
		return strPtr(PaymentRefunded)
	case refunded > 0.004:
		return strPtr(PaymentPartiallyRefunded)
	case refundedPayments > 0 && approved == 0:
		return strPtr(PaymentRefunded)
	case order.Status == "partially_paid":
		return strPtr(PaymentPending)
	case approved > 0:
		return strPtr(PaymentPaid)
	case pending > 0:
		return strPtr(PaymentPending)
	case rejected > 0 && unknown == 0:
		return strPtr(PaymentRejected)
	}
	return strPtr(StatusUnknown)
}

func mercadoLibreShippingStatus(status string) string {
	switch status {
	case "pending", "handling":
		return ShippingPending
	case "ready_to_ship":
		return ShippingReadyToShip
	case "shipped":
		return ShippingShipped
	case "delivered":
		return ShippingDelivered
	case "not_delivered":
		return ShippingNotDelivered
	case "cancelled":
		return ShippingCancelled
	case "to_be_agreed":
		return ShippingNotApplicable
	}
	return StatusUnknown
}

// mercadoLibreStatusDetail admite status_detail como texto u objeto.
func mercadoLibreStatusDetail(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var object struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	}
	if json.Unmarshal(raw, &object) == nil {
		return joinNonEmpty(": ", object.Code, object.Description)
	}
	return ""
}

func mapMercadoLibreShipment(shipmentID string, s *mercadoLibreInfra.SalesShipment) mysqlInfra.SaleShipmentRecord {
	mode, logisticType := s.Mode, s.LogisticType
	if mode == "" {
		mode = s.Logistic.Mode
	}
	if logisticType == "" {
		logisticType = s.Logistic.Type
	}
	record := mysqlInfra.SaleShipmentRecord{
		ExternalID:        shipmentID,
		ExternalStatus:    strPtr(s.Status),
		ExternalSubstatus: strPtr(s.Substatus),
		Mode:              strPtr(mode),
		LogisticType:      strPtr(logisticType),
		Carrier:           strPtr(s.TrackingMethod),
		TrackingNumber:    strPtr(string(s.TrackingNumber)),
		ShippedAt:         parseMercadoLibreTime(s.StatusHistory.DateShipped),
		DeliveredAt:       parseMercadoLibreTime(s.StatusHistory.DateDelivered),
	}

	address := s.ReceiverAddress
	receiverName, receiverPhone := "", ""
	if s.Destination != nil {
		receiverName, receiverPhone = s.Destination.ReceiverName, string(s.Destination.ReceiverPhone)
		if s.Destination.ShippingAddress != nil {
			address = s.Destination.ShippingAddress
		}
	}
	if address != nil {
		if receiverName == "" {
			receiverName = address.ReceiverName
		}
		if receiverPhone == "" {
			receiverPhone = string(address.ReceiverPhone)
		}
		record.ReceiverAddress = strPtr(formatMercadoLibreAddress(address))
	}
	record.ReceiverName = strPtr(receiverName)
	record.ReceiverPhone = strPtr(receiverPhone)

	if s.LeadTime != nil {
		record.EstimatedDeliveryAt = parseMercadoLibreTime(s.LeadTime.EstimatedDeliveryTime.Date)
		record.DispatchDeadline = parseMercadoLibreTime(s.LeadTime.EstimatedHandlingLimit.Date)
	}
	if record.EstimatedDeliveryAt == nil && s.ShippingOption != nil {
		record.EstimatedDeliveryAt = parseMercadoLibreTime(s.ShippingOption.EstimatedDeliveryTime.Date)
	}
	return record
}

func formatMercadoLibreAddress(a *mercadoLibreInfra.SalesAddress) string {
	street := joinNonEmpty(" ", a.StreetName, string(a.StreetNumber))
	if street == "" {
		street = a.AddressLine
	}
	zip := ""
	if a.ZipCode != "" {
		zip = "C.P. " + string(a.ZipCode)
	}
	return joinNonEmpty(", ", street, a.Neighborhood.Name, a.City.Name, a.State.Name, zip, a.Country.Name, a.Comment)
}

func applyMercadoLibreShipmentCosts(record *mysqlInfra.SaleRecord, shipment *mysqlInfra.SaleShipmentRecord, shipmentID string, costs *mercadoLibreInfra.SalesShipmentCosts) {
	if costs.Receiver != nil && costs.Receiver.Cost != nil {
		shipment.BuyerCost = costs.Receiver.Cost
		record.ShippingAmount = costs.Receiver.Cost
		if *costs.Receiver.Cost > 0 {
			record.Charges = append(record.Charges, mysqlInfra.SaleChargeRecord{
				ExternalID: "shipping:buyer:" + shipmentID, Source: chargeSourceShipping, Type: chargeShipping,
				PaidBy: strPtr(paidByBuyer), Amount: *costs.Receiver.Cost, Description: strPtr("Envío pagado por el comprador"),
			})
		}
	}
	if len(costs.Senders) > 0 {
		total := 0.0
		for _, sender := range costs.Senders {
			total += sender.Cost
		}
		shipment.SellerCost = floatPtr(total)
		record.ShippingCostSeller = floatPtr(total)
		if total > 0 {
			record.Charges = append(record.Charges, mysqlInfra.SaleChargeRecord{
				ExternalID: "shipping:seller:" + shipmentID, Source: chargeSourceShipping, Type: chargeShipping,
				PaidBy: strPtr(paidBySeller), Amount: total, Description: strPtr("Envío a cargo del vendedor"),
			})
		}
	}
	record.ChargeSources = append(record.ChargeSources, chargeSourceShipping)
}

func applyMercadoLibreBilling(record *mysqlInfra.SaleRecord, info *mercadoLibreInfra.SalesBillingInfo) {
	billing := info.Buyer.BillingInfo
	record.TaxID = strPtr(string(billing.Identification.Number))
	record.TaxName = strPtr(joinNonEmpty(" ", billing.Name, billing.LastName))
	record.TaxRegime = strPtr(joinNonEmpty(" - ", string(billing.Taxes.TaxpayerType.ID), billing.Taxes.TaxpayerType.Description))
	record.CFDIUse = strPtr(joinNonEmpty(" - ", string(billing.Taxes.CFDI.ID), billing.Taxes.CFDI.Description))
	zip := ""
	if billing.Address.ZipCode != "" {
		zip = "C.P. " + string(billing.Address.ZipCode)
	}
	record.BillingAddress = strPtr(joinNonEmpty(", ",
		joinNonEmpty(" ", billing.Address.StreetName, string(billing.Address.StreetNumber)),
		billing.Address.CityName, billing.Address.State.Name, zip, billing.Address.CountryID))
}

func hasTag(tags []string, tag string) bool {
	for _, candidate := range tags {
		if candidate == tag {
			return true
		}
	}
	return false
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

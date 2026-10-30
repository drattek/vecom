package meli_notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// mercadoLibreChannelCode matches ecom_channels.code case-insensitively (see
// internal/interfaces/schedulers/token_refresh_scheduler.go for the same
// convention). Only connections for this channel are considered — Odoo etc.
// never receive MELI webhooks.
const mercadoLibreChannelCode = "MERCADOLIBRE"

// maxProcessingAttempts bounds how many times a single notification cycles
// through MarkFailed before it's parked at status='failed'. MELI's own
// retries are separate (they happen before Receive is called) — this is
// strictly local retry budget.
const maxProcessingAttempts = 10

// topicUnsupportedShadow is written to last_error when the processor
// marks a notification 'done' without having actually resolved its
// resource (because the topic hasn't been wired yet). The row doesn't
// get retried so it stops polluting backlog counts.
const topicUnsupportedShadow = "shadow: topic not handled yet (parked, no retry)"

// MercadoLibreTokenEnsurer narrows the token service surface the processor
// needs — kept small so tests can fake it without pulling the full sync
// package.
type MercadoLibreTokenEnsurer interface {
	EnsureValidAccessToken(ctx context.Context, connectionID int64) (string, error)
}

// ProcessorConfig bundles tunables for the shadow processor.
type ProcessorConfig struct {
	// BatchSize caps how many pending rows one Run processes.
	BatchSize int
}

// Processor consumes rows from ecom_meli_notifications and, per topic,
// enriches the row with the resource MELI points to (orders_v2 →
// ecom_meli_orders, shipments → ecom_meli_shipments). Shadow by design:
// nothing goes to Odoo from here; fase 2 adds that consumer.
type Processor struct {
	notificationRepository      *mysqlInfra.MeliNotificationRepository
	orderRepository             *mysqlInfra.MeliOrderRepository
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository
	ordersHandler               *mercadoLibreInfra.OrdersHandler
	shipmentsHandler            *mercadoLibreInfra.ShipmentsHandler
	usersHandler                *mercadoLibreInfra.UsersHandler
	tokenEnsurer                MercadoLibreTokenEnsurer
	config                      ProcessorConfig

	// userToConnection caches meli_user_id → connection_id. Rebuilt the
	// first time it's needed, and whenever a notification arrives with a
	// user id not in the cache (a seller may have authorized a new
	// connection since the processor started).
	userToConnection      map[int64]int64
	userToConnectionMutex sync.RWMutex
}

func NewProcessor(
	notificationRepository *mysqlInfra.MeliNotificationRepository,
	orderRepository *mysqlInfra.MeliOrderRepository,
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	ordersHandler *mercadoLibreInfra.OrdersHandler,
	shipmentsHandler *mercadoLibreInfra.ShipmentsHandler,
	usersHandler *mercadoLibreInfra.UsersHandler,
	tokenEnsurer MercadoLibreTokenEnsurer,
	config ProcessorConfig,
) *Processor {
	if config.BatchSize <= 0 {
		config.BatchSize = 50
	}
	return &Processor{
		notificationRepository:      notificationRepository,
		orderRepository:             orderRepository,
		channelConnectionRepository: channelConnectionRepository,
		ordersHandler:               ordersHandler,
		shipmentsHandler:            shipmentsHandler,
		usersHandler:                usersHandler,
		tokenEnsurer:                tokenEnsurer,
		config:                      config,
		userToConnection:            make(map[int64]int64),
	}
}

// RunOnce drains up to BatchSize pending notifications. The scheduler
// calls this on a tick; it is also safe to call manually from an admin
// endpoint if one is added later.
func (p *Processor) RunOnce(ctx context.Context) error {
	pending, err := p.notificationRepository.FindPending(ctx, p.config.BatchSize)
	if err != nil {
		return fmt.Errorf("fetching pending meli notifications: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	log.Printf("meli notifications processor: picked up %d pending", len(pending))

	for _, n := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := p.processOne(ctx, n); err != nil {
			// processOne already stamps MarkFailed. Log + continue.
			log.Printf("meli notifications processor: notif id=%d topic=%s resource=%s failed: %v",
				n.ID, n.Topic, n.Resource, err)
			continue
		}
	}
	return nil
}

func (p *Processor) processOne(ctx context.Context, n mysqlInfra.MeliNotificationDTO) error {
	connectionID, err := p.resolveConnectionID(ctx, n.MeliUserID)
	if err != nil {
		if stampErr := p.notificationRepository.MarkFailed(ctx, n.ID, "resolving connection_id: "+err.Error(), maxProcessingAttempts); stampErr != nil {
			log.Printf("meli notifications processor: notif id=%d, could not mark failed after connection resolve failure: %v", n.ID, stampErr)
		}
		return err
	}
	if n.ConnectionID == nil || *n.ConnectionID != connectionID {
		if err := p.notificationRepository.UpdateConnectionID(ctx, n.ID, connectionID); err != nil {
			return fmt.Errorf("stamping connection_id on notif %d: %w", n.ID, err)
		}
	}

	switch strings.ToLower(n.Topic) {
	case "orders_v2", "orders":
		return p.handleOrder(ctx, n, connectionID)
	case "shipments":
		return p.handleShipment(ctx, n, connectionID)
	default:
		// Park topics we don't handle yet — keeps backlog from piling up
		// forever on items/user_items_prices/stock-locations the sync
		// scheduler itself already covers.
		if err := p.notificationRepository.MarkDone(ctx, n.ID, "", topicUnsupportedShadow); err != nil {
			return fmt.Errorf("parking unsupported topic %q on notif %d: %w", n.Topic, n.ID, err)
		}
		return nil
	}
}

func (p *Processor) handleOrder(ctx context.Context, n mysqlInfra.MeliNotificationDTO, connectionID int64) error {
	orderID := extractResourceID(n.Resource, "/orders/")
	if orderID == "" {
		if err := p.notificationRepository.MarkFailed(ctx, n.ID, "could not extract order id from resource "+n.Resource, maxProcessingAttempts); err != nil {
			log.Printf("meli notifications processor: mark failed on bad resource for notif id=%d: %v", n.ID, err)
		}
		return fmt.Errorf("bad orders_v2 resource %q", n.Resource)
	}

	accessToken, err := p.tokenEnsurer.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		if stampErr := p.notificationRepository.MarkFailed(ctx, n.ID, "fetching access token: "+err.Error(), maxProcessingAttempts); stampErr != nil {
			log.Printf("meli notifications processor: notif id=%d, could not mark failed after token refresh failure: %v", n.ID, stampErr)
		}
		return err
	}

	order, rawBytes, err := p.ordersHandler.GetOrder(ctx, accessToken, orderID)
	if err != nil {
		if stampErr := p.notificationRepository.MarkFailed(ctx, n.ID, "GET /orders/"+orderID+": "+err.Error(), maxProcessingAttempts); stampErr != nil {
			log.Printf("meli notifications processor: notif id=%d, could not mark failed after order fetch failure: %v", n.ID, stampErr)
		}
		return err
	}

	notifID := n.ID
	upsertInput := buildOrderUpsert(order, connectionID, &notifID, string(rawBytes))

	if _, err := p.orderRepository.Upsert(ctx, upsertInput); err != nil {
		if stampErr := p.notificationRepository.MarkFailed(ctx, n.ID, "upserting ecom_meli_orders: "+err.Error(), maxProcessingAttempts); stampErr != nil {
			log.Printf("meli notifications processor: notif id=%d, could not mark failed after upsert failure: %v", n.ID, stampErr)
		}
		return err
	}

	if err := p.notificationRepository.MarkDone(ctx, n.ID, string(rawBytes), ""); err != nil {
		return fmt.Errorf("marking order notif %d done: %w", n.ID, err)
	}
	return nil
}

func (p *Processor) handleShipment(ctx context.Context, n mysqlInfra.MeliNotificationDTO, connectionID int64) error {
	shipmentID := extractResourceID(n.Resource, "/shipments/")
	if shipmentID == "" {
		if err := p.notificationRepository.MarkFailed(ctx, n.ID, "could not extract shipment id from resource "+n.Resource, maxProcessingAttempts); err != nil {
			log.Printf("meli notifications processor: mark failed on bad shipment resource for notif id=%d: %v", n.ID, err)
		}
		return fmt.Errorf("bad shipments resource %q", n.Resource)
	}

	accessToken, err := p.tokenEnsurer.EnsureValidAccessToken(ctx, connectionID)
	if err != nil {
		if stampErr := p.notificationRepository.MarkFailed(ctx, n.ID, "fetching access token: "+err.Error(), maxProcessingAttempts); stampErr != nil {
			log.Printf("meli notifications processor: notif id=%d, could not mark failed after token refresh failure: %v", n.ID, stampErr)
		}
		return err
	}

	shipment, rawBytes, err := p.shipmentsHandler.GetShipment(ctx, accessToken, shipmentID)
	if err != nil {
		if stampErr := p.notificationRepository.MarkFailed(ctx, n.ID, "GET /shipments/"+shipmentID+": "+err.Error(), maxProcessingAttempts); stampErr != nil {
			log.Printf("meli notifications processor: notif id=%d, could not mark failed after shipment fetch failure: %v", n.ID, stampErr)
		}
		return err
	}

	if err := p.orderRepository.UpsertShipment(ctx, buildShipmentUpsert(shipment, connectionID, string(rawBytes))); err != nil {
		if stampErr := p.notificationRepository.MarkFailed(ctx, n.ID, "upserting ecom_meli_shipments: "+err.Error(), maxProcessingAttempts); stampErr != nil {
			log.Printf("meli notifications processor: notif id=%d, could not mark failed after shipment upsert failure: %v", n.ID, stampErr)
		}
		return err
	}

	if err := p.notificationRepository.MarkDone(ctx, n.ID, string(rawBytes), ""); err != nil {
		return fmt.Errorf("marking shipment notif %d done: %w", n.ID, err)
	}
	return nil
}

// resolveConnectionID maps a notification's meli_user_id to the vecom
// connection_id by calling /users/me under each active MERCADOLIBRE
// connection's token and caching the pair. Cheap: 1 call per connection
// per process lifetime (the cache is in-memory). If the cache misses the
// user, we rebuild it before giving up.
func (p *Processor) resolveConnectionID(ctx context.Context, meliUserID int64) (int64, error) {
	if meliUserID <= 0 {
		return 0, errors.New("meli_user_id is zero")
	}

	if connID, ok := p.lookupConnection(meliUserID); ok {
		return connID, nil
	}

	if err := p.rebuildConnectionCache(ctx); err != nil {
		return 0, err
	}
	if connID, ok := p.lookupConnection(meliUserID); ok {
		return connID, nil
	}
	return 0, fmt.Errorf("no MERCADOLIBRE connection matches meli_user_id %d", meliUserID)
}

func (p *Processor) lookupConnection(meliUserID int64) (int64, bool) {
	p.userToConnectionMutex.RLock()
	defer p.userToConnectionMutex.RUnlock()
	connID, ok := p.userToConnection[meliUserID]
	return connID, ok
}

func (p *Processor) rebuildConnectionCache(ctx context.Context) error {
	connections, err := p.channelConnectionRepository.FindActiveByChannelCode(ctx, mercadoLibreChannelCode)
	if err != nil {
		return fmt.Errorf("listing active MERCADOLIBRE connections: %w", err)
	}

	rebuilt := make(map[int64]int64, len(connections))
	for _, conn := range connections {
		accessToken, err := p.tokenEnsurer.EnsureValidAccessToken(ctx, conn.ID)
		if err != nil {
			log.Printf("meli notifications processor: skipping connection %d while rebuilding user cache: %v", conn.ID, err)
			continue
		}
		me, err := p.usersHandler.GetMe(ctx, accessToken)
		if err != nil {
			log.Printf("meli notifications processor: /users/me failed for connection %d: %v", conn.ID, err)
			continue
		}
		if me.ID > 0 {
			rebuilt[me.ID] = conn.ID
		}
	}

	p.userToConnectionMutex.Lock()
	p.userToConnection = rebuilt
	p.userToConnectionMutex.Unlock()
	return nil
}

func extractResourceID(resource, prefix string) string {
	idx := strings.Index(resource, prefix)
	if idx < 0 {
		return ""
	}
	rest := resource[idx+len(prefix):]
	// resource like "/orders/2000012345678" (no trailing slash) or
	// "/sites/MLM/user-products-families/696040..." — split on next slash.
	slash := strings.Index(rest, "/")
	if slash >= 0 {
		rest = rest[:slash]
	}
	q := strings.Index(rest, "?")
	if q >= 0 {
		rest = rest[:q]
	}
	return strings.TrimSpace(rest)
}

func buildOrderUpsert(order *mercadoLibreInfra.OrderDetail, connectionID int64, notifID *int64, rawJSON string) mysqlInfra.UpsertMeliOrderInput {
	tags := ""
	if len(order.Tags) > 0 {
		tags = strings.Join(order.Tags, ",")
	}

	var shippingID *string
	if order.Shipping.ID > 0 {
		s := strconv.FormatInt(order.Shipping.ID, 10)
		shippingID = &s
	}
	var packID *string
	if order.Pack != nil && order.Pack.ID > 0 {
		s := strconv.FormatInt(order.Pack.ID, 10)
		packID = &s
	}

	items := make([]mysqlInfra.UpsertMeliOrderItemInput, 0, len(order.OrderItems))
	for _, it := range order.OrderItems {
		raw, _ := json.Marshal(it)
		items = append(items, mysqlInfra.UpsertMeliOrderItemInput{
			ItemMeliID:      it.Item.ID,
			ItemVariationID: stringPtrIfSet(it.Item.VariationID),
			SellerSKU:       stringPtrIfSet(it.Item.SellerSKU),
			Title:           stringPtrIfSet(it.Item.Title),
			CategoryID:      stringPtrIfSet(it.Item.CategoryID),
			UnitPrice:       float64PtrIfSet(it.UnitPrice),
			FullUnitPrice:   float64PtrIfSet(it.FullUnitPrice),
			Quantity:        intPtrIfSet(it.Quantity),
			CurrencyID:      stringPtrIfSet(it.CurrencyID),
			SaleFee:         float64PtrIfSet(it.SaleFee),
			ListingTypeID:   stringPtrIfSet(it.ListingTypeID),
			RawPayload:      string(raw),
		})
	}

	return mysqlInfra.UpsertMeliOrderInput{
		ConnectionID:       connectionID,
		MeliOrderID:        strconv.FormatInt(order.ID, 10),
		Status:             order.Status,
		StatusDetail:       stringPtrIfSet(order.StatusDetail),
		DateCreated:        parseMeliTime(order.DateCreated),
		DateClosed:         parseMeliTime(order.DateClosed),
		LastUpdated:        parseMeliTime(order.LastUpdated),
		TotalAmount:        float64PtrIfSet(order.TotalAmount),
		PaidAmount:         float64PtrIfSet(order.PaidAmount),
		CurrencyID:         stringPtrIfSet(order.CurrencyID),
		BuyerMeliUserID:    int64PtrIfSet(order.Buyer.ID),
		BuyerNickname:      stringPtrIfSet(order.Buyer.Nickname),
		BuyerFirstName:     stringPtrIfSet(order.Buyer.FirstName),
		BuyerLastName:      stringPtrIfSet(order.Buyer.LastName),
		BuyerEmail:         stringPtrIfSet(order.Buyer.Email),
		ShippingID:         shippingID,
		PackID:             packID,
		Tags:               stringPtrIfSet(tags),
		RawPayload:         rawJSON,
		SeenNotificationID: notifID,
		Items:              items,
	}
}

func buildShipmentUpsert(shipment *mercadoLibreInfra.ShipmentDetail, connectionID int64, rawJSON string) mysqlInfra.UpsertShipmentInput {
	var orderID *string
	if shipment.OrderID > 0 {
		s := strconv.FormatInt(shipment.OrderID, 10)
		orderID = &s
	}
	var serviceID *string
	if shipment.ServiceID > 0 {
		s := strconv.FormatInt(shipment.ServiceID, 10)
		serviceID = &s
	}

	return mysqlInfra.UpsertShipmentInput{
		ConnectionID:   connectionID,
		MeliShipmentID: strconv.FormatInt(shipment.ID, 10),
		MeliOrderID:    orderID,
		Status:         stringPtrIfSet(shipment.Status),
		Substatus:      stringPtrIfSet(shipment.SubStatus),
		Mode:           stringPtrIfSet(shipment.Mode),
		TrackingNumber: stringPtrIfSet(shipment.TrackingNumber),
		TrackingMethod: stringPtrIfSet(shipment.TrackingMethod),
		ServiceID:      serviceID,
		DateCreated:    parseMeliTime(shipment.DateCreated),
		LastUpdated:    parseMeliTime(shipment.LastUpdated),
		RawPayload:     rawJSON,
	}
}

func parseMeliTime(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	// MELI typically uses RFC3339 with offset like "2026-10-30T12:34:56.000-06:00".
	t, err := time.Parse(time.RFC3339, raw)
	if err == nil {
		return &t
	}
	// Fallback for the shorter format MELI returns on a few endpoints.
	t, err = time.Parse("2006-01-02T15:04:05.000-07:00", raw)
	if err == nil {
		return &t
	}
	return nil
}

func stringPtrIfSet(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
func int64PtrIfSet(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}
func intPtrIfSet(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}
func float64PtrIfSet(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}

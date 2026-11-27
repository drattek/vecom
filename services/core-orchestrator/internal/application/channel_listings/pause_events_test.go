package channel_listings

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/testsupport/sqlscript"
)

const eventsTable = "INSERT INTO ecom_channel_listing_events"

// Posiciones de action y message en el INSERT de la bitácora.
const (
	eventActionArg  = 5
	eventMessageArg = 9
)

// newEventsService: la base devuelve la publicación con ese estado y motivo.
func newEventsService(t *testing.T, status string, pausedReason driver.Value) (*Service, *sqlscript.DB) {
	t.Helper()
	now := time.Now()
	database := &sqlscript.DB{Rows: map[string][][]driver.Value{
		// En el orden de channelProductMapColumns.
		"FROM ecom_channel_product_map": {{
			int64(1), "Filtro", int64(10), int64(3), nil, int64(1), "MLM1", nil,
			status, pausedReason, now, nil, int64(1), nil, now, now, nil,
		}},
	}}
	db := database.Open(t)
	return &Service{
		channelProductMapRepository: mysqlInfra.NewChannelProductMapRepository(db),
		listingEventsRepository:     mysqlInfra.NewChannelListingEventsRepository(db),
	}, database
}

func recordedEvent(t *testing.T, database *sqlscript.DB) (action, message driver.Value) {
	t.Helper()
	events := database.Writes(eventsTable)
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	return events[0].Args[eventActionArg], events[0].Args[eventMessageArg]
}

func TestStatusSyncChangesRecordMarketplaceStockPauses(t *testing.T) {
	cases := []struct {
		name       string
		reason     driver.Value
		wantAction string
		isMarker   bool
	}{
		{name: "out of stock by the marketplace", reason: nil, wantAction: mysqlInfra.ListingEventPause, isMarker: true},
		{name: "paused by the seller", reason: "user", wantAction: mysqlInfra.ListingEventStatusSync},
		{name: "own pause recovered after review", reason: "zero_stock", wantAction: mysqlInfra.ListingEventStatusSync},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, database := newEventsService(t, "paused", tc.reason)

			service.recordStatusSyncChanges(context.Background(), 3,
				[]mysqlInfra.ChannelProductMapDTO{{ID: 1, Status: "synced"}})

			action, message := recordedEvent(t, database)
			if action != tc.wantAction {
				t.Errorf("action = %v, want %s", action, tc.wantAction)
			}
			if (message == MarketplaceOutOfStockPauseMessage) != tc.isMarker {
				t.Errorf("message = %v", message)
			}
		})
	}
}

func TestListingChangeMarksWhoPaused(t *testing.T) {
	for name, tc := range map[string]struct {
		reason   driver.Value
		isMarker bool
	}{
		"paused by this system":     {reason: "zero_stock"},
		"paused by the marketplace": {reason: nil, isMarker: true},
	} {
		t.Run(name, func(t *testing.T) {
			service, database := newEventsService(t, "paused", tc.reason)

			service.recordListingChange(context.Background(),
				&mysqlInfra.ChannelProductMapDTO{ID: 1, ProductID: 10, ConnectionID: 3, Status: "synced"},
				mysqlInfra.ListingEventRefresh, "precio/stock actualizado")

			action, message := recordedEvent(t, database)
			if action != mysqlInfra.ListingEventPause {
				t.Errorf("action = %v", action)
			}
			if (message == MarketplaceOutOfStockPauseMessage) != tc.isMarker {
				t.Errorf("message = %v", message)
			}
		})
	}
}

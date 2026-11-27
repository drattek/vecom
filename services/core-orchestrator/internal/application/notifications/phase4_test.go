package notifications

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"core-orchestrator/internal/domain"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// ---- Horarios ---------------------------------------------------------------

func TestLatestSlot(t *testing.T) {
	loc, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		t.Skip("timezone database not available")
	}
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 11, day, hour, minute, 0, 0, loc) }

	cases := []struct {
		now, want time.Time
		times     []clockTime
	}{
		{now: at(25, 0, 0), times: closureCheckTimes, want: at(25, 0, 0)},
		{now: at(25, 11, 59), times: closureCheckTimes, want: at(25, 0, 0)},
		{now: at(25, 12, 1), times: closureCheckTimes, want: at(25, 12, 0)},
		{now: at(25, 7, 59), times: []clockTime{{8, 0}}, want: at(24, 8, 0)},
		{now: at(25, 8, 0), times: []clockTime{{8, 0}}, want: at(25, 8, 0)},
		// La hora se evalúa en la zona configurada, no en la del proceso.
		{now: at(25, 9, 0).UTC(), times: []clockTime{{8, 0}}, want: at(25, 8, 0)},
	}
	for _, tc := range cases {
		if got := latestSlot(tc.now, loc, tc.times); !got.Equal(tc.want) {
			t.Errorf("latestSlot(%s) = %s, want %s", tc.now, got, tc.want)
		}
	}
}

type fakeJobs struct {
	done, running map[string]bool
	windowEnd     *time.Time
	failed        []string
}

func newFakeJobs() *fakeJobs {
	return &fakeJobs{done: map[string]bool{}, running: map[string]bool{}}
}

func jobID(code string, slot time.Time) string { return code + "@" + slot.UTC().Format(time.RFC3339) }

func (f *fakeJobs) Claim(_ context.Context, code string, slot, _ time.Time, _ time.Duration) (bool, error) {
	id := jobID(code, slot)
	if f.done[id] || f.running[id] {
		return false, nil
	}
	f.running[id] = true
	return true, nil
}

func (f *fakeJobs) Complete(_ context.Context, code string, slot, _ time.Time, _, to *time.Time) error {
	f.done[jobID(code, slot)] = true
	delete(f.running, jobID(code, slot))
	if to != nil {
		f.windowEnd = to
	}
	return nil
}

func (f *fakeJobs) Fail(_ context.Context, code string, slot, _ time.Time, reason string) error {
	delete(f.running, jobID(code, slot))
	f.failed = append(f.failed, reason)
	return nil
}

func (f *fakeJobs) IsDone(_ context.Context, code string, slot time.Time) (bool, error) {
	return f.done[jobID(code, slot)], nil
}

func (f *fakeJobs) LastWindowEnd(context.Context, string) (*time.Time, error) {
	return f.windowEnd, nil
}

func TestRunSlotRunsOnceAndRetriesFailures(t *testing.T) {
	service, _, _ := newEventService()
	service.jobs = newFakeJobs()
	runs := 0
	var jobErr error
	run := func() error {
		return service.runSlot(context.Background(), "job", testNow, time.Minute, func(context.Context) (jobWindow, error) {
			runs++
			return jobWindow{}, jobErr
		})
	}

	jobErr = errors.New("sin conexión")
	if err := run(); err == nil {
		t.Fatal("expected the job error")
	}
	jobErr = nil
	for range 3 {
		if err := run(); err != nil {
			t.Fatal(err)
		}
	}
	if runs != 2 {
		t.Fatalf("runs = %d, want the failed one and one successful", runs)
	}
}

// ---- Resumen de pausas por stock --------------------------------------------

type fakeStockPauses struct {
	listings []mysqlInfra.ProductListingDTO
	windows  []time.Duration
}

func (f *fakeStockPauses) StockPausedListings(_ context.Context, window time.Duration, _ string) ([]mysqlInfra.ProductListingDTO, error) {
	f.windows = append(f.windows, window)
	return f.listings, nil
}

func productListing(id int64, connection, externalID, title, status, pausedReason string) mysqlInfra.ProductListingDTO {
	return mysqlInfra.ProductListingDTO{
		ProductID: id, SKU: fmt.Sprintf("SKU-%d", id), ProductName: "Filtro", ConnectionName: connection,
		ExternalID: externalID, Title: title, Status: status, PausedReason: pausedReason,
	}
}

func TestStockSummaryReportsEachProductOnce(t *testing.T) {
	service, store, _ := newEventService()
	jobs := newFakeJobs()
	source := &fakeStockPauses{listings: []mysqlInfra.ProductListingDTO{
		productListing(1, "ML Cuenta 1", "MLM111", "Filtro A", "paused", zeroStockReason),
		productListing(1, "ML Cuenta 1", "MLM112", "Filtro B", "paused", zeroStockReason),
		productListing(1, "ML Cuenta 2", "MLM221", "Filtro", "paused", zeroStockReason),
		productListing(1, "Odoo", "4521", "Filtro de aceite", "paused", zeroStockReason),
		productListing(1, "Tienda", "MLM999", "", "synced", ""),
		// Pausado a mano: no es una pausa por stock.
		productListing(2, "ML Cuenta 1", "MLM300", "Bomba", "paused", "user"),
	}}
	service.jobs, service.stockPauses = jobs, source

	window, err := service.sendStockPauseSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	assertMessages(t, store, "pausadas: 1")
	message := store.deliveries[0].Message
	for _, want := range []string{
		"SKU-1 · Filtro", "ML Cuenta 1: MLM111, MLM112", "ML Cuenta 2: MLM221", "Odoo: Filtro de aceite", "Sigue activo en: Tienda",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("missing %q in %q", want, message)
		}
	}
	if strings.Contains(message, "SKU-2") {
		t.Errorf("a manual pause must not be reported: %q", message)
	}
	if window.from == nil || window.to.Sub(*window.from) != 24*time.Hour {
		t.Errorf("first window must cover 24h: %+v", window)
	}

	// El siguiente resumen parte de donde terminó el anterior.
	previous := testNow.Add(-5 * time.Hour)
	jobs.windowEnd = &previous
	source.listings = nil
	if _, err := service.sendStockPauseSummary(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := source.windows[len(source.windows)-1]; got != 5*time.Hour {
		t.Errorf("window = %s", got)
	}
	if len(store.deliveries) != 1 {
		t.Errorf("without pauses nothing is sent")
	}
}

func TestStockSummaryUsesFileWhenTooLong(t *testing.T) {
	listings := make([]mysqlInfra.ProductListingDTO, 0, 200)
	for id := int64(1); id <= 200; id++ {
		listings = append(listings, productListing(id, "ML Cuenta 1", fmt.Sprintf("MLM%d", id), "", "paused", zeroStockReason))
	}

	n := stockSummaryNotice(groupPausedProducts(listings))

	if n.attachmentName != "sin-stock.csv" || !strings.Contains(n.text, "pausadas: 200") {
		t.Fatalf("unexpected notice: %q %q", n.attachmentName, n.text)
	}
	if lines := strings.Count(string(n.attachment), "\r\n"); lines != 201 {
		t.Errorf("csv lines = %d", lines)
	}
}

// ---- Cierres ----------------------------------------------------------------

type fakeClosureSource struct {
	statuses  []domain.ListingLiveStatus
	reasons   map[string]domain.ClosureReason
	reasonErr error
	history   map[string]bool
	// failing: conexiones cuya consulta de estados falla.
	failing map[int64]bool
	queries map[int64]int
}

func (f *fakeClosureSource) ListingLiveStatuses(_ context.Context, connectionID int64) ([]domain.ListingLiveStatus, error) {
	if f.queries == nil {
		f.queries = map[int64]int{}
	}
	f.queries[connectionID]++
	if f.failing[connectionID] {
		return nil, errors.New("token rechazado")
	}
	return f.statuses, nil
}

func (f *fakeClosureSource) ClosureReason(_ context.Context, _ int64, externalID string, useHistory bool) (domain.ClosureReason, bool, error) {
	f.history[externalID] = useHistory
	if f.reasonErr != nil {
		return domain.ClosureReason{}, false, f.reasonErr
	}
	reason, found := f.reasons[externalID]
	return reason, found, nil
}

type fakeClosures struct {
	store    *memoryStore
	records  []*mysqlInfra.ListingClosureDTO
	notified map[int64]bool
	reopened map[int64]bool
	ownMaps  map[int64]bool
}

func (f *fakeClosures) ListActive(context.Context, int64) ([]mysqlInfra.ListingClosureDTO, error) {
	var active []mysqlInfra.ListingClosureDTO
	for _, record := range f.records {
		if !f.reopened[record.ID] {
			active = append(active, *record)
		}
	}
	return active, nil
}

func (f *fakeClosures) Save(_ context.Context, in mysqlInfra.SaveListingClosureInput) error {
	record := &mysqlInfra.ListingClosureDTO{
		ID: int64(len(f.records) + 1), ExternalID: in.ExternalID, Kind: in.Kind, ReasonStatus: in.ReasonStatus,
		SubStatus: in.SubStatus, Reason: in.Reason, Remedy: in.Remedy, DetectedAt: in.DetectedAt,
		SKU: "SKU-" + in.ExternalID, ConnectionName: "ML Cuenta 1",
	}
	f.records = append(f.records, record)
	if in.NotifiedAt != nil {
		f.notified[record.ID] = true
	}
	return nil
}

func (f *fakeClosures) SetResolution(_ context.Context, id int64, kind, reasonStatus, _, reason, remedy string) error {
	record := f.records[id-1]
	record.Kind, record.ReasonStatus, record.Reason, record.Remedy = kind, reasonStatus, reason, remedy
	return nil
}

func (f *fakeClosures) MarkReopened(_ context.Context, id int64, _ time.Time) error {
	f.reopened[id] = true
	return nil
}

func (f *fakeClosures) ListPendingNotice(context.Context, time.Time) ([]mysqlInfra.ListingClosureDTO, error) {
	for _, id := range f.store.notified {
		f.notified[id] = true
	}
	var pending []mysqlInfra.ListingClosureDTO
	for _, r := range f.records {
		if r.Kind == mysqlInfra.ClosureDefinitive && r.ReasonStatus != mysqlInfra.ClosureReasonPending &&
			!f.notified[r.ID] && !f.reopened[r.ID] {
			pending = append(pending, *r)
		}
	}
	return pending, nil
}

func (f *fakeClosures) HasStatusEvent(_ context.Context, mapID int64, _ string) (bool, error) {
	return f.ownMaps[mapID], nil
}

type closureFixture struct {
	service  *Service
	store    *memoryStore
	source   *fakeClosureSource
	closures *fakeClosures
	closed   []string
	now      time.Time
	slot     time.Time
}

func newClosureFixture() *closureFixture {
	service, store, _ := newEventService()
	f := &closureFixture{
		service: service, store: store, now: testNow,
		source:   &fakeClosureSource{reasons: map[string]domain.ClosureReason{}, history: map[string]bool{}},
		closures: &fakeClosures{store: store, notified: map[int64]bool{}, reopened: map[int64]bool{}, ownMaps: map[int64]bool{}},
	}
	store.connections = []mysqlInfra.ChannelConnectionDTO{
		{ID: 1, Name: "ML Cuenta 1", ChannelCode: "MERCADOLIBRE"},
		{ID: 2, Name: "Odoo", ChannelCode: "ODOO"},
	}
	service.now = func() time.Time { return f.now }
	service.jobs = newFakeJobs()
	service.closures = f.closures
	service.RegisterClosureSource(f.source)
	service.closeListing = func(_ context.Context, _ int64, listing domain.ListingLiveStatus) error {
		f.closed = append(f.closed, listing.ExternalID)
		return nil
	}
	return f
}

func (f *closureFixture) check(t *testing.T) {
	t.Helper()
	// Cada llamada es un horario nuevo.
	f.slot = f.slot.Add(12 * time.Hour)
	if err := f.service.checkListingClosures(context.Background(), f.slot); err != nil {
		t.Fatal(err)
	}
}

func liveListing(mapID int64, externalID, local, raw string, closed bool, subStatus ...string) domain.ListingLiveStatus {
	return domain.ListingLiveStatus{
		MapID: mapID, ProductID: mapID, ExternalID: externalID, LocalStatus: local,
		Found: true, RawStatus: raw, SubStatus: subStatus, Closed: closed,
	}
}

func TestFirstClosureCheckIsSilentBaseline(t *testing.T) {
	f := newClosureFixture()
	f.source.statuses = []domain.ListingLiveStatus{
		liveListing(1, "MLM1", "synced", "closed", true, "forbidden"),
		liveListing(2, "MLM2", "paused", "paused", false),
	}

	f.check(t)
	f.check(t)

	assertMessages(t, f.store)
	if len(f.closures.records) != 1 || len(f.closed) == 0 || f.closed[0] != "MLM1" {
		t.Fatalf("records=%d closed=%v", len(f.closures.records), f.closed)
	}
	if len(f.source.history) != 0 {
		t.Errorf("the baseline must not query reasons")
	}
}

func TestClosuresNotifyOnlyThoseByMercadoLibre(t *testing.T) {
	f := newClosureFixture()
	f.check(t) // línea base sin publicaciones

	f.source.statuses = []domain.ListingLiveStatus{
		liveListing(1, "MLM1", "synced", "under_review", true, "forbidden"),
		liveListing(2, "MLM2", "synced", "closed", true),            // la cerró el vendedor
		liveListing(3, "MLM3", "synced", "closed", true),            // sin subestado, con moderación
		liveListing(4, "MLM4", "closed", "closed", true, "deleted"), // eliminada por el vendedor
		liveListing(5, "MLM5", "paused", "closed", true, "forbidden"),
		liveListing(6, "MLM6", "synced", "paused", false),
	}
	f.source.reasons["MLM1"] = domain.ClosureReason{Reason: "Producto prohibido.", Remedy: ""}
	f.source.reasons["MLM3"] = domain.ClosureReason{Reason: "Infringe propiedad intelectual."}
	f.closures.ownMaps[5] = true // la cerró este sistema

	f.check(t)
	f.check(t)

	assertMessages(t, f.store, "cerradas definitivamente por MercadoLibre: 2")
	message := f.store.deliveries[0].Message
	for _, want := range []string{"MLM1", "Producto prohibido.", "MLM3", "Infringe propiedad intelectual."} {
		if !strings.Contains(message, want) {
			t.Errorf("missing %q in %q", want, message)
		}
	}
	for _, unwanted := range []string{"MLM2", "MLM4", "MLM5", "MLM6"} {
		if strings.Contains(message, unwanted) {
			t.Errorf("%s must not be reported", unwanted)
		}
	}
	// El historial de infracciones solo se usa si ya se sabe que fue MercadoLibre.
	if !f.source.history["MLM1"] || f.source.history["MLM2"] {
		t.Errorf("history usage: %v", f.source.history)
	}
}

func TestClosureWithoutReasonReportsSubStatus(t *testing.T) {
	f := newClosureFixture()
	f.check(t)
	f.source.statuses = []domain.ListingLiveStatus{liveListing(1, "MLM1", "synced", "closed", true, "expired")}

	f.check(t)

	assertMessages(t, f.store, "no informó el motivo (estado: expired)")
}

func TestClosureReasonIsRetriedThenGivenUp(t *testing.T) {
	f := newClosureFixture()
	f.check(t)
	f.source.statuses = []domain.ListingLiveStatus{liveListing(1, "MLM1", "synced", "closed", true, "forbidden")}
	f.source.reasonErr = errors.New("mercadolibre 500")

	f.check(t)
	assertMessages(t, f.store)

	f.now = f.now.Add(12 * time.Hour)
	f.source.statuses[0].LocalStatus = "closed"
	f.check(t)
	assertMessages(t, f.store)

	f.now = f.now.Add(12 * time.Hour)
	f.check(t)
	assertMessages(t, f.store, "no informó el motivo (estado: forbidden)")
}

func TestReopenedListingCanBeReportedAgain(t *testing.T) {
	f := newClosureFixture()
	f.check(t)
	closed := liveListing(1, "MLM1", "synced", "closed", true, "suspended")
	f.source.statuses = []domain.ListingLiveStatus{closed}
	f.check(t)

	f.source.statuses = []domain.ListingLiveStatus{liveListing(1, "MLM1", "closed", "active", false)}
	f.check(t)
	if !f.closures.reopened[1] {
		t.Fatal("the closure must be reopened")
	}

	f.now = f.now.Add(time.Hour)
	f.source.statuses = []domain.ListingLiveStatus{closed}
	f.check(t)

	assertMessages(t, f.store, "MLM1", "MLM1")
}

func TestUnansweredListingIsNotClosed(t *testing.T) {
	f := newClosureFixture()
	f.check(t)
	f.source.statuses = []domain.ListingLiveStatus{{MapID: 1, ExternalID: "MLM1", LocalStatus: "synced"}}

	f.check(t)

	if len(f.closed) != 0 || len(f.closures.records) != 0 {
		t.Fatalf("closed=%v records=%d", f.closed, len(f.closures.records))
	}
}

func TestFailingConnectionIsTheOnlyOneRetried(t *testing.T) {
	f := newClosureFixture()
	f.store.connections = append(f.store.connections,
		mysqlInfra.ChannelConnectionDTO{ID: 3, Name: "ML Cuenta 2", ChannelCode: "MERCADOLIBRE"})
	f.source.failing = map[int64]bool{3: true}
	slot := testNow

	for range 3 {
		if err := f.service.checkListingClosures(context.Background(), slot); err == nil {
			t.Fatal("expected the error of the failing connection")
		}
	}

	if f.source.queries[1] != 1 || f.source.queries[3] != 3 {
		t.Fatalf("queries = %v", f.source.queries)
	}
	if f.source.queries[2] != 0 {
		t.Errorf("only MercadoLibre connections are checked")
	}
}

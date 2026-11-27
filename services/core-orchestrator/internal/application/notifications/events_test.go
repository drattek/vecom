package notifications

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"core-orchestrator/internal/domain"
	odooInfra "core-orchestrator/internal/infrastructure/marketplace/odoo"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/infrastructure/telegram"
)

// memoryStore hace de unidad de trabajo y de lecturas en memoria.
type memoryStore struct {
	nextID      int64
	open        map[string]*mysqlInfra.NotificationIncidentDTO
	recipients  []mysqlInfra.NotificationRecipientDTO
	permissions map[int64][]string
	deliveries  []mysqlInfra.EnqueueNotificationInput

	erpStates   []mysqlInfra.ErpSyncStateDTO
	connections []mysqlInfra.ChannelConnectionDTO
	batches     []mysqlInfra.FinishedBatchDTO
	failedRows  []mysqlInfra.FailedBatchRowDTO
	states      []mysqlInfra.ListingStateDTO
	notified    []int64
}

func (m *memoryStore) MarkClosuresNotified(_ context.Context, ids []int64, _ time.Time) error {
	m.notified = append(m.notified, ids...)
	return nil
}

func (m *memoryStore) LatestListingStates(context.Context, time.Duration, string) ([]mysqlInfra.ListingStateDTO, error) {
	return m.states, nil
}

func (m *memoryStore) FindOpenIncident(_ context.Context, key string) (*mysqlInfra.NotificationIncidentDTO, error) {
	if current, ok := m.open[key]; ok {
		copied := *current
		return &copied, nil
	}
	return nil, nil
}

func (m *memoryStore) OpenIncident(_ context.Context, in mysqlInfra.OpenNotificationIncidentInput) (int64, error) {
	m.nextID++
	m.open[in.IncidentKey] = &mysqlInfra.NotificationIncidentDTO{
		ID: m.nextID, IncidentType: in.IncidentType, IncidentKey: in.IncidentKey,
		Signature: in.Signature, ConnectionID: in.ConnectionID, Message: in.Message,
	}
	return m.nextID, nil
}

func (m *memoryStore) TouchIncident(_ context.Context, id int64, signature, message string, _ time.Time) error {
	for _, incident := range m.open {
		if incident.ID == id {
			incident.Signature = signature
			incident.Message = message
		}
	}
	return nil
}

func (m *memoryStore) ResolveIncident(_ context.Context, id int64, _ time.Time) error {
	for key, incident := range m.open {
		if incident.ID == id {
			delete(m.open, key)
		}
	}
	return nil
}

func (m *memoryStore) Recipients(_ context.Context, _, _ string, only int64) ([]mysqlInfra.NotificationRecipientDTO, error) {
	var found []mysqlInfra.NotificationRecipientDTO
	for _, r := range m.recipients {
		if only == 0 || r.UserID == only {
			found = append(found, r)
		}
	}
	return found, nil
}

func (m *memoryStore) Permissions(_ context.Context, userID int64, _ *int64) ([]string, error) {
	return m.permissions[userID], nil
}

func (m *memoryStore) Delivered(_ context.Context, userID int64, key string) (bool, error) {
	for _, d := range m.deliveries {
		if d.UserID == userID && d.DedupeKey == key {
			return true, nil
		}
	}
	return false, nil
}

func (m *memoryStore) Enqueue(ctx context.Context, in mysqlInfra.EnqueueNotificationInput) error {
	if done, _ := m.Delivered(ctx, in.UserID, in.DedupeKey); !done {
		m.deliveries = append(m.deliveries, in)
	}
	return nil
}

func (m *memoryStore) RecordCompleted(context.Context, string, int, int, time.Time) error { return nil }
func (m *memoryStore) RecordFailed(context.Context, string, string, time.Time) error      { return nil }
func (m *memoryStore) List(context.Context) ([]mysqlInfra.ErpSyncStateDTO, error) {
	return m.erpStates, nil
}

func (m *memoryStore) FindAllActive(context.Context) ([]mysqlInfra.ChannelConnectionDTO, error) {
	return m.connections, nil
}

func (m *memoryStore) FindByID(_ context.Context, id int64) (*mysqlInfra.ChannelConnectionDTO, error) {
	for i := range m.connections {
		if m.connections[i].ID == id {
			return &m.connections[i], nil
		}
	}
	return nil, errors.New("not found")
}

func (m *memoryStore) ListOpenByType(_ context.Context, incidentType string) ([]mysqlInfra.NotificationIncidentDTO, error) {
	var found []mysqlInfra.NotificationIncidentDTO
	for _, incident := range m.open {
		if incident.IncidentType == incidentType {
			found = append(found, *incident)
		}
	}
	return found, nil
}

func (m *memoryStore) FinishedBatches(_ context.Context, kind string, _ time.Duration) ([]mysqlInfra.FinishedBatchDTO, error) {
	var found []mysqlInfra.FinishedBatchDTO
	for _, b := range m.batches {
		if b.Kind == kind {
			found = append(found, b)
		}
	}
	return found, nil
}

func (m *memoryStore) FailedBatchRows(context.Context, string, int64) ([]mysqlInfra.FailedBatchRowDTO, error) {
	return m.failedRows, nil
}

// newEventService: un administrador (id 1) recibe los avisos.
func newEventService() (*Service, *memoryStore, *fakeSettings) {
	store := &memoryStore{
		open:       map[string]*mysqlInfra.NotificationIncidentDTO{},
		recipients: []mysqlInfra.NotificationRecipientDTO{{UserID: 1, Role: domain.AdminRole}},
	}
	service, settings := newTestService(&fakeLinks{}, &fakeBot{})
	settings.current.ErpSilenceHours = 36
	settings.current.ConnectionCheckMinutes = 60
	service.inUnit = func(_ context.Context, fn func(unit) error) error { return fn(store) }
	service.erpState = store
	service.connections = store
	service.incidents = store
	service.batches = store
	service.listingStates = store
	service.connectionChecks = map[string]ConnectionCheck{}
	service.connectionStates = map[int64]*connectionState{}
	return service, store, settings
}

func assertMessages(t *testing.T, store *memoryStore, fragments ...string) {
	t.Helper()
	if len(store.deliveries) != len(fragments) {
		var got []string
		for _, d := range store.deliveries {
			got = append(got, d.Message)
		}
		t.Fatalf("expected %d messages, got %d: %q", len(fragments), len(got), got)
	}
	for i, fragment := range fragments {
		if !strings.Contains(store.deliveries[i].Message, fragment) {
			t.Errorf("message %d = %q, want it to contain %q", i, store.deliveries[i].Message, fragment)
		}
	}
}

func TestErpFailureNotifiesOnceAndRecovery(t *testing.T) {
	service, store, _ := newEventService()
	ctx := context.Background()

	service.ErpSyncFailed(ctx, domain.ErpFeedStock, "timeout")
	service.ErpSyncFailed(ctx, domain.ErpFeedStock, "otro timeout")
	service.ErpSyncCompleted(ctx, domain.ErpSyncResult{Feed: domain.ErpFeedStock, Total: 10})
	service.ErpSyncCompleted(ctx, domain.ErpSyncResult{Feed: domain.ErpFeedStock, Total: 10})

	assertMessages(t, store, "Falló la sincronización de stock del ERP", "volvió a completarse")
}

func TestErpPartialFailureRenotifiesOnlyWhenRecordsChange(t *testing.T) {
	service, store, _ := newEventService()
	run := func(codes ...string) {
		result := domain.ErpSyncResult{Feed: domain.ErpFeedProducts, Total: 100, Failed: len(codes)}
		for _, code := range codes {
			result.AddFailure(code, errors.New("dato inválido"))
		}
		service.ErpSyncCompleted(context.Background(), result)
	}

	run("A", "B")
	run("B", "A") // los mismos, en otro orden
	run("A", "B", "C", "D")
	run()

	assertMessages(t, store, "2 de 100 registros fallidos", "4 de 100 registros fallidos", "sin registros fallidos")
	if store.deliveries[0].AttachmentName != "" {
		t.Errorf("few failures must fit in the message")
	}
	if store.deliveries[1].AttachmentName != "errores-products.csv" {
		t.Errorf("attachment = %q", store.deliveries[1].AttachmentName)
	}
}

func TestErpSilence(t *testing.T) {
	service, store, _ := newEventService()
	recent := testNow.Add(-35 * time.Hour)
	old := testNow.Add(-37 * time.Hour)
	store.erpStates = []mysqlInfra.ErpSyncStateDTO{
		{FeedCode: domain.ErpFeedProducts, LastCompletedAt: &recent},
		{FeedCode: domain.ErpFeedStock, LastCompletedAt: &old},
		{FeedCode: domain.ErpFeedNissanStock, CreatedAt: testNow.Add(-time.Hour)},
	}

	for range 2 {
		if err := service.checkErpSilence(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	assertMessages(t, store, "stock del ERP en más de 36 horas")
}

func TestProblemIsReportedOnceSomeoneCanReceiveIt(t *testing.T) {
	service, store, settings := newEventService()
	admin := store.recipients
	ctx := context.Background()

	settings.current.IsEnabled = false
	service.ErpSyncFailed(ctx, domain.ErpFeedStock, "timeout")
	settings.current.IsEnabled = true
	store.recipients = nil
	service.ErpSyncFailed(ctx, domain.ErpFeedStock, "timeout")
	if len(store.deliveries) != 0 || len(store.open) != 0 {
		t.Fatalf("deliveries=%d open=%d", len(store.deliveries), len(store.open))
	}

	store.recipients = admin
	service.ErpSyncFailed(ctx, domain.ErpFeedStock, "timeout")
	assertMessages(t, store, "Falló la sincronización")
}

func TestRecipientsNeedPermission(t *testing.T) {
	service, store, _ := newEventService()
	store.recipients = []mysqlInfra.NotificationRecipientDTO{{UserID: 2}, {UserID: 3}}
	store.permissions = map[int64][]string{2: {"dashboard.view"}, 3: {"sales.view"}}

	service.ErpSyncFailed(context.Background(), domain.ErpFeedStock, "timeout")

	if len(store.deliveries) != 1 || store.deliveries[0].UserID != 2 {
		t.Fatalf("unexpected deliveries: %+v", store.deliveries)
	}
}

func TestConnectionChecks(t *testing.T) {
	service, store, _ := newEventService()
	now := testNow
	service.now = func() time.Time { return now }
	store.connections = []mysqlInfra.ChannelConnectionDTO{{ID: 7, Name: "Cuenta 1", ChannelCode: "ODOO"}}

	var checkErr error
	checks := 0
	service.RegisterConnectionCheck("odoo", func(context.Context, int64) error {
		checks++
		return checkErr
	})
	step := func(advance time.Duration) {
		now = now.Add(advance)
		if err := service.checkConnections(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	// Un fallo de red aislado no avisa; se vuelve a comprobar a los 5 minutos.
	checkErr = errors.New("dial tcp: timeout")
	step(0)
	assertMessages(t, store)
	step(time.Minute)
	if checks != 1 {
		t.Fatalf("checked too soon: %d", checks)
	}
	step(5 * time.Minute)
	assertMessages(t, store, "no está funcionando")

	checkErr = nil
	step(61 * time.Minute)
	assertMessages(t, store, "no está funcionando", "volvió a funcionar")

	// Un rechazo de credenciales avisa de inmediato.
	checkErr = &odooInfra.StatusError{Status: 401}
	step(61 * time.Minute)
	assertMessages(t, store, "no está funcionando", "volvió a funcionar", "rechazó las credenciales")

	// Al desactivar la conexión el incidente se cierra sin avisar.
	store.connections = nil
	step(time.Minute)
	if len(store.open) != 0 || len(store.deliveries) != 3 {
		t.Fatalf("open=%d deliveries=%d", len(store.open), len(store.deliveries))
	}
}

func TestConnectionAuthResultReactsOnlyToAuthFailures(t *testing.T) {
	service, store, _ := newEventService()
	store.connections = []mysqlInfra.ChannelConnectionDTO{{ID: 3, Name: "Cuenta 2"}}
	ctx := context.Background()

	service.ConnectionAuthResult(ctx, 3, errors.New("mercadolibre oauth request failed (500): boom"), false)
	service.ConnectionAuthResult(ctx, 3, context.Canceled, false)
	assertMessages(t, store)

	service.ConnectionAuthResult(ctx, 3, errors.New("mercadolibre oauth request failed (400): invalid_grant"), false)
	service.ConnectionAuthResult(ctx, 3, nil, false) // token reutilizado
	service.ConnectionAuthResult(ctx, 3, nil, true)  // token renovado

	assertMessages(t, store, "\"Cuenta 2\" rechazó las credenciales", "volvió a funcionar")
}

func TestFinishedBatchNotifiesOnlyInitiatorOnce(t *testing.T) {
	service, store, _ := newEventService()
	store.recipients = []mysqlInfra.NotificationRecipientDTO{
		{UserID: 1, Role: domain.AdminRole}, {UserID: 2, Role: domain.AdminRole},
	}
	store.batches = []mysqlInfra.FinishedBatchDTO{{
		Kind: mysqlInfra.BulkKindProductUpdate, ID: 4, FileName: "precios.xlsx", CreatedBy: 2,
		FinishedAtUnix: 100, Total: 3, Done: 2, Failed: 1,
	}}
	store.failedRows = []mysqlInfra.FailedBatchRowDTO{{SheetRow: 2, SKU: "=A1", Status: "failed", Error: "marca inexistente"}}

	for range 2 {
		if err := service.checkFinishedBatches(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	assertMessages(t, store, "Correctas: 2 · Con error: 1")
	delivery := store.deliveries[0]
	if delivery.UserID != 2 || delivery.AttachmentName != "lote-4-pendientes.csv" {
		t.Fatalf("unexpected delivery: %+v", delivery)
	}
	if !strings.Contains(string(delivery.AttachmentContent), "'=A1") {
		t.Errorf("formula cells must be neutralized: %s", delivery.AttachmentContent)
	}

	// Tras un reintento el lote termina de nuevo y se vuelve a avisar.
	store.batches[0].FinishedAtUnix = 200
	if err := service.checkFinishedBatches(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.deliveries) != 2 {
		t.Fatalf("expected a new notice, got %d", len(store.deliveries))
	}
}

func TestSanitize(t *testing.T) {
	cases := []string{
		`Get "https://api.test/items?access_token=SECRETO123&x=1": timeout`,
		`{"client_secret":"SECRETO123","a":1}`,
		`Authorization: Bearer SECRETO123`,
		`token APP_USR-SECRETO123-456 rejected`,
		`dial mysql: usuario:SECRETO123@tcp(10.0.0.1:3306)/db`,
		`amqp://usuario:SECRETO123@rabbit:5672 refused`,
		`(400): invalid — raw response: {"buyer":"SECRETO123"}`,
		`buyer SECRETO123@correo.com not allowed`,
	}
	for _, input := range cases {
		if got := sanitize(input); strings.Contains(got, "SECRETO123") {
			t.Errorf("secret survived: %q -> %q", input, got)
		}
	}
	const harmless = "el SKU ABC-123 no tiene precio"
	if got := sanitize(harmless); got != harmless {
		t.Errorf("harmless text changed: %q", got)
	}
}

type fakeDeliveries struct{ events []string }

func (f *fakeDeliveries) ExpireStale(context.Context, time.Duration) error { return nil }
func (f *fakeDeliveries) MarkSent(context.Context, int64, time.Time) error {
	f.events = append(f.events, "sent")
	return nil
}
func (f *fakeDeliveries) Retry(context.Context, int64, time.Time, string) error {
	f.events = append(f.events, "retry")
	return nil
}
func (f *fakeDeliveries) Close(_ context.Context, _ int64, status, _ string) error {
	f.events = append(f.events, status)
	return nil
}

type dispatchLinks struct {
	linkStore
	link    *mysqlInfra.TelegramLinkDTO
	blocked bool
}

func (d *dispatchLinks) FindByUserID(context.Context, int64) (*mysqlInfra.TelegramLinkDTO, error) {
	return d.link, nil
}

func (d *dispatchLinks) SetBlockedByChatID(_ context.Context, _ int64, blocked bool) error {
	d.blocked = blocked
	return nil
}

func TestDispatchOutcomes(t *testing.T) {
	chat := int64(55)
	linked := mysqlInfra.TelegramLinkDTO{
		Status: mysqlInfra.TelegramLinkLinked, ChatID: &chat, NotificationsEnabled: true, BotUsername: "vegusa_bot",
	}
	muted := linked
	muted.NotificationsEnabled = false

	cases := []struct {
		name       string
		link       mysqlInfra.TelegramLinkDTO
		sendErr    error
		attempts   int
		wantEvent  string
		wantSent   int
		wantBlock  bool
		minimumGap time.Duration
	}{
		{name: "sent", link: linked, attempts: 1, wantEvent: "sent", wantSent: 1},
		{name: "user muted", link: muted, attempts: 1, wantEvent: "cancelled"},
		{name: "rate limited", link: linked, attempts: maxAttempts, wantEvent: "retry", wantSent: 1,
			sendErr: &telegram.APIError{Code: 429, RetryAfter: 9 * time.Second}, minimumGap: 9 * time.Second},
		{name: "bot blocked", link: linked, attempts: 1, wantEvent: "cancelled", wantSent: 1, wantBlock: true,
			sendErr: &telegram.APIError{Code: 403}},
		{name: "network", link: linked, attempts: 1, wantEvent: "retry", wantSent: 1, sendErr: errors.New("timeout")},
		{name: "gives up", link: linked, attempts: maxAttempts, wantEvent: "failed", wantSent: 1, sendErr: errors.New("timeout")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bot := &fakeBot{sendErr: tc.sendErr}
			service, _ := newTestService(&fakeLinks{}, bot)
			link := tc.link
			links := &dispatchLinks{link: &link}
			store := &fakeDeliveries{}
			service.links = links
			service.deliveries = store
			service.claimDelivery = func(context.Context, time.Time) (*mysqlInfra.NotificationDeliveryDTO, error) {
				return &mysqlInfra.NotificationDeliveryDTO{ID: 1, UserID: 1, Message: "hola", Attempts: tc.attempts}, nil
			}

			// El error de un envío fallido se devuelve para registrarlo.
			delay, _ := service.dispatchOnce(context.Background())

			if len(store.events) != 1 || store.events[0] != tc.wantEvent {
				t.Fatalf("events = %v, want %s", store.events, tc.wantEvent)
			}
			if len(bot.sent) != tc.wantSent || links.blocked != tc.wantBlock {
				t.Fatalf("sent=%d blocked=%v", len(bot.sent), links.blocked)
			}
			if delay < tc.minimumGap {
				t.Errorf("delay = %s", delay)
			}
		})
	}
}

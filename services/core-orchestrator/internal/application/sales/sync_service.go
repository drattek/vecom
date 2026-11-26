package sales

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/shared/safe"
)

const (
	// openSalesMaxAge acota cuánto tiempo se sigue consultando una venta
	// abierta.
	openSalesMaxAge = 90 * 24 * time.Hour
	// maxConsecutiveFailures detiene una corrida cuando todo está fallando
	// (credenciales, caída del marketplace o de la base).
	maxConsecutiveFailures = 10

	lockDuration      = 10 * time.Minute
	lockRenewInterval = 2 * time.Minute
	dueCheckInterval  = time.Minute
	// retryAfterStart evita reintentar sin pausa una conexión que falla.
	retryAfterStart   = 30 * time.Minute
	finishTimeout     = 15 * time.Second
	manualQueueLength = 50
)

var (
	ErrConnectionNotSyncable = errors.New("la conexión no existe, está inactiva o su canal no importa ventas")
	ErrSyncAlreadyRunning    = errors.New("ya hay una sincronización en curso para esa conexión")
	ErrSyncQueueFull         = errors.New("hay demasiadas sincronizaciones en espera")

	errTooManyFailures = errors.New("demasiados fallos seguidos, la corrida se detuvo")
	errLockLost        = errors.New("se perdió el bloqueo de la conexión")
)

type Config struct {
	// Enabled activa la ejecución periódica. La sincronización manual funciona
	// aunque esté apagada.
	Enabled      bool
	Interval     time.Duration
	BackfillDays int
	Overlap      time.Duration
}

func (c Config) withDefaults() Config {
	if c.Interval <= 0 {
		c.Interval = 4 * time.Hour
	}
	if c.BackfillDays <= 0 {
		c.BackfillDays = 30
	}
	if c.Overlap <= 0 {
		c.Overlap = time.Hour
	}
	return c
}

type syncRequest struct {
	connectionID int64
	actorID      *int64
}

// SyncService ejecuta las corridas: ventana, cursor, bloqueo y guardado.
type SyncService struct {
	db      *sql.DB
	sales   *mysqlInfra.SalesRepository
	state   *mysqlInfra.SaleSyncStateRepository
	readers map[string]Reader
	config  Config

	requests chan syncRequest
	mu       sync.Mutex
	running  map[int64]bool
}

func NewSyncService(db *sql.DB, salesRepository *mysqlInfra.SalesRepository, stateRepository *mysqlInfra.SaleSyncStateRepository, config Config) *SyncService {
	return &SyncService{
		db:       db,
		sales:    salesRepository,
		state:    stateRepository,
		readers:  make(map[string]Reader),
		config:   config.withDefaults(),
		requests: make(chan syncRequest, manualQueueLength),
		running:  make(map[int64]bool),
	}
}

// Register agrega el lector de un canal. Llamar antes de Start.
func (s *SyncService) Register(reader Reader) {
	s.readers[strings.ToUpper(reader.ChannelCode())] = reader
}

func (s *SyncService) channelCodes() []string {
	codes := make([]string, 0, len(s.readers))
	for code := range s.readers {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

// Status lista las conexiones que importan ventas y su última corrida.
func (s *SyncService) Status(ctx context.Context) ([]mysqlInfra.SaleSyncStatusDTO, error) {
	statuses, err := s.state.ListStatus(ctx, s.channelCodes(), time.Now())
	if err != nil {
		return nil, err
	}
	for i := range statuses {
		if s.isRunning(statuses[i].ConnectionID) {
			statuses[i].Running = true
		}
	}
	return statuses, nil
}

func (s *SyncService) Settings() (enabled bool, interval time.Duration) {
	return s.config.Enabled, s.config.Interval
}

// Request encola una sincronización inmediata de una conexión, o de todas las
// activas si connectionID es 0. Devuelve las conexiones encoladas.
func (s *SyncService) Request(ctx context.Context, connectionID int64, actorID *int64) ([]int64, error) {
	statuses, err := s.Status(ctx)
	if err != nil {
		return nil, err
	}
	targets := make([]int64, 0, len(statuses))
	found := false
	for _, status := range statuses {
		if !status.Active || (connectionID != 0 && status.ConnectionID != connectionID) {
			continue
		}
		found = true
		if !status.Running {
			targets = append(targets, status.ConnectionID)
		}
	}
	if !found {
		return nil, ErrConnectionNotSyncable
	}
	if len(targets) == 0 {
		return nil, ErrSyncAlreadyRunning
	}

	for _, id := range targets {
		select {
		case s.requests <- syncRequest{connectionID: id, actorID: actorID}:
		default:
			return nil, ErrSyncQueueFull
		}
	}
	return targets, nil
}

// Start atiende las solicitudes manuales y, si está habilitado, lanza las
// corridas periódicas. Bloquea hasta que ctx se cancela y las corridas en
// curso terminan: ejecutar con `go safe.Supervise(...)`.
func (s *SyncService) Start(ctx context.Context) {
	var wg sync.WaitGroup
	defer wg.Wait()

	var tick <-chan time.Time
	if s.config.Enabled {
		ticker := time.NewTicker(dueCheckInterval)
		defer ticker.Stop()
		tick = ticker.C
		log.Printf("sales sync: activo, cada %s", s.config.Interval)
		s.runDue(ctx, &wg)
	} else {
		log.Printf("sales sync: ejecución periódica apagada (SALES_SYNC_ENABLED!=true); solo manual")
	}

	for {
		select {
		case <-ctx.Done():
			return
		case request := <-s.requests:
			s.launch(ctx, &wg, request)
		case <-tick:
			s.runDue(ctx, &wg)
		}
	}
}

func (s *SyncService) runDue(ctx context.Context, wg *sync.WaitGroup) {
	statuses, err := s.Status(ctx)
	if err != nil {
		log.Printf("sales sync: no se pudo leer el estado de las conexiones: %v", err)
		return
	}
	now := time.Now()
	for _, status := range statuses {
		if status.Active && !status.Running && isDue(status.LastSuccessAt, status.LastStartedAt, now, s.config.Interval) {
			s.launch(ctx, wg, syncRequest{connectionID: status.ConnectionID})
		}
	}
}

// isDue: toca correr si el último éxito es más antiguo que el intervalo y no
// se intentó hace poco.
func isDue(lastSuccess, lastStarted *time.Time, now time.Time, interval time.Duration) bool {
	if lastSuccess != nil && now.Sub(*lastSuccess) < interval {
		return false
	}
	pause := retryAfterStart
	if interval < pause {
		pause = interval
	}
	return lastStarted == nil || now.Sub(*lastStarted) >= pause
}

func (s *SyncService) isRunning(connectionID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running[connectionID]
}

func (s *SyncService) launch(ctx context.Context, wg *sync.WaitGroup, request syncRequest) {
	s.mu.Lock()
	if s.running[request.connectionID] {
		s.mu.Unlock()
		return
	}
	s.running[request.connectionID] = true
	s.mu.Unlock()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.running, request.connectionID)
			s.mu.Unlock()
		}()
		safe.Do(fmt.Sprintf("sales sync conexión %d", request.connectionID), func() {
			if err := s.syncConnection(ctx, request.connectionID, request.actorID); err != nil {
				log.Printf("sales sync: conexión %d: %v", request.connectionID, err)
			}
		})
	}()
}

func (s *SyncService) readerFor(ctx context.Context, connectionID int64) (Reader, error) {
	statuses, err := s.state.ListStatus(ctx, s.channelCodes(), time.Now())
	if err != nil {
		return nil, err
	}
	for _, status := range statuses {
		if status.ConnectionID == connectionID && status.Active {
			return s.readers[strings.ToUpper(strings.TrimSpace(status.ChannelCode))], nil
		}
	}
	return nil, nil
}

func newLockToken() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generando el token de bloqueo: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func (s *SyncService) syncConnection(ctx context.Context, connectionID int64, actorID *int64) error {
	reader, err := s.readerFor(ctx, connectionID)
	if err != nil {
		return err
	}
	if reader == nil {
		return ErrConnectionNotSyncable
	}

	startedAt := time.Now()
	if err := s.state.Ensure(ctx, connectionID, startedAt.AddDate(0, 0, -s.config.BackfillDays)); err != nil {
		return err
	}
	token, err := newLockToken()
	if err != nil {
		return err
	}
	acquired, err := s.state.AcquireLock(ctx, connectionID, token, startedAt, startedAt.Add(lockDuration))
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}

	runCtx, cancelRun := context.WithCancelCause(ctx)
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		safe.Do("sales sync heartbeat", func() { s.keepLock(runCtx, cancelRun, connectionID, token) })
	}()

	var result mysqlInfra.SaleSyncResult
	// Finish corre también si la corrida entra en panic o se cancela.
	defer func() {
		cancelRun(nil)
		<-heartbeatDone
		finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
		defer cancel()
		if err := s.state.Finish(finishCtx, connectionID, token, time.Now(), result); err != nil {
			log.Printf("sales sync: conexión %d: no se pudo cerrar la corrida: %v", connectionID, err)
		}
	}()

	interrupted := "la corrida se interrumpió"
	result.Error = &interrupted

	result = s.run(runCtx, reader, connectionID, startedAt, actorID)
	log.Printf("sales sync: conexión %d terminada: %d guardada(s), %d con error, cursor avanzó=%t, duración %s",
		connectionID, result.SavedCount, result.FailedCount, result.CursorAt != nil, time.Since(startedAt).Round(time.Second))
	return nil
}

func (s *SyncService) keepLock(ctx context.Context, cancel context.CancelCauseFunc, connectionID int64, token string) {
	ticker := time.NewTicker(lockRenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			owned, err := s.state.RenewLock(ctx, connectionID, token, time.Now().Add(lockDuration))
			if err != nil {
				log.Printf("sales sync: conexión %d: no se pudo renovar el bloqueo: %v", connectionID, err)
				continue
			}
			if !owned {
				cancel(errLockLost)
				return
			}
		}
	}
}

// run recorre la ventana y las ventas abiertas. El cursor solo avanza (a la
// hora de inicio) si la ventana se recorrió y guardó completa; un fallo al
// refrescar una venta abierta no lo frena, porque se reintenta igual.
func (s *SyncService) run(ctx context.Context, reader Reader, connectionID int64, startedAt time.Time, actorID *int64) mysqlInfra.SaleSyncResult {
	failure := func(err error) mysqlInfra.SaleSyncResult {
		message := err.Error()
		return mysqlInfra.SaleSyncResult{Error: &message}
	}

	state, err := s.state.Find(ctx, connectionID)
	if err != nil {
		return failure(err)
	}
	if state == nil {
		return failure(errors.New("no existe el control de sincronización de la conexión"))
	}

	session, err := reader.Open(ctx, connectionID)
	if err != nil {
		return failure(fmt.Errorf("no se pudo abrir la conexión: %w", err))
	}

	progress := &runProgress{processed: make(map[string]bool)}
	window := syncWindow(state, s.config.Overlap)
	searchErr := session.Search(ctx, window, s.handler(connectionID, state.FloorAt, actorID, true, progress))
	windowFailures := progress.failed

	var openErr error
	if searchErr == nil {
		openErr = s.refreshOpen(ctx, session, connectionID, state.FloorAt, actorID, progress)
	}

	result := mysqlInfra.SaleSyncResult{SavedCount: progress.saved, FailedCount: progress.failed}
	if searchErr == nil && windowFailures == 0 && ctx.Err() == nil {
		result.CursorAt = &startedAt
	}

	messages := make([]string, 0, 4)
	if searchErr != nil {
		messages = append(messages, searchErr.Error())
	}
	if openErr != nil {
		messages = append(messages, "ventas abiertas: "+openErr.Error())
	}
	if progress.failed > 0 {
		messages = append(messages, fmt.Sprintf("%d venta(s) no se pudieron sincronizar. Primer error: %s", progress.failed, progress.firstError))
	}
	messages = append(messages, session.Warnings()...)
	if cause := context.Cause(ctx); cause != nil {
		messages = append(messages, cause.Error())
	}
	if len(messages) > 0 {
		joined := strings.Join(messages, " | ")
		result.Error = &joined
	}
	return result
}

func syncWindow(state *mysqlInfra.SaleSyncStateDTO, overlap time.Duration) Window {
	if state.CursorAt == nil {
		return Window{From: state.FloorAt, Initial: true}
	}
	return Window{From: state.CursorAt.Add(-overlap)}
}

func (s *SyncService) refreshOpen(ctx context.Context, session Session, connectionID int64, floor time.Time, actorID *int64, progress *runProgress) error {
	since := time.Now().Add(-openSalesMaxAge)
	openIDs, err := s.sales.FindOpenExternalIDs(ctx, connectionID, since)
	if err != nil {
		return err
	}
	pending := make([]string, 0, len(openIDs))
	for _, id := range openIDs {
		if !progress.processed[id] {
			pending = append(pending, id)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	progress.consecutive = 0
	return session.Lookup(ctx, pending, s.handler(connectionID, floor, actorID, false, progress))
}

type runProgress struct {
	processed   map[string]bool
	saved       int
	failed      int
	consecutive int
	firstError  string
}

func (p *runProgress) fail(externalID string, err error) error {
	p.failed++
	p.consecutive++
	if p.firstError == "" {
		p.firstError = fmt.Sprintf("venta %s: %v", externalID, err)
	}
	log.Printf("sales sync: venta %s: %v", externalID, err)
	if p.consecutive >= maxConsecutiveFailures {
		return errTooManyFailures
	}
	return nil
}

// isUnchanged: la venta guardada está completa y la orden no se ha modificado,
// así que no hace falta volver a pedir su detalle.
func isUnchanged(known *mysqlInfra.SaleKnownState, updatedAt *time.Time) bool {
	if known == nil || known.Incomplete || known.ExternalUpdatedAt == nil || updatedAt == nil {
		return false
	}
	// DATETIME no guarda fracciones de segundo.
	difference := known.ExternalUpdatedAt.Sub(*updatedAt)
	return difference > -time.Second && difference < time.Second
}

func (s *SyncService) handler(connectionID int64, floor time.Time, actorID *int64, skipUnchanged bool, progress *runProgress) HandleFunc {
	return func(ctx context.Context, candidate Candidate) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if candidate.ExternalID != "" && progress.processed[candidate.ExternalID] {
			return nil
		}

		known, err := s.sales.FindKnownState(ctx, connectionID, candidate.ExternalID)
		if err != nil {
			return progress.fail(candidate.ExternalID, err)
		}
		if skipUnchanged && isUnchanged(known, candidate.ExternalUpdatedAt) {
			return nil
		}

		record, err := candidate.Load(ctx, known)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return progress.fail(candidate.ExternalID, err)
		}
		// El piso solo filtra ventas nuevas.
		if known == nil && record.OrderedAt.Before(floor) {
			return nil
		}
		record.ConnectionID = connectionID

		if err := s.save(ctx, record, actorID); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return progress.fail(candidate.ExternalID, err)
		}
		progress.processed[candidate.ExternalID] = true
		progress.saved++
		progress.consecutive = 0
		return nil
	}
}

func (s *SyncService) save(ctx context.Context, record *mysqlInfra.SaleRecord, actorID *int64) error {
	if err := s.linkProducts(ctx, record); err != nil {
		return err
	}
	return mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		_, err := mysqlInfra.NewSalesRepository(tx).Save(ctx, *record, actorID, time.Now())
		return err
	})
}

// linkProducts vincula cada línea con el catálogo: primero por la publicación
// y después por SKU. Sin coincidencia la línea queda sin vincular.
func (s *SyncService) linkProducts(ctx context.Context, record *mysqlInfra.SaleRecord) error {
	for i := range record.Items {
		item := &record.Items[i]
		if item.ListingExternalID != "" {
			productID, err := s.sales.FindProductIDByListing(ctx, record.ConnectionID, item.ListingExternalID)
			if err != nil {
				return err
			}
			if productID != nil {
				item.ProductID, item.MatchMethod = productID, strPtr("listing")
				continue
			}
		}
		if item.SKU != nil {
			productID, err := s.sales.FindProductIDBySKU(ctx, *item.SKU)
			if err != nil {
				return err
			}
			if productID != nil {
				item.ProductID, item.MatchMethod = productID, strPtr("sku")
			}
		}
	}
	return nil
}

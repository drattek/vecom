package notifications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	channelListingsApp "core-orchestrator/internal/application/channel_listings"
	"core-orchestrator/internal/domain"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

const (
	meliChannelCode = "MERCADOLIBRE"
	closedMapStatus = "closed"

	// reasonGiveUpAfter: si el motivo no se puede consultar en este plazo, el
	// cierre se da por sin motivo.
	reasonGiveUpAfter = 20 * time.Hour
	// closureNoticeMaxAge: un cierre más antiguo ya no se avisa.
	closureNoticeMaxAge = 7 * 24 * time.Hour

	closureRowsInMessage = 5
)

// baselineSlot identifica la corrida línea base de cada conexión en
// ecom_scheduled_job_runs.
var baselineSlot = time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local)

// Estados con los que MercadoLibre deja claro que la baja es suya, y estados que
// permiten corregirla. Pendiente de confirmar con respuestas reales (ADR 0012).
var (
	definitiveSubStatuses = map[string]bool{
		"forbidden": true, "suspended": true, "suspended_for_prevention": true,
		"freezed": true, "held": true, "expired": true,
	}
	fixableSubStatuses = map[string]bool{"waiting_for_patch": true, "pending_documentation": true}
)

// ClosureSource lo implementa el servicio de publicaciones de MercadoLibre.
type ClosureSource interface {
	ListingLiveStatuses(ctx context.Context, connectionID int64) ([]domain.ListingLiveStatus, error)
	ClosureReason(ctx context.Context, connectionID int64, externalID string, useHistory bool) (domain.ClosureReason, bool, error)
}

type closureStore interface {
	ListActive(ctx context.Context, connectionID int64) ([]mysqlInfra.ListingClosureDTO, error)
	Save(ctx context.Context, input mysqlInfra.SaveListingClosureInput) error
	SetResolution(ctx context.Context, id int64, kind, reasonStatus, moderationName, reason, remedy string) error
	MarkReopened(ctx context.Context, id int64, at time.Time) error
	ListPendingNotice(ctx context.Context, since time.Time) ([]mysqlInfra.ListingClosureDTO, error)
	HasStatusEvent(ctx context.Context, mapID int64, message string) (bool, error)
}

// listingCloser deja en el mapa local que una publicación está cerrada, con su
// evento en la bitácora.
type listingCloser func(ctx context.Context, connectionID int64, listing domain.ListingLiveStatus) error

func newSQLListingCloser(db *sql.DB) listingCloser {
	return func(ctx context.Context, connectionID int64, listing domain.ListingLiveStatus) error {
		return mysqlInfra.WithinTx(ctx, db, func(tx *sql.Tx) error {
			if err := mysqlInfra.NewChannelProductMapRepository(tx).UpdateStatus(ctx, listing.MapID, closedMapStatus, ""); err != nil {
				return err
			}
			return mysqlInfra.NewChannelListingEventsRepository(tx).Record(ctx, mysqlInfra.RecordListingEventInput{
				ProductID: listing.ProductID, ConnectionID: connectionID, VehicleFitmentID: listing.VehicleFitmentID,
				ChannelProductMapID: &listing.MapID, ExternalID: listing.ExternalID,
				Action: mysqlInfra.ListingEventStatusSync, Outcome: mysqlInfra.ListingEventSuccess,
				StatusBefore: listing.LocalStatus, StatusAfter: closedMapStatus,
				Message: fmt.Sprintf("el marketplace cambió el estado: %s → %s", listing.LocalStatus, closedMapStatus),
			})
		})
	}
}

// RegisterClosureSource se llama una sola vez, durante el arranque.
func (s *Service) RegisterClosureSource(source ClosureSource) {
	s.closureSource = source
}

// checkListingClosures hace la revisión del horario slot en las conexiones
// activas de MercadoLibre y avisa, agrupados, los cierres definitivos nuevos.
// Cada conexión se registra aparte: si una falla, solo ella se reintenta.
func (s *Service) checkListingClosures(ctx context.Context, slot time.Time) error {
	if s.closureSource == nil {
		return errors.New("no closure source registered")
	}
	connections, err := s.connections.FindAllActive(ctx)
	if err != nil {
		return err
	}

	reviewed := false
	var failures []error
	for _, connection := range connections {
		if !strings.EqualFold(strings.TrimSpace(connection.ChannelCode), meliChannelCode) {
			continue
		}
		jobCode := fmt.Sprintf("meli_closures:%d", connection.ID)
		err := s.runSlot(ctx, jobCode, slot, closureRetryDelay, func(ctx context.Context) (jobWindow, error) {
			reviewed = true
			return jobWindow{}, s.checkConnectionClosures(ctx, connection.ID)
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("connection %d: %w", connection.ID, err))
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	// Lo detectado se avisa aunque otra conexión haya fallado.
	if reviewed {
		if err := s.notifyClosures(ctx); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s *Service) checkConnectionClosures(ctx context.Context, connectionID int64) error {
	// La primera revisión de cada conexión es la línea base: registra lo que ya
	// estaba cerrado sin avisarlo, para no enviar todo el histórico.
	baselineCode := fmt.Sprintf("meli_baseline:%d", connectionID)
	baselined, err := s.jobs.IsDone(ctx, baselineCode, baselineSlot)
	if err != nil {
		return err
	}

	statuses, err := s.closureSource.ListingLiveStatuses(ctx, connectionID)
	if err != nil {
		return err
	}
	active, err := s.closures.ListActive(ctx, connectionID)
	if err != nil {
		return err
	}
	recorded := make(map[string]mysqlInfra.ListingClosureDTO, len(active))
	for _, closure := range active {
		recorded[closure.ExternalID] = closure
	}

	// Varias filas locales pueden apuntar a la misma publicación.
	byExternalID := map[string][]domain.ListingLiveStatus{}
	order := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if _, seen := byExternalID[status.ExternalID]; !seen {
			order = append(order, status.ExternalID)
		}
		byExternalID[status.ExternalID] = append(byExternalID[status.ExternalID], status)
	}

	for _, externalID := range order {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var previous *mysqlInfra.ListingClosureDTO
		if closure, ok := recorded[externalID]; ok {
			previous = &closure
		}
		if err := s.reviewListing(ctx, connectionID, byExternalID[externalID], previous, !baselined); err != nil {
			return err
		}
	}

	if baselined {
		return nil
	}
	if _, err := s.jobs.Claim(ctx, baselineCode, baselineSlot, s.now(), jobLock); err != nil {
		return err
	}
	return s.jobs.Complete(ctx, baselineCode, baselineSlot, s.now(), nil, nil)
}

// reviewListing procesa una publicación (todas sus filas locales).
func (s *Service) reviewListing(ctx context.Context, connectionID int64, rows []domain.ListingLiveStatus, previous *mysqlInfra.ListingClosureDTO, baseline bool) error {
	live := rows[0]
	if live.Found && !live.Closed {
		if previous != nil {
			return s.closures.MarkReopened(ctx, previous.ID, s.now())
		}
		return nil
	}

	if !live.Found {
		// Sin respuesta del marketplace solo cuenta lo que ya estaba cerrado.
		for _, row := range rows {
			if row.LocalStatus != closedMapStatus {
				return nil
			}
		}
	} else {
		for _, row := range rows {
			if row.LocalStatus == closedMapStatus {
				continue
			}
			if err := s.closeListing(ctx, connectionID, row); err != nil {
				return err
			}
		}
	}

	if previous != nil {
		if previous.ReasonStatus != mysqlInfra.ClosureReasonPending {
			return nil
		}
		resolution, err := s.resolveClosure(ctx, connectionID, rows)
		if err != nil {
			return err
		}
		if resolution.reasonStatus == mysqlInfra.ClosureReasonPending && s.now().Sub(previous.DetectedAt) >= reasonGiveUpAfter {
			resolution.reasonStatus = mysqlInfra.ClosureReasonUnavailable
		}
		return s.closures.SetResolution(ctx, previous.ID, resolution.kind, resolution.reasonStatus,
			resolution.reason.ModerationName, resolution.reason.Reason, resolution.reason.Remedy)
	}

	input := mysqlInfra.SaveListingClosureInput{
		ConnectionID: connectionID, ProductID: live.ProductID, MapID: live.MapID, ExternalID: live.ExternalID,
		ExternalStatus: live.RawStatus, SubStatus: strings.Join(live.SubStatus, ", "), DetectedAt: s.now(),
	}
	if baseline {
		// No se consulta el motivo de lo que no se va a avisar.
		now := s.now()
		input.Kind = classifyClosure(live.SubStatus, false)
		input.ReasonStatus = mysqlInfra.ClosureReasonUnavailable
		input.NotifiedAt = &now
		return s.closures.Save(ctx, input)
	}

	resolution, err := s.resolveClosure(ctx, connectionID, rows)
	if err != nil {
		return err
	}
	input.Kind, input.ReasonStatus = resolution.kind, resolution.reasonStatus
	input.ModerationName = resolution.reason.ModerationName
	input.Reason, input.Remedy = resolution.reason.Reason, resolution.reason.Remedy
	return s.closures.Save(ctx, input)
}

type closureResolution struct {
	kind         string
	reasonStatus string
	reason       domain.ClosureReason
}

func hasAny(subStatuses []string, set map[string]bool) bool {
	for _, subStatus := range subStatuses {
		if set[strings.ToLower(strings.TrimSpace(subStatus))] {
			return true
		}
	}
	return false
}

// classifyClosure: sin un estado concluyente, solo se atribuye a MercadoLibre si
// hay una moderación. Sin ella se asume que la cerró el vendedor o el sistema.
func classifyClosure(subStatuses []string, moderated bool) string {
	switch {
	case hasAny(subStatuses, definitiveSubStatuses):
		return mysqlInfra.ClosureDefinitive
	case hasAny(subStatuses, fixableSubStatuses):
		return mysqlInfra.ClosureFixable
	case moderated:
		return mysqlInfra.ClosureDefinitive
	default:
		return mysqlInfra.ClosureSystem
	}
}

func (s *Service) resolveClosure(ctx context.Context, connectionID int64, rows []domain.ListingLiveStatus) (closureResolution, error) {
	for _, row := range rows {
		ours, err := s.closures.HasStatusEvent(ctx, row.MapID, channelListingsApp.ClosedForRepublishMessage)
		if err != nil {
			return closureResolution{}, err
		}
		if ours {
			return closureResolution{kind: mysqlInfra.ClosureSystem, reasonStatus: mysqlInfra.ClosureReasonUnavailable}, nil
		}
	}

	live := rows[0]
	conclusive := hasAny(live.SubStatus, definitiveSubStatuses)
	reason, found, err := s.closureSource.ClosureReason(ctx, connectionID, live.ExternalID, conclusive)
	if err != nil {
		// Se reintenta en la siguiente revisión.
		logReportError("reading closure reason of "+live.ExternalID, err)
		return closureResolution{kind: classifyClosure(live.SubStatus, false), reasonStatus: mysqlInfra.ClosureReasonPending}, nil
	}

	resolution := closureResolution{
		kind:         classifyClosure(live.SubStatus, found),
		reasonStatus: mysqlInfra.ClosureReasonUnavailable,
		reason:       reason,
	}
	if found {
		resolution.reasonStatus = mysqlInfra.ClosureReasonFound
	}
	return resolution, nil
}

func (s *Service) notifyClosures(ctx context.Context) error {
	closures, err := s.closures.ListPendingNotice(ctx, s.now().Add(-closureNoticeMaxAge))
	if err != nil || len(closures) == 0 {
		return err
	}
	n := closureNotice(closures)

	ids := make([]int64, 0, len(closures))
	keys := make([]string, 0, len(closures))
	for _, closure := range closures {
		ids = append(ids, closure.ID)
		keys = append(keys, fmt.Sprint(closure.ID))
	}

	return s.inUnit(ctx, func(tx unit) error {
		// Sin destinatarios queda pendiente para la siguiente revisión.
		users, err := s.audience(ctx, tx, n)
		if err != nil || len(users) == 0 {
			return err
		}
		if err := s.enqueue(ctx, tx, fmt.Sprintf("closures:%.40s", signatureOf(keys)), n); err != nil {
			return err
		}
		return tx.MarkClosuresNotified(ctx, ids, s.now())
	})
}

func closureReasonText(closure mysqlInfra.ListingClosureDTO) string {
	if closure.Reason != "" {
		return shortError(closure.Reason)
	}
	state := closure.SubStatus
	if state == "" {
		state = closedMapStatus
	}
	return "MercadoLibre no informó el motivo (estado: " + state + ")."
}

func closureNotice(closures []mysqlInfra.ListingClosureDTO) notice {
	var text strings.Builder
	fmt.Fprintf(&text, "⛔ Publicaciones cerradas definitivamente por MercadoLibre: %d", len(closures))
	n := notice{notificationType: TypeMeliListingClosed}

	if len(closures) <= closureRowsInMessage {
		for _, closure := range closures {
			fmt.Fprintf(&text, "\n\n• %s · %s · %s\nMotivo: %s", closure.SKU, closure.ConnectionName, closure.ExternalID, closureReasonText(closure))
			if closure.Remedy != "" {
				fmt.Fprintf(&text, "\nSugerencia: %s", shortError(closure.Remedy))
			}
		}
		n.text = text.String()
		return n
	}

	count := map[string]int{}
	reasons := make([]string, 0)
	rows := make([][]string, 0, len(closures))
	for _, closure := range closures {
		reason := closureReasonText(closure)
		if count[reason] == 0 {
			reasons = append(reasons, reason)
		}
		count[reason]++
		rows = append(rows, []string{
			closure.SKU, closure.ProductName, closure.ConnectionName, closure.ExternalID,
			closure.SubStatus, sanitize(closure.Reason), sanitize(closure.Remedy),
		})
	}
	sort.SliceStable(reasons, func(i, j int) bool { return count[reasons[i]] > count[reasons[j]] })

	text.WriteString("\n\nMotivos principales:")
	for i, reason := range reasons {
		if i == publicationTopCauses {
			break
		}
		fmt.Fprintf(&text, "\n• (%d) %s", count[reason], truncate(reason, 180))
	}
	text.WriteString("\n\nEl detalle va en el archivo.")
	n.text = text.String()
	n.attachmentName = "publicaciones-cerradas.csv"
	n.attachment = buildCSV([]string{"sku", "producto", "conexion", "publicacion", "estado", "motivo", "sugerencia"}, rows)
	return n
}

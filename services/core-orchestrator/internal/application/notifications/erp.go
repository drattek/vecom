package notifications

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"core-orchestrator/internal/domain"
)

const (
	// reportTimeout acota lo que un reporte puede demorar a quien lo hace.
	reportTimeout = 20 * time.Second
	// erpSamplesInMessage es cuántos registros fallidos se citan en el texto.
	erpSamplesInMessage = 3
)

var erpFeedLabels = map[string]string{
	domain.ErpFeedProducts:    "productos del ERP",
	domain.ErpFeedStock:       "stock del ERP",
	domain.ErpFeedNissanStock: "existencias Nissan",
}

func erpFeedLabel(feed string) string {
	if label, ok := erpFeedLabels[feed]; ok {
		return label
	}
	return feed
}

// reportContext separa el reporte del ciclo de vida de quien lo hace y lo
// acota: avisar nunca debe interrumpir ni frenar una sincronización.
func reportContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), reportTimeout)
}

func logReportError(what string, err error) {
	if err != nil {
		log.Printf("notifications: %s: %v", what, err)
	}
}

// ErpSyncFailed registra que una sincronización falló. No devuelve error: los
// problemas al avisar solo se registran.
func (s *Service) ErpSyncFailed(ctx context.Context, feed, reason string) {
	ctx, cancel := reportContext(ctx)
	defer cancel()

	reason = shortError(reason)
	logReportError("recording erp failure", s.erpState.RecordFailed(ctx, feed, reason, s.now()))

	logReportError("reporting erp failure", s.openIncident(ctx,
		incidentSpec{incidentType: incidentErpFailed, key: incidentErpFailed + ":" + feed, detail: reason},
		notice{
			notificationType: TypeErpSyncFailure,
			text:             fmt.Sprintf("🔴 Falló la sincronización de %s.\n\n%s", erpFeedLabel(feed), reason),
		},
	))
}

// ErpSyncCompleted registra una sincronización terminada: cierra los incidentes
// de falla y de silencio, y avisa las fallas parciales.
func (s *Service) ErpSyncCompleted(ctx context.Context, result domain.ErpSyncResult) {
	ctx, cancel := reportContext(ctx)
	defer cancel()

	label := erpFeedLabel(result.Feed)
	logReportError("recording erp completion",
		s.erpState.RecordCompleted(ctx, result.Feed, result.Total, result.Failed, s.now()))

	for _, incidentType := range []string{incidentErpFailed, incidentErpSilence} {
		logReportError("resolving erp incident", s.resolveIncident(ctx, incidentType+":"+result.Feed, &notice{
			notificationType: TypeErpSyncFailure,
			text:             fmt.Sprintf("✅ La sincronización de %s volvió a completarse.", label),
		}))
	}

	partialKey := incidentErpPartial + ":" + result.Feed
	if result.Failed == 0 {
		logReportError("resolving erp partial failure", s.resolveIncident(ctx, partialKey, &notice{
			notificationType: TypeErpSyncFailure,
			text:             fmt.Sprintf("✅ La sincronización de %s terminó sin registros fallidos.", label),
		}))
		return
	}

	// La firma son los registros fallidos: mientras sean los mismos, no se
	// vuelve a avisar.
	codes := make([]string, 0, len(result.Failures))
	rows := make([][]string, 0, len(result.Failures))
	var text strings.Builder
	fmt.Fprintf(&text, "🟠 La sincronización de %s terminó con %d de %d registros fallidos.", label, result.Failed, result.Total)
	for i, failure := range result.Failures {
		codes = append(codes, failure.Code)
		rows = append(rows, []string{failure.Code, sanitize(failure.Error)})
		if i < erpSamplesInMessage {
			fmt.Fprintf(&text, "\n• %s: %s", failure.Code, truncate(sanitize(failure.Error), 160))
		}
	}
	if len(codes) == 0 {
		codes = []string{fmt.Sprintf("failed=%d", result.Failed)}
	}

	n := notice{notificationType: TypeErpSyncFailure, text: text.String()}
	if len(rows) > erpSamplesInMessage {
		n.attachmentName = "errores-" + result.Feed + ".csv"
		n.attachment = buildCSV([]string{"codigo", "error"}, rows)
	}

	logReportError("reporting erp partial failure", s.openIncident(ctx, incidentSpec{
		incidentType: incidentErpPartial,
		key:          partialKey,
		signature:    signatureOf(codes),
		detail:       fmt.Sprintf("%d de %d registros fallidos", result.Failed, result.Total),
	}, n))
}

// checkErpSilence avisa de los flujos que llevan demasiado sin completarse.
// Cubre que synapse-bridge esté caído y no publique ningún evento.
func (s *Service) checkErpSilence(ctx context.Context) error {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return err
	}
	states, err := s.erpState.List(ctx)
	if err != nil {
		return err
	}

	limit := time.Duration(settings.ErpSilenceHours) * time.Hour
	for _, state := range states {
		since := state.CreatedAt
		last := "Todavía no se ha registrado ninguna."
		if state.LastCompletedAt != nil {
			since = *state.LastCompletedAt
			last = "La última terminó el " + since.Format("02/01/2006 15:04") + "."
		}
		if s.now().Sub(since) <= limit {
			continue
		}
		if err := s.openIncident(ctx,
			incidentSpec{incidentType: incidentErpSilence, key: incidentErpSilence + ":" + state.FeedCode, detail: last},
			notice{
				notificationType: TypeErpSyncFailure,
				text: fmt.Sprintf("🔴 No se ha completado la sincronización de %s en más de %d horas. %s\nRevisa synapse-bridge y RabbitMQ.",
					erpFeedLabel(state.FeedCode), settings.ErpSilenceHours, last),
			},
		); err != nil {
			return err
		}
	}
	return nil
}

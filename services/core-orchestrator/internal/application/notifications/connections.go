package notifications

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	syncApp "core-orchestrator/internal/application/sync"
	odooInfra "core-orchestrator/internal/infrastructure/marketplace/odoo"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/shared/safe"
)

const (
	// failuresBeforeAlert: fallos seguidos para avisar de un error que no sea
	// de autenticación (p. ej. de red).
	failuresBeforeAlert = 2
	// recheckDelay: tras un primer fallo se vuelve a comprobar pronto.
	recheckDelay = 5 * time.Minute
	// checkTimeout acota la comprobación de una conexión.
	checkTimeout = 90 * time.Second
)

// ConnectionCheck comprueba contra el marketplace que una conexión funciona.
type ConnectionCheck func(ctx context.Context, connectionID int64) error

type connectionLister interface {
	FindAllActive(ctx context.Context) ([]mysqlInfra.ChannelConnectionDTO, error)
	FindByID(ctx context.Context, id int64) (*mysqlInfra.ChannelConnectionDTO, error)
}

type incidentLister interface {
	ListOpenByType(ctx context.Context, incidentType string) ([]mysqlInfra.NotificationIncidentDTO, error)
}

// connectionState es la memoria del monitor sobre una conexión. Se pierde al
// reiniciar; los incidentes abiertos quedan en la base.
type connectionState struct {
	failures  int
	nextCheck time.Time
}

// RegisterConnectionCheck asocia la comprobación de un canal
// (ecom_channels.code). Los canales sin comprobación no se revisan. Llamar solo
// durante el arranque.
func (s *Service) RegisterConnectionCheck(channelCode string, check ConnectionCheck) {
	s.connectionChecks[strings.ToUpper(strings.TrimSpace(channelCode))] = check
}

var authFailurePattern = regexp.MustCompile(
	`oauth request failed (?:\((?:400|401|403)\)|with status (?:400|401|403))|fetch failed: \((?:401|403)\)`)

// isAuthFailure distingue credenciales o token rechazados (se avisa de
// inmediato) de un fallo que puede ser pasajero.
func isAuthFailure(err error) bool {
	if errors.Is(err, syncApp.ErrMissingMercadoLibreSettings) ||
		errors.Is(err, syncApp.ErrMissingOdooSettings) ||
		errors.Is(err, syncApp.ErrMissingOdooCredentials) {
		return true
	}
	var statusErr *odooInfra.StatusError
	if errors.As(err, &statusErr) {
		return statusErr.Status == 401 || statusErr.Status == 403
	}
	return authFailurePattern.MatchString(err.Error())
}

// ConnectionAuthResult recibe el resultado de las autenticaciones que ocurren
// durante la operación normal. Solo reacciona a un rechazo de credenciales y a
// una renovación correcta; lo demás lo confirma la comprobación periódica.
func (s *Service) ConnectionAuthResult(ctx context.Context, connectionID int64, authErr error, refreshed bool) {
	switch {
	case authErr == nil && !refreshed:
		return
	case authErr != nil && (errors.Is(authErr, context.Canceled) || !isAuthFailure(authErr)):
		return
	}

	ctx, cancel := reportContext(ctx)
	defer cancel()

	if authErr == nil {
		logReportError("resolving connection incident", s.connectionRecovered(ctx, connectionID, s.connectionName(ctx, connectionID)))
		return
	}
	logReportError("reporting connection failure", s.connectionDown(ctx, connectionID, s.connectionName(ctx, connectionID), authErr))
}

func (s *Service) connectionName(ctx context.Context, connectionID int64) string {
	connection, err := s.connections.FindByID(ctx, connectionID)
	if err != nil {
		return fmt.Sprintf("conexión %d", connectionID)
	}
	return connection.Name
}

func connectionKey(connectionID int64) string {
	return incidentConnectionDown + ":" + strconv.FormatInt(connectionID, 10)
}

func (s *Service) connectionDown(ctx context.Context, connectionID int64, name string, cause error) error {
	detail := shortError(cause.Error())
	headline := fmt.Sprintf("🔴 La conexión \"%s\" no está funcionando.", name)
	if isAuthFailure(cause) {
		headline = fmt.Sprintf("🔴 La conexión \"%s\" rechazó las credenciales o el token. Vuelve a vincularla o revisa sus datos.", name)
	}
	return s.openIncident(ctx,
		incidentSpec{incidentType: incidentConnectionDown, key: connectionKey(connectionID), connectionID: &connectionID, detail: detail},
		notice{notificationType: TypeConnectionDown, text: headline + "\n\n" + detail},
	)
}

func (s *Service) connectionRecovered(ctx context.Context, connectionID int64, name string) error {
	return s.resolveIncident(ctx, connectionKey(connectionID), &notice{
		notificationType: TypeConnectionDown,
		text:             fmt.Sprintf("✅ La conexión \"%s\" volvió a funcionar.", name),
	})
}

// checkConnections comprueba las conexiones activas a las que ya les toca.
func (s *Service) checkConnections(ctx context.Context) error {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return err
	}
	interval := time.Duration(settings.ConnectionCheckMinutes) * time.Minute

	connections, err := s.connections.FindAllActive(ctx)
	if err != nil {
		return err
	}

	active := make(map[int64]bool, len(connections))
	var failures []error
	for _, connection := range connections {
		active[connection.ID] = true
		check, ok := s.connectionChecks[strings.ToUpper(strings.TrimSpace(connection.ChannelCode))]
		if !ok {
			continue
		}
		state := s.connectionStates[connection.ID]
		if state == nil {
			state = &connectionState{}
			s.connectionStates[connection.ID] = state
		}
		if s.now().Before(state.nextCheck) {
			continue
		}

		var checkErr error
		completed := safe.Do(fmt.Sprintf("connection check %d", connection.ID), func() {
			checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
			defer cancel()
			checkErr = check(checkCtx, connection.ID)
		})
		if ctx.Err() != nil {
			return nil
		}
		state.nextCheck = s.now().Add(interval)
		if !completed {
			continue
		}

		if checkErr == nil {
			state.failures = 0
			if err := s.connectionRecovered(ctx, connection.ID, connection.Name); err != nil {
				failures = append(failures, err)
			}
			continue
		}

		state.failures++
		if !isAuthFailure(checkErr) && state.failures < failuresBeforeAlert {
			state.nextCheck = s.now().Add(min(recheckDelay, interval))
			continue
		}
		if err := s.connectionDown(ctx, connection.ID, connection.Name, checkErr); err != nil {
			failures = append(failures, err)
		}
	}

	// Una conexión dada de baja o desactivada deja de ser un problema.
	open, err := s.incidents.ListOpenByType(ctx, incidentConnectionDown)
	if err != nil {
		return err
	}
	for _, incident := range open {
		if incident.ConnectionID != nil && !active[*incident.ConnectionID] {
			if err := s.resolveIncident(ctx, incident.IncidentKey, nil); err != nil {
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}

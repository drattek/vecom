package notifications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"core-orchestrator/internal/domain"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// Tipos de incidente (ecom_notification_incidents.incident_type).
const (
	incidentErpFailed      = "erp_failed"
	incidentErpPartial     = "erp_partial"
	incidentErpSilence     = "erp_silence"
	incidentConnectionDown = "connection_down"
	// incidentPublicationError es uno por producto y conexión.
	incidentPublicationError = "publication_error"
)

// notice es un aviso por encolar.
type notice struct {
	notificationType string
	text             string
	attachmentName   string
	attachment       []byte
	// onlyUserID > 0 lo limita a ese usuario.
	onlyUserID int64
}

type incidentSpec struct {
	incidentType string
	key          string
	// signature: si cambia en un incidente abierto, se vuelve a avisar. Vacía
	// significa avisar solo al abrirlo.
	signature    string
	connectionID *int64
	detail       string
}

// unit son las operaciones que deben ocurrir juntas al reportar un hecho: el
// incidente y los envíos que provoca.
type unit interface {
	FindOpenIncident(ctx context.Context, key string) (*mysqlInfra.NotificationIncidentDTO, error)
	OpenIncident(ctx context.Context, input mysqlInfra.OpenNotificationIncidentInput) (int64, error)
	TouchIncident(ctx context.Context, id int64, signature, message string, seenAt time.Time) error
	ResolveIncident(ctx context.Context, id int64, at time.Time) error
	Recipients(ctx context.Context, notificationType, botUsername string, onlyUserID int64) ([]mysqlInfra.NotificationRecipientDTO, error)
	Permissions(ctx context.Context, userID int64, roleID *int64) ([]string, error)
	Delivered(ctx context.Context, userID int64, dedupeKey string) (bool, error)
	Enqueue(ctx context.Context, input mysqlInfra.EnqueueNotificationInput) error
	MarkClosuresNotified(ctx context.Context, ids []int64, at time.Time) error
}

type unitRunner func(ctx context.Context, fn func(unit) error) error

type sqlUnit struct {
	incidents  *mysqlInfra.NotificationIncidentRepository
	queries    *mysqlInfra.NotificationQueryRepository
	deliveries *mysqlInfra.NotificationDeliveryRepository
	rbac       *mysqlInfra.RBACRepository
	closures   *mysqlInfra.ListingClosureRepository
}

// maxUnitAttempts: reintentos cuando dos reportes simultáneos chocan.
const maxUnitAttempts = 3

// isConcurrencyConflict: bloqueo mutuo, o el otro reporte abrió antes el mismo
// incidente. La transacción se deshizo entera, así que se puede repetir.
func isConcurrencyConflict(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && (mysqlErr.Number == 1213 || mysqlErr.Number == 1062)
}

func newSQLUnitRunner(db *sql.DB) unitRunner {
	return func(ctx context.Context, fn func(unit) error) error {
		var err error
		for attempt := 0; attempt < maxUnitAttempts; attempt++ {
			err = mysqlInfra.WithinTx(ctx, db, func(tx *sql.Tx) error {
				return fn(sqlUnit{
					incidents:  mysqlInfra.NewNotificationIncidentRepository(tx),
					queries:    mysqlInfra.NewNotificationQueryRepository(tx),
					deliveries: mysqlInfra.NewNotificationDeliveryRepository(tx),
					rbac:       mysqlInfra.NewRBACRepository(tx),
					closures:   mysqlInfra.NewListingClosureRepository(tx),
				})
			})
			if !isConcurrencyConflict(err) {
				break
			}
		}
		return err
	}
}

func (u sqlUnit) FindOpenIncident(ctx context.Context, key string) (*mysqlInfra.NotificationIncidentDTO, error) {
	return u.incidents.FindOpenForUpdate(ctx, key)
}

func (u sqlUnit) OpenIncident(ctx context.Context, input mysqlInfra.OpenNotificationIncidentInput) (int64, error) {
	return u.incidents.Open(ctx, input)
}

func (u sqlUnit) TouchIncident(ctx context.Context, id int64, signature, message string, seenAt time.Time) error {
	return u.incidents.Touch(ctx, id, signature, message, seenAt)
}

func (u sqlUnit) ResolveIncident(ctx context.Context, id int64, at time.Time) error {
	return u.incidents.Resolve(ctx, id, at)
}

func (u sqlUnit) Recipients(ctx context.Context, notificationType, botUsername string, onlyUserID int64) ([]mysqlInfra.NotificationRecipientDTO, error) {
	return u.queries.Recipients(ctx, notificationType, botUsername, onlyUserID)
}

func (u sqlUnit) Permissions(ctx context.Context, userID int64, roleID *int64) ([]string, error) {
	return u.rbac.EffectivePermissions(ctx, userID, roleID)
}

func (u sqlUnit) Delivered(ctx context.Context, userID int64, dedupeKey string) (bool, error) {
	return u.deliveries.Exists(ctx, userID, dedupeKey)
}

func (u sqlUnit) Enqueue(ctx context.Context, input mysqlInfra.EnqueueNotificationInput) error {
	return u.deliveries.Enqueue(ctx, input)
}

func (u sqlUnit) MarkClosuresNotified(ctx context.Context, ids []int64, at time.Time) error {
	return u.closures.MarkNotified(ctx, ids, at)
}

// audience es a quién se le puede encolar ahora un aviso: con las
// notificaciones apagadas, a nadie.
func (s *Service) audience(ctx context.Context, tx unit, n notice) ([]int64, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.IsEnabled || settings.BotTokenEncrypted == "" {
		return nil, nil
	}

	candidates, err := tx.Recipients(ctx, n.notificationType, settings.BotUsername, n.onlyUserID)
	if err != nil {
		return nil, err
	}

	definition, ok := definitionOf(n.notificationType)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownType, n.notificationType)
	}

	// El permiso se comprueba al encolar: pudo quitarse después de activar el
	// aviso (ADR 0012).
	users := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		user := &domain.AuthUser{ID: candidate.UserID, Role: candidate.Role, RoleID: candidate.RoleID}
		if !user.IsAdmin() {
			if user.Permissions, err = tx.Permissions(ctx, candidate.UserID, candidate.RoleID); err != nil {
				return nil, err
			}
		}
		if user.HasAnyPermission(definition.Permissions...) {
			users = append(users, candidate.UserID)
		}
	}
	return users, nil
}

func (s *Service) enqueue(ctx context.Context, tx unit, dedupeKey string, n notice) error {
	users, err := s.audience(ctx, tx, n)
	if err != nil {
		return err
	}
	maxRunes := maxMessageRunes
	if len(n.attachment) > 0 {
		maxRunes = maxCaptionRunes
	}
	for _, userID := range users {
		if err := tx.Enqueue(ctx, mysqlInfra.EnqueueNotificationInput{
			UserID:            userID,
			NotificationType:  n.notificationType,
			DedupeKey:         dedupeKey,
			Message:           truncate(n.text, maxRunes),
			AttachmentName:    n.attachmentName,
			AttachmentContent: n.attachment,
		}); err != nil {
			return err
		}
	}
	return nil
}

// openIncident registra el problema y avisa solo si es nuevo o cambió.
func (s *Service) openIncident(ctx context.Context, spec incidentSpec, n notice) error {
	return s.inUnit(ctx, func(tx unit) error {
		now := s.now()
		current, err := tx.FindOpenIncident(ctx, spec.key)
		if err != nil {
			return err
		}

		var id int64
		if current == nil {
			// Sin nadie a quien avisar no se abre: si no, quien active el aviso
			// después nunca se enteraría de un problema que sigue presente.
			users, err := s.audience(ctx, tx, n)
			if err != nil || len(users) == 0 {
				return err
			}
			if id, err = tx.OpenIncident(ctx, mysqlInfra.OpenNotificationIncidentInput{
				IncidentType: spec.incidentType,
				IncidentKey:  spec.key,
				Signature:    spec.signature,
				ConnectionID: spec.connectionID,
				Message:      spec.detail,
				At:           now,
			}); err != nil {
				return err
			}
		} else {
			if err := tx.TouchIncident(ctx, current.ID, spec.signature, spec.detail, now); err != nil {
				return err
			}
			if current.Signature == spec.signature {
				return nil
			}
			id = current.ID
		}

		// La firma distingue cada cambio; la hora, que un conjunto anterior
		// vuelva a presentarse.
		dedupeKey := fmt.Sprintf("incident:%d:%.16s:%d", id, spec.signature, now.UnixNano())
		return s.enqueue(ctx, tx, dedupeKey, n)
	})
}

// resolveIncident cierra el incidente abierto, si lo hay, y avisa la
// recuperación. Con recovery nil lo cierra sin avisar.
func (s *Service) resolveIncident(ctx context.Context, key string, recovery *notice) error {
	return s.inUnit(ctx, func(tx unit) error {
		current, err := tx.FindOpenIncident(ctx, key)
		if err != nil || current == nil {
			return err
		}
		if err := tx.ResolveIncident(ctx, current.ID, s.now()); err != nil {
			return err
		}
		if recovery == nil {
			return nil
		}
		return s.enqueue(ctx, tx, "incident:"+strconv.FormatInt(current.ID, 10)+":resolved", *recovery)
	})
}

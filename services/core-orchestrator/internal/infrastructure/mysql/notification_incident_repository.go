package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// NotificationIncidentDTO es un problema abierto o resuelto
// (ecom_notification_incidents): permite avisar una sola vez y avisar la
// recuperación.
type NotificationIncidentDTO struct {
	ID           int64
	IncidentType string
	IncidentKey  string
	Signature    string
	ConnectionID *int64
	Message      string
	OpenedAt     time.Time
}

type OpenNotificationIncidentInput struct {
	IncidentType string
	IncidentKey  string
	Signature    string
	ConnectionID *int64
	ProductID    *int64
	Message      string
	At           time.Time
}

type NotificationIncidentRepository struct {
	db Querier
}

func NewNotificationIncidentRepository(db Querier) *NotificationIncidentRepository {
	return &NotificationIncidentRepository{db: db}
}

const notificationIncidentSelect = `
	SELECT id, incident_type, incident_key, signature, connection_id, message, opened_at
	FROM ecom_notification_incidents
	WHERE status = 'open' AND deleted_at IS NULL AND `

func scanNotificationIncident(scanner interface{ Scan(dest ...any) error }) (*NotificationIncidentDTO, error) {
	var (
		dto          NotificationIncidentDTO
		signature    sql.NullString
		connectionID sql.NullInt64
		message      sql.NullString
	)
	if err := scanner.Scan(&dto.ID, &dto.IncidentType, &dto.IncidentKey, &signature, &connectionID, &message, &dto.OpenedAt); err != nil {
		return nil, err
	}
	dto.Signature = signature.String
	dto.Message = message.String
	if connectionID.Valid {
		dto.ConnectionID = &connectionID.Int64
	}
	return &dto, nil
}

// FindOpenForUpdate devuelve (nil, nil) si no hay incidente abierto con la
// clave. Bloquea la fila: usar con un repositorio de transacción.
func (r *NotificationIncidentRepository) FindOpenForUpdate(ctx context.Context, key string) (*NotificationIncidentDTO, error) {
	dto, err := scanNotificationIncident(r.db.QueryRowContext(ctx, notificationIncidentSelect+"incident_key = ? FOR UPDATE", key))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading incident %s: %w", key, err)
	}
	return dto, nil
}

func (r *NotificationIncidentRepository) ListOpenByType(ctx context.Context, incidentType string) ([]NotificationIncidentDTO, error) {
	rows, err := r.db.QueryContext(ctx, notificationIncidentSelect+"incident_type = ?", incidentType)
	if err != nil {
		return nil, fmt.Errorf("listing open incidents: %w", err)
	}
	defer rows.Close()

	incidents := make([]NotificationIncidentDTO, 0)
	for rows.Next() {
		dto, err := scanNotificationIncident(rows)
		if err != nil {
			return nil, fmt.Errorf("reading incident: %w", err)
		}
		incidents = append(incidents, *dto)
	}
	return incidents, rows.Err()
}

func (r *NotificationIncidentRepository) Open(ctx context.Context, input OpenNotificationIncidentInput) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO ecom_notification_incidents
			(incident_type, incident_key, signature, connection_id, product_id, message, opened_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, input.IncidentType, input.IncidentKey, nullableString(input.Signature), input.ConnectionID, input.ProductID,
		input.Message, input.At, input.At)
	if err != nil {
		return 0, fmt.Errorf("opening incident %s: %w", input.IncidentKey, err)
	}
	return result.LastInsertId()
}

func (r *NotificationIncidentRepository) Touch(ctx context.Context, id int64, signature, message string, seenAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_notification_incidents
		SET signature = ?, message = ?, last_seen_at = ?
		WHERE id = ?
	`, nullableString(signature), message, seenAt, id)
	if err != nil {
		return fmt.Errorf("updating incident %d: %w", id, err)
	}
	return nil
}

func (r *NotificationIncidentRepository) Resolve(ctx context.Context, id int64, resolvedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ecom_notification_incidents
		SET status = 'resolved', resolved_at = ?
		WHERE id = ?
	`, resolvedAt, id)
	if err != nil {
		return fmt.Errorf("resolving incident %d: %w", id, err)
	}
	return nil
}

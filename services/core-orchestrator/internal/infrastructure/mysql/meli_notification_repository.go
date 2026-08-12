package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrMeliNotificationNotFound = errors.New("meli notification not found")

type MeliNotificationDTO struct {
	ID               int64      `json:"id"`
	NotificationID   string     `json:"notificationId"`
	Resource         string     `json:"resource"`
	Topic            string     `json:"topic"`
	MeliUserID       int64      `json:"meliUserId"`
	ApplicationID    int64      `json:"applicationId"`
	DeliveryAttempts int        `json:"deliveryAttempts"`
	MeliSentAt       *time.Time `json:"meliSentAt,omitempty"`
	MeliReceivedAt   *time.Time `json:"meliReceivedAt,omitempty"`
	IngestedAt       time.Time  `json:"ingestedAt"`
	ConnectionID     *int64     `json:"connectionId,omitempty"`
	RawPayload       string     `json:"rawPayload"`
	Status           string     `json:"status"`
	LastError        *string    `json:"lastError,omitempty"`
	ProcessedAt      *time.Time `json:"processedAt,omitempty"`
	UpdatedBy        *int64     `json:"updatedBy,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
}

type CreateMeliNotificationInput struct {
	NotificationID   string
	Resource         string
	Topic            string
	MeliUserID       int64
	ApplicationID    int64
	DeliveryAttempts int
	MeliSentAt       *time.Time
	MeliReceivedAt   *time.Time
	RawPayload       string
}

type MeliNotificationRepository struct {
	db *sql.DB
}

func NewMeliNotificationRepository(db *sql.DB) *MeliNotificationRepository {
	return &MeliNotificationRepository{db: db}
}

const meliNotificationColumns = `
	id, notification_id, resource, topic, meli_user_id, application_id, delivery_attempts,
	meli_sent_at, meli_received_at, ingested_at, connection_id, raw_payload, status, last_error,
	processed_at, updated_by, created_at, updated_at, deleted_at
`

// FindByNotificationID looks up a previously stored notification by
// MercadoLibre's own `_id`, used by the application layer to make Create
// idempotent against MercadoLibre's at-least-once redelivery.
func (r *MeliNotificationRepository) FindByNotificationID(notificationID string) (*MeliNotificationDTO, error) {
	query := `
		SELECT ` + meliNotificationColumns + `
		FROM ecom_meli_notifications
		WHERE notification_id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, notificationID)
	notification, err := scanMeliNotification(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMeliNotificationNotFound
		}
		return nil, err
	}

	return &notification, nil
}

func (r *MeliNotificationRepository) FindByID(id int64) (*MeliNotificationDTO, error) {
	query := `
		SELECT ` + meliNotificationColumns + `
		FROM ecom_meli_notifications
		WHERE id = ? AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRow(query, id)
	notification, err := scanMeliNotification(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMeliNotificationNotFound
		}
		return nil, err
	}

	return &notification, nil
}

func (r *MeliNotificationRepository) Create(input CreateMeliNotificationInput) (*MeliNotificationDTO, error) {
	query := `
		INSERT INTO ecom_meli_notifications
			(notification_id, resource, topic, meli_user_id, application_id, delivery_attempts, meli_sent_at, meli_received_at, raw_payload, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending')
	`

	result, err := r.db.Exec(
		query,
		input.NotificationID,
		input.Resource,
		input.Topic,
		input.MeliUserID,
		input.ApplicationID,
		input.DeliveryAttempts,
		input.MeliSentAt,
		input.MeliReceivedAt,
		input.RawPayload,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating meli notification: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting meli notification id: %w", err)
	}

	return r.FindByID(id)
}

func scanMeliNotification(scanner interface{ Scan(dest ...any) error }) (MeliNotificationDTO, error) {
	var n MeliNotificationDTO
	var meliSentAt sql.NullTime
	var meliReceivedAt sql.NullTime
	var connectionID sql.NullInt64
	var lastError sql.NullString
	var processedAt sql.NullTime
	var updatedBy sql.NullInt64
	var deletedAt sql.NullTime

	err := scanner.Scan(
		&n.ID,
		&n.NotificationID,
		&n.Resource,
		&n.Topic,
		&n.MeliUserID,
		&n.ApplicationID,
		&n.DeliveryAttempts,
		&meliSentAt,
		&meliReceivedAt,
		&n.IngestedAt,
		&connectionID,
		&n.RawPayload,
		&n.Status,
		&lastError,
		&processedAt,
		&updatedBy,
		&n.CreatedAt,
		&n.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return MeliNotificationDTO{}, fmt.Errorf("error scanning meli notification: %w", err)
	}

	if meliSentAt.Valid {
		n.MeliSentAt = &meliSentAt.Time
	}
	if meliReceivedAt.Valid {
		n.MeliReceivedAt = &meliReceivedAt.Time
	}
	if connectionID.Valid {
		n.ConnectionID = &connectionID.Int64
	}
	if lastError.Valid {
		n.LastError = &lastError.String
	}
	if processedAt.Valid {
		n.ProcessedAt = &processedAt.Time
	}
	if updatedBy.Valid {
		n.UpdatedBy = &updatedBy.Int64
	}
	if deletedAt.Valid {
		n.DeletedAt = &deletedAt.Time
	}

	return n, nil
}

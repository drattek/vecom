package meli_notifications

import (
	"errors"
	"strings"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var ErrInvalidMeliNotificationPayload = errors.New("invalid meli notification payload")

type ReceiveNotificationInput struct {
	NotificationID string
	Resource       string
	Topic          string
	MeliUserID     int64
	ApplicationID  int64
	Attempts       int
	Sent           *time.Time
	Received       *time.Time
	RawPayload     string
}

type MeliNotificationService struct {
	repository *mysqlInfra.MeliNotificationRepository
}

func NewMeliNotificationService(repository *mysqlInfra.MeliNotificationRepository) *MeliNotificationService {
	return &MeliNotificationService{repository: repository}
}

// Receive persists a MercadoLibre notification exactly once per
// notification_id (MercadoLibre's `_id`). MercadoLibre redelivers a
// notification with the same _id (and a growing `attempts` count) whenever
// it doesn't get a fast 2xx back, so a duplicate delivery is expected and
// silently acknowledged by returning the already-stored row instead of
// erroring or inserting a second copy.
func (s *MeliNotificationService) Receive(input ReceiveNotificationInput) (*mysqlInfra.MeliNotificationDTO, error) {
	input.NotificationID = strings.TrimSpace(input.NotificationID)
	input.Resource = strings.TrimSpace(input.Resource)
	input.Topic = strings.TrimSpace(input.Topic)

	if input.NotificationID == "" || input.Resource == "" || input.Topic == "" || input.MeliUserID <= 0 {
		return nil, ErrInvalidMeliNotificationPayload
	}

	existing, err := s.repository.FindByNotificationID(input.NotificationID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, mysqlInfra.ErrMeliNotificationNotFound) {
		return nil, err
	}

	return s.repository.Create(mysqlInfra.CreateMeliNotificationInput{
		NotificationID:   input.NotificationID,
		Resource:         input.Resource,
		Topic:            input.Topic,
		MeliUserID:       input.MeliUserID,
		ApplicationID:    input.ApplicationID,
		DeliveryAttempts: input.Attempts,
		MeliSentAt:       input.Sent,
		MeliReceivedAt:   input.Received,
		RawPayload:       input.RawPayload,
	})
}

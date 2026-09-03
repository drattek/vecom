package consumers

import (
	"context"
	"encoding/json"
	"log"

	"core-orchestrator/internal/application/sync"
	"core-orchestrator/internal/domain"
)

func NissanExistenciasSyncCompletedConsumer(service *sync.SyncService) func(context.Context, []byte) error {
	return func(ctx context.Context, body []byte) error {
		var event domain.SyncCompletedEvent

		err := json.Unmarshal(body, &event)

		if err != nil {
			return err
		}

		log.Printf("[nissan.existencias.sync.completed] received: %+v", event)

		return service.ProcessNissanExistenciasSyncCompleted(ctx, event)
	}
}

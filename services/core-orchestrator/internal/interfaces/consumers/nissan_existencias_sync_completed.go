package consumers

import (
	"encoding/json"
	"log"

	"core-orchestrator/internal/application/sync"
	"core-orchestrator/internal/domain"
)

func NissanExistenciasSyncCompletedConsumer(service *sync.SyncService) func([]byte) error {
	return func(body []byte) error {
		var event domain.SyncCompletedEvent

		err := json.Unmarshal(body, &event)

		if err != nil {
			return err
		}

		log.Printf("[nissan.existencias.sync.completed] received: %+v", event)

		return service.ProcessNissanExistenciasSyncCompleted(event)
	}
}

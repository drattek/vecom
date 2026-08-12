package consumers

import (
	"encoding/json"
	"log"

	"core-orchestrator/internal/application/sync"
	"core-orchestrator/internal/domain"
)

func ERPCompletedConsumer(service *sync.SyncService) func([]byte) error {
	return func(body []byte) error {
		var event domain.SyncCompletedEvent

		err := json.Unmarshal(body, &event)

		if err != nil {
			return err
		}

		log.Printf("[item.sync.completed] received: %+v", event)

		return service.ProcessERPCompleted(event)
	}
}

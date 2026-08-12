package consumers

import (
	"encoding/json"
	"log"

	"core-orchestrator/internal/application/sync"
	"core-orchestrator/internal/domain"
)

func ERPPageProcessedConsumer(service *sync.SyncService) func([]byte) error {
	return func(body []byte) error {
		var event domain.PageProcessedEvent

		err := json.Unmarshal(body, &event)

		if err != nil {
			return err
		}

		log.Printf("[item.page.processed] received: %+v", event)

		return service.ProcessERPPageProcessed(event)
	}
}

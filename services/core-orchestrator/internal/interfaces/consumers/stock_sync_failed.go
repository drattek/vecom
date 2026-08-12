package consumers

import (
	"encoding/json"
	"log"

	"core-orchestrator/internal/application/sync"
	"core-orchestrator/internal/domain"
)

func StockSyncFailedConsumer(service *sync.SyncService) func([]byte) error {
	return func(body []byte) error {
		var event domain.SyncFailedEvent

		err := json.Unmarshal(body, &event)

		if err != nil {
			return err
		}

		log.Printf("[stock.sync.failed] received: %+v", event)

		return service.ProcessStockSyncFailed(event)
	}
}

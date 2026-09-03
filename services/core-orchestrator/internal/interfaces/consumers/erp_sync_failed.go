package consumers

import (
	"context"
	"encoding/json"
	"log"

	"core-orchestrator/internal/application/sync"
	"core-orchestrator/internal/domain"
)

func ERPSyncFailedConsumer(service *sync.SyncService) func(context.Context, []byte) error {
	return func(ctx context.Context, body []byte) error {
		var event domain.SyncFailedEvent

		err := json.Unmarshal(body, &event)

		if err != nil {
			return err
		}

		log.Printf("[item.sync.failed] received: %+v", event)

		return service.ProcessERPSyncFailed(ctx, event)
	}
}

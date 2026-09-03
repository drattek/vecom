package consumers

import (
	"context"
	"encoding/json"
	"log"

	"core-orchestrator/internal/application/sync"
	"core-orchestrator/internal/domain"
)

func StockPageProcessedConsumer(service *sync.SyncService) func(context.Context, []byte) error {
	return func(ctx context.Context, body []byte) error {
		var event domain.PageProcessedEvent

		err := json.Unmarshal(body, &event)

		if err != nil {
			return err
		}

		log.Printf("[stock.page.processed] received: %+v", event)

		return service.ProcessStockPageProcessed(ctx, event)
	}
}

package main

import (
	"errors"
	"fmt"
	"log"

	syncApp "core-orchestrator/internal/application/sync"
	rmq "core-orchestrator/internal/infrastructure/rabbitmq"
	consumer "core-orchestrator/internal/interfaces/consumers"
)

// Toda cola arrancada aquí debe coincidir 1:1 con los routing keys que
// synapse-bridge declara en RabbitConfig.java y publica en EventPublisher.java.
// Ver infrastructure/docs/events.md para la tabla de paridad publisher/consumer.
func startConsumers(syncService *syncApp.SyncService) error {
	log.Println("Starting RabbitMQ consumers...")

	queues := []struct {
		name    string
		handler func([]byte) error
	}{
		{"item.page.processed", consumer.ERPPageProcessedConsumer(syncService)},
		{"item.sync.completed", consumer.ERPCompletedConsumer(syncService)},
		{"item.sync.failed", consumer.ERPSyncFailedConsumer(syncService)},
		{"stock.page.processed", consumer.StockPageProcessedConsumer(syncService)},
		{"stock.sync.completed", consumer.StockSyncCompletedConsumer(syncService)},
		{"stock.sync.failed", consumer.StockSyncFailedConsumer(syncService)},
		{"nissan.existencias.page.processed", consumer.NissanExistenciasPageProcessedConsumer(syncService)},
		{"nissan.existencias.sync.completed", consumer.NissanExistenciasSyncCompletedConsumer(syncService)},
		{"nissan.existencias.sync.failed", consumer.NissanExistenciasSyncFailedConsumer(syncService)},
	}

	// Cada cola se intenta de forma independiente: que una falle (p.ej. porque
	// synapse-bridge todavía no la declaró) no debe impedir que las demás arranquen.
	var errs []error

	for _, q := range queues {
		if err := rmq.StartConsumer(q.name, q.handler); err != nil {
			log.Printf("✗ %s consumer failed to start: %v", q.name, err)
			errs = append(errs, fmt.Errorf("%s: %w", q.name, err))
			continue
		}
		log.Printf("✓ %s consumer started", q.name)
	}

	return errors.Join(errs...)
}

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	syncApp "core-orchestrator/internal/application/sync"
	"core-orchestrator/internal/config"
	rmq "core-orchestrator/internal/infrastructure/rabbitmq"
	consumer "core-orchestrator/internal/interfaces/consumers"
	"core-orchestrator/internal/shared/safe"
)

// Toda cola arrancada aquí debe coincidir 1:1 con los routing keys que
// synapse-bridge declara en RabbitConfig.java y publica en EventPublisher.java.
// Ver infrastructure/docs/events.md para la tabla de paridad publisher/consumer.
//
// ctx gobierna el ciclo de vida de todos los consumers: al cancelarse (apagado
// ordenado) cada supervisor cancela su suscripción, deja terminar el mensaje en
// curso y sale. startConsumers bloquea hasta que el último supervisor terminó,
// así el llamador puede tratarlo como una unidad y esperarlo en el apagado.
func startConsumers(ctx context.Context, cfg config.Config, syncService *syncApp.SyncService) {
	log.Println("Starting RabbitMQ consumers...")

	queues := []struct {
		name    string
		handler func(context.Context, []byte) error
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

	// Cada cola se arranca de forma independiente y en paralelo: que una tarde
	// en conectar (p.ej. porque synapse-bridge todavía no la declaró) no debe
	// demorar el arranque de las demás. StartConsumer se auto-repara: reconecta
	// con backoff hasta que ctx se cancele, así que un error acá solo significa
	// "todavía no conectó dentro del plazo de arranque", nunca "consumer muerto"
	// — sigue reintentando en segundo plano sin reiniciar el proceso.
	var (
		supervisors sync.WaitGroup // vivos hasta el apagado
		startupWG   sync.WaitGroup // solo el intento de arranque
		mu          sync.Mutex
		errs        []error
	)

	for _, q := range queues {
		startupWG.Add(1)
		go func(name string, handler func(context.Context, []byte) error) {
			defer startupWG.Done()

			safe.Do("consumer startup "+name, func() {
				if err := rmq.StartConsumer(ctx, cfg, &supervisors, name, handler); err != nil {
					log.Printf("⏳ %s: %v", name, err)
					mu.Lock()
					errs = append(errs, fmt.Errorf("%s: %w", name, err))
					mu.Unlock()
					return
				}
				log.Printf("✓ %s consumer started", name)
			})
		}(q.name, q.handler)
	}

	startupWG.Wait()

	if joined := errors.Join(errs...); joined != nil {
		// No es fatal: cada consumer se auto-repara y sigue reintentando en
		// segundo plano hasta conectar (ver rabbitmq.StartConsumer).
		log.Printf("Consumers aún sin conectar al arrancar (reintentando en segundo plano): %v", joined)
	}

	// Bloquea hasta que el apagado (ctx) haga salir a todos los supervisores.
	supervisors.Wait()
}

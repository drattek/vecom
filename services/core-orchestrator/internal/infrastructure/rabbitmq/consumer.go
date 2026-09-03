package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"core-orchestrator/internal/config"
	"core-orchestrator/internal/shared/safe"
)

const (
	// Backoff entre reintentos de reconexión. Ante cualquier caída (RabbitMQ
	// reiniciado, corte de red, canal roto por un error de protocolo, o
	// synapse-bridge que todavía no declaró la cola) el consumer reconecta solo,
	// indefinidamente: el proceso Go corre 24/7 y nunca debe hacer falta
	// reiniciarlo para recuperar un consumer.
	consumerRetryMinDelay = 2 * time.Second
	consumerRetryMaxDelay = 30 * time.Second

	// StartConsumer espera como máximo esto a que la primera suscripción quede
	// activa antes de devolver el control, para que el log de arranque sea
	// significativo. Agotado el plazo, el supervisor sigue reintentando en
	// segundo plano igual (el consumer no está muerto).
	consumerStartupTimeout = 30 * time.Second
)

// StartConsumer arranca un consumer auto-reparable para queue: declara la cola
// (idempotente), se suscribe y procesa mensajes; ante cualquier cierre de canal
// o de conexión reconecta con backoff, hasta que ctx se cancele.
//
// wg se incrementa antes de lanzar el supervisor y se libera cuando este sale
// (tras un ctx.Done()): así main puede esperar en el apagado a que el consumer
// termine el mensaje en curso y cancele la suscripción.
//
// Devuelve nil cuando la primera suscripción quedó activa. Devuelve error si eso
// no ocurrió dentro de consumerStartupTimeout: en ese caso el consumer NO está
// muerto — el supervisor sigue intentando en segundo plano y se enganchará en
// cuanto RabbitMQ y la cola estén disponibles, sin reiniciar el proceso.
func StartConsumer(ctx context.Context, cfg config.Config, wg *sync.WaitGroup, queue string, handler func(context.Context, []byte) error) error {
	var readyOnce sync.Once
	ready := make(chan struct{})
	signalReady := func() { readyOnce.Do(func() { close(ready) }) }

	wg.Add(1)
	go func() {
		defer wg.Done()
		superviseConsumer(ctx, cfg, queue, handler, signalReady)
	}()

	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("consumer %s: apagado solicitado antes de conectar", queue)
	case <-time.After(consumerStartupTimeout):
		return fmt.Errorf("consumer %s: sin conexión tras %s (reintentando en segundo plano)", queue, consumerStartupTimeout)
	}
}

// superviseConsumer mantiene una suscripción viva hasta que ctx se cancele: cada
// vez que runConsumer retorna (setup fallido, o conexión/canal caídos) espera un
// backoff y vuelve a intentar. El backoff crece de forma exponencial hasta
// consumerRetryMaxDelay mientras no se logre conectar, y se reinicia en cuanto
// una conexión llega a suscribirse. Al cancelarse ctx retorna sin reintentar.
func superviseConsumer(ctx context.Context, cfg config.Config, queue string, handler func(context.Context, []byte) error, signalReady func()) {
	backoff := consumerRetryMinDelay

	for {
		if ctx.Err() != nil {
			return
		}

		// safe.Do contiene un panic que escape de runConsumer (p. ej. dentro de
		// la lib amqp): se loguea con stack y el supervisor lo trata como una
		// caída no conectada, reconectando con backoff en vez de tumbar el
		// proceso. Un panic del handler de mensaje se maneja aparte, adentro de
		// runConsumer, para poder hacer nack sin requeue.
		var connected bool
		var err error
		if ok := safe.Do("rabbitmq consumer "+queue, func() {
			connected, err = runConsumer(ctx, cfg, queue, handler, signalReady)
		}); !ok {
			connected = false
			err = errors.New("el consumer terminó por un panic (ver stack arriba)")
		}

		if ctx.Err() != nil {
			log.Printf("Consumer %s: detenido por apagado (%v)", queue, err)
			return
		}

		if connected {
			backoff = consumerRetryMinDelay
		}

		log.Printf("Consumer %s: %v; reintentando en %s", queue, err, backoff)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		if !connected {
			backoff = min(backoff*2, consumerRetryMaxDelay)
		}
	}
}

// runConsumer abre conexión y canal, declara la cola, se suscribe y procesa
// entregas hasta que el canal o la conexión se cierren, o hasta que ctx se
// cancele. Bloquea hasta entonces y siempre devuelve un error no nil
// describiendo por qué terminó. El bool indica si se llegó a suscribir (para
// que el supervisor reinicie el backoff).
func runConsumer(ctx context.Context, cfg config.Config, queue string, handler func(context.Context, []byte) error, signalReady func()) (connected bool, err error) {
	conn, err := newConnection(cfg)
	if err != nil {
		return false, fmt.Errorf("abriendo conexión: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return false, fmt.Errorf("abriendo canal: %w", err)
	}
	defer ch.Close()

	// Declaración idempotente de la cola: si synapse-bridge todavía no la creó,
	// la creamos con los mismos atributos (durable, no auto-delete, no
	// exclusive) para que el orden de arranque entre servicios deje de importar.
	// Si ya existe con esos atributos, es no-op.
	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return false, fmt.Errorf("declarando cola: %w", err)
	}

	// Un mensaje sin ack por vez: RabbitMQ no adelanta el siguiente hasta que el
	// handler termina con el actual.
	if err := ch.Qos(1, 0, false); err != nil {
		return false, fmt.Errorf("configurando QoS: %w", err)
	}

	// Consumer tag explícito para poder cancelar la suscripción de forma
	// ordenada en el apagado (ch.Cancel) antes de cerrar el canal.
	consumerTag := "core-orchestrator-" + queue
	msgs, err := ch.Consume(queue, consumerTag, false, false, false, false, nil)
	if err != nil {
		return false, fmt.Errorf("suscribiéndose: %w", err)
	}

	log.Printf("✓ %s consumer conectado", queue)
	signalReady()

	// Avisos de cierre de conexión y de canal (buffer 1: el emisor nunca se
	// bloquea aunque nadie lea de inmediato). Al cerrarse, msgs también se
	// cierra; estos canales solo aportan el motivo para el log.
	connClosed := make(chan *amqp.Error, 1)
	chClosed := make(chan *amqp.Error, 1)
	conn.NotifyClose(connClosed)
	ch.NotifyClose(chClosed)

	for {
		// Chequeo explícito antes de cada iteración: si ctx se canceló mientras
		// el handler anterior corría, no arrancamos con un mensaje nuevo.
		if ctx.Err() != nil {
			_ = ch.Cancel(consumerTag, false)
			return true, fmt.Errorf("apagado solicitado")
		}

		select {
		case <-ctx.Done():
			// Apagado ordenado: cancelar la suscripción para que RabbitMQ deje
			// de entregar. Los mensajes ya entregados sin ack se reencolan solos
			// al cerrarse el canal.
			_ = ch.Cancel(consumerTag, false)
			return true, fmt.Errorf("apagado solicitado")
		case reason := <-connClosed:
			return true, fmt.Errorf("conexión cerrada: %s", closeReason(reason))
		case reason := <-chClosed:
			return true, fmt.Errorf("canal cerrado: %s", closeReason(reason))
		case msg, ok := <-msgs:
			if !ok {
				return true, fmt.Errorf("canal de entregas cerrado")
			}

			handleDelivery(ctx, queue, handler, msg)
		}
	}
}

// handleDelivery ejecuta el handler bajo barrera de panic y decide el ack:
//   - éxito              → Ack.
//   - error normal       → Nack(requeue=true): puede ser transitorio (DB caída,
//     timeout), se reintenta en la próxima entrega.
//   - panic del handler  → Nack(requeue=false): un panic es casi siempre un bug
//     de código o un mensaje envenenado; reencolarlo lo haría reventar en loop
//     y bloquear la cola. Sin DLQ configurada el cuerpo queda solo en el log
//     (con stack), así que el panic es visible y accionable.
func handleDelivery(ctx context.Context, queue string, handler func(context.Context, []byte) error, msg amqp.Delivery) {
	err := safe.Guard(func() error { return handler(ctx, msg.Body) })

	var panicErr *safe.PanicError
	switch {
	case errors.As(err, &panicErr):
		log.Printf("Consumer %s: PANIC en el handler, mensaje descartado sin requeue: %v", queue, err)
		_ = msg.Nack(false, false)
	case err != nil:
		log.Printf("Consumer %s: handler falló, reencolando: %v", queue, err)
		_ = msg.Nack(false, true)
	default:
		_ = msg.Ack(false)
	}
}

func closeReason(err *amqp.Error) string {
	if err == nil {
		return "cierre limpio"
	}
	return err.Error()
}

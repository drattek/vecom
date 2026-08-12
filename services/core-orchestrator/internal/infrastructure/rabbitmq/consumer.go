package rabbitmq

import (
	"fmt"
	"log"
	"time"
)

const (
	startConsumerMaxAttempts = 15
	startConsumerRetryDelay  = 2 * time.Second
)

// RabbitMQ (y las colas que synapse-bridge declara al arrancar) puede tardar más en
// quedar listo que este binario en llegar a este punto, por eso se reintenta con
// backoff en vez de fallar en el primer intento.
func StartConsumer(queue string, handler func([]byte) error) error {
	var lastErr error

	for attempt := 1; attempt <= startConsumerMaxAttempts; attempt++ {
		if err := tryStartConsumer(queue, handler); err != nil {
			lastErr = err
			log.Printf("Consumer %s: attempt %d/%d failed: %v", queue, attempt, startConsumerMaxAttempts, err)
			time.Sleep(startConsumerRetryDelay)
			continue
		}

		return nil
	}

	return fmt.Errorf("consumer %s: giving up after %d attempts: %w", queue, startConsumerMaxAttempts, lastErr)
}

func tryStartConsumer(queue string, handler func([]byte) error) error {
	conn, err := newConnection()

	if err != nil {
		return err
	}

	ch, err := conn.Channel()

	if err != nil {
		conn.Close()
		return err
	}

	msgs, err := ch.Consume(queue, "", false, false, false, false, nil)

	if err != nil {
		ch.Close()
		conn.Close()
		return err
	}

	go func() {
		for msg := range msgs {
			err := handler(msg.Body)

			if err != nil {
				log.Printf("Consumer %s: handler failed, requeueing: %v", queue, err)
				msg.Nack(false, true)
				continue
			}

			msg.Ack(false)
		}
	}()

	return nil
}

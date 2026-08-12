package rabbitmq

import (
	"fmt"
	"net"
	"time"

	"core-orchestrator/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

func newConnection() (*amqp.Connection, error) {
	cfg := config.Load()

	url := fmt.Sprintf("amqp://%s:%s@%s:%s/",
		cfg.RabbitMQUser,
		cfg.RabbitMQPassword,
		cfg.RabbitMQHost,
		cfg.RabbitMQPort,
	)

	return amqp.DialConfig(url, amqp.Config{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 10*time.Second)
		},
	})
}

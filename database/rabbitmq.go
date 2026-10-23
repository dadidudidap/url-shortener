package database

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ClickQueueName    = "clicks_queue"
	rabbitRetryDelay  = 5 * time.Second
	rabbitDialTimeout = 5 * time.Second
)

var (
	// Rabbit is nil until main wires it up; handlers treat a missing publisher as analytics disabled.
	Rabbit                *RabbitPublisher
	ErrRabbitNotConnected = errors.New("rabbitmq publisher is not connected")
)

// RabbitPublisher owns one connection and one publish channel. publishSlot serializes
// publishes because an AMQP channel is not safe for concurrent use; stateMu protects the
// connection and channel themselves.
type RabbitPublisher struct {
	url   string
	queue string

	stateMu sync.Mutex
	conn    *amqp.Connection
	ch      *amqp.Channel

	publishSlot chan struct{}
}

func NewRabbitPublisher(url string) *RabbitPublisher {
	return &RabbitPublisher{
		url:         url,
		queue:       ClickQueueName,
		publishSlot: make(chan struct{}, 1),
	}
}

func RabbitURL() string {
	if url := os.Getenv("RABBITMQ_URL"); url != "" {
		return url
	}
	return "amqp://guest:guest@localhost:5672/"
}

// Run keeps the publish connection alive. Redirects never wait for this loop; a failed
// publish is logged and the next click tries again after the connection is rebuilt.
func (p *RabbitPublisher) Run(ctx context.Context) {
	for {
		if !p.connected() {
			if err := p.Connect(); err != nil {
				log.Printf("RabbitMQ publisher unavailable: %v", err)
			}
		}

		select {
		case <-ctx.Done():
			p.Close()
			return
		case <-time.After(rabbitRetryDelay):
		}
	}
}

func (p *RabbitPublisher) Connect() error {
	conn, ch, err := openRabbitChannel(p.url, p.queue, 0)
	if err != nil {
		return err
	}

	p.stateMu.Lock()
	p.closeLocked()
	p.conn, p.ch = conn, ch
	p.stateMu.Unlock()

	log.Println("RabbitMQ publisher connected")
	return nil
}

func (p *RabbitPublisher) Publish(ctx context.Context, shortCode string) error {
	select {
	case p.publishSlot <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Run the socket write in its own goroutine so a wedged connection cannot hold the
	// redirect past its context deadline. At most one publish is in flight at a time.
	result := make(chan error, 1)
	go func() {
		defer func() { <-p.publishSlot }()
		result <- p.publish(shortCode)
	}()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *RabbitPublisher) publish(shortCode string) error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	if p.ch == nil {
		return ErrRabbitNotConnected
	}

	err := p.ch.PublishWithContext(context.Background(), "", p.queue, false, false, amqp.Publishing{
		ContentType:  "text/plain",
		DeliveryMode: amqp.Persistent,
		Body:         []byte(shortCode),
	})
	if err != nil {
		p.closeLocked()
		return err
	}

	return nil
}

func (p *RabbitPublisher) Close() {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	p.closeLocked()
}

func (p *RabbitPublisher) connected() bool {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.ch != nil && !p.ch.IsClosed() && p.conn != nil && !p.conn.IsClosed()
}

func (p *RabbitPublisher) closeLocked() {
	if p.ch != nil {
		_ = p.ch.Close()
		p.ch = nil
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
}

// OpenClickConsumer gives the worker its own connection so a publisher reconnect cannot
// cancel in-flight deliveries. Closing the connection closes both channels.
func OpenClickConsumer(url string) (*amqp.Connection, <-chan amqp.Delivery, error) {
	return openClickConsumer(url, ClickQueueName)
}

func openClickConsumer(url, queue string) (*amqp.Connection, <-chan amqp.Delivery, error) {
	conn, ch, err := openRabbitChannel(url, queue, 1)
	if err != nil {
		return nil, nil, err
	}

	msgs, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	return conn, msgs, nil
}

func openRabbitChannel(url, queue string, prefetch int) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.DialConfig(url, amqp.Config{
		Heartbeat: 10 * time.Second,
		Dial:      amqp.DefaultDial(rabbitDialTimeout),
	})
	if err != nil {
		return nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	if prefetch > 0 {
		if err := ch.Qos(prefetch, 0, false); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, nil, err
		}
	}

	// The publisher and consumer both declare the same durable queue; declaration is idempotent.
	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, err
	}

	return conn, ch, nil
}

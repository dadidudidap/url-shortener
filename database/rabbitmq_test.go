package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestRabbitPublishConsume(t *testing.T) {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		t.Skip("set RABBITMQ_URL to run the RabbitMQ integration test")
	}

	queue := fmt.Sprintf("clicks_test_%d", time.Now().UnixNano())
	conn, msgs, err := openClickConsumer(url, queue)
	if err != nil {
		t.Fatalf("open consumer: %v", err)
	}

	defer func() {
		_ = conn.Close()

		cleanup, err := amqp.Dial(url)
		if err != nil {
			return
		}
		defer cleanup.Close()

		ch, err := cleanup.Channel()
		if err != nil {
			return
		}
		_, _ = ch.QueueDelete(queue, false, false, false)
		_ = ch.Close()
	}()

	pub := NewRabbitPublisher(url)
	pub.queue = queue
	if err := pub.Connect(); err != nil {
		t.Fatalf("connect publisher: %v", err)
	}
	defer pub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const code = "rabbit-integration-test"
	if err := pub.Publish(ctx, code); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case d, ok := <-msgs:
		if !ok {
			t.Fatal("delivery channel closed")
		}
		if got := string(d.Body); got != code {
			t.Fatalf("got %q, want %q", got, code)
		}
		if err := d.Ack(false); err != nil {
			t.Fatalf("ack: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for message")
	}
}

package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/dadidudidap/url-shortener/database"
	"github.com/dadidudidap/url-shortener/database/db"
)

const (
	clickPublishTimeout = 50 * time.Millisecond
	clickWriteTimeout   = 500 * time.Millisecond
	clickRetryDelay     = 500 * time.Millisecond
	rabbitReconnectWait = 5 * time.Second
)

// StartClickWorker keeps one consumer alive and reconnects after a broker restart.
func StartClickWorker(ctx context.Context, rabbitURL string) {
	queries := db.New(database.Pool)

	for {
		if err := consumeClicks(ctx, rabbitURL, queries); err != nil && ctx.Err() == nil {
			log.Printf("Click worker stopped: %v; reconnecting in %s", err, rabbitReconnectWait)
		}

		if ctx.Err() != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(rabbitReconnectWait):
		}
	}
}

func consumeClicks(ctx context.Context, rabbitURL string, queries *db.Queries) error {
	conn, msgs, err := database.OpenClickConsumer(rabbitURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	log.Println("Click worker connected to RabbitMQ")

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-msgs:
			if !ok {
				return fmt.Errorf("delivery channel closed")
			}
			if err := handleClick(ctx, queries, d); err != nil {
				return err
			}
		}
	}
}

func handleClick(ctx context.Context, queries *db.Queries, d amqp.Delivery) error {
	shortCode := string(d.Body)

	dbCtx, cancel := context.WithTimeout(ctx, clickWriteTimeout)
	err := queries.IncrementClicks(dbCtx, shortCode)
	cancel()

	if err != nil {
		log.Printf("Click worker failed to increment %s: %v", shortCode, err)

		// Keep the message unacked while backing off; an immediate Nack would hot-loop
		// against a database that is still down.
		select {
		case <-ctx.Done():
			_ = d.Nack(false, true)
			return nil
		case <-time.After(clickRetryDelay):
		}

		if err := d.Nack(false, true); err != nil {
			return fmt.Errorf("nack click %q: %w", shortCode, err)
		}
		return nil
	}

	if err := d.Ack(false); err != nil {
		return fmt.Errorf("ack click %q: %w", shortCode, err)
	}
	return nil
}

// publishClick queues a click and logs failures so analytics cannot break redirects.
func publishClick(shortCode string) {
	if database.Rabbit == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), clickPublishTimeout)
	defer cancel()

	if err := database.Rabbit.Publish(ctx, shortCode); err != nil {
		log.Printf("Warning: failed to publish click for %s: %v", shortCode, err)
	}
}

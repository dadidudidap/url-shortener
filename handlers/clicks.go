package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
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
	defer cancel()

	clicks, err := queries.IncrementClicks(dbCtx, shortCode)

	if errors.Is(err, pgx.ErrNoRows) {
		// The link was deleted after the redirect; retrying cannot succeed.
		log.Printf("Dropping click for removed code %s", shortCode)
		return d.Ack(false)
	}

	if err != nil {
		log.Printf("Click worker failed to increment %s: %v", shortCode, err)

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

	msg := fmt.Sprintf("%s:%d", shortCode, clicks)
	if err := database.RedisClient.Publish(context.Background(), "ws_broadcasts", msg).Err(); err != nil {
		log.Printf("Warning: failed to publish live stats for %s: %v", shortCode, err)
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

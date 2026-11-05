package handlers

import (
	"context"
	"log"
	"time"

	"github.com/dadidudidap/url-shortener/shared/database"
)

const clickPublishTimeout = 50 * time.Millisecond

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

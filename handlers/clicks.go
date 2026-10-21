package handlers

import (
	"context"
	"log"
	"time"

	"github.com/dadidudidap/url-shortener/database"
	"github.com/dadidudidap/url-shortener/database/db" // sqlc generated package
)

// clickChan holds pending clicks; the buffer keeps redirects from blocking on analytics.
var clickChan = make(chan string, 1000)

// StartClickWorker processes queued clicks one by one until the channel is closed.
func StartClickWorker() {
	queries := db.New(database.Pool)

	for shortCode := range clickChan {
		// Use context.Background() because this is no longer tied to the user's HTTP request
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)

		err := queries.IncrementClicks(ctx, shortCode)
		if err != nil {
			log.Printf("Background worker failed to increment click for %s: %v\n", shortCode, err)
		}

		cancel() // Prevent memory leaks
	}
}

func trackClick(shortCode string) {
	select {
	case clickChan <- shortCode:
		// Successfully handed off to the background worker
	default:
		log.Printf("Warning: Analytics channel full, dropping click for %s\n", shortCode)
	}
}

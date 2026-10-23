package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/dadidudidap/url-shortener/database"
	"github.com/dadidudidap/url-shortener/database/db" // sqlc generated package
)

func RedirectHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "code")
	queries := db.New(database.Pool)

	redisCtx, cancelRedis := context.WithTimeout(r.Context(), 50*time.Millisecond)
	defer cancelRedis() // Clean up the timer to prevent memory leaks
	dbCtx, cancelDB := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancelDB()

	// sqlc automatically returns exactly one string for GetURL
	// If the database is too slow, this aborts and returns a context.DeadlineExceeded error
	longURL, redisErr := database.RedisClient.Get(redisCtx, shortCode).Result() //Try to fetch the long URL from Redis first

	if redisErr != nil {
		// Fetch long URL from Postgres if long URL not in Redis or some error occured
		var dbErr error
		longURL, dbErr = queries.GetURL(dbCtx, shortCode)
		if dbErr != nil {
			http.Error(w, "URL not found", http.StatusNotFound)
			return
		}

		// Check the error is cache miss or otherwise
		if redisErr == redis.Nil {
			// Cache miss
			// Save longURL to Redis for future requests (e.g., cache for 24 hours)
			setCtx, cancelSet := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancelSet()

			cacheErr := database.RedisClient.Set(setCtx, shortCode, longURL, 24*time.Hour).Err()
			if cacheErr != nil {
				log.Printf("Warning: Failed to cache URL %s: %v\n", shortCode, cacheErr)
			}
		} else {
			// Error, Redis is down or acting up
			log.Printf("Redis error: %v\n", redisErr)
		}
	}

	// Queue the click; publish failures are logged and never block the redirect.
	publishClick(shortCode)

	http.Redirect(w, r, longURL, http.StatusFound)
}

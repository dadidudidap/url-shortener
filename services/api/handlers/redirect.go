package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"github.com/dadidudidap/url-shortener/shared/database"
	"github.com/dadidudidap/url-shortener/shared/database/db" // sqlc generated package
)

func RedirectHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "code")
	queries := db.New(database.Pool)

	redisCtx, cancelRedis := context.WithTimeout(r.Context(), 50*time.Millisecond)
	defer cancelRedis()
	dbCtx, cancelDB := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancelDB()

	if target, ok := cachedRedirectTarget(redisCtx, shortCode); ok {
		decision := decideRedirect(target, time.Now())
		if decision.Active {
			publishClick(shortCode)
			http.Redirect(w, r, decision.LongURL, http.StatusFound)
			return
		}
		if decision.DeleteGuest {
			if _, err := queries.DeleteExpiredGuestURL(dbCtx, shortCode); err != nil {
				log.Printf("Failed to delete expired guest link %s: %v\n", shortCode, err)
			}
		}
		deleteCachedRedirect(shortCode)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	row, err := queries.GetURLForRedirect(dbCtx, shortCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		http.Error(w, "Could not look up URL", http.StatusInternalServerError)
		return
	}

	target := cachedRedirectFromRow(row)
	decision := decideRedirect(target, time.Now())
	if !decision.Active {
		if decision.DeleteGuest {
			if _, err := queries.DeleteExpiredGuestURL(dbCtx, shortCode); err != nil {
				log.Printf("Failed to delete expired guest link %s: %v\n", shortCode, err)
			}
		}
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	cacheRedirect(shortCode, target)
	publishClick(shortCode)
	http.Redirect(w, r, decision.LongURL, http.StatusFound)
}

func cachedRedirectTarget(ctx context.Context, shortCode string) (cachedRedirect, bool) {
	raw, err := database.RedisClient.Get(ctx, shortCode).Result()
	if err != nil {
		if err != redis.Nil {
			log.Printf("Redis error: %v\n", err)
		}
		return cachedRedirect{}, false
	}

	target, err := decodeRedirect([]byte(raw))
	if err != nil {
		deleteCachedRedirect(shortCode)
		return cachedRedirect{}, false
	}
	return target, true
}

func cacheRedirect(shortCode string, target cachedRedirect) {
	ttl := cacheTTL(target.ExpiresAt, time.Now())
	if ttl <= 0 {
		return
	}

	encoded, err := encodeRedirect(target)
	if err != nil {
		log.Printf("Failed to encode cached URL %s: %v\n", shortCode, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := database.RedisClient.Set(ctx, shortCode, encoded, ttl).Err(); err != nil {
		log.Printf("Failed to cache URL %s: %v\n", shortCode, err)
	}
}

func deleteCachedRedirect(shortCode string) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := database.RedisClient.Del(ctx, shortCode).Err(); err != nil {
		log.Printf("Failed to delete cached URL %s: %v\n", shortCode, err)
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/dadidudidap/url-shortener/database"
	"github.com/dadidudidap/url-shortener/database/db" // sqlc generated package
)

func GetStatsHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "code")
	queries := db.New(database.Pool)

	// Give Postgres 500ms to fetch the stats
	dbCtx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	// sqlc automatically maps the result to a db.Url struct
	stats, err := queries.GetURLStats(dbCtx, shortCode)
	if err != nil {
		http.Error(w, "URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"short_code": stats.ShortCode,
		"long_url":   stats.LongUrl,
		"clicks":     stats.Clicks,
		"created_at": stats.CreatedAt.Time, // Extract the standard Go time.Time
	})
}

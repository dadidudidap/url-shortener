package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/dadidudidap/url-shortener/services/api/middleware"
	"github.com/dadidudidap/url-shortener/shared/database"
	"github.com/dadidudidap/url-shortener/shared/database/db" // sqlc generated package
)

func GetStatsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	shortCode := chi.URLParam(r, "code")
	queries := db.New(database.Pool)

	dbCtx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	stats, err := queries.GetURLForOwner(dbCtx, db.GetURLForOwnerParams{
		ShortCode: shortCode,
		UserID:    pgtype.Int4{Int32: userID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "URL not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Could not fetch stats", http.StatusInternalServerError)
		return
	}

	var expiresAt any
	if stats.ExpiresAt.Valid {
		expiresAt = stats.ExpiresAt.Time
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"short_code": stats.ShortCode,
		"long_url":   stats.LongUrl,
		"clicks":     stats.Clicks,
		"created_at": stats.CreatedAt.Time,
		"expires_at": expiresAt,
		"expired":    isExpired(stats.ExpiresAt, time.Now()),
	})
}

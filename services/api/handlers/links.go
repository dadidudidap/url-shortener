package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/dadidudidap/url-shortener/services/api/middleware"
	"github.com/dadidudidap/url-shortener/shared/database"
	"github.com/dadidudidap/url-shortener/shared/database/db"
)

type linkPayload struct {
	ShortCode string     `json:"short_code"`
	LongURL   string     `json:"long_url"`
	Clicks    int32      `json:"clicks"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	Expired   bool       `json:"expired"`
}

// ListLinksHandler returns links owned by the caller. The owner always comes
// from the token, never from the request body or query string.
func ListLinksHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	rows, err := db.New(database.Pool).GetURLsByUser(ctx, pgtype.Int4{Int32: userID, Valid: true})
	if err != nil {
		log.Printf("Failed to list links for user %d: %v", userID, err)
		http.Error(w, "Failed to list links", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	links := make([]linkPayload, 0, len(rows))
	for _, row := range rows {
		expiresAt, expired := expiryPayload(row.ExpiresAt, now)
		links = append(links, linkPayload{
			ShortCode: row.ShortCode,
			LongURL:   row.LongUrl,
			Clicks:    row.Clicks,
			CreatedAt: row.CreatedAt.Time,
			ExpiresAt: expiresAt,
			Expired:   expired,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"links": links})
}

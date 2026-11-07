package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/dadidudidap/url-shortener/services/api/middleware"
	"github.com/dadidudidap/url-shortener/shared/database"
	"github.com/dadidudidap/url-shortener/shared/database/db"
)

var errInvalidShortCode = errors.New("short code must be 3-32 characters and use letters, numbers, _ or -")

type updateLinkRequest struct {
	ShortCode string `json:"short_code"`
	ExpiresAt string `json:"expires_at"`
	Lifetime  bool   `json:"lifetime"`
}

func (req updateLinkRequest) validate(now time.Time) (pgtype.Timestamptz, error) {
	if !validCustomCode(req.ShortCode) {
		return pgtype.Timestamptz{}, errInvalidShortCode
	}
	return expiresAtFromEdit(req.ExpiresAt, req.Lifetime, now)
}

func UpdateLinkHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	oldCode := chi.URLParam(r, "code")
	var req updateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	req.ShortCode = strings.TrimSpace(req.ShortCode)

	now := time.Now()
	expiresAt, err := req.validate(now)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	updated, err := db.New(database.Pool).UpdateURL(ctx, db.UpdateURLParams{
		ShortCode:   req.ShortCode,
		ExpiresAt:   expiresAt,
		ShortCode_2: oldCode,
		UserID:      pgtype.Int4{Int32: userID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "URL not found", http.StatusNotFound)
			return
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, "Short code already taken", http.StatusConflict)
			return
		}
		http.Error(w, "Could not update URL", http.StatusInternalServerError)
		return
	}

	deleteCachedRedirect(oldCode)
	deleteCachedRedirect(req.ShortCode)
	writeJSON(w, http.StatusOK, linkPayloadFromURL(updated, now))
}

func DeleteLinkHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	shortCode := chi.URLParam(r, "code")
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	_, err := db.New(database.Pool).DeleteURL(ctx, db.DeleteURLParams{
		ShortCode: shortCode,
		UserID:    pgtype.Int4{Int32: userID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "URL not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Could not delete URL", http.StatusInternalServerError)
		return
	}

	deleteCachedRedirect(shortCode)
	w.WriteHeader(http.StatusNoContent)
}

func linkPayloadFromURL(row db.Url, now time.Time) linkPayload {
	expiresAt, expired := expiryPayload(row.ExpiresAt, now)
	return linkPayload{
		ShortCode: row.ShortCode,
		LongURL:   row.LongUrl,
		Clicks:    row.Clicks,
		CreatedAt: row.CreatedAt.Time,
		ExpiresAt: expiresAt,
		Expired:   expired,
	}
}

func expiryPayload(expiresAt pgtype.Timestamptz, now time.Time) (*time.Time, bool) {
	if !expiresAt.Valid {
		return nil, false
	}
	value := expiresAt.Time
	return &value, isExpired(expiresAt, now)
}

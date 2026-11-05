package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/dadidudidap/url-shortener/services/api/middleware"
	"github.com/dadidudidap/url-shortener/shared/database"
	"github.com/dadidudidap/url-shortener/shared/database/db" // sqlc generated package
)

const (
	charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// Short codes are 6 chars of 62, so a collision retry almost never fires.
	maxCodeAttempts = 5
)

var errCodeExhausted = errors.New("short code space exhausted for this attempt")

// createUniqueCode inserts until a generated code is free. insert reports rows
// affected: 0 means another request took that code.
func createUniqueCode(insert func(code string) (int64, error)) (string, error) {
	for attempt := 0; attempt < maxCodeAttempts; attempt++ {
		code := generateShortCode()

		rows, err := insert(code)
		if err != nil {
			return "", err
		}
		if rows == 1 {
			return code, nil
		}
	}
	return "", errCodeExhausted
}

func generateShortCode() string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}

// ownerParam is NULL for anonymous requests and the user id otherwise.
func ownerParam(ctx context.Context) pgtype.Int4 {
	id, ok := middleware.UserID(ctx)
	if !ok {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: id, Valid: true}
}

func ShortenHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LongURL string `json:"long_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	longURL, ok := normalizeURL(req.LongURL)
	if !ok {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	queries := db.New(database.Pool)
	shortCode, err := createUniqueCode(func(code string) (int64, error) {
		_, err := queries.CreateURL(r.Context(), db.CreateURLParams{
			ShortCode: code,
			LongUrl:   longURL,
			UserID:    ownerParam(r.Context()),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil // code taken by a concurrent request
		}
		if err != nil {
			return 0, err
		}
		return 1, nil
	})
	if err != nil {
		log.Printf("Failed to create URL in database: %v\n", err)
		http.Error(w, "Failed to create URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"short_url": baseURL(r) + "/" + shortCode,
	})
}

// normalizeURL accepts links with or without a scheme and defaults to https.
func normalizeURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	candidate := raw
	if !strings.Contains(raw, "://") {
		candidate = "https://" + raw
	}

	parsed, err := url.ParseRequestURI(candidate)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || len(candidate) > 2048 {
		return "", false
	}
	return candidate, true
}

// baseURL uses the request host so links work on any deployment.
func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

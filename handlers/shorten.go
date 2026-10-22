package handlers

import (
	"encoding/json"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"

	"github.com/dadidudidap/url-shortener/database"
	"github.com/dadidudidap/url-shortener/database/db" // sqlc generated package
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateShortCode() string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}

func ShortenHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LongURL string `json:"long_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	shortCode := generateShortCode()
	queries := db.New(database.Pool)

	longURL, ok := normalizeURL(req.LongURL)
	if !ok {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// sqlc generates CreateURLParams so you never pass the wrong types
	err := queries.CreateURL(r.Context(), db.CreateURLParams{
		ShortCode: shortCode,
		LongUrl:   longURL,
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

package handlers

import (
	"encoding/json"
	"log"
	"math/rand/v2"
	"net/http"

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

	// sqlc generates CreateURLParams so you never pass the wrong types
	err := queries.CreateURL(r.Context(), db.CreateURLParams{
		ShortCode: shortCode,
		LongUrl:   req.LongURL,
	})

	if err != nil {
		log.Printf("Failed to create URL in database: %v\n", err)
		http.Error(w, "Failed to create URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"short_url": "http://localhost:8000/" + shortCode,
	})
}

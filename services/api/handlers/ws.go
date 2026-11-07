package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/dadidudidap/url-shortener/services/api/ws"
	"github.com/dadidudidap/url-shortener/shared/database"
	"github.com/dadidudidap/url-shortener/shared/database/db"
	"github.com/dadidudidap/url-shortener/shared/token"
)

func LiveStatsHandler(secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerFromProtocols(r.Header.Get("Sec-WebSocket-Protocol"))
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := token.Verify(secret, raw)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		shortCode := chi.URLParam(r, "code")
		dbCtx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
		defer cancel()

		_, err = db.New(database.Pool).GetURLForOwner(dbCtx, db.GetURLForOwnerParams{
			ShortCode: shortCode,
			UserID:    pgtype.Int4{Int32: claims.UserID, Valid: true},
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.Error(w, "URL not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Could not fetch URL", http.StatusInternalServerError)
			return
		}

		ws.Manager.ServeWS(w, r, shortCode)
	}
}

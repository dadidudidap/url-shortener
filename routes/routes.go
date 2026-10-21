package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dadidudidap/url-shortener/handlers"
	"github.com/dadidudidap/url-shortener/middleware"
)

func RegisterRoutes() http.Handler {
	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		r.With(middleware.RateLimit).Post("/shorten", handlers.ShortenHandler)
		r.Get("/stats/{code}", handlers.GetStatsHandler)
	})
	r.Get("/{code}", handlers.RedirectHandler)

	return r
}

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
		r.Get("/qr/{code}", handlers.QRHandler)
		r.Get("/ws/{code}", handlers.LiveStatsHandler)
	})
	// Only six-character codes reach the redirect handler, so asset paths
	// such as /styles.css fall through to the static file server below.
	r.Get("/{code:[A-Za-z0-9]{6}}", handlers.RedirectHandler)

	staticHandler := http.FileServer(http.Dir("src"))
	r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		staticHandler.ServeHTTP(w, req)
	}))

	return r
}

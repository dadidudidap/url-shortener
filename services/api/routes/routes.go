package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dadidudidap/url-shortener/services/api/handlers"
	"github.com/dadidudidap/url-shortener/services/api/middleware"
	"github.com/dadidudidap/url-shortener/shared/pb"
)

func RegisterRoutes(authClient pb.AuthServiceClient, jwtSecret string) *chi.Mux {
	r := chi.NewRouter()

	r.Route("/api", func(r chi.Router) {
		r.With(middleware.RateLimit, middleware.OptionalAuth(jwtSecret)).Post("/shorten", handlers.ShortenHandler)
		r.With(middleware.RateLimit).Post("/register", handlers.RegisterHandler(authClient))
		r.With(middleware.RateLimit).Post("/login", handlers.LoginHandler(authClient))
		r.With(middleware.Auth(jwtSecret)).Get("/links", handlers.ListLinksHandler)
		r.Get("/stats/{code}", handlers.GetStatsHandler)
		r.Get("/qr/{code}", handlers.QRHandler)
		r.Get("/ws/{code}", handlers.LiveStatsHandler)
	})
	// Asset paths such as /styles.css contain a dot and fall through to the
	// static file server below.
	r.Get("/{code:[A-Za-z0-9_-]{1,32}}", handlers.RedirectHandler)

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

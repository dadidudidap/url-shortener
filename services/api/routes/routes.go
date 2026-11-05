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
		r.Post("/register", handlers.RegisterHandler(authClient))
		r.Post("/login", handlers.LoginHandler(authClient))
		r.With(middleware.Auth(jwtSecret)).Get("/protected", handlers.UserHandler)
		r.With(middleware.Auth(jwtSecret)).Get("/links", handlers.ListLinksHandler)
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

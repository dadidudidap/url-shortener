package handlers

import (
	"net/http"

	"github.com/dadidudidap/url-shortener/services/api/middleware"
)

// AuthHandler answers a request already validated by middleware.Auth.
func AuthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Welcome, " + middleware.UserID(r.Context()) + "! You accessed the protected route."))
}

package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dadidudidap/url-shortener/ws"
)

func LiveStatsHandler(w http.ResponseWriter, r *http.Request) {
	ws.Manager.ServeWS(w, r, chi.URLParam(r, "code"))
}

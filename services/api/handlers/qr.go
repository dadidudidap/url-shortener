package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"
)

// QRHandler returns a PNG QR code that points at the short link.
func QRHandler(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if !shortCodePattern.MatchString(code) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	png, err := qrcode.Encode(baseURL(r)+"/"+code, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "Could not generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(png)
}

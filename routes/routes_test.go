package routes

import (
	"bytes"
	"image/png"
	"net/http/httptest"
	"testing"
)

func TestQRRoute(t *testing.T) {
	rec := httptest.NewRecorder()
	RegisterRoutes().ServeHTTP(rec, httptest.NewRequest("GET", "/api/qr/abc123", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}

	img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if bounds := img.Bounds(); bounds.Dx() != 256 || bounds.Dy() != 256 {
		t.Errorf("size = %dx%d, want 256x256", bounds.Dx(), bounds.Dy())
	}
}

func TestQRRouteRejectsBadCode(t *testing.T) {
	rec := httptest.NewRecorder()
	RegisterRoutes().ServeHTTP(rec, httptest.NewRequest("GET", "/api/qr/nope", nil))

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

package routes

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRootServesFrontend(t *testing.T) {
	t.Chdir("..")
	rec := httptest.NewRecorder()
	RegisterRoutes().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`id="shorten-form"`)) {
		t.Errorf("page is missing the shorten form")
	}
}

func TestStaticAssets(t *testing.T) {
	t.Chdir("..")

	tests := []struct {
		path        string
		contentType string
	}{
		{"/styles.css", "text/css; charset=utf-8"},
		{"/app.js", "text/javascript; charset=utf-8"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			RegisterRoutes().ServeHTTP(rec, httptest.NewRequest("GET", test.path, nil))

			if rec.Code != 200 {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if rec.Body.Len() == 0 {
				t.Fatal("empty body")
			}
			if got := rec.Header().Get("Content-Type"); got != test.contentType {
				t.Errorf("Content-Type = %q, want %q", got, test.contentType)
			}
		})
	}
}

func TestStaticHandlerRejectsWrites(t *testing.T) {
	t.Chdir("..")
	rec := httptest.NewRecorder()
	RegisterRoutes().ServeHTTP(rec, httptest.NewRequest("POST", "/styles.css", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

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

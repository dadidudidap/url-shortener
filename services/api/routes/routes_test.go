package routes

import (
	"bytes"
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"

	"github.com/dadidudidap/url-shortener/shared/pb"
)

func TestRootServesFrontend(t *testing.T) {
	t.Chdir("../../..")
	rec := httptest.NewRecorder()
	RegisterRoutes(nil).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

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
	t.Chdir("../../..")

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
			RegisterRoutes(nil).ServeHTTP(rec, httptest.NewRequest("GET", test.path, nil))

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
	t.Chdir("../../..")
	rec := httptest.NewRecorder()
	RegisterRoutes(nil).ServeHTTP(rec, httptest.NewRequest("POST", "/styles.css", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestQRRoute(t *testing.T) {
	rec := httptest.NewRecorder()
	RegisterRoutes(nil).ServeHTTP(rec, httptest.NewRequest("GET", "/api/qr/abc123", nil))

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
	RegisterRoutes(nil).ServeHTTP(rec, httptest.NewRequest("GET", "/api/qr/nope", nil))

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

type fakeAuth struct{ valid bool }

func (f fakeAuth) ValidateToken(context.Context, *pb.TokenRequest, ...grpc.CallOption) (*pb.TokenResponse, error) {
	if !f.valid {
		return &pb.TokenResponse{}, nil
	}
	return &pb.TokenResponse{IsValid: true, UserId: "user_999"}, nil
}

func TestProtectedRoute(t *testing.T) {
	tests := []struct {
		name    string
		auth    pb.AuthServiceClient
		apiKey  string
		want    int
		welcome bool
	}{
		{name: "missing key", auth: fakeAuth{valid: true}, want: http.StatusUnauthorized},
		{name: "invalid key", auth: fakeAuth{}, apiKey: "nope", want: http.StatusUnauthorized},
		{name: "valid key", auth: fakeAuth{valid: true}, apiKey: "ok", want: http.StatusOK, welcome: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/protected", nil)
			if test.apiKey != "" {
				req.Header.Set("X-API-Key", test.apiKey)
			}
			rec := httptest.NewRecorder()
			RegisterRoutes(test.auth).ServeHTTP(rec, req)

			if rec.Code != test.want {
				t.Fatalf("status = %d, want %d", rec.Code, test.want)
			}
			if test.welcome && !bytes.Contains(rec.Body.Bytes(), []byte("user_999")) {
				t.Errorf("body = %q, want resolved user id", rec.Body.String())
			}
		})
	}
}

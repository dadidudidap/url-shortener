package routes

import (
	"bytes"
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dadidudidap/url-shortener/shared/database"
	"github.com/dadidudidap/url-shortener/shared/pb"
)

const testSecret = "test-secret"

// TestMain points the rate limiter at a dead address so it fails open instead
// of panicking on a nil client. The 429 path needs a live Redis, so it is
// covered by the end-to-end run, not here.
func TestMain(m *testing.M) {
	database.RedisClient = redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:0",
		DialTimeout: 50 * time.Millisecond,
		MaxRetries:  -1,
	})
	os.Exit(m.Run())
}

func TestRootServesFrontend(t *testing.T) {
	t.Chdir("../../..")
	rec := httptest.NewRecorder()
	RegisterRoutes(nil, testSecret).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

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
			RegisterRoutes(nil, testSecret).ServeHTTP(rec, httptest.NewRequest("GET", test.path, nil))

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
	RegisterRoutes(nil, testSecret).ServeHTTP(rec, httptest.NewRequest("POST", "/styles.css", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestQRRoute(t *testing.T) {
	rec := httptest.NewRecorder()
	RegisterRoutes(nil, testSecret).ServeHTTP(rec, httptest.NewRequest("GET", "/api/qr/abc123", nil))

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
	RegisterRoutes(nil, testSecret).ServeHTTP(rec, httptest.NewRequest("GET", "/api/qr/bad.code", nil))

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

type fakeAuth struct{ loginErr error }

func (f fakeAuth) Register(_ context.Context, in *pb.RegisterRequest, _ ...grpc.CallOption) (*pb.UserResponse, error) {
	return &pb.UserResponse{Id: 7, Email: in.GetEmail()}, nil
}

func (f fakeAuth) Login(_ context.Context, in *pb.LoginRequest, _ ...grpc.CallOption) (*pb.LoginResponse, error) {
	if f.loginErr != nil {
		return nil, f.loginErr
	}
	return &pb.LoginResponse{
		Token: "test-token",
		User:  &pb.UserResponse{Id: 7, Email: in.GetEmail()},
	}, nil
}

func TestRegisterEndpoint(t *testing.T) {
	body := strings.NewReader(`{"email":"a@b.com","password":"secret123"}`)
	rec := httptest.NewRecorder()
	RegisterRoutes(fakeAuth{}, testSecret).ServeHTTP(rec, httptest.NewRequest("POST", "/api/register", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("a@b.com")) {
		t.Errorf("body = %q, want the registered email", rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("password")) {
		t.Errorf("body = %q, leaks password material", rec.Body.String())
	}
}

func TestRegisterEndpointRejectsBadJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	RegisterRoutes(fakeAuth{}, testSecret).ServeHTTP(rec, httptest.NewRequest("POST", "/api/register", strings.NewReader("{")))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestLoginEndpoint(t *testing.T) {
	body := strings.NewReader(`{"email":"a@b.com","password":"secret123"}`)
	rec := httptest.NewRecorder()
	RegisterRoutes(fakeAuth{}, testSecret).ServeHTTP(rec, httptest.NewRequest("POST", "/api/login", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("test-token")) {
		t.Errorf("body = %q, want the token", rec.Body.String())
	}
}

func TestLoginEndpointGRPCError(t *testing.T) {
	auth := fakeAuth{loginErr: status.Error(codes.Unavailable, "auth down")}
	body := strings.NewReader(`{"email":"a@b.com","password":"secret123"}`)
	rec := httptest.NewRecorder()
	RegisterRoutes(auth, testSecret).ServeHTTP(rec, httptest.NewRequest("POST", "/api/login", body))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

func TestLinksRequiresToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
	}{
		{name: "missing token"},
		{name: "invalid token", authHeader: "Bearer nope"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/links", nil)
			if test.authHeader != "" {
				req.Header.Set("Authorization", test.authHeader)
			}
			rec := httptest.NewRecorder()
			RegisterRoutes(fakeAuth{}, testSecret).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}

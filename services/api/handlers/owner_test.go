package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dadidudidap/url-shortener/services/api/middleware"
	"github.com/dadidudidap/url-shortener/shared/token"
)

func TestOwnerParamAnonymous(t *testing.T) {
	got := ownerParam(context.Background())
	if got.Valid {
		t.Errorf("ownerParam() = %+v, want NULL owner", got)
	}
}

func TestOwnerParamAuthenticated(t *testing.T) {
	raw, err := token.Sign("test-secret", token.Claims{UserID: 42}, time.Hour)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	var ctx context.Context
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ctx = r.Context() })
	req := httptest.NewRequest("POST", "/api/shorten", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	middleware.OptionalAuth("test-secret")(next).ServeHTTP(httptest.NewRecorder(), req)

	got := ownerParam(ctx)
	if !got.Valid || got.Int32 != 42 {
		t.Errorf("ownerParam() = %+v, want 42", got)
	}
}

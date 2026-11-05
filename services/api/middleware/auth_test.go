package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dadidudidap/url-shortener/shared/token"
)

const testSecret = "test-secret"

func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestAuthRejectsMissingHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler ran without a token")
	})

	rec := serve(Auth(testSecret)(next), httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthRejectsGarbageToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler ran with a bad token")
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer nope")

	rec := serve(Auth(testSecret)(next), req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthAcceptsValidToken(t *testing.T) {
	raw, err := token.Sign(testSecret, token.Claims{UserID: 42}, time.Hour)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	var got int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := UserID(r.Context())
		if !ok {
			t.Error("UserID() ok = false, want true")
		}
		got = id
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)

	rec := serve(Auth(testSecret)(next), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got != 42 {
		t.Errorf("UserID() = %d, want 42", got)
	}
}

func TestAuthRejectsExpiredToken(t *testing.T) {
	raw, err := token.Sign(testSecret, token.Claims{UserID: 1}, -time.Minute)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler ran with an expired token")
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)

	rec := serve(Auth(testSecret)(next), req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthRejectsForeignSecret(t *testing.T) {
	raw, err := token.Sign("other-secret", token.Claims{UserID: 1}, time.Hour)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler ran with a foreign token")
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)

	rec := serve(Auth(testSecret)(next), req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestOptionalAuthAllowsAnonymous(t *testing.T) {
	var ran bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ran = true
		if _, ok := UserID(r.Context()); ok {
			t.Error("UserID() ok = true for an anonymous request")
		}
	})

	rec := serve(OptionalAuth(testSecret)(next), httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK || !ran {
		t.Errorf("status = %d, ran = %v; want 200 and next called", rec.Code, ran)
	}
}

func TestOptionalAuthRejectsBadToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler ran with a bad token")
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer nope")

	rec := serve(OptionalAuth(testSecret)(next), req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestOptionalAuthRejectsEmptyBearer(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler ran with an empty bearer token")
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer ")

	rec := serve(OptionalAuth(testSecret)(next), req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuthAcceptsLowercaseScheme(t *testing.T) {
	raw, err := token.Sign(testSecret, token.Claims{UserID: 5}, time.Hour)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	var ran bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ran = true })

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "bearer "+raw)

	rec := serve(Auth(testSecret)(next), req)
	if rec.Code != http.StatusOK || !ran {
		t.Errorf("status = %d, ran = %v; want 200", rec.Code, ran)
	}
}

func TestOptionalAuthRejectsExpiredToken(t *testing.T) {
	raw, err := token.Sign(testSecret, token.Claims{UserID: 1}, -time.Minute)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler ran with an expired token")
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)

	rec := serve(OptionalAuth(testSecret)(next), req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatsRequiresToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
	}{
		{name: "missing token"},
		{name: "invalid token", authHeader: "Bearer nope"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/stats/abc123", nil)
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

func TestWSRequiresToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/ws/abc123", nil)
	rec := httptest.NewRecorder()
	RegisterRoutes(fakeAuth{}, testSecret).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

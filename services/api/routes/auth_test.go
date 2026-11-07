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

func TestEditAndDeleteRequireToken(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{name: "edit", method: http.MethodPut},
		{name: "delete", method: http.MethodDelete},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(test.method, "/api/links/abc123", nil)
			RegisterRoutes(fakeAuth{}, testSecret).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}

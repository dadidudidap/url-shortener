package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesPage(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, `id="shorten-form"`) || !strings.Contains(body, `id="stats-form"`) {
		t.Errorf("page is missing its forms")
	}
}

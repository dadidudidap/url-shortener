package handlers

import (
	"strings"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"bare domain", "example.com/path", "https://example.com/path", true},
		{"bare domain with port", "example.com:8080/path", "https://example.com:8080/path", true},
		{"https kept", "https://example.com/path", "https://example.com/path", true},
		{"http kept", "http://example.com", "http://example.com", true},
		{"spaces trimmed", "  example.com  ", "https://example.com", true},
		{"empty", "   ", "", false},
		{"ftp rejected", "ftp://example.com", "", false},
		{"javascript rejected", "javascript:alert(1)", "", false},
		{"missing host", "https://", "", false},
		{"too long", "https://example.com/" + strings.Repeat("a", 2049), "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := normalizeURL(test.in)
			if got != test.want || ok != test.ok {
				t.Errorf("normalizeURL(%q) = (%q, %v), want (%q, %v)", test.in, got, ok, test.want, test.ok)
			}
		})
	}
}

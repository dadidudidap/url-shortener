package main

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNormalizeEmail(t *testing.T) {
	tests := map[string]string{
		" User@Example.COM ": "user@example.com",
		"user@example.com":   "user@example.com",
		"":                   "",
	}

	for in, want := range tests {
		if got := normalizeEmail(in); got != want {
			t.Errorf("normalizeEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
	}{
		{"valid", "user@example.com", "secret123", false},
		{"empty email", "", "secret123", true},
		{"short password", "user@example.com", "short", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateCredentials(test.email, test.password)
			if (err != nil) != test.wantErr {
				t.Errorf("validateCredentials() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestIsDuplicateKey(t *testing.T) {
	duplicate := &pgconn.PgError{Code: "23505"}
	if !isDuplicateKey(duplicate) {
		t.Error("isDuplicateKey() = false for 23505")
	}
	if isDuplicateKey(&pgconn.PgError{Code: "23503"}) {
		t.Error("isDuplicateKey() = true for 23503")
	}
	if isDuplicateKey(errors.New("boom")) {
		t.Error("isDuplicateKey() = true for a plain error")
	}
}

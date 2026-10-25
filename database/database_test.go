package database

import (
	"strings"
	"testing"
)

func TestDatabaseURLBuildsFromParts(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_USER", "user")
	t.Setenv("POSTGRES_PASSWORD", "p@ss word")
	t.Setenv("POSTGRES_DB", "url_db")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")

	got, err := databaseURL()
	if err != nil {
		t.Fatalf("databaseURL() error: %v", err)
	}
	want := "postgres://user:p%40ss%20word@localhost:5432/url_db?sslmode=disable"
	if got != want {
		t.Errorf("databaseURL() = %q, want %q", got, want)
	}
}

func TestDatabaseURLPrefersFullURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://custom/db")
	t.Setenv("POSTGRES_USER", "ignored")

	got, err := databaseURL()
	if err != nil {
		t.Fatalf("databaseURL() error: %v", err)
	}
	if got != "postgres://custom/db" {
		t.Errorf("databaseURL() = %q", got)
	}
}

func TestDatabaseURLRequiresEveryPart(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_USER", "user")
	t.Setenv("POSTGRES_PASSWORD", "")
	t.Setenv("POSTGRES_DB", "url_db")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")

	_, err := databaseURL()
	if err == nil {
		t.Fatal("databaseURL() succeeded with a missing part")
	}
	if !strings.Contains(err.Error(), "DB_PORT") {
		t.Errorf("error does not name the required variables: %v", err)
	}
}

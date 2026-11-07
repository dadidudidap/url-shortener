package handlers

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestExpiresAtDefault(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	got := expiresAtDefault(now)

	if !got.Valid {
		t.Fatal("expiresAtDefault() is NULL, want a time")
	}
	if want := now.Add(defaultLinkLifetime); !got.Time.Equal(want) {
		t.Errorf("expiresAtDefault() = %v, want %v", got.Time, want)
	}
}

func TestExpiresAtFromEdit(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name     string
		raw      string
		lifetime bool
		want     pgtype.Timestamptz
		wantErr  bool
	}{
		{
			name: "empty uses three day default",
			want: pgtype.Timestamptz{Time: now.Add(defaultLinkLifetime), Valid: true},
		},
		{
			name: "future date is preserved",
			raw:  future.Format(time.RFC3339),
			want: pgtype.Timestamptz{Time: future, Valid: true},
		},
		{
			name:    "past date is rejected",
			raw:     past.Format(time.RFC3339),
			wantErr: true,
		},
		{
			name:     "lifetime ignores a past date",
			raw:      past.Format(time.RFC3339),
			lifetime: true,
			want:     pgtype.Timestamptz{},
		},
		{
			name:     "lifetime ignores an empty date",
			lifetime: true,
			want:     pgtype.Timestamptz{},
		},
		{
			name:    "bad date is rejected",
			raw:     "tomorrow",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := expiresAtFromEdit(test.raw, test.lifetime, now)
			if test.wantErr {
				if err == nil {
					t.Fatal("expiresAtFromEdit() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("expiresAtFromEdit() error = %v", err)
			}
			if got.Valid != test.want.Valid || (got.Valid && !got.Time.Equal(test.want.Time)) {
				t.Errorf("expiresAtFromEdit() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestIsExpired(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt pgtype.Timestamptz
		want      bool
	}{
		{name: "null is lifetime", want: false},
		{name: "past is expired", expiresAt: pgtype.Timestamptz{Time: now.Add(-time.Second), Valid: true}, want: true},
		{name: "future is active", expiresAt: pgtype.Timestamptz{Time: now.Add(time.Second), Valid: true}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isExpired(test.expiresAt, now); got != test.want {
				t.Errorf("isExpired() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestValidCustomCode(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{code: "my-link_2", want: true},
		{code: "abc", want: true},
		{code: "ab", want: false},
		{code: "api", want: false},
		{code: "API", want: false},
		{code: "bad code", want: false},
		{code: "123456789012345678901234567890123", want: false},
		{code: "has.dot", want: false},
	}

	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			if got := validCustomCode(test.code); got != test.want {
				t.Errorf("validCustomCode(%q) = %v, want %v", test.code, got, test.want)
			}
		})
	}
}

func TestShortCodePattern(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{code: "a", want: true},
		{code: "abc-123_x", want: true},
		{code: "has.dot", want: false},
		{code: "123456789012345678901234567890123", want: false},
	}

	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			if got := shortCodePattern.MatchString(test.code); got != test.want {
				t.Errorf("shortCodePattern.MatchString(%q) = %v, want %v", test.code, got, test.want)
			}
		})
	}
}

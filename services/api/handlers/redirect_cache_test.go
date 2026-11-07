package handlers

import (
	"testing"
	"time"
)

func ptrTime(t time.Time) *time.Time { return &t }
func ptrInt32(v int32) *int32        { return &v }

func TestDecodeRedirectRejectsLegacyString(t *testing.T) {
	if _, err := decodeRedirect([]byte("https://example.com/old")); err == nil {
		t.Fatal("decodeRedirect() accepted a legacy raw URL")
	}
}

func TestDecideRedirect(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		target      cachedRedirect
		wantActive  bool
		wantDelete  bool
		wantLongURL string
	}{
		{
			name: "active user link",
			target: cachedRedirect{
				LongURL:   "https://example.com/a",
				UserID:    ptrInt32(7),
				ExpiresAt: ptrTime(now.Add(time.Hour)),
			},
			wantActive:  true,
			wantLongURL: "https://example.com/a",
		},
		{
			name: "lifetime guest link",
			target: cachedRedirect{
				LongURL: "https://example.com/b",
			},
			wantActive:  true,
			wantLongURL: "https://example.com/b",
		},
		{
			name: "expired guest link",
			target: cachedRedirect{
				LongURL:   "https://example.com/c",
				ExpiresAt: ptrTime(now.Add(-time.Second)),
			},
			wantDelete: true,
		},
		{
			name: "expired user link",
			target: cachedRedirect{
				LongURL:   "https://example.com/d",
				UserID:    ptrInt32(7),
				ExpiresAt: ptrTime(now.Add(-time.Second)),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := decideRedirect(test.target, now)
			if got.Active != test.wantActive {
				t.Errorf("decideRedirect().Active = %v, want %v", got.Active, test.wantActive)
			}
			if got.DeleteGuest != test.wantDelete {
				t.Errorf("decideRedirect().DeleteGuest = %v, want %v", got.DeleteGuest, test.wantDelete)
			}
			if got.LongURL != test.wantLongURL {
				t.Errorf("decideRedirect().LongURL = %q, want %q", got.LongURL, test.wantLongURL)
			}
		})
	}
}

func TestCacheTTL(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      time.Duration
	}{
		{name: "lifetime uses default", want: 24 * time.Hour},
		{name: "short expiry wins", expiresAt: ptrTime(now.Add(time.Hour)), want: time.Hour},
		{name: "long expiry caps at default", expiresAt: ptrTime(now.Add(48 * time.Hour)), want: 24 * time.Hour},
		{name: "expired is zero", expiresAt: ptrTime(now.Add(-time.Second)), want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := cacheTTL(test.expiresAt, now); got != test.want {
				t.Errorf("cacheTTL() = %v, want %v", got, test.want)
			}
		})
	}
}

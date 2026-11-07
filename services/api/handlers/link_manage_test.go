package handlers

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/dadidudidap/url-shortener/shared/database/db"
)

func TestUpdateLinkRequestValidation(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name    string
		req     updateLinkRequest
		want    pgtype.Timestamptz
		wantErr bool
	}{
		{
			name:    "short code rejected",
			req:     updateLinkRequest{ShortCode: "ab"},
			wantErr: true,
		},
		{
			name:    "past date rejected",
			req:     updateLinkRequest{ShortCode: "my-link", ExpiresAt: past.Format(time.RFC3339)},
			wantErr: true,
		},
		{
			name: "lifetime wins over past date",
			req:  updateLinkRequest{ShortCode: "my-link", ExpiresAt: past.Format(time.RFC3339), Lifetime: true},
			want: pgtype.Timestamptz{},
		},
		{
			name: "future date preserved",
			req:  updateLinkRequest{ShortCode: "my-link", ExpiresAt: future.Format(time.RFC3339)},
			want: pgtype.Timestamptz{Time: future, Valid: true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.req.validate(now)
			if test.wantErr {
				if err == nil {
					t.Fatal("validate() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("validate() error = %v", err)
			}
			if got.Valid != test.want.Valid || (got.Valid && !got.Time.Equal(test.want.Time)) {
				t.Errorf("validate() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestLinkPayloadExpiry(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		expiresAt   pgtype.Timestamptz
		wantNil     bool
		wantExpired bool
	}{
		{name: "null is lifetime", wantNil: true},
		{name: "past is expired", expiresAt: pgtype.Timestamptz{Time: now.Add(-time.Second), Valid: true}, wantExpired: true},
		{name: "future is active", expiresAt: pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := linkPayloadFromURL(db.Url{ExpiresAt: test.expiresAt}, now)
			if (payload.ExpiresAt == nil) != test.wantNil {
				t.Errorf("ExpiresAt = %v, want nil = %v", payload.ExpiresAt, test.wantNil)
			}
			if payload.Expired != test.wantExpired {
				t.Errorf("Expired = %v, want %v", payload.Expired, test.wantExpired)
			}
		})
	}
}

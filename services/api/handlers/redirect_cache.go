package handlers

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/dadidudidap/url-shortener/shared/database/db"
)

const redirectCacheTTL = 24 * time.Hour

type cachedRedirect struct {
	LongURL   string     `json:"long_url"`
	UserID    *int32     `json:"user_id"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type redirectDecision struct {
	LongURL     string
	Active      bool
	DeleteGuest bool
}

func encodeRedirect(target cachedRedirect) ([]byte, error) {
	return json.Marshal(target)
}

func decodeRedirect(raw []byte) (cachedRedirect, error) {
	var target cachedRedirect
	if err := json.Unmarshal(raw, &target); err != nil {
		return cachedRedirect{}, err
	}
	if target.LongURL == "" {
		return cachedRedirect{}, errors.New("cached redirect has no URL")
	}
	return target, nil
}

func decideRedirect(target cachedRedirect, now time.Time) redirectDecision {
	if target.ExpiresAt != nil && !target.ExpiresAt.After(now) {
		return redirectDecision{DeleteGuest: target.UserID == nil}
	}
	return redirectDecision{LongURL: target.LongURL, Active: true}
}

func cacheTTL(expiresAt *time.Time, now time.Time) time.Duration {
	if expiresAt == nil {
		return redirectCacheTTL
	}
	remaining := expiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	if remaining > redirectCacheTTL {
		return redirectCacheTTL
	}
	return remaining
}

func cachedRedirectFromRow(row db.GetURLForRedirectRow) cachedRedirect {
	target := cachedRedirect{LongURL: row.LongUrl}
	if row.UserID.Valid {
		userID := row.UserID.Int32
		target.UserID = &userID
	}
	if row.ExpiresAt.Valid {
		expiresAt := row.ExpiresAt.Time
		target.ExpiresAt = &expiresAt
	}
	return target
}

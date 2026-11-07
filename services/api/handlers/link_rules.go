package handlers

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

const defaultLinkLifetime = 72 * time.Hour

var (
	shortCodePattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
	customCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)
)

var errExpiryInPast = errors.New("expiry must be in the future")

func expiresAtDefault(now time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: now.Add(defaultLinkLifetime), Valid: true}
}

func expiresAtFromEdit(raw string, lifetime bool, now time.Time) (pgtype.Timestamptz, error) {
	if lifetime {
		return pgtype.Timestamptz{}, nil
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return expiresAtDefault(now), nil
	}

	expiresAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return pgtype.Timestamptz{}, err
	}
	if !expiresAt.After(now) {
		return pgtype.Timestamptz{}, errExpiryInPast
	}
	return pgtype.Timestamptz{Time: expiresAt, Valid: true}, nil
}

func isExpired(expiresAt pgtype.Timestamptz, now time.Time) bool {
	return expiresAt.Valid && !expiresAt.Time.After(now)
}

func validCustomCode(code string) bool {
	return customCodePattern.MatchString(code) && !strings.EqualFold(code, "api")
}

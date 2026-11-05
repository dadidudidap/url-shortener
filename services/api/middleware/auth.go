package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/dadidudidap/url-shortener/shared/token"
)

type userIDKey struct{}

// UserID returns the user resolved by Auth, or ok=false on a public route.
func UserID(ctx context.Context) (int32, bool) {
	id, ok := ctx.Value(userIDKey{}).(int32)
	return id, ok
}

func bearerToken(r *http.Request) (string, bool) {
	raw, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		return "", false
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	return raw, true
}

func resolve(secret string, r *http.Request) (context.Context, bool) {
	raw, ok := bearerToken(r)
	if !ok {
		return nil, false
	}

	claims, err := token.Verify(secret, raw)
	if err != nil {
		return nil, false
	}
	return context.WithValue(r.Context(), userIDKey{}, claims.UserID), true
}

// Auth requires a valid bearer token and puts the user id in the context.
func Auth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, ok := resolve(secret, r)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth allows anonymous requests but rejects a token that is present and invalid.
func OptionalAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := bearerToken(r); !ok {
				next.ServeHTTP(w, r)
				return
			}

			ctx, valid := resolve(secret, r)
			if !valid {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

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

// tokenFromHeader splits "Bearer <token>". present reports whether the header
// was sent at all, ok whether it carried a usable bearer token.
func tokenFromHeader(r *http.Request) (raw string, present, ok bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false, false
	}

	scheme, value, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", true, false
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", true, false
	}
	return value, true, true
}

func resolve(secret string, r *http.Request) (context.Context, bool) {
	raw, _, ok := tokenFromHeader(r)
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
			_, present, ok := tokenFromHeader(r)
			if !present {
				next.ServeHTTP(w, r)
				return
			}
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
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

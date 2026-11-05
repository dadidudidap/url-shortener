package middleware

import (
	"context"
	"net/http"

	"github.com/dadidudidap/url-shortener/shared/pb"
)

type userIDKey struct{}

// UserID returns the user resolved by Auth, or "" when the request skipped it.
func UserID(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey{}).(string)
	return id
}

func Auth(authClient pb.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				http.Error(w, "Unauthorized: Missing API Key", http.StatusUnauthorized)
				return
			}

			resp, err := authClient.ValidateToken(r.Context(), &pb.TokenRequest{
				ApiKey: apiKey,
			})

			// If the microservice is down or the key is invalid, reject the request
			if err != nil || !resp.IsValid {
				http.Error(w, "Unauthorized: Invalid API Key", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, resp.UserId)))
		})
	}
}

package middleware

import (
	"log"
	"net"
	"net/http"
	"time"

	"github.com/dadidudidap/url-shortener/database"
)

func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		key := "rate_limit:" + ip
		ctx := r.Context()

		requests, err := database.RedisClient.Incr(ctx, key).Result()
		if err != nil {
			log.Printf("Rate limit error: %v\n", err)
			next.ServeHTTP(w, r)
			return
		}

		if requests == 1 {
			database.RedisClient.Expire(ctx, key, 60*time.Second)
		}

		if requests > 5 {
			http.Error(w, "Too many requests. Please wait a minute.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

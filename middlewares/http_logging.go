package middlewares

import (
	"log"
	"net/http"
	"time"
)

// HTTPLoggingMiddleware logs each HTTP request and its duration to logger.
func HTTPLoggingMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			logger.Printf("➡️  %s %s", r.Method, r.URL.Path)

			next.ServeHTTP(w, r)

			duration := time.Since(start)
			logger.Printf("⬅️  %s %s (%s)", r.Method, r.URL.Path, duration)
		})
	}
}

package middleware

import (
	"net/http"
	"time"

	"github.com/KOFI-GYIMAH/github-monitor/pkg/logger"
)

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rr := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rr, r)

		duration := time.Since(start)

		logger.Info("%s %s %d %s", r.Method, r.RequestURI, rr.statusCode, duration)
	})
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

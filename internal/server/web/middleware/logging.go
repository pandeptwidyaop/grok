package middleware

import (
	"net/http"
	"time"

	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/rs/zerolog"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// Flush implements http.Flusher interface for SSE support
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// HTTPLogger logs all HTTP requests with detailed information
func HTTPLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default status code
		}

		// Get user info from context if available
		username := ""
		userID := ""
		role := ""
		if claims := GetClaimsFromContext(r.Context()); claims != nil {
			username = claims.Username
			userID = claims.UserID
			role = claims.Role
		}

		// Process request
		next.ServeHTTP(rw, r)

		// Calculate duration
		duration := time.Since(start)

		// Choose log level based on status code
		var logEvent *zerolog.Event
		if rw.statusCode >= 500 {
			logEvent = logger.ErrorEvent()
		} else if rw.statusCode >= 400 {
			logEvent = logger.WarnEvent()
		} else {
			logEvent = logger.InfoEvent()
		}

		// Build log event with all details
		logEvent = logEvent.
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent()).
			Int("status", rw.statusCode).
			Int64("bytes", rw.written).
			Dur("duration", duration).
			Str("duration_ms", duration.Round(time.Millisecond).String())

		// Add user info if authenticated
		if username != "" {
			logEvent = logEvent.
				Str("user_id", userID).
				Str("username", username).
				Str("role", role)
		}

		// Add query params if present
		if r.URL.RawQuery != "" {
			logEvent = logEvent.Str("query", r.URL.RawQuery)
		}

		logEvent.Msg("HTTP request")
	})
}

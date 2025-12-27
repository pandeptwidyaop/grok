package middleware

import "net/http"

// SecurityHeaders adds security-related HTTP headers to responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking attacks
		w.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS protection (legacy browsers)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Control referrer information
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Enforce HTTPS (only if request is HTTPS)
		// Note: Max-Age of 1 year (31536000 seconds)
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Content Security Policy (CSP)
		// Restrictive policy: only allow resources from same origin
		// Adjust based on application needs
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline'; " + // unsafe-inline needed for React
			"style-src 'self' 'unsafe-inline'; " + // unsafe-inline needed for styled-components
			"img-src 'self' data: https:; " +
			"font-src 'self' data:; " +
			"connect-src 'self'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
		w.Header().Set("Content-Security-Policy", csp)

		// Permissions Policy (formerly Feature-Policy)
		// Disable potentially dangerous browser features
		permissions := "camera=(), microphone=(), geolocation=(), payment=()"
		w.Header().Set("Permissions-Policy", permissions)

		next.ServeHTTP(w, r)
	})
}

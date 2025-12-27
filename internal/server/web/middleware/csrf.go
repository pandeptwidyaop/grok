package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

// CSRFProtection provides CSRF token generation and validation.
type CSRFProtection struct {
	tokens map[string]time.Time // token -> expiry
	mu     sync.RWMutex
}

// NewCSRFProtection creates a new CSRF protection middleware.
func NewCSRFProtection() *CSRFProtection {
	csrf := &CSRFProtection{
		tokens: make(map[string]time.Time),
	}

	// Cleanup expired tokens every 10 minutes
	go csrf.cleanupLoop()

	return csrf
}

// GenerateToken creates a new CSRF token.
func (c *CSRFProtection) GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	token := base64.URLEncoding.EncodeToString(bytes)

	c.mu.Lock()
	// Token valid for 1 hour
	c.tokens[token] = time.Now().Add(1 * time.Hour)
	c.mu.Unlock()

	return token, nil
}

// ValidateToken checks if a CSRF token is valid.
// Tokens are single-use and deleted after successful validation.
func (c *CSRFProtection) ValidateToken(token string) bool {
	if token == "" {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	expiry, exists := c.tokens[token]
	if !exists {
		return false
	}

	// Always delete token after validation attempt (single-use)
	delete(c.tokens, token)

	// Check if expired
	if time.Now().After(expiry) {
		return false
	}

	return true
}

// Protect wraps an HTTP handler with CSRF validation.
// Only validates for state-changing methods (POST, PUT, PATCH, DELETE).
// For SPAs, a new CSRF token is returned in the X-CSRF-Token response header
// after successful validation, allowing the client to use it for subsequent requests.
func (c *CSRFProtection) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only validate CSRF for state-changing methods
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" || r.Method == "DELETE" {
			// Get CSRF token from header
			csrfToken := r.Header.Get("X-CSRF-Token")

			if !c.ValidateToken(csrfToken) {
				http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
				return
			}

			// Generate new token for SPA to use in next request
			newToken, err := c.GenerateToken()
			if err == nil {
				w.Header().Set("X-CSRF-Token", newToken)
			}
		}

		next.ServeHTTP(w, r)
	})
}

// cleanupLoop periodically removes expired tokens.
func (c *CSRFProtection) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for token, expiry := range c.tokens {
			if now.After(expiry) {
				delete(c.tokens, token)
			}
		}
		c.mu.Unlock()
	}
}

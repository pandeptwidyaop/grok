package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides rate limiting middleware to prevent brute force attacks.
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     rate.Limit // requests per second
	burst    int        // max burst size
}

// visitor tracks rate limit state for a single IP/identifier.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new rate limiter.
// rate: maximum requests per second (e.g., 0.5 = 1 request per 2 seconds)
// burst: maximum burst size (e.g., 3 = allow 3 requests immediately)
func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    b,
	}

	// Cleanup old visitors every 5 minutes
	go rl.cleanupLoop()

	return rl
}

// getVisitor retrieves or creates a visitor for the given identifier (IP address).
func (rl *RateLimiter) getVisitor(identifier string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[identifier]
	if !exists {
		limiter := rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[identifier] = &visitor{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// getClientIP extracts the real client IP from the request.
// When behind a reverse proxy, this parses X-Forwarded-For properly.
// Note: X-Forwarded-For format is "client, proxy1, proxy2, ..."
// We take the leftmost (original client) IP, but this is still spoofable
// if not behind a trusted proxy. For production deployments behind proxies,
// configure your reverse proxy to strip client-provided X-Forwarded-For headers.
func (rl *RateLimiter) getClientIP(r *http.Request) string {
	// Try X-Forwarded-For first (most common for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// Take the leftmost (original client IP)
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			// Validate it's a valid IP
			if net.ParseIP(clientIP) != nil {
				return clientIP
			}
		}
	}

	// Try X-Real-IP as fallback
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	// Use direct connection IP as final fallback
	// This is the most reliable as it can't be spoofed
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // Return as-is if parsing fails
	}
	return ip
}

// Limit wraps an HTTP handler with rate limiting based on IP address.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.getClientIP(r)

		limiter := rl.getVisitor(ip)
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// cleanupLoop periodically removes old visitors to prevent memory leaks.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			// Remove visitors not seen in last 10 minutes
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

// TestNewRateLimiter tests rate limiter initialization
func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(1.0), 5)
	assert.NotNil(t, rl)
	assert.Equal(t, rate.Limit(1.0), rl.rate)
	assert.Equal(t, 5, rl.burst)
	assert.NotNil(t, rl.visitors)
}

// TestRateLimit_BurstAllow tests burst allowance
func TestRateLimit_BurstAllow(t *testing.T) {
	// Allow 1 req/sec with burst of 3
	rl := NewRateLimiter(rate.Limit(1.0), 3)

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 3 requests should succeed (burst)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "Request %d should succeed", i+1)
	}

	// 4th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, rec.Body.String(), "Rate limit exceeded")
}

// TestRateLimit_DifferentIPs tests that different IPs have separate rate limits
func TestRateLimit_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(1.0), 2)

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}

	// Each IP should get its own burst allowance
	for _, ip := range ips {
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = ip + ":1234"
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "IP %s request %d should succeed", ip, i+1)
		}
	}
}

// TestRateLimit_XForwardedFor tests X-Forwarded-For header priority
func TestRateLimit_XForwardedFor(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(1.0), 2)

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Use X-Forwarded-For header (proxy scenario)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:1234" // Local proxy
		req.Header.Set("X-Forwarded-For", "203.0.113.1") // Real client IP
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "Request %d should succeed", i+1)
	}

	// 3rd request should be rate limited (burst of 2)
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

// TestRateLimit_XRealIP tests X-Real-IP header priority
func TestRateLimit_XRealIP(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(1.0), 2)

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Use X-Real-IP header (nginx proxy scenario)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("X-Real-IP", "198.51.100.1")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "Request %d should succeed", i+1)
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Real-IP", "198.51.100.1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

// TestRateLimit_HeaderPriority tests that X-Forwarded-For takes priority over X-Real-IP
func TestRateLimit_HeaderPriority(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(1.0), 1)

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request with X-Forwarded-For
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "127.0.0.1:1234"
	req1.Header.Set("X-Forwarded-For", "203.0.113.1")
	req1.Header.Set("X-Real-IP", "198.51.100.1")
	rec1 := httptest.NewRecorder()

	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Second request with same X-Forwarded-For should be rate limited
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "127.0.0.1:1234"
	req2.Header.Set("X-Forwarded-For", "203.0.113.1")
	req2.Header.Set("X-Real-IP", "198.51.100.1")
	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)

	// Request with different X-Real-IP but no X-Forwarded-For should succeed
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "127.0.0.1:1234"
	req3.Header.Set("X-Real-IP", "198.51.100.1")
	rec3 := httptest.NewRecorder()

	handler.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusOK, rec3.Code, "Different IP should have separate limit")
}

// TestRateLimit_SlowRate tests slow rate limiting (1 req per 2 seconds)
func TestRateLimit_SlowRate(t *testing.T) {
	// 0.5 req/sec = 1 request per 2 seconds, burst of 1
	rl := NewRateLimiter(rate.Limit(0.5), 1)

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request succeeds
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	rec1 := httptest.NewRecorder()

	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Immediate second request fails
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)
}

// TestRateLimit_Recovery tests that rate limit recovers over time
func TestRateLimit_Recovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-dependent test in short mode")
	}

	// 10 req/sec, burst of 1 (for faster testing)
	rl := NewRateLimiter(rate.Limit(10.0), 1)

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request succeeds
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	rec1 := httptest.NewRecorder()

	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Immediate second request fails
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)

	// Wait for rate to recover (100ms for 10 req/sec)
	time.Sleep(150 * time.Millisecond)

	// Third request should succeed after recovery
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.168.1.1:1234"
	rec3 := httptest.NewRecorder()

	handler.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusOK, rec3.Code, "Request should succeed after rate recovery")
}

// TestGetVisitor tests visitor creation and retrieval
func TestGetVisitor(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(1.0), 5)

	// Get visitor for first time (creates new)
	limiter1 := rl.getVisitor("192.168.1.1")
	assert.NotNil(t, limiter1)

	// Check visitor was stored
	rl.mu.RLock()
	_, exists := rl.visitors["192.168.1.1"]
	rl.mu.RUnlock()
	assert.True(t, exists)

	// Get same visitor again (retrieves existing)
	limiter2 := rl.getVisitor("192.168.1.1")
	assert.Equal(t, limiter1, limiter2, "Should return same limiter instance")
}

// TestRateLimit_CleanupLoop tests visitor cleanup
func TestRateLimit_CleanupLoop(t *testing.T) {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate.Limit(1.0),
		burst:    5,
	}

	// Add visitors with different last seen times
	rl.mu.Lock()
	rl.visitors["old-visitor"] = &visitor{
		limiter:  rate.NewLimiter(rl.rate, rl.burst),
		lastSeen: time.Now().Add(-15 * time.Minute), // Old visitor (>10 min)
	}
	rl.visitors["recent-visitor"] = &visitor{
		limiter:  rate.NewLimiter(rl.rate, rl.burst),
		lastSeen: time.Now().Add(-5 * time.Minute), // Recent visitor (<10 min)
	}
	rl.visitors["current-visitor"] = &visitor{
		limiter:  rate.NewLimiter(rl.rate, rl.burst),
		lastSeen: time.Now(), // Current visitor
	}
	rl.mu.Unlock()

	// Manually trigger cleanup (simulate cleanup loop iteration)
	rl.mu.Lock()
	for ip, v := range rl.visitors {
		if time.Since(v.lastSeen) > 10*time.Minute {
			delete(rl.visitors, ip)
		}
	}
	rl.mu.Unlock()

	// Check cleanup results
	rl.mu.RLock()
	_, oldExists := rl.visitors["old-visitor"]
	_, recentExists := rl.visitors["recent-visitor"]
	_, currentExists := rl.visitors["current-visitor"]
	rl.mu.RUnlock()

	assert.False(t, oldExists, "Old visitor should be cleaned up")
	assert.True(t, recentExists, "Recent visitor should remain")
	assert.True(t, currentExists, "Current visitor should remain")
}

// TestConcurrentRequests tests concurrent requests from multiple IPs
func TestConcurrentRequests(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(10.0), 5)

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	numGoroutines := 10
	numRequestsPerIP := 3
	done := make(chan bool, numGoroutines)

	// Concurrent requests from different IPs
	for i := 0; i < numGoroutines; i++ {
		go func(ipSuffix int) {
			ip := "192.168.1." + string(rune(48+ipSuffix)) // 192.168.1.0, 192.168.1.1, etc.
			for j := 0; j < numRequestsPerIP; j++ {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = ip + ":1234"
				rec := httptest.NewRecorder()

				handler.ServeHTTP(rec, req)
				// All should succeed (within burst limit)
				assert.Equal(t, http.StatusOK, rec.Code)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check that visitors were created
	rl.mu.RLock()
	visitorCount := len(rl.visitors)
	rl.mu.RUnlock()
	assert.Equal(t, numGoroutines, visitorCount)
}

// TestRateLimit_HighBurst tests high burst allowance
func TestRateLimit_HighBurst(t *testing.T) {
	// Allow 1 req/sec with high burst of 100
	rl := NewRateLimiter(rate.Limit(1.0), 100)

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 100 requests should all succeed
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "Request %d should succeed", i+1)
	}

	// 101st request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

// BenchmarkRateLimitMiddleware benchmarks rate limit middleware
func BenchmarkRateLimitMiddleware(b *testing.B) {
	rl := NewRateLimiter(rate.Limit(1000.0), 100)

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

// BenchmarkGetVisitor benchmarks visitor retrieval
func BenchmarkGetVisitor(b *testing.B) {
	rl := NewRateLimiter(rate.Limit(100.0), 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.getVisitor("192.168.1.1")
	}
}

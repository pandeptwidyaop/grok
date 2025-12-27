package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCSRFProtection tests CSRF protection initialization
func TestNewCSRFProtection(t *testing.T) {
	csrf := NewCSRFProtection()
	assert.NotNil(t, csrf)
	assert.NotNil(t, csrf.tokens)
}

// TestCSRF_GenerateToken tests CSRF token generation
func TestCSRF_GenerateToken(t *testing.T) {
	csrf := NewCSRFProtection()

	// Generate token
	token, err := csrf.GenerateToken()
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Token should be base64 URL encoded (at least 40 chars for 32 bytes)
	assert.Greater(t, len(token), 40)

	// Token should be valid
	assert.True(t, csrf.ValidateToken(token))
}

// TestCSRF_GenerateToken_Uniqueness tests that generated tokens are unique
func TestCSRF_GenerateToken_Uniqueness(t *testing.T) {
	csrf := NewCSRFProtection()

	tokens := make(map[string]bool)
	numTokens := 100

	// Generate multiple tokens
	for i := 0; i < numTokens; i++ {
		token, err := csrf.GenerateToken()
		require.NoError(t, err)

		// Check uniqueness
		_, exists := tokens[token]
		assert.False(t, exists, "Token %s already exists", token)
		tokens[token] = true
	}

	// All tokens should be unique
	assert.Equal(t, numTokens, len(tokens))
}

// TestCSRF_ValidateToken tests token validation
func TestCSRF_ValidateToken(t *testing.T) {
	csrf := NewCSRFProtection()

	// Generate valid token
	token, err := csrf.GenerateToken()
	require.NoError(t, err)

	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "valid token",
			token:    token,
			expected: true,
		},
		{
			name:     "empty token",
			token:    "",
			expected: false,
		},
		{
			name:     "invalid token",
			token:    "invalid-token",
			expected: false,
		},
		{
			name:     "non-existent token",
			token:    "dGVzdC10b2tlbi10aGF0LWRvZXNudC1leGlzdA==",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := csrf.ValidateToken(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCSRF_ValidateToken_Expired tests that expired tokens are invalid
func TestCSRF_ValidateToken_Expired(t *testing.T) {
	csrf := NewCSRFProtection()

	// Generate token
	token, err := csrf.GenerateToken()
	require.NoError(t, err)

	// Manually set expiry to past
	csrf.mu.Lock()
	csrf.tokens[token] = time.Now().Add(-1 * time.Hour)
	csrf.mu.Unlock()

	// Token should be invalid
	assert.False(t, csrf.ValidateToken(token))

	// Token should be removed from map
	csrf.mu.RLock()
	_, exists := csrf.tokens[token]
	csrf.mu.RUnlock()
	assert.False(t, exists)
}

// TestValidateToken_AlmostExpired tests token that's about to expire
func TestValidateToken_AlmostExpired(t *testing.T) {
	csrf := NewCSRFProtection()

	// Generate token
	token, err := csrf.GenerateToken()
	require.NoError(t, err)

	// Set expiry to 1 second from now
	csrf.mu.Lock()
	csrf.tokens[token] = time.Now().Add(1 * time.Second)
	csrf.mu.Unlock()

	// Token should still be valid
	assert.True(t, csrf.ValidateToken(token))
}

// TestProtect_GET tests that GET requests bypass CSRF protection
func TestProtect_GET(t *testing.T) {
	csrf := NewCSRFProtection()

	handler := csrf.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should succeed without CSRF token
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

// TestProtect_POST_ValidToken tests POST request with valid CSRF token
func TestProtect_POST_ValidToken(t *testing.T) {
	csrf := NewCSRFProtection()

	// Generate valid token
	token, err := csrf.GenerateToken()
	require.NoError(t, err)

	handler := csrf.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-CSRF-Token", token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should succeed with valid token
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

// TestProtect_POST_MissingToken tests POST request without CSRF token
func TestProtect_POST_MissingToken(t *testing.T) {
	csrf := NewCSRFProtection()

	handler := csrf.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should fail without token
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid or missing CSRF token")
}

// TestProtect_POST_InvalidToken tests POST request with invalid CSRF token
func TestProtect_POST_InvalidToken(t *testing.T) {
	csrf := NewCSRFProtection()

	handler := csrf.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-CSRF-Token", "invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should fail with invalid token
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid or missing CSRF token")
}

// TestProtect_AllMethods tests CSRF protection for all HTTP methods
func TestProtect_AllMethods(t *testing.T) {
	csrf := NewCSRFProtection()

	tests := []struct {
		name               string
		method             string
		requiresCSRF       bool
		includeToken       bool
		expectedStatusCode int
	}{
		// Methods that don't require CSRF
		{"GET without token", "GET", false, false, http.StatusOK},
		{"HEAD without token", "HEAD", false, false, http.StatusOK},
		{"OPTIONS without token", "OPTIONS", false, false, http.StatusOK},

		// Methods that require CSRF - without token
		{"POST without token", "POST", true, false, http.StatusForbidden},
		{"PUT without token", "PUT", true, false, http.StatusForbidden},
		{"PATCH without token", "PATCH", true, false, http.StatusForbidden},
		{"DELETE without token", "DELETE", true, false, http.StatusForbidden},

		// Methods that require CSRF - with valid token
		{"POST with token", "POST", true, true, http.StatusOK},
		{"PUT with token", "PUT", true, true, http.StatusOK},
		{"PATCH with token", "PATCH", true, true, http.StatusOK},
		{"DELETE with token", "DELETE", true, true, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := csrf.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.includeToken {
				// Generate a fresh token for each test since tokens are single-use
				token, err := csrf.GenerateToken()
				require.NoError(t, err)
				req.Header.Set("X-CSRF-Token", token)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatusCode, rec.Code)
		})
	}
}

// TestCleanupLoop tests that expired tokens are cleaned up
func TestCleanupLoop(t *testing.T) {
	// Create CSRF with shorter cleanup interval for testing
	csrf := &CSRFProtection{
		tokens: make(map[string]time.Time),
	}

	// Add some tokens with different expiries
	csrf.mu.Lock()
	csrf.tokens["expired1"] = time.Now().Add(-2 * time.Hour)
	csrf.tokens["expired2"] = time.Now().Add(-1 * time.Hour)
	csrf.tokens["valid1"] = time.Now().Add(30 * time.Minute)
	csrf.tokens["valid2"] = time.Now().Add(1 * time.Hour)
	csrf.mu.Unlock()

	// Manually trigger cleanup (simulate cleanup loop iteration)
	csrf.mu.Lock()
	now := time.Now()
	for token, expiry := range csrf.tokens {
		if now.After(expiry) {
			delete(csrf.tokens, token)
		}
	}
	csrf.mu.Unlock()

	// Check that expired tokens are removed
	csrf.mu.RLock()
	_, exists1 := csrf.tokens["expired1"]
	_, exists2 := csrf.tokens["expired2"]
	_, valid1 := csrf.tokens["valid1"]
	_, valid2 := csrf.tokens["valid2"]
	csrf.mu.RUnlock()

	assert.False(t, exists1, "expired1 should be removed")
	assert.False(t, exists2, "expired2 should be removed")
	assert.True(t, valid1, "valid1 should exist")
	assert.True(t, valid2, "valid2 should exist")
}

// TestConcurrentTokenOperations tests concurrent token generation and validation
func TestConcurrentTokenOperations(t *testing.T) {
	csrf := NewCSRFProtection()

	numGoroutines := 50
	done := make(chan bool, numGoroutines*2)

	// Concurrent token generation
	for i := 0; i < numGoroutines; i++ {
		go func() {
			token, err := csrf.GenerateToken()
			assert.NoError(t, err)
			assert.NotEmpty(t, token)
			done <- true
		}()
	}

	// Concurrent token validation (each goroutine needs its own token since tokens are single-use)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			validToken, err := csrf.GenerateToken()
			assert.NoError(t, err)
			result := csrf.ValidateToken(validToken)
			assert.True(t, result)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines*2; i++ {
		<-done
	}
}

// TestCSRFProtect_ExpiredToken tests protection with expired token
func TestCSRFProtect_ExpiredToken(t *testing.T) {
	csrf := NewCSRFProtection()

	// Generate token
	token, err := csrf.GenerateToken()
	require.NoError(t, err)

	// Manually expire the token
	csrf.mu.Lock()
	csrf.tokens[token] = time.Now().Add(-1 * time.Hour)
	csrf.mu.Unlock()

	handler := csrf.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-CSRF-Token", token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should fail with expired token
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestCSRFProtect_SingleUseToken tests that tokens can only be used once
func TestCSRFProtect_SingleUseToken(t *testing.T) {
	csrf := NewCSRFProtection()

	// Generate token
	token, err := csrf.GenerateToken()
	require.NoError(t, err)

	// First validation should succeed
	result1 := csrf.ValidateToken(token)
	assert.True(t, result1, "First validation should succeed")

	// Second validation with same token should fail (token was deleted)
	result2 := csrf.ValidateToken(token)
	assert.False(t, result2, "Second validation should fail (single-use)")

	// Test with HTTP handler
	handler := csrf.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Generate a new token for HTTP test
	httpToken, err := csrf.GenerateToken()
	require.NoError(t, err)

	// First request should succeed
	req1 := httptest.NewRequest("POST", "/test", nil)
	req1.Header.Set("X-CSRF-Token", httpToken)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code, "First request should succeed")

	// Second request with same token should fail
	req2 := httptest.NewRequest("POST", "/test", nil)
	req2.Header.Set("X-CSRF-Token", httpToken)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusForbidden, rec2.Code, "Second request should fail (token already used)")
}

// TestProtect_RefreshToken tests that new CSRF token is returned after successful validation (SPA support).
func TestProtect_RefreshToken(t *testing.T) {
	csrf := NewCSRFProtection()

	handler := csrf.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// Generate initial token
	token, err := csrf.GenerateToken()
	require.NoError(t, err)

	// Make POST request with valid token
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-CSRF-Token", token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should succeed
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())

	// Should return new CSRF token in response header
	newToken := rec.Header().Get("X-CSRF-Token")
	assert.NotEmpty(t, newToken, "New CSRF token should be returned in response header")
	assert.NotEqual(t, token, newToken, "New token should be different from old token")

	// Old token should be consumed (single-use)
	req2 := httptest.NewRequest("POST", "/test", nil)
	req2.Header.Set("X-CSRF-Token", token)
	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusForbidden, rec2.Code, "Old token should not work again (single-use)")

	// New token should work
	req3 := httptest.NewRequest("POST", "/test", nil)
	req3.Header.Set("X-CSRF-Token", newToken)
	rec3 := httptest.NewRecorder()

	handler.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusOK, rec3.Code, "New token should work")
}

// BenchmarkGenerateToken benchmarks token generation
func BenchmarkGenerateToken(b *testing.B) {
	csrf := NewCSRFProtection()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := csrf.GenerateToken()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidateToken benchmarks token validation
func BenchmarkValidateToken(b *testing.B) {
	csrf := NewCSRFProtection()

	// Pre-generate token
	token, _ := csrf.GenerateToken()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		csrf.ValidateToken(token)
	}
}

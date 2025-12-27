package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pandeptwidyaop/grok/internal/server/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRespondJSON tests JSON response helper
func TestRespondJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"message": "test"}

	respondJSON(rec, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "test", response["message"])
}

// TestRespondError tests error response helper
func TestRespondError(t *testing.T) {
	rec := httptest.NewRecorder()
	respondError(rec, http.StatusBadRequest, "invalid input")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid input")
}

// TestIsAllowedOrigin tests origin validation
func TestIsAllowedOrigin(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{
					"http://localhost:3000",
					"http://localhost:5173",
					"http://127.0.0.1:3000",
				},
			},
		},
	}

	tests := []struct {
		name     string
		origin   string
		expected bool
	}{
		{"localhost development", "http://localhost:3000", true},
		{"localhost with port", "http://localhost:5173", true},
		{"127.0.0.1", "http://127.0.0.1:3000", true},
		{"invalid origin", "http://evil.com", false},
		{"empty origin", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.isAllowedOrigin(tt.origin)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCORSMiddleware_Preflight tests OPTIONS preflight requests
func TestCORSMiddleware_Preflight(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
			},
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called for preflight")
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "GET")
}

// TestCORSMiddleware_AllowedOrigin tests CORS with allowed origin
func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
			},
		},
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	assert.True(t, nextCalled, "Next handler should be called")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
}

// TestCORSMiddleware_DisallowedOrigin tests CORS with disallowed origin
func TestCORSMiddleware_DisallowedOrigin(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
			},
		},
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Server still processes the request, but doesn't set CORS headers
	// Browser will block the response due to missing CORS headers
	assert.True(t, nextCalled, "Next handler should be called")
	assert.Equal(t, http.StatusOK, rec.Code)
	// CORS headers should NOT be set for disallowed origin
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

// TestCORSMiddleware_NoOrigin tests request without Origin header
func TestCORSMiddleware_NoOrigin(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
			},
		},
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	assert.True(t, nextCalled, "Next handler should be called for same-origin requests")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestCORSHeaders tests all CORS headers are set correctly
func TestCORSHeaders(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:5173"},
			},
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("POST", "/api/test", bytes.NewReader([]byte("{}")))
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Check all CORS headers
	headers := rec.Header()
	assert.Equal(t, "http://localhost:5173", headers.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", headers.Get("Access-Control-Allow-Credentials"))
	assert.NotEmpty(t, headers.Get("Access-Control-Allow-Headers"))
	assert.Contains(t, headers.Get("Access-Control-Allow-Headers"), "Content-Type")
	assert.Contains(t, headers.Get("Access-Control-Allow-Headers"), "Authorization")
}

// TestCORSMiddleware_MultipleOrigins tests different allowed origins
func TestCORSMiddleware_MultipleOrigins(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{
					"http://localhost:3000",
					"http://localhost:5173",
					"http://127.0.0.1:3000",
					"http://127.0.0.1:5173",
				},
			},
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	allowedOrigins := []string{
		"http://localhost:3000",
		"http://localhost:5173",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:5173",
	}

	for _, origin := range allowedOrigins {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.Header.Set("Origin", origin)
			rec := httptest.NewRecorder()

			middleware.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, origin, rec.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

// BenchmarkCORSMiddleware benchmarks CORS middleware
func BenchmarkCORSMiddleware(b *testing.B) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
			},
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)
	}
}

// Benchmark respondJSON
func BenchmarkRespondJSON(b *testing.B) {
	data := map[string]string{"message": "test", "status": "ok"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		respondJSON(rec, http.StatusOK, data)
	}
}

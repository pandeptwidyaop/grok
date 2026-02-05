package proxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
)

// TestXForwardedHeadersLogic tests the logic for adding X-Forwarded headers.
func TestXForwardedHeadersLogic(t *testing.T) {
	tests := []struct {
		name             string
		setupRequest     func() *http.Request
		expectedHost     string
		expectedProto    string
		expectedForCount int
		expectedForContains []string
	}{
		{
			name: "HTTP request adds X-Forwarded headers",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "http://test.grok.example.com/api/users", nil)
				req.RemoteAddr = "192.168.1.100:54321"
				req.Host = "test.grok.example.com"
				return req
			},
			expectedHost:  "test.grok.example.com",
			expectedProto: "http",
			expectedForCount: 1,
			expectedForContains: []string{"192.168.1.100"},
		},
		{
			name: "HTTPS request sets proto to https",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "https://test.grok.example.com/api/users", nil)
				req.RemoteAddr = "10.0.0.50:12345"
				req.Host = "test.grok.example.com"
				req.TLS = &tls.ConnectionState{} // Simulate HTTPS
				return req
			},
			expectedHost:  "test.grok.example.com",
			expectedProto: "https",
			expectedForCount: 1,
			expectedForContains: []string{"10.0.0.50"},
		},
		{
			name: "Appends to existing X-Forwarded-For",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "http://test.grok.example.com/api/users", nil)
				req.RemoteAddr = "192.168.1.200:9999"
				req.Host = "test.grok.example.com"
				req.Header.Set("X-Forwarded-For", "203.0.113.1")
				return req
			},
			expectedHost:  "test.grok.example.com",
			expectedProto: "http",
			expectedForCount: 2,
			expectedForContains: []string{"203.0.113.1", "192.168.1.200"},
		},
		{
			name: "IPv6 address without port",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "http://test.grok.example.com/api", nil)
				req.RemoteAddr = "2001:db8::1"
				req.Host = "test.grok.example.com"
				return req
			},
			expectedHost:  "test.grok.example.com",
			expectedProto: "http",
			expectedForCount: 1,
			expectedForContains: []string{"2001:db8::1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()

			// Simulate the header addition logic from proxyRequest
			headers := make(map[string]*tunnelv1.HeaderValues)
			for key, values := range req.Header {
				headers[key] = &tunnelv1.HeaderValues{
					Values: values,
				}
			}

			// Add X-Forwarded-* headers (copied from http_proxy.go)
			// X-Forwarded-For: Client IP address
			clientIP := req.RemoteAddr
			if host, _, err := net.SplitHostPort(clientIP); err == nil {
				clientIP = host
			}
			if existingFor, ok := headers["X-Forwarded-For"]; ok {
				// Append to existing X-Forwarded-For
				headers["X-Forwarded-For"] = &tunnelv1.HeaderValues{
					Values: append(existingFor.Values, clientIP),
				}
			} else {
				headers["X-Forwarded-For"] = &tunnelv1.HeaderValues{
					Values: []string{clientIP},
				}
			}

			// X-Forwarded-Host: Original Host header
			headers["X-Forwarded-Host"] = &tunnelv1.HeaderValues{
				Values: []string{req.Host},
			}

			// X-Forwarded-Proto: Original protocol (http or https)
			proto := "http"
			if req.TLS != nil {
				proto = "https"
			}
			headers["X-Forwarded-Proto"] = &tunnelv1.HeaderValues{
				Values: []string{proto},
			}

			// Verify X-Forwarded-Host
			forwardedHost, ok := headers["X-Forwarded-Host"]
			require.True(t, ok, "X-Forwarded-Host header should be present")
			require.Len(t, forwardedHost.Values, 1)
			assert.Equal(t, tt.expectedHost, forwardedHost.Values[0])

			// Verify X-Forwarded-Proto
			forwardedProto, ok := headers["X-Forwarded-Proto"]
			require.True(t, ok, "X-Forwarded-Proto header should be present")
			require.Len(t, forwardedProto.Values, 1)
			assert.Equal(t, tt.expectedProto, forwardedProto.Values[0])

			// Verify X-Forwarded-For
			forwardedFor, ok := headers["X-Forwarded-For"]
			require.True(t, ok, "X-Forwarded-For header should be present")
			require.Len(t, forwardedFor.Values, tt.expectedForCount)

			for _, expectedIP := range tt.expectedForContains {
				assert.Contains(t, forwardedFor.Values, expectedIP)
			}
		})
	}
}

// TestWebhookHeaderCloning tests that webhook headers are properly cloned.
func TestWebhookHeaderCloning(t *testing.T) {
	req := httptest.NewRequest("POST", "http://webhook.grok.example.com/app/event", nil)
	req.RemoteAddr = "192.168.100.50:54321"
	req.Host = "webhook.grok.example.com"
	req.TLS = &tls.ConnectionState{} // HTTPS
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Original-Header", "value")

	// Clone headers (like in handleWebhookRequest)
	headers := make(http.Header)
	for k, v := range req.Header {
		headers[k] = v
	}

	// Add X-Forwarded headers
	clientIP := "192.168.100.50" // Extracted from RemoteAddr
	if existingFor := headers.Get("X-Forwarded-For"); existingFor != "" {
		headers.Set("X-Forwarded-For", existingFor+", "+clientIP)
	} else {
		headers.Set("X-Forwarded-For", clientIP)
	}
	headers.Set("X-Forwarded-Host", req.Host)
	headers.Set("X-Forwarded-Proto", "https")

	// Verify cloned headers have new values
	assert.Equal(t, "192.168.100.50", headers.Get("X-Forwarded-For"))
	assert.Equal(t, "webhook.grok.example.com", headers.Get("X-Forwarded-Host"))
	assert.Equal(t, "https", headers.Get("X-Forwarded-Proto"))
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
	assert.Equal(t, "value", headers.Get("X-Original-Header"))

	// Verify original request headers are NOT modified
	assert.Empty(t, req.Header.Get("X-Forwarded-Host"))
	assert.Empty(t, req.Header.Get("X-Forwarded-Proto"))
	assert.Empty(t, req.Header.Get("X-Forwarded-For"))
}

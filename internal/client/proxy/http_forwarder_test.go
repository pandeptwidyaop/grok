package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/client/config"
)

// createTestForwarder creates a forwarder with default test config.
func createTestForwarder(addr string) *HTTPForwarder {
	cfg := config.PerformanceConfig{}
	cfg.ConnectionPool.Enabled = true
	cfg.ConnectionPool.MinSize = 1
	cfg.ConnectionPool.MaxSize = 10
	cfg.ConnectionPool.IdleTimeout = 90 * time.Second
	cfg.ConnectionPool.HealthCheckInterval = 30 * time.Second

	// Create adaptive buffer pool config
	cfg.BufferPool.Enabled = true

	return NewHTTPForwarder(addr, cfg)
}

// TestNewHTTPForwarder tests HTTP forwarder creation.
func TestNewHTTPForwarder(t *testing.T) {
	forwarder := createTestForwarder("localhost:3000")

	require.NotNil(t, forwarder)
	assert.Equal(t, "localhost:3000", forwarder.localAddr)
	assert.NotNil(t, forwarder.httpClient)
	assert.Equal(t, time.Duration(0), forwarder.httpClient.Timeout) // No timeout for chunked transfers
}

// TestHTTPForwarder_Forward_SimpleGET tests forwarding a simple GET request.
func TestHTTPForwarder_Forward_SimpleGET(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/test", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	// Extract host:port from server URL
	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	// Create gRPC request
	req := &tunnelv1.HTTPRequest{
		Method:  "GET",
		Path:    "/test",
		Headers: make(map[string]*tunnelv1.HeaderValues),
	}

	// Forward request
	resp, err := forwarder.Forward(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(200), resp.StatusCode)
	assert.Equal(t, "Hello, World!", string(resp.Body))
}

// TestHTTPForwarder_Forward_POST tests forwarding a POST request with body.
func TestHTTPForwarder_Forward_POST(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/data", r.URL.Path)

		// Read body
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		assert.Equal(t, `{"key":"value"}`, string(body[:n]))

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"created"}`))
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:  "POST",
		Path:    "/api/data",
		Headers: make(map[string]*tunnelv1.HeaderValues),
		Body:    []byte(`{"key":"value"}`),
	}

	resp, err := forwarder.Forward(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, int32(201), resp.StatusCode)
	assert.Equal(t, `{"status":"created"}`, string(resp.Body))
}

// TestHTTPForwarder_Forward_QueryString tests forwarding with query string.
func TestHTTPForwarder_Forward_QueryString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, "test", r.URL.Query().Get("q"))
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:      "GET",
		Path:        "/search",
		QueryString: "q=test&limit=10",
		Headers:     make(map[string]*tunnelv1.HeaderValues),
	}

	resp, err := forwarder.Forward(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, int32(200), resp.StatusCode)
}

// TestHTTPForwarder_Forward_Headers tests forwarding with custom headers.
func TestHTTPForwarder_Forward_Headers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom-Header"))

		w.Header().Set("X-Response-Header", "response-value")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method: "GET",
		Path:   "/test",
		Headers: map[string]*tunnelv1.HeaderValues{
			"Content-Type":    {Values: []string{"application/json"}},
			"Authorization":   {Values: []string{"Bearer token123"}},
			"X-Custom-Header": {Values: []string{"custom-value"}},
		},
	}

	resp, err := forwarder.Forward(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, int32(200), resp.StatusCode)
	assert.NotNil(t, resp.Headers["X-Response-Header"])
	assert.Equal(t, "response-value", resp.Headers["X-Response-Header"].Values[0])
}

// TestHTTPForwarder_Forward_XForwardedFor tests X-Forwarded-For header injection.
func TestHTTPForwarder_Forward_XForwardedFor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "192.168.1.1:12345", r.Header.Get("X-Forwarded-For"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:     "GET",
		Path:       "/test",
		Headers:    make(map[string]*tunnelv1.HeaderValues),
		RemoteAddr: "192.168.1.1:12345",
	}

	resp, err := forwarder.Forward(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, int32(200), resp.StatusCode)
}

// TestHTTPForwarder_Forward_HostHeader tests Host header override.
func TestHTTPForwarder_Forward_HostHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "example.com", r.Host)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method: "GET",
		Path:   "/test",
		Headers: map[string]*tunnelv1.HeaderValues{
			"Host": {Values: []string{"example.com"}},
		},
	}

	resp, err := forwarder.Forward(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, int32(200), resp.StatusCode)
}

// TestHTTPForwarder_Forward_4xxError tests forwarding 4xx error responses.
func TestHTTPForwarder_Forward_4xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:  "GET",
		Path:    "/notfound",
		Headers: make(map[string]*tunnelv1.HeaderValues),
	}

	resp, err := forwarder.Forward(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, int32(404), resp.StatusCode)
	assert.Equal(t, "Not Found", string(resp.Body))
}

// TestHTTPForwarder_Forward_5xxError tests forwarding 5xx error responses.
func TestHTTPForwarder_Forward_5xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:  "GET",
		Path:    "/error",
		Headers: make(map[string]*tunnelv1.HeaderValues),
	}

	resp, err := forwarder.Forward(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, int32(500), resp.StatusCode)
	assert.Equal(t, "Internal Server Error", string(resp.Body))
}

// TestHTTPForwarder_Forward_Redirect tests that redirects are not followed.
func TestHTTPForwarder_Forward_Redirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/redirected", http.StatusFound)
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:  "GET",
		Path:    "/redirect",
		Headers: make(map[string]*tunnelv1.HeaderValues),
	}

	resp, err := forwarder.Forward(context.Background(), req)

	require.NoError(t, err)
	// Should return 302, not follow to final destination
	assert.Equal(t, int32(302), resp.StatusCode)
	assert.NotNil(t, resp.Headers["Location"])
	assert.Equal(t, "/redirected", resp.Headers["Location"].Values[0])
}

// TestHTTPForwarder_Forward_InvalidLocalAddr tests error when local service unreachable.
func TestHTTPForwarder_Forward_InvalidLocalAddr(t *testing.T) {
	// Use an invalid address that will fail to connect
	forwarder := createTestForwarder("localhost:99999")

	req := &tunnelv1.HTTPRequest{
		Method:  "GET",
		Path:    "/test",
		Headers: make(map[string]*tunnelv1.HeaderValues),
	}

	resp, err := forwarder.Forward(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to execute HTTP request")
}

// TestHTTPForwarder_Forward_ContextCanceled tests context cancellation.
func TestHTTPForwarder_Forward_ContextCanceled(t *testing.T) {
	// Create server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:  "GET",
		Path:    "/slow",
		Headers: make(map[string]*tunnelv1.HeaderValues),
	}

	// Create context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := forwarder.Forward(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

// TestHTTPForwarder_ForwardChunked tests chunked response streaming.
func TestHTTPForwarder_ForwardChunked(t *testing.T) {
	// Create large response (larger than ChunkSize)
	largeBody := strings.Repeat("A", ChunkSize+1000)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeBody))
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:  "GET",
		Path:    "/large",
		Headers: make(map[string]*tunnelv1.HeaderValues),
	}

	// Collect chunks
	var chunks []*tunnelv1.HTTPResponse
	var receivedData []byte

	err := forwarder.ForwardChunked(context.Background(), req, func(resp *tunnelv1.HTTPResponse, _ bool) error {
		chunks = append(chunks, resp)
		receivedData = append(receivedData, resp.Body...)
		return nil
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(chunks), 2) // Should have at least 2 chunks
	assert.Equal(t, largeBody, string(receivedData))

	// First chunk should have headers
	assert.NotNil(t, chunks[0].Headers)

	// Subsequent chunks should not have headers
	if len(chunks) > 1 {
		assert.Nil(t, chunks[1].Headers)
	}
}

// TestHTTPForwarder_ForwardChunked_SmallResponse tests chunked with small response.
func TestHTTPForwarder_ForwardChunked_SmallResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Small response"))
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:  "GET",
		Path:    "/small",
		Headers: make(map[string]*tunnelv1.HeaderValues),
	}

	var chunks []*tunnelv1.HTTPResponse
	err := forwarder.ForwardChunked(context.Background(), req, func(resp *tunnelv1.HTTPResponse, _ bool) error {
		chunks = append(chunks, resp)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, len(chunks)) // Should have exactly 1 chunk
	assert.Equal(t, "Small response", string(chunks[0].Body))
}

// TestHTTPForwarder_ForwardChunked_SendError tests chunk send callback error.
func TestHTTPForwarder_ForwardChunked_SendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Test response"))
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:  "GET",
		Path:    "/test",
		Headers: make(map[string]*tunnelv1.HeaderValues),
	}

	// Callback returns error
	err := forwarder.ForwardChunked(context.Background(), req, func(_ *tunnelv1.HTTPResponse, _ bool) error {
		return fmt.Errorf("send error")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send chunk")
}

// TestIsWebSocketUpgrade tests WebSocket upgrade detection.
func TestIsWebSocketUpgrade(t *testing.T) {
	tests := []struct {
		name     string
		req      *tunnelv1.HTTPRequest
		expected bool
	}{
		{
			name: "valid WebSocket upgrade",
			req: &tunnelv1.HTTPRequest{
				Headers: map[string]*tunnelv1.HeaderValues{
					"Upgrade":    {Values: []string{"websocket"}},
					"Connection": {Values: []string{"Upgrade"}},
				},
			},
			expected: true,
		},
		{
			name: "case insensitive WebSocket",
			req: &tunnelv1.HTTPRequest{
				Headers: map[string]*tunnelv1.HeaderValues{
					"Upgrade":    {Values: []string{"WebSocket"}},
					"Connection": {Values: []string{"upgrade"}},
				},
			},
			expected: true,
		},
		{
			name: "connection keep-alive, upgrade",
			req: &tunnelv1.HTTPRequest{
				Headers: map[string]*tunnelv1.HeaderValues{
					"Upgrade":    {Values: []string{"websocket"}},
					"Connection": {Values: []string{"keep-alive, Upgrade"}},
				},
			},
			expected: true,
		},
		{
			name: "missing Upgrade header",
			req: &tunnelv1.HTTPRequest{
				Headers: map[string]*tunnelv1.HeaderValues{
					"Connection": {Values: []string{"Upgrade"}},
				},
			},
			expected: false,
		},
		{
			name: "missing Connection header",
			req: &tunnelv1.HTTPRequest{
				Headers: map[string]*tunnelv1.HeaderValues{
					"Upgrade": {Values: []string{"websocket"}},
				},
			},
			expected: false,
		},
		{
			name: "wrong upgrade value",
			req: &tunnelv1.HTTPRequest{
				Headers: map[string]*tunnelv1.HeaderValues{
					"Upgrade":    {Values: []string{"h2c"}},
					"Connection": {Values: []string{"Upgrade"}},
				},
			},
			expected: false,
		},
		{
			name:     "nil headers",
			req:      &tunnelv1.HTTPRequest{Headers: nil},
			expected: false,
		},
		{
			name: "empty header values",
			req: &tunnelv1.HTTPRequest{
				Headers: map[string]*tunnelv1.HeaderValues{
					"Upgrade":    {Values: []string{}},
					"Connection": {Values: []string{}},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsWebSocketUpgrade(tt.req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHTTPForwarder_ForwardWebSocketUpgrade tests WebSocket upgrade handling.
func TestHTTPForwarder_ForwardWebSocketUpgrade(t *testing.T) {
	// Create a mock server using httptest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack failed: %v", err)
			return
		}
		defer conn.Close()

		// Send WebSocket upgrade response
		response := "HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\n" +
			"Connection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=\r\n" +
			"\r\n"
		buf.WriteString(response)
		buf.Flush()

		// Keep connection open implies successful upgrade
		// We wait for client to close or test to end
		// But in this test, the client closes 'conn' at the end of function.
		// We can just return here? No, if we return, Hijack connection closes?
		// Usually yes.
		// Let's block a bit or read until EOF
		io.Copy(io.Discard, conn)
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method: "GET",
		Path:   "/ws",
		Headers: map[string]*tunnelv1.HeaderValues{
			"Upgrade":               {Values: []string{"websocket"}},
			"Connection":            {Values: []string{"Upgrade"}},
			"Sec-WebSocket-Key":     {Values: []string{"dGhlIHNhbXBsZSBub25jZQ=="}},
			"Sec-WebSocket-Version": {Values: []string{"13"}},
			"Host":                  {Values: []string{"localhost"}},
		},
	}

	resp, conn, err := forwarder.ForwardWebSocketUpgrade(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, conn)
	defer conn.Close()

	assert.Equal(t, int32(101), resp.StatusCode)
	if resp.Headers["Upgrade"] != nil {
		assert.Equal(t, "websocket", resp.Headers["Upgrade"].Values[0])
	}
	if resp.Headers["Connection"] != nil {
		assert.Equal(t, "Upgrade", resp.Headers["Connection"].Values[0])
	}
}

// TestHTTPForwarder_ForwardWebSocketUpgrade_ConnectError tests connection error.
func TestHTTPForwarder_ForwardWebSocketUpgrade_ConnectError(t *testing.T) {
	forwarder := createTestForwarder("localhost:99999")

	req := &tunnelv1.HTTPRequest{
		Method: "GET",
		Path:   "/ws",
		Headers: map[string]*tunnelv1.HeaderValues{
			"Upgrade":    {Values: []string{"websocket"}},
			"Connection": {Values: []string{"Upgrade"}},
		},
	}

	resp, conn, err := forwarder.ForwardWebSocketUpgrade(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to connect to local service")
}

// BenchmarkHTTPForwarder_Forward benchmarks HTTP forwarding.
func BenchmarkHTTPForwarder_Forward(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:  "GET",
		Path:    "/test",
		Headers: make(map[string]*tunnelv1.HeaderValues),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		forwarder.Forward(context.Background(), req)
	}
}

// BenchmarkHTTPForwarder_ForwardChunked benchmarks chunked forwarding.
func BenchmarkHTTPForwarder_ForwardChunked(b *testing.B) {
	largeBody := strings.Repeat("A", ChunkSize+1000)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeBody))
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "http://")
	forwarder := createTestForwarder(serverAddr)

	req := &tunnelv1.HTTPRequest{
		Method:  "GET",
		Path:    "/large",
		Headers: make(map[string]*tunnelv1.HeaderValues),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		forwarder.ForwardChunked(context.Background(), req, func(_ *tunnelv1.HTTPResponse, _ bool) error {
			return nil
		})
	}
}

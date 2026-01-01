package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/client/config"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/pool"
)

// HTTPForwarder forwards HTTP requests to local service.
type HTTPForwarder struct {
	localAddr  string
	httpClient *http.Client // No timeout to support large file downloads via chunked transfer
	connPool   *pool.ConnectionPool
	bufferPool *pool.AdaptiveBufferPool
}

// NewHTTPForwarder creates a new HTTP forwarder.
func NewHTTPForwarder(localAddr string, cfg config.PerformanceConfig) *HTTPForwarder {
	// Create connection pool if enabled
	var connPool *pool.ConnectionPool
	var err error

	if cfg.ConnectionPool.Enabled {
		connPool, err = pool.NewConnectionPool(pool.Config{
			MinSize:             cfg.ConnectionPool.MinSize,
			MaxSize:             cfg.ConnectionPool.MaxSize,
			IdleTimeout:         cfg.ConnectionPool.IdleTimeout,
			HealthCheckInterval: cfg.ConnectionPool.HealthCheckInterval,
			MaxWaitTime:         5 * time.Second,
			Factory: func() (net.Conn, error) {
				return net.DialTimeout("tcp", localAddr, 10*time.Second)
			},
		})
	}

	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("local_addr", localAddr).
			Msg("Failed to create connection pool, falling back to default transport")
		// Connect pool will be nil, fallback logic below handles it
	}

	// Create custom transport
	transport := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     200,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		WriteBufferSize:     32 * 1024,
		ReadBufferSize:      32 * 1024,
	}

	// Use connection pool for dialing if available
	if connPool != nil {
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return connPool.Get(ctx)
		}

		logger.InfoEvent().
			Str("local_addr", localAddr).
			Int("pool_min", cfg.ConnectionPool.MinSize).
			Int("pool_max", cfg.ConnectionPool.MaxSize).
			Msg("HTTP forwarder initialized with connection pool")
	} else {
		logger.InfoEvent().
			Str("local_addr", localAddr).
			Msg("HTTP forwarder initialized with standard transport (no pool)")
	}

	// Create adaptive buffer pool if enabled
	// Note: Currently AdaptiveBufferPool doesn't support configuration in constructor,
	// but we could extend it. For now using defaults.
	// If needed we can add config support to NewAdaptiveBufferPool later.
	bufferPool := pool.NewAdaptiveBufferPool()

	return &HTTPForwarder{
		localAddr: localAddr,
		httpClient: &http.Client{
			Timeout:   0,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		connPool:   connPool,
		bufferPool: bufferPool,
	}
}

const (
	// ChunkSize is the size of each chunk for streaming large responses (4MB).
	ChunkSize = 4 * 1024 * 1024
)

// Forward forwards a gRPC HTTP request to local service.
// For small responses, returns complete response. For large responses, this is deprecated - use ForwardChunked.
func (f *HTTPForwarder) Forward(ctx context.Context, req *tunnelv1.HTTPRequest) (*tunnelv1.HTTPResponse, error) {
	// Build URL using strings.Builder to reduce allocations
	var urlBuilder strings.Builder
	urlBuilder.Grow(len("http://") + len(f.localAddr) + len(req.Path) + 1 + len(req.QueryString))
	urlBuilder.WriteString("http://")
	urlBuilder.WriteString(f.localAddr)
	urlBuilder.WriteString(req.Path)
	if req.QueryString != "" {
		urlBuilder.WriteByte('?')
		urlBuilder.WriteString(req.QueryString)
	}
	url := urlBuilder.String()

	logger.DebugEvent().
		Str("method", req.Method).
		Str("url", url).
		Msg("Forwarding HTTP request to local service")

	// Create HTTP request
	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for name, headerVals := range req.Headers {
		for _, val := range headerVals.Values {
			httpReq.Header.Add(name, val)
		}
	}

	// Override Host header if present
	if host := httpReq.Header.Get("Host"); host != "" {
		httpReq.Host = host
	}

	// Set X-Forwarded headers
	if req.RemoteAddr != "" {
		httpReq.Header.Set("X-Forwarded-For", req.RemoteAddr)
	}

	// Execute request
	httpResp, err := f.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Convert response headers
	headers := make(map[string]*tunnelv1.HeaderValues, len(httpResp.Header))
	for name, values := range httpResp.Header {
		headers[name] = &tunnelv1.HeaderValues{
			Values: values,
		}
	}

	logger.DebugEvent().
		Int("status", httpResp.StatusCode).
		Int("body_size", len(respBody)).
		Msg("Received response from local service")

	// Build gRPC HTTP response
	// StatusCode conversion is safe: HTTP status codes are 100-599, well within int32 range
	return &tunnelv1.HTTPResponse{
		StatusCode: int32(httpResp.StatusCode), //nolint:gosec // Safe conversion: HTTP status codes are always 100-599
		Headers:    headers,
		Body:       respBody,
	}, nil
}

// ForwardChunked forwards HTTP request and sends response in chunks via callback.
// This is memory-efficient for large responses as it streams data in 4MB chunks.
// The sendChunk callback is called for each chunk with (response, isLastChunk).
func (f *HTTPForwarder) ForwardChunked(ctx context.Context, req *tunnelv1.HTTPRequest, sendChunk func(*tunnelv1.HTTPResponse, bool) error) error {
	// Build URL using strings.Builder to reduce allocations
	var urlBuilder strings.Builder
	urlBuilder.Grow(len("http://") + len(f.localAddr) + len(req.Path) + 1 + len(req.QueryString))
	urlBuilder.WriteString("http://")
	urlBuilder.WriteString(f.localAddr)
	urlBuilder.WriteString(req.Path)
	if req.QueryString != "" {
		urlBuilder.WriteByte('?')
		urlBuilder.WriteString(req.QueryString)
	}
	url := urlBuilder.String()

	logger.DebugEvent().
		Str("method", req.Method).
		Str("url", url).
		Msg("Forwarding HTTP request to local service (chunked)")

	// Create HTTP request
	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for name, headerVals := range req.Headers {
		for _, val := range headerVals.Values {
			httpReq.Header.Add(name, val)
		}
	}

	// Override Host header if present
	if host := httpReq.Header.Get("Host"); host != "" {
		httpReq.Host = host
	}

	// Set X-Forwarded headers
	if req.RemoteAddr != "" {
		httpReq.Header.Set("X-Forwarded-For", req.RemoteAddr)
	}

	// Execute request (no timeout for large file downloads)
	httpResp, err := f.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer httpResp.Body.Close()

	// Convert response headers
	headers := make(map[string]*tunnelv1.HeaderValues, len(httpResp.Header))
	for name, values := range httpResp.Header {
		headers[name] = &tunnelv1.HeaderValues{
			Values: values,
		}
	}

	logger.InfoEvent().
		Int("status", httpResp.StatusCode).
		Int64("content_length", httpResp.ContentLength).
		Msg("Received response from local service, streaming in chunks")

	// Get adaptive buffer based on Content-Length
	// Validate Content-Length to prevent DoS attacks
	const MaxBufferSize = 100 * 1024 * 1024 // 100MB limit
	estimatedSize := int(httpResp.ContentLength)
	if estimatedSize < 0 || estimatedSize > MaxBufferSize {
		estimatedSize = 0 // Unknown or too large, use safe default (64KB medium buffer)
		if httpResp.ContentLength > MaxBufferSize {
			logger.WarnEvent().
				Int64("content_length", httpResp.ContentLength).
				Int("max_buffer_size", MaxBufferSize).
				Msg("Content-Length exceeds max buffer size, using default buffer")
		}
	}

	buffer := f.bufferPool.Get(estimatedSize)
	defer buffer.Release()

	totalBytes := 0

	for {
		n, err := httpResp.Body.Read(buffer.Bytes())
		if n > 0 {
			totalBytes += n

			// Create chunk response
			chunk := &tunnelv1.HTTPResponse{
				StatusCode: int32(httpResp.StatusCode), //nolint:gosec // Safe conversion
				Headers:    headers,
				Body:       append([]byte(nil), buffer.Bytes()[:n]...),
			}

			// Check if this is the last chunk
			isLast := (err == io.EOF)

			// Send chunk via callback
			if sendErr := sendChunk(chunk, isLast); sendErr != nil {
				return fmt.Errorf("failed to send chunk: %w", sendErr)
			}

			logger.DebugEvent().
				Int("chunk_size", n).
				Int("total_bytes", totalBytes).
				Bool("is_last", isLast).
				Msg("Sent response chunk")

			// Clear headers after first chunk (only send headers once)
			headers = nil
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
	}

	logger.InfoEvent().
		Int("total_bytes", totalBytes).
		Msg("Completed chunked response streaming")

	return nil
}

// IsWebSocketUpgrade checks if the request is a WebSocket upgrade request.
func IsWebSocketUpgrade(req *tunnelv1.HTTPRequest) bool {
	if req.Headers == nil {
		return false
	}

	// Check Upgrade header
	if upgrade, ok := req.Headers["Upgrade"]; ok && len(upgrade.Values) > 0 {
		if strings.ToLower(upgrade.Values[0]) == "websocket" {
			// Check Connection header
			if conn, ok := req.Headers["Connection"]; ok && len(conn.Values) > 0 {
				return strings.Contains(strings.ToLower(conn.Values[0]), "upgrade")
			}
		}
	}

	return false
}

// ForwardWebSocketUpgrade handles WebSocket upgrade and returns the upgrade response and connection.
func (f *HTTPForwarder) ForwardWebSocketUpgrade(ctx context.Context, req *tunnelv1.HTTPRequest) (*tunnelv1.HTTPResponse, net.Conn, error) {
	// Dial raw TCP connection to local service
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", f.localAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to local service: %w", err)
	}

	logger.InfoEvent().
		Str("local_addr", f.localAddr).
		Str("path", req.Path).
		Msg("Established TCP connection for WebSocket upgrade")

	// Build and write HTTP upgrade request
	url := req.Path
	if req.QueryString != "" {
		url += "?" + req.QueryString
	}

	// Write request line
	requestLine := fmt.Sprintf("%s %s HTTP/1.1\r\n", req.Method, url)
	if _, err := conn.Write([]byte(requestLine)); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to write request line: %w", err)
	}

	// Write headers
	for name, headerVals := range req.Headers {
		for _, val := range headerVals.Values {
			headerLine := fmt.Sprintf("%s: %s\r\n", name, val)
			if _, err := conn.Write([]byte(headerLine)); err != nil {
				conn.Close()
				return nil, nil, fmt.Errorf("failed to write header: %w", err)
			}
		}
	}

	// End of headers
	if _, err := conn.Write([]byte("\r\n")); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to write header end: %w", err)
	}

	logger.DebugEvent().Msg("Sent WebSocket upgrade request to local service")

	// Read upgrade response
	reader := bufio.NewReader(conn)

	// Read status line
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to read status line: %w", err)
	}

	// Parse status code
	var statusCode int
	if _, err := fmt.Sscanf(statusLine, "HTTP/1.1 %d", &statusCode); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to parse status code: %w", err)
	}

	// Read headers
	// Pre-allocate with typical WebSocket upgrade header count (~8 headers)
	headers := make(map[string]*tunnelv1.HeaderValues, 8)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, nil, fmt.Errorf("failed to read header: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// End of headers
			break
		}

		// Parse header
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			if existing, ok := headers[name]; ok {
				existing.Values = append(existing.Values, value)
			} else {
				headers[name] = &tunnelv1.HeaderValues{
					Values: []string{value},
				}
			}
		}
	}

	logger.InfoEvent().
		Int("status_code", statusCode).
		Msg("Received WebSocket upgrade response from local service")

	// Build gRPC response
	response := &tunnelv1.HTTPResponse{
		StatusCode: int32(statusCode), //nolint:gosec // Safe conversion: HTTP status codes are always 100-599
		Headers:    headers,
		Body:       nil, // No body for upgrade response
	}

	// Return response and connection for bidirectional streaming
	return response, conn, nil
}

// Close gracefully shuts down the HTTP forwarder and its connection pool.
func (f *HTTPForwarder) Close() error {
	if f.connPool != nil {
		return f.connPool.Close()
	}
	return nil
}

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
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// HTTPForwarder forwards HTTP requests to local service.
type HTTPForwarder struct {
	localAddr  string
	httpClient *http.Client
}

// NewHTTPForwarder creates a new HTTP forwarder.
func NewHTTPForwarder(localAddr string) *HTTPForwarder {
	// Create custom transport with optimized settings
	transport := &http.Transport{
		MaxIdleConns:        100,              // Reuse connections
		MaxIdleConnsPerHost: 10,               // Keep connections to local service alive
		IdleConnTimeout:     90 * time.Second, // Keep idle connections longer
		DisableCompression:  false,            // Allow compression if server supports it
		WriteBufferSize:     32 * 1024,        // 32KB write buffer (default is 4KB)
		ReadBufferSize:      32 * 1024,        // 32KB read buffer (default is 4KB)
	}

	return &HTTPForwarder{
		localAddr: localAddr,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				// Don't follow redirects
				return http.ErrUseLastResponse
			},
		},
	}
}

const (
	// ChunkSize is the size of each chunk for streaming large responses (4MB)
	ChunkSize = 4 * 1024 * 1024
)

// Forward forwards a gRPC HTTP request to local service.
// For small responses, returns complete response. For large responses, this is deprecated - use ForwardChunked.
func (f *HTTPForwarder) Forward(ctx context.Context, req *tunnelv1.HTTPRequest) (*tunnelv1.HTTPResponse, error) {
	// Build URL
	url := fmt.Sprintf("http://%s%s", f.localAddr, req.Path)
	if req.QueryString != "" {
		url += "?" + req.QueryString
	}

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
	headers := make(map[string]*tunnelv1.HeaderValues)
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
	// Build URL
	url := fmt.Sprintf("http://%s%s", f.localAddr, req.Path)
	if req.QueryString != "" {
		url += "?" + req.QueryString
	}

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

	// Execute request
	httpResp, err := f.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer httpResp.Body.Close()

	// Convert response headers
	headers := make(map[string]*tunnelv1.HeaderValues)
	for name, values := range httpResp.Header {
		headers[name] = &tunnelv1.HeaderValues{
			Values: values,
		}
	}

	logger.InfoEvent().
		Int("status", httpResp.StatusCode).
		Msg("Received response from local service, streaming in chunks")

	// Read and send response body in chunks
	buffer := make([]byte, ChunkSize)
	totalBytes := 0

	for {
		n, err := httpResp.Body.Read(buffer)
		if n > 0 {
			totalBytes += n

			// Create chunk response
			chunk := &tunnelv1.HTTPResponse{
				StatusCode: int32(httpResp.StatusCode), //nolint:gosec // Safe conversion
				Headers:    headers,
				Body:       make([]byte, n),
			}
			copy(chunk.Body, buffer[:n])

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
	headers := make(map[string]*tunnelv1.HeaderValues)
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

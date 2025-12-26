package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
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
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects
				return http.ErrUseLastResponse
			},
		},
	}
}

// Forward forwards a gRPC HTTP request to local service.
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
	return &tunnelv1.HTTPResponse{
		StatusCode: int32(httpResp.StatusCode),
		Headers:    headers,
		Body:       respBody,
	}, nil
}

package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
)

const (
	// DefaultRequestTimeout is the default timeout for proxied requests
	DefaultRequestTimeout = 30 * time.Second
)

// HTTPProxy handles HTTP reverse proxying
type HTTPProxy struct {
	router        *Router
	tunnelManager *tunnel.Manager
}

// NewHTTPProxy creates a new HTTP proxy
func NewHTTPProxy(router *Router, tunnelManager *tunnel.Manager) *HTTPProxy {
	return &HTTPProxy{
		router:        router,
		tunnelManager: tunnelManager,
	}
}

// ServeHTTP implements http.Handler
func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Route to tunnel
	tun, err := p.router.RouteToTunnel(r.Host)
	if err != nil {
		logger.WarnEvent().
			Err(err).
			Str("host", r.Host).
			Str("path", r.URL.Path).
			Msg("Failed to route request")

		if err == pkgerrors.ErrTunnelNotFound {
			http.Error(w, "Tunnel not found", http.StatusNotFound)
		} else {
			http.Error(w, "Bad gateway", http.StatusBadGateway)
		}
		return
	}

	// Update tunnel activity
	tun.UpdateActivity()

	// Proxy request through tunnel
	resp, reqBytes, err := p.proxyRequest(r, tun)
	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("tunnel_id", tun.ID.String()).
			Str("subdomain", tun.Subdomain).
			Str("path", r.URL.Path).
			Msg("Failed to proxy request")

		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Write response
	respBytes := p.writeResponse(w, resp)

	// Update tunnel statistics
	tun.UpdateStats(reqBytes, respBytes)

	// Save stats to database (async to avoid blocking)
	go func() {
		if err := p.tunnelManager.SaveTunnelStats(context.Background(), tun.ID); err != nil {
			logger.WarnEvent().
				Err(err).
				Str("tunnel_id", tun.ID.String()).
				Msg("Failed to save tunnel stats")
		}
	}()

	// Log request
	duration := time.Since(start)
	logger.InfoEvent().
		Str("tunnel_id", tun.ID.String()).
		Str("subdomain", tun.Subdomain).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Int("status", int(resp.StatusCode)).
		Dur("duration", duration).
		Int64("bytes_in", reqBytes).
		Int64("bytes_out", respBytes).
		Msg("Request proxied")
}

// proxyRequest proxies an HTTP request through a tunnel
// Returns: (response, requestBytes, error)
func (p *HTTPProxy) proxyRequest(r *http.Request, tun *tunnel.Tunnel) (*tunnelv1.HTTPResponse, int64, error) {
	// Generate request ID
	requestID := utils.GenerateRequestID()

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, 0, pkgerrors.Wrap(err, "failed to read request body")
	}
	defer r.Body.Close()

	// Calculate request size (headers + body)
	requestBytes := int64(len(body))
	for key, values := range r.Header {
		for _, val := range values {
			requestBytes += int64(len(key) + len(val) + 4) // key: value\r\n
		}
	}
	requestBytes += int64(len(r.Method) + len(r.URL.Path) + len(r.URL.RawQuery) + 20) // Method, path, query, protocol

	// Convert HTTP headers to proto format
	headers := make(map[string]*tunnelv1.HeaderValues)
	for key, values := range r.Header {
		headers[key] = &tunnelv1.HeaderValues{
			Values: values,
		}
	}

	// Create proxy request
	proxyReq := &tunnelv1.ProxyRequest{
		RequestId: requestID,
		TunnelId:  tun.ID.String(),
		Payload: &tunnelv1.ProxyRequest_Http{
			Http: &tunnelv1.HTTPRequest{
				Method:      r.Method,
				Path:        r.URL.Path,
				Headers:     headers,
				Body:        body,
				QueryString: r.URL.RawQuery,
				RemoteAddr:  r.RemoteAddr,
			},
		},
	}

	// Create response channel
	responseCh := make(chan *tunnelv1.ProxyResponse, 1)
	tun.ResponseMap.Store(requestID, responseCh)
	defer tun.ResponseMap.Delete(requestID)

	// Send request to tunnel via gRPC stream
	proxyMsg := &tunnelv1.ProxyMessage{
		Message: &tunnelv1.ProxyMessage_Request{
			Request: proxyReq,
		},
	}

	if err := tun.Stream.SendMsg(proxyMsg); err != nil {
		return nil, 0, pkgerrors.Wrap(err, "failed to send request to tunnel")
	}

	// Wait for response with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRequestTimeout)
	defer cancel()

	select {
	case proxyResp := <-responseCh:
		// Got response
		if httpResp := proxyResp.GetHttp(); httpResp != nil {
			return httpResp, requestBytes, nil
		}
		return nil, 0, pkgerrors.NewAppError("INVALID_RESPONSE", "invalid response type", nil)

	case <-ctx.Done():
		return nil, 0, pkgerrors.ErrRequestTimeout
	}
}

// writeResponse writes a proto HTTP response to http.ResponseWriter
// Returns: bytes written
func (p *HTTPProxy) writeResponse(w http.ResponseWriter, resp *tunnelv1.HTTPResponse) int64 {
	// Calculate response size (headers + body)
	responseBytes := int64(len(resp.Body))
	for key, headerVals := range resp.Headers {
		for _, val := range headerVals.Values {
			responseBytes += int64(len(key) + len(val) + 4) // key: value\r\n
		}
	}
	responseBytes += 20 // Status line overhead

	// Write headers
	for key, headerVals := range resp.Headers {
		for _, val := range headerVals.Values {
			w.Header().Add(key, val)
		}
	}

	// Write status code
	w.WriteHeader(int(resp.StatusCode))

	// Write body
	if len(resp.Body) > 0 {
		if _, err := io.Copy(w, bytes.NewReader(resp.Body)); err != nil {
			logger.ErrorEvent().
				Err(err).
				Msg("Failed to write response body")
		}
	}

	return responseBytes
}

// HealthCheckHandler returns a simple health check handler
func HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	}
}

package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
)

const (
	// DefaultRequestTimeout is the default timeout for proxied requests.
	DefaultRequestTimeout = 30 * time.Second
)

// HTTPProxy handles HTTP reverse proxying.
type HTTPProxy struct {
	router        *Router
	webhookRouter *WebhookRouter
	tunnelManager *tunnel.Manager
	db            *gorm.DB
}

// NewHTTPProxy creates a new HTTP proxy.
func NewHTTPProxy(router *Router, webhookRouter *WebhookRouter, tunnelManager *tunnel.Manager, db *gorm.DB) *HTTPProxy {
	return &HTTPProxy{
		router:        router,
		webhookRouter: webhookRouter,
		tunnelManager: tunnelManager,
		db:            db,
	}
}

// ServeHTTP implements http.Handler.
func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Check if this is a webhook request
	if p.webhookRouter != nil && p.webhookRouter.IsWebhookRequest(r.Host) {
		p.handleWebhookRequest(w, r, start)
		return
	}

	// Regular tunnel routing
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

	// Save request log to database (async)
	go p.saveRequestLog(tun.ID, r, int(resp.StatusCode), duration, reqBytes, respBytes)
}

// Returns: (response, requestBytes, error).
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

// Returns: bytes written.
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

// handleWebhookRequest handles webhook broadcast requests.
func (p *HTTPProxy) handleWebhookRequest(w http.ResponseWriter, r *http.Request, start time.Time) {
	// Extract webhook components
	orgSubdomain, appName, userPath, err := p.webhookRouter.ExtractWebhookComponents(r.Host, r.URL.Path)
	if err != nil {
		logger.WarnEvent().
			Err(err).
			Str("host", r.Host).
			Str("path", r.URL.Path).
			Msg("Invalid webhook URL format")
		http.Error(w, "Invalid webhook URL", http.StatusBadRequest)
		return
	}

	logger.DebugEvent().
		Str("org", orgSubdomain).
		Str("app", appName).
		Str("user_path", userPath).
		Msg("Webhook request received")

	// Get webhook routes
	cache, err := p.webhookRouter.GetWebhookRoutes(orgSubdomain, appName)
	if err != nil {
		logger.WarnEvent().
			Err(err).
			Str("org", orgSubdomain).
			Str("app", appName).
			Msg("Failed to get webhook routes")

		if err == ErrWebhookAppNotFound {
			http.Error(w, "Webhook app not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to read webhook request body")
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Prepare request data
	requestData := &RequestData{
		Method:  r.Method,
		Path:    userPath, // Use user-defined path, not the full path
		Headers: r.Header,
		Body:    body,
	}

	// Broadcast to all enabled tunnels
	ctx := context.Background()
	result, err := p.webhookRouter.BroadcastToTunnels(ctx, cache, userPath, requestData)

	duration := time.Since(start)

	// Log webhook event (async)
	go p.logWebhookEvent(cache.AppID, r, result, duration, err)

	// Handle response
	if err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("app", appName).
			Int("tunnel_count", result.TunnelCount).
			Int("success_count", result.SuccessCount).
			Msg("Webhook broadcast failed")

		http.Error(w, "No available tunnels", http.StatusServiceUnavailable)
		return
	}

	// Return first successful response
	if result.FirstSuccess != nil {
		// Write headers
		for key, values := range result.FirstSuccess.Headers {
			for _, val := range values {
				w.Header().Add(key, val)
			}
		}

		// Write status code
		w.WriteHeader(result.FirstSuccess.StatusCode)

		// Write body
		if len(result.FirstSuccess.Body) > 0 {
			_, _ = w.Write(result.FirstSuccess.Body) // Ignore error - handled by HTTP layer
		}

		logger.InfoEvent().
			Str("app", appName).
			Str("method", r.Method).
			Str("path", userPath).
			Int("status", result.FirstSuccess.StatusCode).
			Int("tunnel_count", result.TunnelCount).
			Int("success_count", result.SuccessCount).
			Dur("duration", duration).
			Msg("Webhook request broadcast completed")
	} else {
		http.Error(w, "All tunnels failed", http.StatusServiceUnavailable)
	}
}

// logWebhookEvent logs a webhook event to the database.
func (p *HTTPProxy) logWebhookEvent(appID uuid.UUID, r *http.Request, result *BroadcastResult, duration time.Duration, _ error) {
	// This would be implemented to save webhook events to database
	// For now, just log to console
	logger.DebugEvent().
		Str("app_id", appID.String()).
		Str("path", r.URL.Path).
		Str("method", r.Method).
		Int("tunnel_count", result.TunnelCount).
		Int("success_count", result.SuccessCount).
		Dur("duration", duration).
		Msg("Webhook event logged")
}

// saveRequestLog saves HTTP request log to database.
func (p *HTTPProxy) saveRequestLog(tunnelID uuid.UUID, r *http.Request, statusCode int, duration time.Duration, bytesIn, bytesOut int64) {
	if p.db == nil {
		return
	}

	// Build full path with query parameters
	fullPath := r.URL.Path
	if r.URL.RawQuery != "" {
		fullPath = r.URL.Path + "?" + r.URL.RawQuery
	}

	requestLog := &models.RequestLog{
		TunnelID:   tunnelID,
		Method:     r.Method,
		Path:       fullPath,
		StatusCode: statusCode,
		DurationMs: int(duration.Milliseconds()),
		BytesIn:    int(bytesIn),
		BytesOut:   int(bytesOut),
		ClientIP:   r.RemoteAddr,
	}

	if err := p.db.Create(requestLog).Error; err != nil {
		logger.WarnEvent().
			Err(err).
			Str("tunnel_id", tunnelID.String()).
			Msg("Failed to save request log")
	}
}

// HealthCheckHandler returns a simple health check handler.
func HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	}
}

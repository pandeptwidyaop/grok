package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/errorpages"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
)

// wsServerBufferPool pools 32KB buffers for WebSocket read operations to reduce GC pressure.
var wsServerBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32*1024) // 32KB
		return &buf
	},
}

const (
	// DefaultRequestTimeout is the default timeout for proxied requests.
	DefaultRequestTimeout = 30 * time.Second
	// MaxRequestBodySize is the maximum allowed request body size (10MB).
	// This prevents memory exhaustion from large request bodies.
	MaxRequestBodySize = 10 * 1024 * 1024 // 10MB
	// CleanupDebounceInterval is the minimum time between cleanup operations per tunnel.
	// This prevents excessive database queries on high-traffic tunnels.
	CleanupDebounceInterval = 1 * time.Minute
)

// HTTPProxy handles HTTP reverse proxying.
type HTTPProxy struct {
	router         *Router
	webhookRouter  *WebhookRouter
	tunnelManager  *tunnel.Manager
	db             *gorm.DB
	httpLogLevel   string // HTTP request log level: silent, error, warn, info
	maxRequestLogs int    // Maximum number of request logs to keep per tunnel

	// Cleanup debouncing: track last cleanup time per tunnel
	lastCleanup sync.Map // tunnelID → time.Time
}

// NewHTTPProxy creates a new HTTP proxy.
func NewHTTPProxy(router *Router, webhookRouter *WebhookRouter, tunnelManager *tunnel.Manager, db *gorm.DB, httpLogLevel string, maxRequestLogs int) *HTTPProxy {
	return &HTTPProxy{
		router:         router,
		webhookRouter:  webhookRouter,
		tunnelManager:  tunnelManager,
		db:             db,
		httpLogLevel:   httpLogLevel,
		maxRequestLogs: maxRequestLogs,
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
			subdomain := strings.Split(r.Host, ".")[0]
			errorpages.TunnelNotFound(w, r, subdomain)
		} else {
			http.Error(w, "Bad gateway", http.StatusBadGateway)
		}
		return
	}

	// Update tunnel activity
	tun.UpdateActivity()

	// Check if this is a WebSocket upgrade request
	if isWebSocketUpgrade(r) {
		p.handleWebSocketProxy(w, r, tun)
		return
	}

	// Proxy regular HTTP request through tunnel (simple, full response)
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
	statusCode := int(resp.StatusCode)

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

	// Log request based on configured level
	duration := time.Since(start)

	shouldLog := p.shouldLogHTTPRequest(statusCode)
	if shouldLog {
		// Choose log level based on status code
		var logEvent *zerolog.Event
		if statusCode >= 500 {
			logEvent = logger.ErrorEvent()
		} else if statusCode >= 400 {
			logEvent = logger.WarnEvent()
		} else {
			logEvent = logger.InfoEvent()
		}

		logEvent.
			Str("tunnel_id", tun.ID.String()).
			Str("subdomain", tun.Subdomain).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", statusCode).
			Dur("duration", duration).
			Int64("bytes_in", reqBytes).
			Int64("bytes_out", respBytes).
			Msg("Request proxied (chunked)")
	}

	// Save request log to database (async)
	go p.saveRequestLog(tun.ID, r, statusCode, duration, reqBytes, respBytes)
}

// isWebSocketUpgrade checks if the request is a WebSocket upgrade request.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Upgrade")) == "websocket" &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// readAndValidateRequestBody reads and validates request body size.
func (p *HTTPProxy) readAndValidateRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBodySize+1))
	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to read request body")
	}
	defer r.Body.Close()

	if int64(len(body)) > MaxRequestBodySize {
		return nil, pkgerrors.NewAppError("REQUEST_TOO_LARGE", "request body exceeds maximum size of 10MB", nil)
	}
	return body, nil
}

// calculateRequestSize calculates total request size including headers.
func (p *HTTPProxy) calculateRequestSize(r *http.Request, bodySize int) int64 {
	size := int64(bodySize)
	for key, values := range r.Header {
		for _, val := range values {
			size += int64(len(key) + len(val) + 4)
		}
	}
	size += int64(len(r.Method) + len(r.URL.Path) + len(r.URL.RawQuery) + 20)
	return size
}

// convertHeadersToProto converts HTTP headers to protobuf format.
func (p *HTTPProxy) convertHeadersToProto(headers http.Header) map[string]*tunnelv1.HeaderValues {
	protoHeaders := make(map[string]*tunnelv1.HeaderValues, len(headers))
	for key, values := range headers {
		protoHeaders[key] = &tunnelv1.HeaderValues{Values: values}
	}
	return protoHeaders
}

// writeResponseHeaders writes HTTP response headers and status code.
// Returns bytes written.
func (p *HTTPProxy) writeResponseHeaders(w http.ResponseWriter, httpResp *tunnelv1.HTTPResponse) int64 {
	bytesWritten := int64(0)

	for key, headerVals := range httpResp.Headers {
		for _, val := range headerVals.Values {
			w.Header().Add(key, val)
			bytesWritten += int64(len(key) + len(val) + 4)
		}
	}

	w.WriteHeader(int(httpResp.StatusCode))
	bytesWritten += 20 // Status line overhead

	return bytesWritten
}

// writeResponse writes complete HTTP response (headers + body) to ResponseWriter.
// Returns total bytes written.
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

// proxyRequestChunked proxies request and streams response chunks directly to client.
// Returns: (requestBytes, responseBytes, statusCode, error).
// proxyRequest proxies a single HTTP request through the tunnel (simple, non-chunked).
// Returns: (httpResponse, requestBytes, error).
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

func (p *HTTPProxy) proxyRequestChunked(r *http.Request, tun *tunnel.Tunnel, w http.ResponseWriter) (int64, int64, int, error) {
	requestID := utils.GenerateRequestID()

	body, err := p.readAndValidateRequestBody(r)
	if err != nil {
		return 0, 0, 0, err
	}

	requestBytes := p.calculateRequestSize(r, len(body))
	headers := p.convertHeadersToProto(r.Header)

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

	// Create response channel for receiving chunks
	// Buffer size 50 for performance (blocking send with backpressure supports unlimited file size)
	responseCh := make(chan *tunnelv1.ProxyResponse, 50)
	tun.ResponseMap.Store(requestID, responseCh)
	defer tun.ResponseMap.Delete(requestID)

	// Send request to tunnel via RequestQueue (instead of direct stream send)
	// This prevents race condition: multiple HTTP handlers cannot write to stream concurrently
	pendingReq := &tunnel.PendingRequest{
		RequestID:  requestID,
		Request:    proxyReq,
		ResponseCh: responseCh,
		Timeout:    DefaultRequestTimeout,
		CreatedAt:  time.Now(),
	}

	// Push to queue with timeout to prevent blocking if queue is full
	select {
	case tun.RequestQueue <- pendingReq:
		// Successfully queued
	case <-time.After(5 * time.Second):
		return 0, 0, 0, pkgerrors.NewAppError("QUEUE_FULL", "tunnel request queue is full", nil)
	}

	// Wait for response chunks and stream directly to client
	// No timeout for chunked streaming to support large file downloads
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	responseBytes := int64(0)
	headersWritten := false
	statusCode := 0

	for {
		select {
		case proxyResp := <-responseCh:
			if proxyResp == nil {
				return requestBytes, responseBytes, statusCode, pkgerrors.NewAppError("INVALID_RESPONSE", "received nil response", nil)
			}

			httpResp := proxyResp.GetHttp()
			if httpResp == nil {
				return requestBytes, responseBytes, statusCode, pkgerrors.NewAppError("INVALID_RESPONSE", "invalid response type", nil)
			}

			// Write headers on first chunk
			if !headersWritten {
				statusCode = int(httpResp.StatusCode)
				responseBytes += p.writeResponseHeaders(w, httpResp)
				headersWritten = true
			}

			// Write chunk body
			if len(httpResp.Body) > 0 {
				n, err := w.Write(httpResp.Body)
				if err != nil {
					logger.ErrorEvent().
						Err(err).
						Str("request_id", requestID).
						Msg("Failed to write response chunk to client")
					return requestBytes, responseBytes, statusCode, pkgerrors.Wrap(err, "failed to write response chunk")
				}
				responseBytes += int64(n)

				logger.DebugEvent().
					Str("request_id", requestID).
					Int("chunk_size", n).
					Bool("end_of_stream", proxyResp.EndOfStream).
					Msg("Wrote response chunk to client")
			}

			// Flush to ensure chunk is sent immediately
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

			// Check if this is the last chunk
			if proxyResp.EndOfStream {
				logger.DebugEvent().
					Str("request_id", requestID).
					Int64("total_bytes", responseBytes).
					Msg("Completed streaming chunked response")
				return requestBytes, responseBytes, statusCode, nil
			}

		case <-ctx.Done():
			return requestBytes, responseBytes, statusCode, pkgerrors.ErrRequestTimeout
		}
	}
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
		errorpages.InvalidWebhookURL(w, r.Host+r.URL.Path)
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
	} else {
		// Cleanup old request logs if limit exceeded
		p.cleanupOldRequestLogs(tunnelID)
	}
}

// cleanupOldRequestLogs removes old request logs when limit is exceeded.
// Deletes oldest requests first to keep recent history.
// Uses debouncing to prevent excessive cleanup operations on high-traffic tunnels.
func (p *HTTPProxy) cleanupOldRequestLogs(tunnelID uuid.UUID) {
	if p.maxRequestLogs <= 0 {
		return // Unlimited if not set or disabled
	}

	// Debounce: Check if we recently cleaned up this tunnel
	now := time.Now()
	if lastCleanupInterface, ok := p.lastCleanup.Load(tunnelID); ok {
		if lastCleanup, ok := lastCleanupInterface.(time.Time); ok {
			if now.Sub(lastCleanup) < CleanupDebounceInterval {
				// Skip cleanup - too soon since last cleanup
				return
			}
		}
	}

	// Update last cleanup time before starting (prevent concurrent cleanups)
	p.lastCleanup.Store(tunnelID, now)

	// Count total request logs for this tunnel
	var count int64
	if err := p.db.Model(&models.RequestLog{}).
		Where("tunnel_id = ?", tunnelID).
		Count(&count).Error; err != nil {
		logger.WarnEvent().
			Err(err).
			Str("tunnel_id", tunnelID.String()).
			Msg("Failed to count request logs for cleanup")
		return
	}

	// Calculate how many to delete
	if count <= int64(p.maxRequestLogs) {
		return // Within limit, no cleanup needed
	}

	toDelete := count - int64(p.maxRequestLogs)

	// Get IDs of oldest requests to delete
	var oldLogIDs []uuid.UUID
	if err := p.db.Model(&models.RequestLog{}).
		Select("id").
		Where("tunnel_id = ?", tunnelID).
		Order("created_at ASC").
		Limit(int(toDelete)).
		Pluck("id", &oldLogIDs).Error; err != nil {
		logger.WarnEvent().
			Err(err).
			Str("tunnel_id", tunnelID.String()).
			Int64("to_delete", toDelete).
			Msg("Failed to fetch old request log IDs for cleanup")
		return
	}

	// Delete old request logs
	if err := p.db.Where("id IN ?", oldLogIDs).Delete(&models.RequestLog{}).Error; err != nil {
		logger.WarnEvent().
			Err(err).
			Str("tunnel_id", tunnelID.String()).
			Int("deleted_count", len(oldLogIDs)).
			Msg("Failed to delete old request logs")
		return
	}

	logger.DebugEvent().
		Str("tunnel_id", tunnelID.String()).
		Int("deleted_count", len(oldLogIDs)).
		Int64("remaining_count", count-toDelete).
		Msg("Request logs cleanup completed")
}

// handleWebSocketProxy handles WebSocket upgrade and bidirectional proxying.
func (p *HTTPProxy) handleWebSocketProxy(w http.ResponseWriter, r *http.Request, tun *tunnel.Tunnel) {
	conn, err := p.hijackWebSocketConnection(w)
	if err != nil {
		return
	}
	defer conn.Close()

	logger.InfoEvent().
		Str("tunnel_id", tun.ID.String()).
		Str("subdomain", tun.Subdomain).
		Str("path", r.URL.Path).
		Msg("WebSocket upgrade request received")

	requestID := utils.GenerateRequestID()

	responseCh, err := p.sendWebSocketUpgradeRequest(r, tun, requestID)
	if err != nil {
		return
	}
	defer tun.ResponseMap.Delete(requestID)

	upgradeResp, err := p.waitForWebSocketUpgradeResponse(requestID, responseCh)
	if err != nil {
		return
	}

	if err := p.writeWebSocketUpgradeResponse(conn, upgradeResp); err != nil {
		return
	}

	if upgradeResp.StatusCode != 101 {
		logger.WarnEvent().
			Int("status_code", int(upgradeResp.StatusCode)).
			Str("request_id", requestID).
			Msg("WebSocket upgrade failed")
		return
	}

	logger.InfoEvent().
		Str("request_id", requestID).
		Str("tunnel_id", tun.ID.String()).
		Msg("WebSocket upgraded successfully, starting bidirectional proxy")

	p.proxyWebSocketData(conn, tun, requestID)
}

// hijackWebSocketConnection hijacks HTTP connection for WebSocket upgrade.
func (p *HTTPProxy) hijackWebSocketConnection(w http.ResponseWriter) (net.Conn, error) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logger.ErrorEvent().Msg("ResponseWriter does not support hijacking")
		http.Error(w, "WebSocket not supported", http.StatusInternalServerError)
		return nil, fmt.Errorf("hijacking not supported")
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to hijack connection")
		http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
		return nil, err
	}

	return conn, nil
}

// sendWebSocketUpgradeRequest sends WebSocket upgrade request to tunnel client.
func (p *HTTPProxy) sendWebSocketUpgradeRequest(r *http.Request, tun *tunnel.Tunnel, requestID string) (chan *tunnelv1.ProxyResponse, error) {
	headers := make(map[string]*tunnelv1.HeaderValues, len(r.Header))
	for key, values := range r.Header {
		headers[key] = &tunnelv1.HeaderValues{Values: values}
	}

	upgradeReq := &tunnelv1.ProxyRequest{
		RequestId: requestID,
		TunnelId:  tun.ID.String(),
		Payload: &tunnelv1.ProxyRequest_Http{
			Http: &tunnelv1.HTTPRequest{
				Method:      r.Method,
				Path:        r.URL.Path,
				Headers:     headers,
				QueryString: r.URL.RawQuery,
				RemoteAddr:  r.RemoteAddr,
			},
		},
	}

	responseCh := make(chan *tunnelv1.ProxyResponse, 1)
	tun.ResponseMap.Store(requestID, responseCh)

	pendingReq := &tunnel.PendingRequest{
		RequestID:  requestID,
		Request:    upgradeReq,
		ResponseCh: responseCh,
		Timeout:    10 * time.Second,
		CreatedAt:  time.Now(),
	}

	select {
	case tun.RequestQueue <- pendingReq:
		return responseCh, nil
	case <-time.After(5 * time.Second):
		logger.ErrorEvent().
			Str("request_id", requestID).
			Msg("Failed to queue WebSocket upgrade request - queue full")
		return nil, fmt.Errorf("queue full")
	}
}

// waitForWebSocketUpgradeResponse waits for upgrade response from tunnel client.
func (p *HTTPProxy) waitForWebSocketUpgradeResponse(requestID string, responseCh chan *tunnelv1.ProxyResponse) (*tunnelv1.HTTPResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	select {
	case proxyResp := <-responseCh:
		if httpResp := proxyResp.GetHttp(); httpResp != nil {
			return httpResp, nil
		}
		logger.ErrorEvent().Str("request_id", requestID).Msg("Invalid upgrade response type")
		return nil, fmt.Errorf("invalid response type")
	case <-ctx.Done():
		logger.ErrorEvent().Str("request_id", requestID).Msg("WebSocket upgrade timeout")
		return nil, fmt.Errorf("timeout")
	}
}

// writeWebSocketUpgradeResponse writes HTTP upgrade response to connection.
func (p *HTTPProxy) writeWebSocketUpgradeResponse(conn net.Conn, upgradeResp *tunnelv1.HTTPResponse) error {
	statusLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", upgradeResp.StatusCode, http.StatusText(int(upgradeResp.StatusCode)))
	if _, err := conn.Write([]byte(statusLine)); err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to write status line")
		return err
	}

	for key, headerVals := range upgradeResp.Headers {
		for _, val := range headerVals.Values {
			headerLine := fmt.Sprintf("%s: %s\r\n", key, val)
			if _, err := conn.Write([]byte(headerLine)); err != nil {
				logger.ErrorEvent().Err(err).Msg("Failed to write header")
				return err
			}
		}
	}

	if _, err := conn.Write([]byte("\r\n")); err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to write header end")
		return err
	}

	return nil
}

// proxyWebSocketData handles bidirectional WebSocket data streaming.
func (p *HTTPProxy) proxyWebSocketData(conn net.Conn, tun *tunnel.Tunnel, requestID string) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Channel for errors
	errCh := make(chan error, 2)

	// Client -> Tunnel (read from HTTP connection, send to gRPC stream)
	go func() {
		defer wg.Done()

		// Get buffer from pool
		bufPtr := wsServerBufferPool.Get().(*[]byte) //nolint:errcheck // sync.Pool.Get() doesn't return error
		buffer := *bufPtr
		defer wsServerBufferPool.Put(bufPtr)

		sequence := int64(0)

		for {
			n, err := conn.Read(buffer)
			if err != nil {
				if err != io.EOF {
					logger.DebugEvent().Err(err).Msg("Client connection read error")
					errCh <- err
				}
				return
			}

			if n > 0 {
				// Send data to tunnel via gRPC stream
				// CRITICAL: Must copy buffer data before sending to avoid data corruption
				// when buffer is reused in next Read() iteration
				tcpData := &tunnelv1.TCPData{
					Data:     append([]byte(nil), buffer[:n]...),
					Sequence: sequence,
				}
				sequence++

				proxyMsg := &tunnelv1.ProxyMessage{
					Message: &tunnelv1.ProxyMessage_Request{
						Request: &tunnelv1.ProxyRequest{
							RequestId: requestID,
							TunnelId:  tun.ID.String(),
							Payload: &tunnelv1.ProxyRequest_Tcp{
								Tcp: tcpData,
							},
						},
					},
				}

				// Use StreamMu to prevent concurrent sends for WebSocket data
				// Multiple WebSocket connections may try to send concurrently
				tun.StreamMu.Lock()
				err := tun.Stream.SendMsg(proxyMsg)
				tun.StreamMu.Unlock()

				if err != nil {
					logger.ErrorEvent().Err(err).Msg("Failed to send WebSocket data to tunnel")
					errCh <- err
					return
				}

				// Update stats
				tun.UpdateStats(int64(n), 0)
			}
		}
	}()

	// Tunnel -> Client (receive from gRPC stream, write to HTTP connection)
	go func() {
		defer wg.Done()

		// Create channel for WebSocket responses
		wsCh := make(chan []byte, 100)
		tun.ResponseMap.Store(requestID+":ws", wsCh)
		defer tun.ResponseMap.Delete(requestID + ":ws")

		for {
			select {
			case data := <-wsCh:
				if len(data) > 0 {
					if _, err := conn.Write(data); err != nil {
						logger.ErrorEvent().Err(err).Msg("Failed to write WebSocket data to client")
						errCh <- err
						return
					}

					// Update stats
					tun.UpdateStats(0, int64(len(data)))
				}
			case <-time.After(5 * time.Minute):
				// Timeout if no data for 5 minutes
				logger.InfoEvent().Str("request_id", requestID).Msg("WebSocket idle timeout")
				return
			}
		}
	}()

	// Wait for either goroutine to finish
	wg.Wait()

	logger.InfoEvent().
		Str("request_id", requestID).
		Str("tunnel_id", tun.ID.String()).
		Msg("WebSocket connection closed")
}

// shouldLogHTTPRequest determines if HTTP request should be logged based on configured level.
func (p *HTTPProxy) shouldLogHTTPRequest(statusCode int) bool {
	switch p.httpLogLevel {
	case "silent":
		return false
	case "error":
		return statusCode >= 500
	case "warn":
		return statusCode >= 400
	case "info":
		return true
	default:
		return true // Default to logging everything
	}
}

// HealthCheckHandler returns a simple health check handler.
func HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	}
}

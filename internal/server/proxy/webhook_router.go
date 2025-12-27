package proxy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
)

var (
	// ErrNoHealthyTunnels is returned when no healthy tunnels are available for webhook.
	ErrNoHealthyTunnels = errors.New("no healthy tunnels available for webhook")

	// ErrWebhookAppNotFound is returned when webhook app is not found.
	ErrWebhookAppNotFound = errors.New("webhook app not found")

	// ErrInvalidWebhookURL is returned when webhook URL format is invalid.
	ErrInvalidWebhookURL = errors.New("invalid webhook URL format")
)

// WebhookEventType represents the type of webhook event.
type WebhookEventType string

const (
	EventWebhookReceived WebhookEventType = "webhook_received"
	EventWebhookSuccess  WebhookEventType = "webhook_success"
	EventWebhookFailed   WebhookEventType = "webhook_failed"
)

// WebhookEvent represents a webhook processing event.
type WebhookEvent struct {
	Type         WebhookEventType
	AppID        uuid.UUID
	RequestPath  string
	Method       string
	StatusCode   int
	DurationMs   int64
	BytesIn      int64
	BytesOut     int64
	ClientIP     string
	TunnelCount  int
	SuccessCount int
	ErrorMessage string
}

// WebhookEventHandler is a callback for webhook events.
type WebhookEventHandler func(interface{})

// WebhookRouter handles routing for webhook requests with broadcast support.
type WebhookRouter struct {
	db            *gorm.DB
	tunnelManager *tunnel.Manager
	baseDomain    string

	// In-memory cache: org_subdomain → *WebhookRouteCache
	webhookCache sync.Map

	// Cache configuration
	cacheRefreshInterval time.Duration

	// Event handlers
	eventHandlers []WebhookEventHandler
	eventMu       sync.RWMutex
}

// WebhookRouteCache holds cached webhook routing information.
type WebhookRouteCache struct {
	AppID        uuid.UUID
	AppName      string
	OrgSubdomain string
	Routes       []*WebhookRouteCacheEntry
	LastRefresh  time.Time
	mu           sync.RWMutex
}

// WebhookRouteCacheEntry represents a single route in cache.
type WebhookRouteCacheEntry struct {
	RouteID      uuid.UUID
	TunnelID     uuid.UUID
	Priority     int
	IsEnabled    bool
	HealthStatus string
}

// BroadcastResult contains results from broadcasting to tunnels.
type BroadcastResult struct {
	TunnelCount  int
	SuccessCount int
	Responses    []*TunnelResponse
	FirstSuccess *TunnelResponse
	ErrorMessage string
}

// TunnelResponse represents response from a single tunnel.
type TunnelResponse struct {
	TunnelID     uuid.UUID
	StatusCode   int
	Body         []byte
	Headers      map[string][]string
	DurationMs   int64
	Success      bool
	ErrorMessage string
}

// NewWebhookRouter creates a new webhook router.
func NewWebhookRouter(db *gorm.DB, tunnelManager *tunnel.Manager, baseDomain string) *WebhookRouter {
	return &WebhookRouter{
		db:                   db,
		tunnelManager:        tunnelManager,
		baseDomain:           baseDomain,
		cacheRefreshInterval: 30 * time.Second,
		eventHandlers:        make([]WebhookEventHandler, 0),
	}
}

// OnWebhookEvent subscribes to webhook events.
func (wr *WebhookRouter) OnWebhookEvent(handler WebhookEventHandler) {
	wr.eventMu.Lock()
	defer wr.eventMu.Unlock()
	wr.eventHandlers = append(wr.eventHandlers, handler)
}

// emitEvent emits a webhook event to all subscribers.
func (wr *WebhookRouter) emitEvent(event WebhookEvent) {
	wr.eventMu.RLock()
	defer wr.eventMu.RUnlock()

	// Call all event handlers in goroutines to avoid blocking
	for _, handler := range wr.eventHandlers {
		go handler(event)
	}
}

// Example: trofeo-webhook.grok.io/payment-app/stripe/callback.
func (wr *WebhookRouter) IsWebhookRequest(host string) bool {
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Check if host ends with base domain
	suffix := "." + wr.baseDomain
	if !strings.HasSuffix(host, suffix) {
		return false
	}

	// Extract subdomain
	subdomain := strings.TrimSuffix(host, suffix)

	// Check if subdomain ends with "-webhook"
	return strings.HasSuffix(subdomain, "-webhook")
}

// Returns: orgSubdomain="trofeo", appName="payment-app", userPath="/stripe/callback".
func (wr *WebhookRouter) ExtractWebhookComponents(host, path string) (orgSubdomain, appName, userPath string, err error) {
	// Remove port from host
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Extract subdomain from host
	suffix := "." + wr.baseDomain
	if !strings.HasSuffix(host, suffix) {
		return "", "", "", ErrInvalidWebhookURL
	}

	webhookSubdomain := strings.TrimSuffix(host, suffix)

	// Validate webhook subdomain format: must end with "-webhook"
	// Pattern: {app-name}-{org-subdomain}-webhook.{domain}/{user_webhook_path}
	if !strings.HasSuffix(webhookSubdomain, "-webhook") {
		return "", "", "", ErrInvalidWebhookURL
	}

	// Remove "-webhook" suffix to get: {app-name}-{org-subdomain}
	appOrgPart := strings.TrimSuffix(webhookSubdomain, "-webhook")

	// Query database to find matching webhook app
	// We need to find a webhook app where concatenating app.Name + "-" + org.Subdomain
	// matches the appOrgPart
	// Note: Use || for string concatenation (SQLite compatible)
	var webhookApp models.WebhookApp
	err = wr.db.Preload("Organization").
		Joins("JOIN organizations ON organizations.id = webhook_apps.organization_id").
		Where("webhook_apps.name || '-' || organizations.subdomain = ?", appOrgPart).
		First(&webhookApp).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", "", fmt.Errorf("webhook app not found for subdomain: %s", webhookSubdomain)
		}
		return "", "", "", fmt.Errorf("failed to query webhook app: %w", err)
	}

	// Extract components from database result
	appName = webhookApp.Name
	orgSubdomain = webhookApp.Organization.Subdomain

	// User path is the entire request path
	if path == "" {
		userPath = "/"
	} else {
		userPath = path
	}

	return orgSubdomain, appName, userPath, nil
}

// GetWebhookRoutes retrieves webhook routes from cache or database.
func (wr *WebhookRouter) GetWebhookRoutes(orgSubdomain, appName string) (*WebhookRouteCache, error) {
	cacheKey := orgSubdomain + ":" + appName

	// Check cache first
	if cached, ok := wr.webhookCache.Load(cacheKey); ok {
		cache, ok := cached.(*WebhookRouteCache)
		if !ok {
			return nil, fmt.Errorf("invalid cache type")
		}
		cache.mu.RLock()
		defer cache.mu.RUnlock()

		// Check if cache is still fresh
		if time.Since(cache.LastRefresh) < wr.cacheRefreshInterval {
			return cache, nil
		}
	}

	// Cache miss or stale - refresh from database
	return wr.RefreshCache(orgSubdomain, appName)
}

// RefreshCache refreshes webhook route cache from database.
func (wr *WebhookRouter) RefreshCache(orgSubdomain, appName string) (*WebhookRouteCache, error) {
	// Find organization by subdomain
	var org models.Organization
	if err := wr.db.Where("subdomain = ?", orgSubdomain).First(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("organization not found: %s", orgSubdomain)
		}
		return nil, fmt.Errorf("failed to query organization: %w", err)
	}

	// Find webhook app
	var app models.WebhookApp
	if err := wr.db.Where("organization_id = ? AND name = ? AND is_active = ?", org.ID, appName, true).
		First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWebhookAppNotFound
		}
		return nil, fmt.Errorf("failed to query webhook app: %w", err)
	}

	// Load active routes
	var routes []models.WebhookRoute
	if err := wr.db.Where("webhook_app_id = ?", app.ID).
		Order("priority ASC").
		Find(&routes).Error; err != nil {
		return nil, fmt.Errorf("failed to query webhook routes: %w", err)
	}

	// Build cache entries
	cacheEntries := make([]*WebhookRouteCacheEntry, 0, len(routes))
	for _, route := range routes {
		cacheEntries = append(cacheEntries, &WebhookRouteCacheEntry{
			RouteID:      route.ID,
			TunnelID:     route.TunnelID,
			Priority:     route.Priority,
			IsEnabled:    route.IsEnabled,
			HealthStatus: route.HealthStatus,
		})
	}

	// Create cache object
	cache := &WebhookRouteCache{
		AppID:        app.ID,
		AppName:      app.Name,
		OrgSubdomain: orgSubdomain,
		Routes:       cacheEntries,
		LastRefresh:  time.Now(),
	}

	// Store in cache
	cacheKey := orgSubdomain + ":" + appName
	wr.webhookCache.Store(cacheKey, cache)

	logger.DebugEvent().
		Str("org", orgSubdomain).
		Str("app", appName).
		Int("routes", len(cacheEntries)).
		Msg("Webhook route cache refreshed")

	return cache, nil
}

// collectResponses collects responses from all tunnels.
func collectResponses(responseCh chan *TunnelResponse, tunnelCount int) *BroadcastResult {
	result := &BroadcastResult{
		TunnelCount: tunnelCount,
		Responses:   make([]*TunnelResponse, 0, tunnelCount),
	}

	for response := range responseCh {
		result.Responses = append(result.Responses, response)
		if response.Success {
			result.SuccessCount++
			if result.FirstSuccess == nil {
				result.FirstSuccess = response
			}
		}
	}

	return result
}

// emitWebhookEvent emits a webhook processing event.
func (wr *WebhookRouter) emitWebhookEvent(cache *WebhookRouteCache, userPath string, request *RequestData, result *BroadcastResult, durationMs int64) {
	statusCode := 0
	if result.FirstSuccess != nil {
		statusCode = result.FirstSuccess.StatusCode
	}

	bytesIn := int64(len(request.Body))
	var bytesOut int64
	if result.FirstSuccess != nil {
		bytesOut = int64(len(result.FirstSuccess.Body))
	}

	clientIP := ""
	if xForwardedFor := request.Headers["X-Forwarded-For"]; len(xForwardedFor) > 0 {
		clientIP = xForwardedFor[0]
	} else if xRealIP := request.Headers["X-Real-Ip"]; len(xRealIP) > 0 {
		clientIP = xRealIP[0]
	}

	eventType := EventWebhookSuccess
	if result.SuccessCount == 0 {
		eventType = EventWebhookFailed
	}

	wr.emitEvent(WebhookEvent{
		Type:         eventType,
		AppID:        cache.AppID,
		RequestPath:  userPath,
		Method:       request.Method,
		StatusCode:   statusCode,
		DurationMs:   durationMs,
		BytesIn:      bytesIn,
		BytesOut:     bytesOut,
		ClientIP:     clientIP,
		TunnelCount:  result.TunnelCount,
		SuccessCount: result.SuccessCount,
		ErrorMessage: result.ErrorMessage,
	})
}

// broadcastToSingleTunnel broadcasts request to a single tunnel.
func (wr *WebhookRouter) broadcastToSingleTunnel(ctx context.Context, route *WebhookRouteCacheEntry, userPath string, request *RequestData, responseCh chan *TunnelResponse) {
	start := time.Now()

	tun, ok := wr.tunnelManager.GetTunnelByID(route.TunnelID)
	if !ok {
		responseCh <- &TunnelResponse{
			TunnelID:     route.TunnelID,
			Success:      false,
			ErrorMessage: "tunnel not found in manager",
		}
		return
	}

	if tun.GetStatus() != "active" {
		responseCh <- &TunnelResponse{
			TunnelID:     route.TunnelID,
			Success:      false,
			ErrorMessage: "tunnel not active",
		}
		return
	}

	response := wr.sendToTunnel(ctx, tun, userPath, request)
	response.TunnelID = route.TunnelID
	response.DurationMs = time.Since(start).Milliseconds()

	responseCh <- response
}

// Returns the first successful response.
func (wr *WebhookRouter) BroadcastToTunnels(ctx context.Context, cache *WebhookRouteCache, userPath string, request *RequestData) (*BroadcastResult, error) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	var enabledRoutes []*WebhookRouteCacheEntry
	for _, route := range cache.Routes {
		if route.IsEnabled && route.HealthStatus != "unhealthy" {
			enabledRoutes = append(enabledRoutes, route)
		}
	}

	if len(enabledRoutes) == 0 {
		return &BroadcastResult{
			TunnelCount:  0,
			SuccessCount: 0,
			ErrorMessage: "no enabled tunnels available",
		}, ErrNoHealthyTunnels
	}

	broadcastStart := time.Now()
	broadcastCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	responseCh := make(chan *TunnelResponse, len(enabledRoutes))
	var wg sync.WaitGroup

	for _, route := range enabledRoutes {
		wg.Add(1)
		go func(r *WebhookRouteCacheEntry) {
			defer wg.Done()
			wr.broadcastToSingleTunnel(broadcastCtx, r, userPath, request, responseCh)
		}(route)
	}

	go func() {
		wg.Wait()
		close(responseCh)
	}()

	result := collectResponses(responseCh, len(enabledRoutes))
	durationMs := time.Since(broadcastStart).Milliseconds()

	wr.emitWebhookEvent(cache, userPath, request, result, durationMs)

	if result.SuccessCount == 0 {
		var errMsgs []string
		for _, resp := range result.Responses {
			if resp.ErrorMessage != "" {
				errMsgs = append(errMsgs, resp.ErrorMessage)
			}
		}
		result.ErrorMessage = strings.Join(errMsgs, "; ")
		return result, fmt.Errorf("all tunnels failed: %s", result.ErrorMessage)
	}

	return result, nil
}

// sendToTunnel sends request to a single tunnel via gRPC stream.
func (wr *WebhookRouter) sendToTunnel(ctx context.Context, tun *tunnel.Tunnel, userPath string, request *RequestData) *TunnelResponse {
	// Generate request ID
	requestID := utils.GenerateRequestID()

	// Convert headers to proto format
	headers := make(map[string]*tunnelv1.HeaderValues)
	for key, values := range request.Headers {
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
				Method:      request.Method,
				Path:        userPath,
				Headers:     headers,
				Body:        request.Body,
				QueryString: "",
				RemoteAddr:  "", // Will be populated by handleWebhookRequest
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
		return &TunnelResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to send to tunnel: %v", err),
		}
	}

	// Wait for response with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	select {
	case proxyResp := <-responseCh:
		// Got response from tunnel
		if httpResp := proxyResp.GetHttp(); httpResp != nil {
			// Convert proto headers back to map
			respHeaders := make(map[string][]string)
			for key, headerVals := range httpResp.Headers {
				respHeaders[key] = headerVals.Values
			}

			return &TunnelResponse{
				StatusCode: int(httpResp.StatusCode),
				Body:       httpResp.Body,
				Headers:    respHeaders,
				Success:    true,
			}
		}
		return &TunnelResponse{
			Success:      false,
			ErrorMessage: "invalid response type from tunnel",
		}

	case <-timeoutCtx.Done():
		return &TunnelResponse{
			Success:      false,
			ErrorMessage: "tunnel response timeout (30s)",
		}
	}
}

// RequestData holds HTTP request data for proxying.
type RequestData struct {
	Method  string
	Path    string
	Headers map[string][]string
	Body    []byte
}

// InvalidateCache invalidates the cache for a specific webhook app.
func (wr *WebhookRouter) InvalidateCache(orgSubdomain, appName string) {
	cacheKey := orgSubdomain + ":" + appName
	wr.webhookCache.Delete(cacheKey)

	logger.DebugEvent().
		Str("org", orgSubdomain).
		Str("app", appName).
		Msg("Webhook cache invalidated")
}

// InvalidateAllCache invalidates all cached webhook routes.
func (wr *WebhookRouter) InvalidateAllCache() {
	wr.webhookCache.Range(func(key, _ interface{}) bool {
		wr.webhookCache.Delete(key)
		return true
	})

	logger.InfoEvent().Msg("All webhook caches invalidated")
}

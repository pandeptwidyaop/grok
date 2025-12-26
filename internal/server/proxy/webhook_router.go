package proxy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"gorm.io/gorm"
)

var (
	// ErrNoHealthyTunnels is returned when no healthy tunnels are available for webhook
	ErrNoHealthyTunnels = errors.New("no healthy tunnels available for webhook")

	// ErrWebhookAppNotFound is returned when webhook app is not found
	ErrWebhookAppNotFound = errors.New("webhook app not found")

	// ErrInvalidWebhookURL is returned when webhook URL format is invalid
	ErrInvalidWebhookURL = errors.New("invalid webhook URL format")
)

// WebhookEventType represents the type of webhook event
type WebhookEventType string

const (
	EventWebhookReceived WebhookEventType = "webhook_received"
	EventWebhookSuccess  WebhookEventType = "webhook_success"
	EventWebhookFailed   WebhookEventType = "webhook_failed"
)

// WebhookEvent represents a webhook processing event
type WebhookEvent struct {
	Type         WebhookEventType
	AppID        uuid.UUID
	RequestPath  string
	Method       string
	StatusCode   int
	TunnelCount  int
	SuccessCount int
	ErrorMessage string
}

// WebhookEventHandler is a callback for webhook events
type WebhookEventHandler func(interface{})

// WebhookRouter handles routing for webhook requests with broadcast support
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

	mu sync.RWMutex
}

// WebhookRouteCache holds cached webhook routing information
type WebhookRouteCache struct {
	AppID         uuid.UUID
	AppName       string
	OrgSubdomain  string
	Routes        []*WebhookRouteCacheEntry
	LastRefresh   time.Time
	mu            sync.RWMutex
}

// WebhookRouteCacheEntry represents a single route in cache
type WebhookRouteCacheEntry struct {
	RouteID      uuid.UUID
	TunnelID     uuid.UUID
	Priority     int
	IsEnabled    bool
	HealthStatus string
}

// BroadcastResult contains results from broadcasting to tunnels
type BroadcastResult struct {
	TunnelCount   int
	SuccessCount  int
	Responses     []*TunnelResponse
	FirstSuccess  *TunnelResponse
	ErrorMessage  string
}

// TunnelResponse represents response from a single tunnel
type TunnelResponse struct {
	TunnelID     uuid.UUID
	StatusCode   int
	Body         []byte
	Headers      map[string][]string
	DurationMs   int64
	Success      bool
	ErrorMessage string
}

// NewWebhookRouter creates a new webhook router
func NewWebhookRouter(db *gorm.DB, tunnelManager *tunnel.Manager, baseDomain string) *WebhookRouter {
	return &WebhookRouter{
		db:                   db,
		tunnelManager:        tunnelManager,
		baseDomain:           baseDomain,
		cacheRefreshInterval: 30 * time.Second,
		eventHandlers:        make([]WebhookEventHandler, 0),
	}
}

// OnWebhookEvent subscribes to webhook events
func (wr *WebhookRouter) OnWebhookEvent(handler WebhookEventHandler) {
	wr.eventMu.Lock()
	defer wr.eventMu.Unlock()
	wr.eventHandlers = append(wr.eventHandlers, handler)
}

// emitEvent emits a webhook event to all subscribers
func (wr *WebhookRouter) emitEvent(event WebhookEvent) {
	wr.eventMu.RLock()
	defer wr.eventMu.RUnlock()

	// Call all event handlers in goroutines to avoid blocking
	for _, handler := range wr.eventHandlers {
		go handler(event)
	}
}

// IsWebhookRequest checks if the request matches webhook subdomain pattern
// Pattern: {org}-webhook.{baseDomain}
// Example: trofeo-webhook.grok.io
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

// ExtractWebhookComponents extracts organization subdomain, app name, and user path from request
// Example URL: trofeo-webhook.grok.io/payment-app/stripe/callback
// Returns: orgSubdomain="trofeo", appName="payment-app", userPath="/stripe/callback"
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

	// Extract org subdomain by removing "-webhook" suffix
	if !strings.HasSuffix(webhookSubdomain, "-webhook") {
		return "", "", "", ErrInvalidWebhookURL
	}
	orgSubdomain = strings.TrimSuffix(webhookSubdomain, "-webhook")

	// Parse path: /{app_name}/{user_webhook_path}
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		return "", "", "", fmt.Errorf("webhook path must start with /")
	}

	// Split path into parts
	parts := strings.SplitN(path, "/", 3) // ["", "app_name", "user_path"]
	if len(parts) < 2 || parts[1] == "" {
		return "", "", "", fmt.Errorf("webhook path must include app name: /{app_name}/...")
	}

	appName = parts[1]

	// User path is everything after app name
	if len(parts) >= 3 {
		userPath = "/" + parts[2]
	} else {
		userPath = "/"
	}

	return orgSubdomain, appName, userPath, nil
}

// GetWebhookRoutes retrieves webhook routes from cache or database
func (wr *WebhookRouter) GetWebhookRoutes(orgSubdomain, appName string) (*WebhookRouteCache, error) {
	cacheKey := orgSubdomain + ":" + appName

	// Check cache first
	if cached, ok := wr.webhookCache.Load(cacheKey); ok {
		cache := cached.(*WebhookRouteCache)
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

// RefreshCache refreshes webhook route cache from database
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

// BroadcastToTunnels broadcasts a webhook request to all enabled tunnels
// Returns the first successful response
func (wr *WebhookRouter) BroadcastToTunnels(ctx context.Context, cache *WebhookRouteCache, userPath string, request *ProxyRequestData) (*BroadcastResult, error) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	// Filter enabled and healthy routes
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

	// Create context with timeout
	broadcastCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Channel to collect responses
	responseCh := make(chan *TunnelResponse, len(enabledRoutes))
	var wg sync.WaitGroup

	// Broadcast to all enabled tunnels concurrently
	for _, route := range enabledRoutes {
		wg.Add(1)
		go func(r *WebhookRouteCacheEntry) {
			defer wg.Done()

			start := time.Now()

			// Get tunnel from manager
			tun, ok := wr.tunnelManager.GetTunnelByID(r.TunnelID)
			if !ok {
				responseCh <- &TunnelResponse{
					TunnelID:     r.TunnelID,
					Success:      false,
					ErrorMessage: "tunnel not found in manager",
				}
				return
			}

			// Check if tunnel is active
			if tun.GetStatus() != "active" {
				responseCh <- &TunnelResponse{
					TunnelID:     r.TunnelID,
					Success:      false,
					ErrorMessage: "tunnel not active",
				}
				return
			}

			// Send request to tunnel
			// Note: This would use the existing proxy logic
			// For now, we'll simulate the response structure
			response := wr.sendToTunnel(broadcastCtx, tun, userPath, request)
			response.TunnelID = r.TunnelID
			response.DurationMs = time.Since(start).Milliseconds()

			responseCh <- response
		}(route)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(responseCh)
	}()

	// Collect responses
	result := &BroadcastResult{
		TunnelCount: len(enabledRoutes),
		Responses:   make([]*TunnelResponse, 0, len(enabledRoutes)),
	}

	for response := range responseCh {
		result.Responses = append(result.Responses, response)

		if response.Success {
			result.SuccessCount++
			// Store first successful response
			if result.FirstSuccess == nil {
				result.FirstSuccess = response
			}
		}
	}

	// Determine status code from first successful response
	statusCode := 0
	if result.FirstSuccess != nil {
		statusCode = result.FirstSuccess.StatusCode
	}

	// Emit webhook event
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
		TunnelCount:  result.TunnelCount,
		SuccessCount: result.SuccessCount,
		ErrorMessage: result.ErrorMessage,
	})

	// If no successful responses, aggregate error messages
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

// sendToTunnel sends request to a single tunnel (placeholder for actual implementation)
func (wr *WebhookRouter) sendToTunnel(ctx context.Context, tun *tunnel.Tunnel, userPath string, request *ProxyRequestData) *TunnelResponse {
	// TODO: Implement actual tunnel proxying logic
	// This will be integrated with the existing HTTP proxy mechanism
	// For now, return a placeholder response
	return &TunnelResponse{
		StatusCode: 200,
		Success:    true,
	}
}

// ProxyRequestData holds HTTP request data for proxying
type ProxyRequestData struct {
	Method  string
	Path    string
	Headers map[string][]string
	Body    []byte
}

// InvalidateCache invalidates the cache for a specific webhook app
func (wr *WebhookRouter) InvalidateCache(orgSubdomain, appName string) {
	cacheKey := orgSubdomain + ":" + appName
	wr.webhookCache.Delete(cacheKey)

	logger.DebugEvent().
		Str("org", orgSubdomain).
		Str("app", appName).
		Msg("Webhook cache invalidated")
}

// InvalidateAllCache invalidates all cached webhook routes
func (wr *WebhookRouter) InvalidateAllCache() {
	wr.webhookCache.Range(func(key, value interface{}) bool {
		wr.webhookCache.Delete(key)
		return true
	})

	logger.InfoEvent().Msg("All webhook caches invalidated")
}

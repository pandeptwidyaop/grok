package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	"github.com/pandeptwidyaop/grok/internal/server/config"
	"github.com/pandeptwidyaop/grok/internal/server/proxy"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	"github.com/pandeptwidyaop/grok/internal/server/web/middleware"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
	"gorm.io/gorm"
)

// Handler handles dashboard API requests
type Handler struct {
	db            *gorm.DB
	tokenService  *auth.TokenService
	tunnelManager *tunnel.Manager
	webhookRouter *proxy.WebhookRouter
	config        *config.Config
	authMW        *middleware.AuthMiddleware
	sseBroker     *SSEBroker
}

// NewHandler creates a new dashboard API handler
func NewHandler(db *gorm.DB, tokenService *auth.TokenService, tunnelManager *tunnel.Manager, webhookRouter *proxy.WebhookRouter, cfg *config.Config) *Handler {
	h := &Handler{
		db:            db,
		tokenService:  tokenService,
		tunnelManager: tunnelManager,
		webhookRouter: webhookRouter,
		config:        cfg,
		authMW:        middleware.NewAuthMiddleware(cfg.Auth.JWTSecret),
		sseBroker:     NewSSEBroker(),
	}

	// Subscribe to tunnel events and broadcast via SSE
	tunnelManager.OnTunnelEvent(func(event tunnel.TunnelEvent) {
		// Broadcast tunnel event via SSE
		h.sseBroker.Broadcast(SSEEvent{
			Type: string(event.Type),
			Data: map[string]interface{}{
				"tunnel_id": event.TunnelID.String(),
				"tunnel":    event.Tunnel,
			},
		})
	})

	// Subscribe to webhook events and broadcast via SSE + save to database
	if webhookRouter != nil {
		webhookRouter.OnWebhookEvent(func(event interface{}) {
			// Type assert to WebhookEvent
			if webhookEvent, ok := event.(proxy.WebhookEvent); ok {
				// Determine routing status
				routingStatus := "success"
				if webhookEvent.SuccessCount == 0 {
					routingStatus = "failed"
				} else if webhookEvent.SuccessCount < webhookEvent.TunnelCount {
					routingStatus = "partial"
				}

				// Save webhook event to database
				dbEvent := &models.WebhookEvent{
					WebhookAppID:  webhookEvent.AppID,
					RequestPath:   webhookEvent.RequestPath,
					Method:        webhookEvent.Method,
					StatusCode:    webhookEvent.StatusCode,
					DurationMs:    webhookEvent.DurationMs,
					BytesIn:       webhookEvent.BytesIn,
					BytesOut:      webhookEvent.BytesOut,
					ClientIP:      webhookEvent.ClientIP,
					RoutingStatus: routingStatus,
					TunnelCount:   webhookEvent.TunnelCount,
					SuccessCount:  webhookEvent.SuccessCount,
					ErrorMessage:  webhookEvent.ErrorMessage,
				}

				if err := h.db.Create(dbEvent).Error; err != nil {
					logger.ErrorEvent().
						Err(err).
						Str("app_id", webhookEvent.AppID.String()).
						Msg("Failed to save webhook event to database")
				}

				// Broadcast webhook event via SSE
				h.sseBroker.Broadcast(SSEEvent{
					Type: string(webhookEvent.Type),
					Data: map[string]interface{}{
						"app_id":        webhookEvent.AppID.String(),
						"request_path":  webhookEvent.RequestPath,
						"method":        webhookEvent.Method,
						"status_code":   webhookEvent.StatusCode,
						"tunnel_count":  webhookEvent.TunnelCount,
						"success_count": webhookEvent.SuccessCount,
						"error_message": webhookEvent.ErrorMessage,
					},
				})
			}
		})
	}

	return h
}

// CORSMiddleware adds CORS headers to all responses
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}

		// Handle preflight OPTIONS request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Create organization handler, webhook handler, version handler, 2FA handler, and RBAC middleware
	orgHandler := NewOrganizationHandler(h.db, h.config.Server.Domain)
	webhookHandler := NewWebhookHandler(h.db, h.tunnelManager)
	versionHandler := NewVersionHandler()
	twoFAHandler := NewTwoFAHandler(h.db, h.config.Server.Domain)
	rbac := middleware.NewPermissionChecker()

	// Public routes
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.HandleFunc("GET /api/version", versionHandler.GetVersion)
	mux.HandleFunc("GET /api/version/check-updates", versionHandler.CheckUpdates)

	// Protected routes (require JWT)
	mux.Handle("GET /api/tokens", h.authMW.Protect(http.HandlerFunc(h.listTokens)))
	mux.Handle("POST /api/tokens", h.authMW.Protect(http.HandlerFunc(h.createToken)))
	mux.Handle("DELETE /api/tokens/{id}", h.authMW.Protect(http.HandlerFunc(h.deleteToken)))
	mux.Handle("PATCH /api/tokens/{id}/toggle", h.authMW.Protect(http.HandlerFunc(h.toggleToken)))

	mux.Handle("GET /api/tunnels", h.authMW.Protect(http.HandlerFunc(h.listTunnels)))
	mux.Handle("GET /api/tunnels/{id}", h.authMW.Protect(http.HandlerFunc(h.getTunnel)))
	mux.Handle("GET /api/tunnels/{id}/logs", h.authMW.Protect(http.HandlerFunc(h.getTunnelLogs)))
	mux.Handle("DELETE /api/tunnels/{id}", h.authMW.Protect(http.HandlerFunc(h.deleteTunnel)))

	mux.Handle("GET /api/stats", h.authMW.Protect(http.HandlerFunc(h.getStats)))
	mux.Handle("GET /api/config", h.authMW.Protect(http.HandlerFunc(h.getConfig)))

	// 2FA routes
	mux.Handle("GET /api/2fa/status", h.authMW.Protect(http.HandlerFunc(twoFAHandler.GetStatus)))
	mux.Handle("POST /api/2fa/setup", h.authMW.Protect(http.HandlerFunc(twoFAHandler.EnableSetup)))
	mux.Handle("POST /api/2fa/verify", h.authMW.Protect(http.HandlerFunc(twoFAHandler.VerifyEnable)))
	mux.Handle("POST /api/2fa/disable", h.authMW.Protect(http.HandlerFunc(twoFAHandler.Disable)))

	// Organization routes - Super Admin only
	mux.Handle("POST /api/organizations",
		h.authMW.Protect(rbac.RequireRole(string(models.RoleSuperAdmin))(http.HandlerFunc(orgHandler.CreateOrganization))))
	mux.Handle("GET /api/organizations",
		h.authMW.Protect(rbac.RequireRole(string(models.RoleSuperAdmin))(http.HandlerFunc(orgHandler.ListOrganizations))))
	mux.Handle("GET /api/organizations/{id}",
		h.authMW.Protect(rbac.RequireRole(string(models.RoleSuperAdmin))(http.HandlerFunc(orgHandler.GetOrganization))))
	mux.Handle("PATCH /api/organizations/{id}",
		h.authMW.Protect(rbac.RequireRole(string(models.RoleSuperAdmin))(http.HandlerFunc(orgHandler.UpdateOrganization))))
	mux.Handle("DELETE /api/organizations/{id}",
		h.authMW.Protect(rbac.RequireRole(string(models.RoleSuperAdmin))(http.HandlerFunc(orgHandler.DeleteOrganization))))
	mux.Handle("PATCH /api/organizations/{id}/toggle",
		h.authMW.Protect(rbac.RequireRole(string(models.RoleSuperAdmin))(http.HandlerFunc(orgHandler.ToggleOrganization))))

	// Organization user routes - Org Admin + Super Admin
	mux.Handle("GET /api/organizations/{org_id}/users",
		h.authMW.Protect(rbac.RequireOrgMembership(rbac.RequireOrgAdmin(http.HandlerFunc(orgHandler.ListOrgUsers)))))
	mux.Handle("POST /api/organizations/{org_id}/users",
		h.authMW.Protect(rbac.RequireOrgMembership(rbac.RequireOrgAdmin(http.HandlerFunc(orgHandler.CreateOrgUser)))))
	mux.Handle("PATCH /api/organizations/{org_id}/users/{user_id}",
		h.authMW.Protect(rbac.RequireOrgMembership(rbac.RequireOrgAdmin(http.HandlerFunc(orgHandler.UpdateUserRole)))))
	mux.Handle("DELETE /api/organizations/{org_id}/users/{user_id}",
		h.authMW.Protect(rbac.RequireOrgMembership(rbac.RequireOrgAdmin(http.HandlerFunc(orgHandler.DeleteOrgUser)))))
	mux.Handle("POST /api/organizations/{org_id}/users/{user_id}/reset-password",
		h.authMW.Protect(rbac.RequireOrgMembership(rbac.RequireOrgAdmin(http.HandlerFunc(orgHandler.ResetUserPassword)))))

	// Organization stats and tunnels - Org Admin + Super Admin
	mux.Handle("GET /api/organizations/{org_id}/stats",
		h.authMW.Protect(rbac.RequireOrgMembership(rbac.RequireOrgAdmin(http.HandlerFunc(orgHandler.GetOrgStats)))))
	mux.Handle("GET /api/organizations/{org_id}/tunnels",
		h.authMW.Protect(rbac.RequireOrgMembership(rbac.RequireOrgAdmin(http.HandlerFunc(orgHandler.ListOrgTunnels)))))

	// Webhook routes - Org membership required
	// Webhook App Management
	mux.Handle("POST /api/webhooks/apps",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.CreateApp))))
	mux.Handle("GET /api/webhooks/apps",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.ListApps))))
	mux.Handle("GET /api/webhooks/apps/{id}",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.GetApp))))
	mux.Handle("PATCH /api/webhooks/apps/{id}",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.UpdateApp))))
	mux.Handle("DELETE /api/webhooks/apps/{id}",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.DeleteApp))))
	mux.Handle("PATCH /api/webhooks/apps/{id}/toggle",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.ToggleApp))))

	// Webhook Route Management
	mux.Handle("GET /api/webhooks/apps/{app_id}/routes",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.ListRoutes))))
	mux.Handle("POST /api/webhooks/apps/{app_id}/routes",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.AddRoute))))
	mux.Handle("PATCH /api/webhooks/apps/{app_id}/routes/{route_id}",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.UpdateRoute))))
	mux.Handle("DELETE /api/webhooks/apps/{app_id}/routes/{route_id}",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.DeleteRoute))))
	mux.Handle("PATCH /api/webhooks/apps/{app_id}/routes/{route_id}/toggle",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.ToggleRoute))))

	// Webhook Events & Stats
	mux.Handle("GET /api/webhooks/apps/{app_id}/events",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.GetEvents))))
	mux.Handle("GET /api/webhooks/apps/{app_id}/stats",
		h.authMW.Protect(rbac.RequireOrganization(http.HandlerFunc(webhookHandler.GetStats))))

	// Server-Sent Events (SSE) for real-time updates
	mux.Handle("GET /api/sse", h.authMW.Protect(http.HandlerFunc(h.HandleSSE)))
}

// Response helpers
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// Token handlers
func (h *Handler) listTokens(w http.ResponseWriter, r *http.Request) {
	// Get claims from context
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var tokens []models.AuthToken
	// Preload User and their Organization
	query := h.db.Preload("User.Organization")

	// Filter by organization for non-super-admins
	if claims.Role != string(models.RoleSuperAdmin) {
		// Org admin sees their org's tokens, org user sees only their own
		if claims.Role == string(models.RoleOrgAdmin) && claims.OrganizationID != nil {
			// Get all users in the organization first
			var userIDs []string
			h.db.Model(&models.User{}).
				Where("organization_id = ?", *claims.OrganizationID).
				Pluck("id", &userIDs)
			query = query.Where("user_id IN ?", userIDs)
		} else {
			// Org user sees only their own tokens
			query = query.Where("user_id = ?", claims.UserID)
		}
	}

	if err := query.Find(&tokens).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to list tokens")
		respondError(w, http.StatusInternalServerError, "Failed to list tokens")
		return
	}

	// Transform to include user info and organization name
	type TokenResponse struct {
		models.AuthToken
		OwnerEmail       string  `json:"owner_email"`
		OwnerName        string  `json:"owner_name"`
		OrganizationName *string `json:"organization_name,omitempty"`
	}

	response := make([]TokenResponse, len(tokens))
	for i, token := range tokens {
		resp := TokenResponse{
			AuthToken: token,
		}
		if token.User.ID != uuid.Nil {
			resp.OwnerEmail = token.User.Email
			resp.OwnerName = token.User.Name
			if token.User.Organization != nil {
				resp.OrganizationName = &token.User.Organization.Name
			}
		}
		response[i] = resp
	}

	respondJSON(w, http.StatusOK, response)
}

type createTokenRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

func (h *Handler) createToken(w http.ResponseWriter, r *http.Request) {
	// Get claims from context
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "Token name is required")
		return
	}

	// Parse user ID from claims
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Invalid user ID in claims")
		respondError(w, http.StatusInternalServerError, "Invalid user ID")
		return
	}

	token, rawToken, err := h.tokenService.CreateToken(r.Context(), userID, req.Name, nil)
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to create token")
		respondError(w, http.StatusInternalServerError, "Failed to create token")
		return
	}

	// Include raw token in response (only time it's visible)
	response := map[string]interface{}{
		"id":           token.ID,
		"name":         token.Name,
		"token":        rawToken, // Raw token only returned on creation
		"scopes":       token.Scopes,
		"is_active":    token.IsActive,
		"created_at":   token.CreatedAt,
		"expires_at":   token.ExpiresAt,
		"last_used_at": token.LastUsedAt,
	}

	respondJSON(w, http.StatusCreated, response)
}

func (h *Handler) deleteToken(w http.ResponseWriter, r *http.Request) {
	tokenID := r.PathValue("id")
	if tokenID == "" {
		respondError(w, http.StatusBadRequest, "Token ID is required")
		return
	}

	if err := h.db.Delete(&models.AuthToken{}, "id = ?", tokenID).Error; err != nil {
		logger.ErrorEvent().Err(err).Str("token_id", tokenID).Msg("Failed to delete token")
		respondError(w, http.StatusInternalServerError, "Failed to delete token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Token deleted"})
}

func (h *Handler) toggleToken(w http.ResponseWriter, r *http.Request) {
	tokenID := r.PathValue("id")
	if tokenID == "" {
		respondError(w, http.StatusBadRequest, "Token ID is required")
		return
	}

	var token models.AuthToken
	if err := h.db.First(&token, "id = ?", tokenID).Error; err != nil {
		respondError(w, http.StatusNotFound, "Token not found")
		return
	}

	token.IsActive = !token.IsActive
	if err := h.db.Save(&token).Error; err != nil {
		logger.ErrorEvent().Err(err).Str("token_id", tokenID).Msg("Failed to toggle token")
		respondError(w, http.StatusInternalServerError, "Failed to toggle token")
		return
	}

	respondJSON(w, http.StatusOK, token)
}

// Tunnel handlers
func (h *Handler) listTunnels(w http.ResponseWriter, r *http.Request) {
	// Get claims from context
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var tunnels []models.Tunnel
	// Show both active and offline tunnels (all persistent tunnels)
	// Preload User and Organization to include owner and org info in response
	query := h.db.Preload("User").Preload("Organization").
		Where("status IN (?)", []string{"active", "offline"}).
		Order("status ASC, connected_at DESC") // Active first, then by recent activity

	// Filter by organization for non-super-admins
	if claims.Role != string(models.RoleSuperAdmin) {
		// Org admin sees their org's tunnels, org user sees only their own
		if claims.Role == string(models.RoleOrgAdmin) && claims.OrganizationID != nil {
			query = query.Where("organization_id = ?", *claims.OrganizationID)
		} else {
			// Org user sees only their own tunnels
			query = query.Where("user_id = ?", claims.UserID)
		}
	}

	if err := query.Find(&tunnels).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to list tunnels")
		respondError(w, http.StatusInternalServerError, "Failed to list tunnels")
		return
	}

	// Transform to include organization and owner info
	type TunnelResponse struct {
		models.Tunnel
		OwnerEmail       string  `json:"owner_email"`
		OwnerName        string  `json:"owner_name"`
		OrganizationName *string `json:"organization_name,omitempty"`
	}

	response := make([]TunnelResponse, len(tunnels))
	for i, tun := range tunnels {
		resp := TunnelResponse{
			Tunnel: tun,
		}
		if tun.User != nil {
			resp.OwnerEmail = tun.User.Email
			resp.OwnerName = tun.User.Name
		}
		if tun.Organization != nil {
			resp.OrganizationName = &tun.Organization.Name
		}
		response[i] = resp
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *Handler) getTunnel(w http.ResponseWriter, r *http.Request) {
	tunnelID := r.PathValue("id")
	if tunnelID == "" {
		respondError(w, http.StatusBadRequest, "Tunnel ID is required")
		return
	}

	var tun models.Tunnel
	if err := h.db.First(&tun, "id = ?", tunnelID).Error; err != nil {
		respondError(w, http.StatusNotFound, "Tunnel not found")
		return
	}

	respondJSON(w, http.StatusOK, tun)
}

func (h *Handler) getTunnelLogs(w http.ResponseWriter, r *http.Request) {
	tunnelID := r.PathValue("id")
	if tunnelID == "" {
		respondError(w, http.StatusBadRequest, "Tunnel ID is required")
		return
	}

	// Parse pagination parameters
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := (page - 1) * limit

	// Parse optional path filter
	pathFilter := r.URL.Query().Get("path")

	// Build query
	query := h.db.Where("tunnel_id = ?", tunnelID)
	if pathFilter != "" {
		query = query.Where("path LIKE ?", "%"+pathFilter+"%")
	}

	// Get total count
	var total int64
	if err := query.Model(&models.RequestLog{}).Count(&total).Error; err != nil {
		logger.ErrorEvent().Err(err).Str("tunnel_id", tunnelID).Msg("Failed to count tunnel logs")
		respondError(w, http.StatusInternalServerError, "Failed to count tunnel logs")
		return
	}

	// Get paginated logs
	var logs []models.RequestLog
	if err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error; err != nil {
		logger.ErrorEvent().Err(err).Str("tunnel_id", tunnelID).Msg("Failed to get tunnel logs")
		respondError(w, http.StatusInternalServerError, "Failed to get tunnel logs")
		return
	}

	// Calculate pagination metadata
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"logs":        logs,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

// deleteTunnel forcefully disconnects and deletes a tunnel
func (h *Handler) deleteTunnel(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse tunnel ID
	tunnelIDStr := r.PathValue("id")
	if tunnelIDStr == "" {
		respondError(w, http.StatusBadRequest, "Tunnel ID is required")
		return
	}

	tunnelID, err := uuid.Parse(tunnelIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid tunnel ID")
		return
	}

	// Get tunnel from database
	var tun models.Tunnel
	if err := h.db.First(&tun, tunnelID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(w, http.StatusNotFound, "Tunnel not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to get tunnel")
		return
	}

	// Check permissions: only tunnel owner, org admin, or super admin can delete
	isOwner := tun.UserID.String() == claims.UserID
	isOrgAdmin := claims.Role == string(models.RoleOrgAdmin) || claims.Role == string(models.RoleSuperAdmin)

	// Check org membership if tunnel belongs to an org
	sameOrg := true
	if tun.OrganizationID != nil {
		if claims.OrganizationID == nil {
			sameOrg = false
		} else {
			sameOrg = tun.OrganizationID.String() == *claims.OrganizationID
		}
	}

	if !isOwner && !isOrgAdmin {
		respondError(w, http.StatusForbidden, "Access denied")
		return
	}

	if !sameOrg {
		respondError(w, http.StatusForbidden, "Access denied")
		return
	}

	// Disconnect tunnel from tunnel manager if active
	if err := h.tunnelManager.UnregisterTunnel(r.Context(), tunnelID); err != nil {
		logger.WarnEvent().
			Err(err).
			Str("tunnel_id", tunnelID.String()).
			Msg("Failed to unregister tunnel (may already be offline)")
	}

	// Delete domain reservation for this subdomain
	if err := h.db.Where("subdomain = ?", tun.Subdomain).Delete(&models.Domain{}).Error; err != nil {
		logger.WarnEvent().
			Err(err).
			Str("subdomain", tun.Subdomain).
			Msg("Failed to delete domain reservation (may not exist)")
	}

	// Delete tunnel from database (cascade will delete logs)
	if err := h.db.Delete(&tun).Error; err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("tunnel_id", tunnelID.String()).
			Msg("Failed to delete tunnel")
		respondError(w, http.StatusInternalServerError, "Failed to delete tunnel")
		return
	}

	logger.InfoEvent().
		Str("tunnel_id", tunnelID.String()).
		Str("user_id", claims.UserID).
		Msg("Tunnel deleted")

	respondJSON(w, http.StatusOK, map[string]string{"message": "Tunnel deleted successfully"})
}

// Stats handler
func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	// Get claims from context
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var stats struct {
		TotalTunnels  int64 `json:"total_tunnels"`
		ActiveTunnels int64 `json:"active_tunnels"`
		TotalRequests int64 `json:"total_requests"`
		TotalBytesIn  int64 `json:"total_bytes_in"`
		TotalBytesOut int64 `json:"total_bytes_out"`
	}

	// Build base query with org filtering
	baseQuery := h.db.Model(&models.Tunnel{})
	if claims.Role != string(models.RoleSuperAdmin) {
		// Org admin sees their org's stats, org user sees only their own
		if claims.Role == string(models.RoleOrgAdmin) && claims.OrganizationID != nil {
			baseQuery = baseQuery.Where("organization_id = ?", *claims.OrganizationID)
		} else {
			// Org user sees only their own stats
			baseQuery = baseQuery.Where("user_id = ?", claims.UserID)
		}
	}

	// Count tunnels
	baseQuery.Count(&stats.TotalTunnels)

	activeQuery := h.db.Model(&models.Tunnel{}).Where("status = ?", "active")
	if claims.Role != string(models.RoleSuperAdmin) {
		if claims.Role == string(models.RoleOrgAdmin) && claims.OrganizationID != nil {
			activeQuery = activeQuery.Where("organization_id = ?", *claims.OrganizationID)
		} else {
			activeQuery = activeQuery.Where("user_id = ?", claims.UserID)
		}
	}
	activeQuery.Count(&stats.ActiveTunnels)

	// Sum request stats from tunnels
	baseQuery.Select("COALESCE(SUM(requests_count), 0)").Row().Scan(&stats.TotalRequests)
	baseQuery.Select("COALESCE(SUM(bytes_in), 0)").Row().Scan(&stats.TotalBytesIn)
	baseQuery.Select("COALESCE(SUM(bytes_out), 0)").Row().Scan(&stats.TotalBytesOut)

	respondJSON(w, http.StatusOK, stats)
}

// Config handler
func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"domain": h.config.Server.Domain,
	}
	respondJSON(w, http.StatusOK, config)
}

// Auth handlers

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	OTPCode  string `json:"otp_code,omitempty"` // For 2FA verification
}

type loginResponse struct {
	Token            string  `json:"token,omitempty"`
	User             string  `json:"user"`
	Role             string  `json:"role,omitempty"`
	OrganizationID   *string `json:"organization_id,omitempty"`
	OrganizationName *string `json:"organization_name,omitempty"`
	Requires2FA      bool    `json:"requires_2fa,omitempty"` // Indicates 2FA is needed
}

// health returns a simple health check response
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "healthy",
		"service": "grok-server",
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate credentials
	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	// Load user with organization
	var user models.User
	if err := h.db.Preload("Organization").Where("email = ?", req.Username).First(&user).Error; err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Verify password
	if !utils.ComparePassword(user.Password, req.Password) {
		respondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Check if user is active
	if !user.IsActive {
		respondError(w, http.StatusUnauthorized, "User account is disabled")
		return
	}

	// Check if 2FA is enabled
	if user.TwoFactorEnabled {
		// If 2FA is enabled but no OTP code provided, return requires_2fa
		if req.OTPCode == "" {
			respondJSON(w, http.StatusOK, loginResponse{
				User:        user.Email,
				Requires2FA: true,
			})
			return
		}

		// Verify OTP code
		totpService := auth.NewTOTPService()
		if !totpService.ValidateCode(user.TwoFactorSecret, req.OTPCode) {
			respondError(w, http.StatusUnauthorized, "Invalid OTP code")
			return
		}
	}

	// Prepare org_id for token
	var orgIDStr *string
	var orgName *string
	if user.OrganizationID != nil {
		orgIDString := user.OrganizationID.String()
		orgIDStr = &orgIDString
		if user.Organization != nil {
			orgName = &user.Organization.Name
		}
	}

	// Generate JWT token with role and org_id
	token, err := h.authMW.GenerateToken(
		user.ID.String(),
		user.Email,
		string(user.Role),
		orgIDStr,
	)
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to generate token")
		respondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	respondJSON(w, http.StatusOK, loginResponse{
		Token:            token,
		User:             user.Email,
		Role:             string(user.Role),
		OrganizationID:   orgIDStr,
		OrganizationName: orgName,
	})
}

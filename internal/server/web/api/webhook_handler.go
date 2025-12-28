package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	"github.com/pandeptwidyaop/grok/internal/server/web/middleware"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
	"gorm.io/gorm"
)

// WebhookHandler handles webhook-related API requests
type WebhookHandler struct {
	db            *gorm.DB
	tunnelManager *tunnel.Manager
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(db *gorm.DB, tunnelManager *tunnel.Manager) *WebhookHandler {
	return &WebhookHandler{
		db:            db,
		tunnelManager: tunnelManager,
	}
}

// ===================================================================
// Webhook App Management
// ===================================================================

// CreateApp creates a new webhook app
func (wh *WebhookHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse request
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate app name
	if err := utils.ValidateWebhookAppNameOrError(req.Name); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Normalize app name
	req.Name = utils.NormalizeWebhookAppName(req.Name)

	// Get user's organization ID
	if claims.OrganizationID == nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "user must belong to an organization"})
		return
	}

	orgID, err := uuid.Parse(*claims.OrganizationID)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization ID"})
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	// Check if app with same name already exists in organization
	var existingApp models.WebhookApp
	if err := wh.db.Where("organization_id = ? AND name = ?", orgID, req.Name).First(&existingApp).Error; err == nil {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "webhook app with this name already exists in your organization"})
		return
	}

	// Create webhook app
	app := models.WebhookApp{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Name:           req.Name,
		Description:    req.Description,
		IsActive:       true,
	}

	if err := wh.db.Create(&app).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to create webhook app")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create webhook app"})
		return
	}

	// Load organization for webhook URL building
	wh.db.First(&app.Organization, app.OrganizationID)

	logger.InfoEvent().
		Str("app_id", app.ID.String()).
		Str("app_name", app.Name).
		Str("org_id", orgID.String()).
		Msg("Webhook app created")

	// Build webhook URL with proper protocol and port
	webhookURL := wh.buildWebhookURL(&app)

	// Return app with webhook URL
	response := struct {
		models.WebhookApp
		WebhookURL string `json:"webhook_url"`
	}{
		WebhookApp: app,
		WebhookURL: webhookURL,
	}

	respondJSON(w, http.StatusCreated, response)
}

// ListApps lists all webhook apps for the user's organization (or all orgs for super_admin)
func (wh *WebhookHandler) ListApps(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Query webhook apps with organization and user
	var apps []models.WebhookApp
	query := wh.db.Model(&models.WebhookApp{})

	// Super admin can see all webhook apps from all organizations
	if claims.Role != "super_admin" {
		// For non-super-admin, filter by organization
		if claims.OrganizationID == nil {
			respondJSON(w, http.StatusOK, []models.WebhookApp{})
			return
		}

		orgID, err := uuid.Parse(*claims.OrganizationID)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization ID"})
			return
		}

		query = query.Where("organization_id = ?", orgID)
	}

	if err := query.
		Preload("Routes").
		Preload("Organization").
		Preload("User").
		Order("created_at DESC").
		Find(&apps).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to list webhook apps")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list webhook apps"})
		return
	}

	// Build response with webhook URLs and owner info
	type AppResponse struct {
		models.WebhookApp
		WebhookURL       string `json:"webhook_url"`
		OwnerName        string `json:"owner_name,omitempty"`
		OwnerEmail       string `json:"owner_email,omitempty"`
		OrganizationName string `json:"organization_name,omitempty"`
	}

	response := make([]AppResponse, len(apps))
	for i, app := range apps {
		resp := AppResponse{
			WebhookApp: app,
			WebhookURL: wh.buildWebhookURL(&app),
		}
		if app.User != nil {
			resp.OwnerName = app.User.Name
			resp.OwnerEmail = app.User.Email
		}
		if app.Organization != nil {
			resp.OrganizationName = app.Organization.Name
		}
		response[i] = resp
	}

	respondJSON(w, http.StatusOK, response)
}

// GetApp retrieves a single webhook app by ID
func (wh *WebhookHandler) GetApp(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse app ID from path
	appID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	// Query app
	var app models.WebhookApp
	if err := wh.db.Preload("Routes.Tunnel").Preload("Organization").First(&app, appID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
			return
		}
		logger.ErrorEvent().Err(err).Msg("Failed to get webhook app")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get webhook app"})
		return
	}

	// Verify organization membership (super_admin can access all apps)
	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Build webhook URL
	webhookURL := wh.buildWebhookURL(&app)

	// Return app with webhook URL
	response := struct {
		models.WebhookApp
		WebhookURL string `json:"webhook_url"`
	}{
		WebhookApp: app,
		WebhookURL: webhookURL,
	}

	respondJSON(w, http.StatusOK, response)
}

// UpdateApp updates a webhook app
func (wh *WebhookHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse app ID
	appID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	// Parse request
	var req struct {
		Description *string `json:"description,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Get existing app
	var app models.WebhookApp
	if err := wh.db.First(&app, appID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get webhook app"})
		return
	}

	// Verify organization membership (super_admin can access all apps)
	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Update fields
	if req.Description != nil {
		app.Description = *req.Description
	}

	if err := wh.db.Save(&app).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to update webhook app")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update webhook app"})
		return
	}

	respondJSON(w, http.StatusOK, app)
}

// DeleteApp deletes a webhook app
func (wh *WebhookHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse app ID
	appID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	// Get existing app
	var app models.WebhookApp
	if err := wh.db.First(&app, appID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get webhook app"})
		return
	}

	// Verify organization membership (super_admin can access all apps)
	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Check permissions: org_admin or app creator can delete
	isOrgAdmin := claims.Role == string(models.RoleOrgAdmin) || claims.Role == string(models.RoleSuperAdmin)
	isCreator := app.UserID.String() == claims.UserID

	if !isOrgAdmin && !isCreator {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "only org admin or app creator can delete"})
		return
	}

	// Delete app (CASCADE will delete routes and events)
	if err := wh.db.Delete(&app).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to delete webhook app")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete webhook app"})
		return
	}

	logger.InfoEvent().
		Str("app_id", appID.String()).
		Str("app_name", app.Name).
		Msg("Webhook app deleted")

	respondJSON(w, http.StatusNoContent, nil)
}

// ToggleApp toggles webhook app active status
func (wh *WebhookHandler) ToggleApp(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse app ID
	appID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	// Get existing app
	var app models.WebhookApp
	if err := wh.db.First(&app, appID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get webhook app"})
		return
	}

	// Verify organization membership (super_admin can access all apps)
	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Toggle status
	app.IsActive = !app.IsActive

	if err := wh.db.Save(&app).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to toggle webhook app")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to toggle webhook app"})
		return
	}

	respondJSON(w, http.StatusOK, app)
}

// ===================================================================
// Webhook Route Management
// ===================================================================

// ListRoutes lists all routes for a webhook app
func (wh *WebhookHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse app ID
	appID, err := uuid.Parse(r.PathValue("app_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	// Verify app belongs to user's organization (super_admin can access all)
	var app models.WebhookApp
	if err := wh.db.First(&app, appID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get webhook app"})
		return
	}

	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Query routes
	var routes []models.WebhookRoute
	if err := wh.db.Where("webhook_app_id = ?", appID).
		Preload("Tunnel").
		Order("priority ASC").
		Find(&routes).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to list webhook routes")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list routes"})
		return
	}

	respondJSON(w, http.StatusOK, routes)
}

// AddRoute adds a new route to a webhook app
func (wh *WebhookHandler) AddRoute(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse app ID
	appID, err := uuid.Parse(r.PathValue("app_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	// Parse request
	var req struct {
		TunnelID string `json:"tunnel_id"`
		Priority int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	tunnelID, err := uuid.Parse(req.TunnelID)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid tunnel ID"})
		return
	}

	// Verify app belongs to user's organization (super_admin can access all)
	var app models.WebhookApp
	if err := wh.db.First(&app, appID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get webhook app"})
		return
	}

	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Verify tunnel exists and belongs to same organization
	var tunnel models.Tunnel
	if err := wh.db.First(&tunnel, tunnelID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "tunnel not found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get tunnel"})
		return
	}

	if tunnel.OrganizationID.String() != app.OrganizationID.String() {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "tunnel does not belong to the same organization"})
		return
	}

	// Check if route already exists
	var existingRoute models.WebhookRoute
	if err := wh.db.Where("webhook_app_id = ? AND tunnel_id = ?", appID, tunnelID).First(&existingRoute).Error; err == nil {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "route already exists for this tunnel"})
		return
	}

	// Set default priority if not provided
	if req.Priority == 0 {
		req.Priority = 100
	}

	// Create route
	route := models.WebhookRoute{
		ID:           uuid.New(),
		WebhookAppID: appID,
		TunnelID:     tunnelID,
		IsEnabled:    true,
		Priority:     req.Priority,
		HealthStatus: "unknown",
	}

	if err := wh.db.Create(&route).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to create webhook route")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create route"})
		return
	}

	// Load tunnel relation
	wh.db.Preload("Tunnel").First(&route, route.ID)

	logger.InfoEvent().
		Str("route_id", route.ID.String()).
		Str("app_id", appID.String()).
		Str("tunnel_id", tunnelID.String()).
		Msg("Webhook route created")

	respondJSON(w, http.StatusCreated, route)
}

// UpdateRoute updates a webhook route
func (wh *WebhookHandler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse IDs
	appID, err := uuid.Parse(r.PathValue("app_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	routeID, err := uuid.Parse(r.PathValue("route_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid route ID"})
		return
	}

	// Parse request
	var req struct {
		Priority *int `json:"priority,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Verify ownership (super_admin can access all)
	var app models.WebhookApp
	if err := wh.db.First(&app, appID).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
		return
	}

	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Get route
	var route models.WebhookRoute
	if err := wh.db.Where("id = ? AND webhook_app_id = ?", routeID, appID).First(&route).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "route not found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get route"})
		return
	}

	// Update fields
	if req.Priority != nil {
		route.Priority = *req.Priority
	}

	if err := wh.db.Save(&route).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to update webhook route")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update route"})
		return
	}

	respondJSON(w, http.StatusOK, route)
}

// ToggleRoute toggles route enabled status
func (wh *WebhookHandler) ToggleRoute(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse IDs
	appID, err := uuid.Parse(r.PathValue("app_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	routeID, err := uuid.Parse(r.PathValue("route_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid route ID"})
		return
	}

	// Verify ownership (super_admin can access all)
	var app models.WebhookApp
	if err := wh.db.First(&app, appID).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
		return
	}

	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Get route
	var route models.WebhookRoute
	if err := wh.db.Where("id = ? AND webhook_app_id = ?", routeID, appID).First(&route).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "route not found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get route"})
		return
	}

	// Toggle
	route.IsEnabled = !route.IsEnabled

	if err := wh.db.Save(&route).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to toggle webhook route")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to toggle route"})
		return
	}

	respondJSON(w, http.StatusOK, route)
}

// DeleteRoute deletes a webhook route
func (wh *WebhookHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse IDs
	appID, err := uuid.Parse(r.PathValue("app_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	routeID, err := uuid.Parse(r.PathValue("route_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid route ID"})
		return
	}

	// Verify ownership (super_admin can access all)
	var app models.WebhookApp
	if err := wh.db.First(&app, appID).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
		return
	}

	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Delete route
	result := wh.db.Where("id = ? AND webhook_app_id = ?", routeID, appID).Delete(&models.WebhookRoute{})
	if result.Error != nil {
		logger.ErrorEvent().Err(result.Error).Msg("Failed to delete webhook route")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete route"})
		return
	}

	if result.RowsAffected == 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "route not found"})
		return
	}

	logger.InfoEvent().
		Str("route_id", routeID.String()).
		Str("app_id", appID.String()).
		Msg("Webhook route deleted")

	respondJSON(w, http.StatusNoContent, nil)
}

// ===================================================================
// Webhook Events & Stats
// ===================================================================

// GetEvents retrieves webhook events for an app
func (wh *WebhookHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse app ID
	appID, err := uuid.Parse(r.PathValue("app_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	// Verify ownership (super_admin can access all)
	var app models.WebhookApp
	if err := wh.db.First(&app, appID).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
		return
	}

	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Get limit from query params (default 100)
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := parseInt(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 1000 {
				limit = 1000 // Cap at 1000
			}
		}
	}

	// Query events
	var events []models.WebhookEvent
	if err := wh.db.Where("webhook_app_id = ?", appID).
		Order("created_at DESC").
		Limit(limit).
		Find(&events).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to get webhook events")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get events"})
		return
	}

	respondJSON(w, http.StatusOK, events)
}

// GetStats retrieves statistics for a webhook app
func (wh *WebhookHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse app ID
	appID, err := uuid.Parse(r.PathValue("app_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	// Verify ownership
	var app models.WebhookApp
	if err := wh.db.First(&app, appID).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
		return
	}

	// Super admin can view stats for any app, others must own the organization
	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Calculate stats
	var stats struct {
		TotalEvents    int64   `json:"total_events"`
		SuccessCount   int64   `json:"success_count"`
		FailureCount   int64   `json:"failure_count"`
		AverageDuration float64 `json:"average_duration_ms"`
		TotalBytesIn   int64   `json:"total_bytes_in"`
		TotalBytesOut  int64   `json:"total_bytes_out"`
	}

	// Total events
	wh.db.Model(&models.WebhookEvent{}).Where("webhook_app_id = ?", appID).Count(&stats.TotalEvents)

	// Success/failure counts
	wh.db.Model(&models.WebhookEvent{}).
		Where("webhook_app_id = ? AND routing_status = ?", appID, "success").
		Count(&stats.SuccessCount)

	stats.FailureCount = stats.TotalEvents - stats.SuccessCount

	// Average duration
	var avgDuration sql.NullFloat64
	wh.db.Model(&models.WebhookEvent{}).
		Where("webhook_app_id = ?", appID).
		Select("AVG(duration_ms)").
		Scan(&avgDuration)

	if avgDuration.Valid {
		stats.AverageDuration = avgDuration.Float64
	}

	// Total bytes
	wh.db.Model(&models.WebhookEvent{}).
		Where("webhook_app_id = ?", appID).
		Select("SUM(bytes_in) as total_bytes_in, SUM(bytes_out) as total_bytes_out").
		Scan(&stats)

	respondJSON(w, http.StatusOK, stats)
}

// GetEventDetail retrieves detailed information for a single webhook event.
func (wh *WebhookHandler) GetEventDetail(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse app ID and event ID
	appID, err := uuid.Parse(r.PathValue("app_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid app ID"})
		return
	}

	eventID, err := uuid.Parse(r.PathValue("event_id"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid event ID"})
		return
	}

	// Verify app ownership
	var app models.WebhookApp
	if err := wh.db.First(&app, appID).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook app not found"})
		return
	}

	if claims.Role != "super_admin" {
		if claims.OrganizationID == nil || app.OrganizationID.String() != *claims.OrganizationID {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Fetch event with tunnel responses
	var event models.WebhookEvent
	if err := wh.db.Where("id = ? AND webhook_app_id = ?", eventID, appID).
		First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get event"})
		return
	}

	// Fetch per-tunnel responses
	var tunnelResponses []models.WebhookTunnelResponse
	if err := wh.db.Where("webhook_event_id = ?", eventID).
		Order("duration_ms ASC"). // Fastest first
		Find(&tunnelResponses).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to get tunnel responses")
		// Continue without tunnel responses (not critical)
	}

	// Parse JSON headers back to map[string][]string
	var requestHeaders map[string][]string
	if event.RequestHeaders != "" {
		json.Unmarshal([]byte(event.RequestHeaders), &requestHeaders)
	}

	var responseHeaders map[string][]string
	if event.ResponseHeaders != "" {
		json.Unmarshal([]byte(event.ResponseHeaders), &responseHeaders)
	}

	// Build response
	type TunnelResponseDetail struct {
		ID              string              `json:"id"`
		TunnelID        string              `json:"tunnel_id"`
		TunnelSubdomain string              `json:"tunnel_subdomain"`
		StatusCode      int                 `json:"status_code"`
		DurationMs      int64               `json:"duration_ms"`
		Success         bool                `json:"success"`
		ErrorMessage    string              `json:"error_message,omitempty"`
		ResponseHeaders map[string][]string `json:"response_headers,omitempty"`
		ResponseBody    string              `json:"response_body,omitempty"`
	}

	tunnelDetails := make([]TunnelResponseDetail, 0, len(tunnelResponses))
	for _, tr := range tunnelResponses {
		var respHeaders map[string][]string
		if tr.ResponseHeaders != "" {
			json.Unmarshal([]byte(tr.ResponseHeaders), &respHeaders)
		}

		tunnelDetails = append(tunnelDetails, TunnelResponseDetail{
			ID:              tr.ID.String(),
			TunnelID:        tr.TunnelID.String(),
			TunnelSubdomain: tr.TunnelSubdomain,
			StatusCode:      tr.StatusCode,
			DurationMs:      tr.DurationMs,
			Success:         tr.Success,
			ErrorMessage:    tr.ErrorMessage,
			ResponseHeaders: respHeaders,
			ResponseBody:    tr.ResponseBody,
		})
	}

	response := struct {
		models.WebhookEvent
		RequestHeadersParsed  map[string][]string    `json:"request_headers_parsed"`
		ResponseHeadersParsed map[string][]string    `json:"response_headers_parsed"`
		TunnelResponses       []TunnelResponseDetail `json:"tunnel_responses"`
	}{
		WebhookEvent:          event,
		RequestHeadersParsed:  requestHeaders,
		ResponseHeadersParsed: responseHeaders,
		TunnelResponses:       tunnelDetails,
	}

	respondJSON(w, http.StatusOK, response)
}

// Helper function to parse int from string
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// buildWebhookURL constructs the full webhook URL for an app
func (wh *WebhookHandler) buildWebhookURL(app *models.WebhookApp) string {
	// Load organization if not already loaded
	if app.Organization.ID == uuid.Nil {
		wh.db.First(&app.Organization, app.OrganizationID)
	}

	if app.Organization.ID == uuid.Nil {
		return ""
	}

	// Build webhook subdomain: {app-name}-{org-subdomain}-webhook
	webhookSubdomain := fmt.Sprintf("%s-%s-webhook", app.Name, app.Organization.Subdomain)

	// Get server config from tunnel manager
	var scheme string
	var port int
	var defaultPort int

	if wh.tunnelManager.IsTLSEnabled() {
		scheme = "https"
		port = wh.tunnelManager.GetHTTPSPort()
		defaultPort = 443
	} else {
		scheme = "http"
		port = wh.tunnelManager.GetHTTPPort()
		defaultPort = 80
	}

	// Build full host
	host := fmt.Sprintf("%s.%s", webhookSubdomain, wh.tunnelManager.GetBaseDomain())

	// Build URL with or without port
	var baseURL string
	if port != defaultPort && port != 0 {
		baseURL = fmt.Sprintf("%s://%s:%d", scheme, host, port)
	} else {
		baseURL = fmt.Sprintf("%s://%s", scheme, host)
	}

	// Path is user's webhook path (no app name in path anymore)
	return fmt.Sprintf("%s/*", baseURL)
}

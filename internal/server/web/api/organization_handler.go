package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/web/middleware"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
	"gorm.io/gorm"
)

// OrganizationHandler handles organization-related API requests
type OrganizationHandler struct {
	db     *gorm.DB
	domain string
}

// NewOrganizationHandler creates a new organization handler
func NewOrganizationHandler(db *gorm.DB, domain string) *OrganizationHandler {
	return &OrganizationHandler{
		db:     db,
		domain: domain,
	}
}

// DTOs for organization management

type CreateOrganizationRequest struct {
	Name        string `json:"name"`
	Subdomain   string `json:"subdomain"`
	Description string `json:"description,omitempty"`
}

type UpdateOrganizationRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

type CreateOrgUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"` // org_admin or org_user
}

type UpdateUserRoleRequest struct {
	Role string `json:"role"`
}

type OrganizationResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Subdomain   string `json:"subdomain"`
	FullDomain  string `json:"full_domain"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// toResponse converts an Organization model to OrganizationResponse
func (h *OrganizationHandler) toResponse(org *models.Organization) OrganizationResponse {
	return OrganizationResponse{
		ID:          org.ID.String(),
		Name:        org.Name,
		Subdomain:   org.Subdomain,
		FullDomain:  fmt.Sprintf("%s.%s", org.Subdomain, h.domain),
		Description: org.Description,
		IsActive:    org.IsActive,
		CreatedAt:   org.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   org.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// Organization CRUD Handlers (Super Admin Only)

// CreateOrganization creates a new organization
func (h *OrganizationHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	var req CreateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" || req.Subdomain == "" {
		respondError(w, http.StatusBadRequest, "Name and subdomain are required")
		return
	}

	// Validate subdomain format
	if !utils.IsValidSubdomain(req.Subdomain) {
		respondError(w, http.StatusBadRequest, "Invalid subdomain format (4-63 chars, alphanumeric and hyphens)")
		return
	}

	// Normalize subdomain
	subdomain := utils.NormalizeSubdomain(req.Subdomain)

	// Check if subdomain already exists
	var existing models.Organization
	if err := h.db.Where("subdomain = ?", subdomain).First(&existing).Error; err == nil {
		respondError(w, http.StatusConflict, "Organization subdomain already taken")
		return
	}

	// Create organization
	org := &models.Organization{
		Name:        req.Name,
		Subdomain:   subdomain,
		Description: req.Description,
		IsActive:    true,
	}

	if err := h.db.Create(org).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to create organization")
		respondError(w, http.StatusInternalServerError, "Failed to create organization")
		return
	}

	logger.InfoEvent().
		Str("org_id", org.ID.String()).
		Str("subdomain", org.Subdomain).
		Msg("Organization created")

	respondJSON(w, http.StatusCreated, h.toResponse(org))
}

// ListOrganizations lists all organizations
func (h *OrganizationHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())

	var orgs []models.Organization
	query := h.db

	// Org admins only see their organization
	if claims.Role != string(models.RoleSuperAdmin) && claims.OrganizationID != nil {
		orgID, err := uuid.Parse(*claims.OrganizationID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid organization ID")
			return
		}
		query = query.Where("id = ?", orgID)
	}

	if err := query.Find(&orgs).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to list organizations")
		respondError(w, http.StatusInternalServerError, "Failed to list organizations")
		return
	}

	// Convert to response DTOs
	responses := make([]OrganizationResponse, len(orgs))
	for i, org := range orgs {
		responses[i] = h.toResponse(&org)
	}

	respondJSON(w, http.StatusOK, responses)
}

// GetOrganization gets a single organization by ID
func (h *OrganizationHandler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if orgID == "" {
		respondError(w, http.StatusBadRequest, "Organization ID required")
		return
	}

	var org models.Organization
	if err := h.db.Where("id = ?", orgID).First(&org).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Organization not found")
			return
		}
		logger.ErrorEvent().Err(err).Msg("Failed to get organization")
		respondError(w, http.StatusInternalServerError, "Failed to get organization")
		return
	}

	respondJSON(w, http.StatusOK, h.toResponse(&org))
}

// UpdateOrganization updates an organization
func (h *OrganizationHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if orgID == "" {
		respondError(w, http.StatusBadRequest, "Organization ID required")
		return
	}

	var req UpdateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var org models.Organization
	if err := h.db.Where("id = ?", orgID).First(&org).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Organization not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to get organization")
		return
	}

	// Update fields
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		if err := h.db.Model(&org).Updates(updates).Error; err != nil {
			logger.ErrorEvent().Err(err).Msg("Failed to update organization")
			respondError(w, http.StatusInternalServerError, "Failed to update organization")
			return
		}
	}

	// Reload to get updated data
	h.db.Where("id = ?", orgID).First(&org)

	respondJSON(w, http.StatusOK, h.toResponse(&org))
}

// DeleteOrganization deletes an organization
func (h *OrganizationHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if orgID == "" {
		respondError(w, http.StatusBadRequest, "Organization ID required")
		return
	}

	result := h.db.Delete(&models.Organization{}, "id = ?", orgID)
	if result.Error != nil {
		logger.ErrorEvent().Err(result.Error).Msg("Failed to delete organization")
		respondError(w, http.StatusInternalServerError, "Failed to delete organization")
		return
	}

	if result.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Organization not found")
		return
	}

	logger.InfoEvent().Str("org_id", orgID).Msg("Organization deleted")
	respondJSON(w, http.StatusOK, map[string]string{"message": "Organization deleted successfully"})
}

// ToggleOrganization activates/deactivates an organization
func (h *OrganizationHandler) ToggleOrganization(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if orgID == "" {
		respondError(w, http.StatusBadRequest, "Organization ID required")
		return
	}

	var org models.Organization
	if err := h.db.Where("id = ?", orgID).First(&org).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Organization not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to get organization")
		return
	}

	// Toggle active status
	org.IsActive = !org.IsActive
	if err := h.db.Save(&org).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to toggle organization status")
		respondError(w, http.StatusInternalServerError, "Failed to toggle organization status")
		return
	}

	respondJSON(w, http.StatusOK, h.toResponse(&org))
}

// User Management Handlers (Org Admin + Super Admin)

// ListOrgUsers lists users in an organization
func (h *OrganizationHandler) ListOrgUsers(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")
	if orgID == "" {
		respondError(w, http.StatusBadRequest, "Organization ID required")
		return
	}

	var users []models.User
	if err := h.db.Where("organization_id = ?", orgID).Find(&users).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to list users")
		respondError(w, http.StatusInternalServerError, "Failed to list users")
		return
	}

	respondJSON(w, http.StatusOK, users)
}

// CreateOrgUser creates a new user in an organization
func (h *OrganizationHandler) CreateOrgUser(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")
	if orgID == "" {
		respondError(w, http.StatusBadRequest, "Organization ID required")
		return
	}

	var req CreateOrgUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Email == "" || req.Name == "" || req.Password == "" || req.Role == "" {
		respondError(w, http.StatusBadRequest, "Email, name, password, and role are required")
		return
	}

	// Validate role
	if req.Role != string(models.RoleOrgAdmin) && req.Role != string(models.RoleOrgUser) {
		respondError(w, http.StatusBadRequest, "Invalid role (must be org_admin or org_user)")
		return
	}

	// Verify organization exists and is active
	var org models.Organization
	if err := h.db.Where("id = ? AND is_active = ?", orgID, true).First(&org).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Organization not found or inactive")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to verify organization")
		return
	}

	// Check if email already exists
	var existingUser models.User
	if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		respondError(w, http.StatusConflict, "User with this email already exists")
		return
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	orgUUID, _ := uuid.Parse(orgID)
	user := &models.User{
		OrganizationID: &orgUUID,
		Email:          req.Email,
		Name:           req.Name,
		Password:       hashedPassword,
		Role:           models.UserRole(req.Role),
		IsActive:       true,
	}

	if err := h.db.Create(user).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to create user")
		respondError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	logger.InfoEvent().
		Str("user_id", user.ID.String()).
		Str("org_id", orgID).
		Str("role", req.Role).
		Msg("User created in organization")

	// Don't return password
	user.Password = ""
	respondJSON(w, http.StatusCreated, user)
}

// GetOrgUser gets a user from an organization
func (h *OrganizationHandler) GetOrgUser(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")
	userID := r.PathValue("user_id")

	var user models.User
	if err := h.db.Where("id = ? AND organization_id = ?", userID, orgID).
		Select("id, email, name, role, is_active, created_at, updated_at, organization_id").
		First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "User not found in organization")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}

	user.Password = ""
	respondJSON(w, http.StatusOK, user)
}

// UpdateUserRole updates a user's role within an organization
func (h *OrganizationHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")
	userID := r.PathValue("user_id")

	var req UpdateUserRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate role
	if req.Role != string(models.RoleOrgAdmin) && req.Role != string(models.RoleOrgUser) {
		respondError(w, http.StatusBadRequest, "Invalid role (must be org_admin or org_user)")
		return
	}

	// Verify user belongs to organization
	var user models.User
	if err := h.db.Where("id = ? AND organization_id = ?", userID, orgID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "User not found in organization")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}

	// Update role
	if err := h.db.Model(&user).Update("role", req.Role).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to update user role")
		respondError(w, http.StatusInternalServerError, "Failed to update user role")
		return
	}

	logger.InfoEvent().
		Str("user_id", userID).
		Str("org_id", orgID).
		Str("new_role", req.Role).
		Msg("User role updated")

	user.Password = ""
	respondJSON(w, http.StatusOK, user)
}

// DeleteOrgUser removes a user from an organization
func (h *OrganizationHandler) DeleteOrgUser(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")
	userID := r.PathValue("user_id")

	claims := middleware.GetClaimsFromContext(r.Context())

	// Prevent self-deletion
	if claims.UserID == userID {
		respondError(w, http.StatusBadRequest, "Cannot delete your own account")
		return
	}

	// Delete user (cascade to tokens, tunnels, etc.)
	result := h.db.Where("id = ? AND organization_id = ?", userID, orgID).Delete(&models.User{})
	if result.Error != nil {
		logger.ErrorEvent().Err(result.Error).Msg("Failed to delete user")
		respondError(w, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	if result.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "User not found in organization")
		return
	}

	logger.InfoEvent().
		Str("user_id", userID).
		Str("org_id", orgID).
		Msg("User deleted from organization")

	respondJSON(w, http.StatusOK, map[string]string{"message": "User deleted successfully"})
}

// GetOrgStats gets statistics for an organization
func (h *OrganizationHandler) GetOrgStats(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")

	var stats struct {
		TotalUsers    int64 `json:"total_users"`
		ActiveUsers   int64 `json:"active_users"`
		TotalTunnels  int64 `json:"total_tunnels"`
		ActiveTunnels int64 `json:"active_tunnels"`
		TotalRequests int64 `json:"total_requests"`
		TotalBytesIn  int64 `json:"total_bytes_in"`
		TotalBytesOut int64 `json:"total_bytes_out"`
	}

	// Count users
	h.db.Model(&models.User{}).Where("organization_id = ?", orgID).Count(&stats.TotalUsers)
	h.db.Model(&models.User{}).Where("organization_id = ? AND is_active = ?", orgID, true).Count(&stats.ActiveUsers)

	// Count tunnels
	h.db.Model(&models.Tunnel{}).Where("organization_id = ?", orgID).Count(&stats.TotalTunnels)
	h.db.Model(&models.Tunnel{}).Where("organization_id = ? AND status = ?", orgID, "active").Count(&stats.ActiveTunnels)

	// Sum metrics
	h.db.Model(&models.Tunnel{}).Where("organization_id = ?", orgID).
		Select("COALESCE(SUM(requests_count), 0)").Row().Scan(&stats.TotalRequests)
	h.db.Model(&models.Tunnel{}).Where("organization_id = ?", orgID).
		Select("COALESCE(SUM(bytes_in), 0)").Row().Scan(&stats.TotalBytesIn)
	h.db.Model(&models.Tunnel{}).Where("organization_id = ?", orgID).
		Select("COALESCE(SUM(bytes_out), 0)").Row().Scan(&stats.TotalBytesOut)

	respondJSON(w, http.StatusOK, stats)
}

// ListOrgTunnels lists tunnels for an organization
func (h *OrganizationHandler) ListOrgTunnels(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")

	var tunnels []models.Tunnel
	// Show both active and offline tunnels (all persistent tunnels)
	if err := h.db.Where("organization_id = ? AND status IN (?)", orgID, []string{"active", "offline"}).
		Order("status ASC, connected_at DESC").
		Find(&tunnels).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to list org tunnels")
		respondError(w, http.StatusInternalServerError, "Failed to list tunnels")
		return
	}

	respondJSON(w, http.StatusOK, tunnels)
}

// ResetUserPassword resets a user's password
func (h *OrganizationHandler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")
	userID := r.PathValue("user_id")

	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.NewPassword == "" || len(req.NewPassword) < 8 {
		respondError(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}

	// Hash the new password
	hashedPassword, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to hash password")
		respondError(w, http.StatusInternalServerError, "Failed to reset password")
		return
	}

	// Update user password
	result := h.db.Model(&models.User{}).
		Where("id = ? AND organization_id = ?", userID, orgID).
		Update("password", hashedPassword)

	if result.Error != nil {
		logger.ErrorEvent().Err(result.Error).Msg("Failed to update password")
		respondError(w, http.StatusInternalServerError, "Failed to reset password")
		return
	}

	if result.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "User not found in organization")
		return
	}

	logger.InfoEvent().
		Str("user_id", userID).
		Str("org_id", orgID).
		Msg("Password reset for user")

	respondJSON(w, http.StatusOK, map[string]string{"message": "Password reset successfully"})
}

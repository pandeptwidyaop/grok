package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	"github.com/pandeptwidyaop/grok/internal/server/config"
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
	config        *config.Config
	authMW        *middleware.AuthMiddleware
}

// NewHandler creates a new dashboard API handler
func NewHandler(db *gorm.DB, tokenService *auth.TokenService, tunnelManager *tunnel.Manager, cfg *config.Config) *Handler {
	return &Handler{
		db:            db,
		tokenService:  tokenService,
		tunnelManager: tunnelManager,
		config:        cfg,
		authMW:        middleware.NewAuthMiddleware(cfg.Auth.JWTSecret),
	}
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Public routes
	mux.HandleFunc("POST /api/auth/login", h.login)

	// Protected routes (require JWT)
	mux.Handle("GET /api/tokens", h.authMW.Protect(http.HandlerFunc(h.listTokens)))
	mux.Handle("POST /api/tokens", h.authMW.Protect(http.HandlerFunc(h.createToken)))
	mux.Handle("DELETE /api/tokens/{id}", h.authMW.Protect(http.HandlerFunc(h.deleteToken)))
	mux.Handle("PATCH /api/tokens/{id}/toggle", h.authMW.Protect(http.HandlerFunc(h.toggleToken)))

	mux.Handle("GET /api/tunnels", h.authMW.Protect(http.HandlerFunc(h.listTunnels)))
	mux.Handle("GET /api/tunnels/{id}", h.authMW.Protect(http.HandlerFunc(h.getTunnel)))
	mux.Handle("GET /api/tunnels/{id}/logs", h.authMW.Protect(http.HandlerFunc(h.getTunnelLogs)))

	mux.Handle("GET /api/stats", h.authMW.Protect(http.HandlerFunc(h.getStats)))
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
	var tokens []models.AuthToken
	if err := h.db.Find(&tokens).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to list tokens")
		respondError(w, http.StatusInternalServerError, "Failed to list tokens")
		return
	}

	respondJSON(w, http.StatusOK, tokens)
}

type createTokenRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

func (h *Handler) createToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "Token name is required")
		return
	}

	// Get or create system user
	var user models.User
	if err := h.db.Where("email = ?", "system@grok.local").First(&user).Error; err != nil {
		// Create system user if not exists
		user = models.User{
			Email:    "system@grok.local",
			Name:     "System User",
			IsActive: true,
		}
		if err := h.db.Create(&user).Error; err != nil {
			logger.ErrorEvent().Err(err).Msg("Failed to create system user")
			respondError(w, http.StatusInternalServerError, "Failed to create user")
			return
		}
	}

	token, rawToken, err := h.tokenService.CreateToken(r.Context(), user.ID, req.Name, nil)
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
	var tunnels []models.Tunnel
	if err := h.db.Where("status = ?", "active").Find(&tunnels).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to list tunnels")
		respondError(w, http.StatusInternalServerError, "Failed to list tunnels")
		return
	}

	respondJSON(w, http.StatusOK, tunnels)
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

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	var logs []models.RequestLog
	if err := h.db.Where("tunnel_id = ?", tunnelID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		logger.ErrorEvent().Err(err).Str("tunnel_id", tunnelID).Msg("Failed to get tunnel logs")
		respondError(w, http.StatusInternalServerError, "Failed to get tunnel logs")
		return
	}

	respondJSON(w, http.StatusOK, logs)
}

// Stats handler
func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	var stats struct {
		TotalTunnels   int64 `json:"total_tunnels"`
		ActiveTunnels  int64 `json:"active_tunnels"`
		TotalRequests  int64 `json:"total_requests"`
		TotalBytesIn   int64 `json:"total_bytes_in"`
		TotalBytesOut  int64 `json:"total_bytes_out"`
	}

	// Count tunnels
	h.db.Model(&models.Tunnel{}).Count(&stats.TotalTunnels)
	h.db.Model(&models.Tunnel{}).Where("status = ?", "active").Count(&stats.ActiveTunnels)

	// Sum request stats from tunnels
	h.db.Model(&models.Tunnel{}).
		Select("COALESCE(SUM(requests_count), 0)").
		Row().
		Scan(&stats.TotalRequests)

	h.db.Model(&models.Tunnel{}).
		Select("COALESCE(SUM(bytes_in), 0)").
		Row().
		Scan(&stats.TotalBytesIn)

	h.db.Model(&models.Tunnel{}).
		Select("COALESCE(SUM(bytes_out), 0)").
		Row().
		Scan(&stats.TotalBytesOut)

	respondJSON(w, http.StatusOK, stats)
}

// Auth handlers

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
	User  string `json:"user"`
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

	// Check against admin credentials from database
	var user models.User
	if err := h.db.Where("email = ?", req.Username).First(&user).Error; err != nil {
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

	// Generate JWT token
	token, err := h.authMW.GenerateToken(user.Email)
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to generate token")
		respondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	respondJSON(w, http.StatusOK, loginResponse{
		Token: token,
		User:  user.Email,
	})
}

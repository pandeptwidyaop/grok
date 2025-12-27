package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	"github.com/pandeptwidyaop/grok/internal/server/web/middleware"
	"github.com/pandeptwidyaop/grok/pkg/logger"
	"github.com/pandeptwidyaop/grok/pkg/utils"
	"gorm.io/gorm"
)

// TwoFAHandler handles two-factor authentication API requests
type TwoFAHandler struct {
	db          *gorm.DB
	totpService *auth.TOTPService
	domain      string
}

// NewTwoFAHandler creates a new 2FA handler
func NewTwoFAHandler(db *gorm.DB, domain string) *TwoFAHandler {
	return &TwoFAHandler{
		db:          db,
		totpService: auth.NewTOTPService(),
		domain:      domain,
	}
}

// GetStatus returns the 2FA status for the current user
func (h *TwoFAHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": user.TwoFactorEnabled,
	})
}

// EnableSetup initiates 2FA setup and returns the QR code
func (h *TwoFAHandler) EnableSetup(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	if user.TwoFactorEnabled {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "2FA is already enabled"})
		return
	}

	// Generate new secret with domain
	secret, qrURL, err := h.totpService.GenerateSecret(h.domain, user.Email)
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to generate TOTP secret")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate secret"})
		return
	}

	// Store secret temporarily (not enabled yet, will be enabled after verification)
	user.TwoFactorSecret = secret
	if err := h.db.Save(&user).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to save TOTP secret")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save secret"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"secret": secret,
		"qr_url": qrURL,
	})
}

// VerifyEnable verifies the TOTP code and enables 2FA
func (h *TwoFAHandler) VerifyEnable(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Code == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "code is required"})
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	if user.TwoFactorSecret == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "2FA setup not initiated"})
		return
	}

	// Validate the code
	if !h.totpService.ValidateCode(user.TwoFactorSecret, req.Code) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid code"})
		return
	}

	// Enable 2FA
	user.TwoFactorEnabled = true
	if err := h.db.Save(&user).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to enable 2FA")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to enable 2FA"})
		return
	}

	logger.InfoEvent().
		Str("user_id", userID.String()).
		Str("email", user.Email).
		Msg("2FA enabled")

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "2FA enabled successfully",
	})
}

// Disable disables 2FA for the current user
func (h *TwoFAHandler) Disable(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Password == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "password is required"})
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	// Verify password before disabling
	if !utils.ComparePassword(user.Password, req.Password) {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid password"})
		return
	}

	// Disable 2FA
	user.TwoFactorEnabled = false
	user.TwoFactorSecret = ""
	if err := h.db.Save(&user).Error; err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to disable 2FA")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to disable 2FA"})
		return
	}

	logger.InfoEvent().
		Str("user_id", userID.String()).
		Str("email", user.Email).
		Msg("2FA disabled")

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "2FA disabled successfully",
	})
}

package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/pandeptwidyaop/grok/internal/db/models"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/pandeptwidyaop/grok/pkg/utils"
)

// TokenService handles token operations.
type TokenService struct {
	db *gorm.DB
}

// NewTokenService creates a new token service.
func NewTokenService(db *gorm.DB) *TokenService {
	return &TokenService{
		db: db,
	}
}

// CreateToken creates a new authentication token for a user.
func (s *TokenService) CreateToken(ctx context.Context, userID uuid.UUID, name string, expiresIn *time.Duration) (*models.AuthToken, string, error) {
	// Generate token
	token, tokenHash, err := utils.GenerateAuthToken()
	if err != nil {
		return nil, "", pkgerrors.Wrap(err, "failed to generate token")
	}

	// Calculate expiry
	var expiresAt *time.Time
	if expiresIn != nil {
		expiry := time.Now().Add(*expiresIn)
		expiresAt = &expiry
	}

	// Create token record
	authToken := &models.AuthToken{
		UserID:    userID,
		TokenHash: tokenHash,
		Name:      name,
		ExpiresAt: expiresAt,
		IsActive:  true,
	}

	if err := s.db.WithContext(ctx).Create(authToken).Error; err != nil {
		return nil, "", pkgerrors.Wrap(err, "failed to create token")
	}

	return authToken, token, nil
}

// ValidateToken validates a token and returns the associated auth token record.
func (s *TokenService) ValidateToken(ctx context.Context, token string) (*models.AuthToken, error) {
	// Hash the token
	tokenHash := utils.HashToken(token)

	// Find token in database
	var authToken models.AuthToken
	err := s.db.WithContext(ctx).
		Where("token_hash = ? AND is_active = ?", tokenHash, true).
		Preload("User").
		First(&authToken).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerrors.ErrInvalidToken
		}
		return nil, pkgerrors.Wrap(err, "failed to query token")
	}

	// Check expiry
	if authToken.ExpiresAt != nil && authToken.ExpiresAt.Before(time.Now()) {
		return nil, pkgerrors.ErrTokenExpired
	}

	// Check if user is active
	if !authToken.User.IsActive {
		return nil, pkgerrors.ErrUnauthorized
	}

	// Update last used timestamp
	now := time.Now()
	authToken.LastUsedAt = &now
	s.db.WithContext(ctx).Model(&authToken).Update("last_used_at", now)

	return &authToken, nil
}

// RevokeToken revokes a token.
func (s *TokenService) RevokeToken(ctx context.Context, tokenID uuid.UUID) error {
	result := s.db.WithContext(ctx).
		Model(&models.AuthToken{}).
		Where("id = ?", tokenID).
		Update("is_active", false)

	if result.Error != nil {
		return pkgerrors.Wrap(result.Error, "failed to revoke token")
	}

	if result.RowsAffected == 0 {
		return pkgerrors.ErrInvalidToken
	}

	return nil
}

// ListTokens lists all tokens for a user.
func (s *TokenService) ListTokens(ctx context.Context, userID uuid.UUID) ([]models.AuthToken, error) {
	var tokens []models.AuthToken
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&tokens).Error
	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to list tokens")
	}

	return tokens, nil
}

// GetTokenByID retrieves a token by its ID.
func (s *TokenService) GetTokenByID(ctx context.Context, tokenID uuid.UUID) (*models.AuthToken, error) {
	var token models.AuthToken
	err := s.db.WithContext(ctx).
		Where("id = ?", tokenID).
		Preload("User").
		First(&token).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerrors.ErrInvalidToken
		}
		return nil, pkgerrors.Wrap(err, "failed to get token")
	}

	return &token, nil
}

// GetUserByID retrieves a user by ID with organization preloaded.
func (s *TokenService) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	var user models.User
	err := s.db.WithContext(ctx).
		Where("id = ?", userID).
		Preload("Organization").
		First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerrors.ErrUserNotFound
		}
		return nil, pkgerrors.Wrap(err, "failed to get user")
	}

	return &user, nil
}

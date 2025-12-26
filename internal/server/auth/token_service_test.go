package auth

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/pandeptwidyaop/grok/internal/db"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/pkg/utils"
)

func setupTestDB(t *testing.T) *gorm.DB {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(database)
	require.NoError(t, err)

	return database
}

func createTestUser(t *testing.T, database *gorm.DB) *models.User {
	user := &models.User{
		Email:    "test@example.com",
		Password: "hashedpassword",
		Name:     "Test User",
		IsActive: true,
	}
	require.NoError(t, database.Create(user).Error)
	return user
}

func createTestToken(t *testing.T, database *gorm.DB, userID uuid.UUID, tokenString string, scopes []string) *models.AuthToken {
	tokenHash := utils.HashToken(tokenString)

	scopesJSON := datatypes.JSON(`[]`)
	if len(scopes) > 0 {
		scopesJSON = datatypes.JSON(`["` + scopes[0] + `"]`)
	}

	token := &models.AuthToken{
		UserID:    userID,
		TokenHash: tokenHash,
		Name:      "Test Token",
		Scopes:    scopesJSON,
		IsActive:  true,
	}
	require.NoError(t, database.Create(token).Error)
	return token
}

func TestValidateToken(t *testing.T) {
	database := setupTestDB(t)
	service := NewTokenService(database)
	ctx := context.Background()

	user := createTestUser(t, database)
	tokenString := "test-token-12345"
	createTestToken(t, database, user.ID, tokenString, []string{"tunnel:create"})

	t.Run("valid token", func(t *testing.T) {
		authToken, err := service.ValidateToken(ctx, tokenString)
		require.NoError(t, err)
		assert.NotNil(t, authToken)
		assert.Equal(t, user.ID, authToken.UserID)
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := service.ValidateToken(ctx, "invalid-token")
		assert.Error(t, err)
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := service.ValidateToken(ctx, "")
		assert.Error(t, err)
	})
}

func TestValidateTokenWithInactiveUser(t *testing.T) {
	database := setupTestDB(t)
	service := NewTokenService(database)
	ctx := context.Background()

	user := createTestUser(t, database)
	tokenString := "test-token-active"
	createTestToken(t, database, user.ID, tokenString, []string{"tunnel:create"})

	// Deactivate user
	user.IsActive = false
	require.NoError(t, database.Save(user).Error)

	_, err := service.ValidateToken(ctx, tokenString)
	assert.Error(t, err)
}

func TestValidateTokenWithInactiveToken(t *testing.T) {
	database := setupTestDB(t)
	service := NewTokenService(database)
	ctx := context.Background()

	user := createTestUser(t, database)
	tokenString := "test-token-inactive"
	token := createTestToken(t, database, user.ID, tokenString, []string{"tunnel:create"})

	// Deactivate token
	token.IsActive = false
	require.NoError(t, database.Save(token).Error)

	_, err := service.ValidateToken(ctx, tokenString)
	assert.Error(t, err)
}

func TestValidateTokenWithExpiredToken(t *testing.T) {
	database := setupTestDB(t)
	service := NewTokenService(database)
	ctx := context.Background()

	user := createTestUser(t, database)
	tokenString := "test-token-expired"
	token := createTestToken(t, database, user.ID, tokenString, []string{"tunnel:create"})

	// Set expiration to past
	expiresAt := time.Now().Add(-1 * time.Hour)
	token.ExpiresAt = &expiresAt
	require.NoError(t, database.Save(token).Error)

	_, err := service.ValidateToken(ctx, tokenString)
	assert.Error(t, err)
}

func TestCreateToken(t *testing.T) {
	database := setupTestDB(t)
	service := NewTokenService(database)
	ctx := context.Background()

	user := createTestUser(t, database)

	// Create token with 7 day expiration
	expiresIn := 7 * 24 * time.Hour
	authToken, tokenString, err := service.CreateToken(ctx, user.ID, "My App Token", &expiresIn)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenString)
	assert.NotNil(t, authToken)
	assert.Equal(t, user.ID, authToken.UserID)
	assert.Equal(t, "My App Token", authToken.Name)

	// Verify token can be validated
	validatedToken, err := service.ValidateToken(ctx, tokenString)
	require.NoError(t, err)
	assert.Equal(t, authToken.ID, validatedToken.ID)
}

func TestRevokeToken(t *testing.T) {
	database := setupTestDB(t)
	service := NewTokenService(database)
	ctx := context.Background()

	user := createTestUser(t, database)
	tokenString := "test-token-revoke"
	token := createTestToken(t, database, user.ID, tokenString, []string{"tunnel:create"})

	// Revoke token
	err := service.RevokeToken(ctx, token.ID)
	require.NoError(t, err)

	// Token should no longer be valid
	_, err = service.ValidateToken(ctx, tokenString)
	assert.Error(t, err)

	// Verify token is marked inactive in database
	var dbToken models.AuthToken
	err = database.First(&dbToken, token.ID).Error
	require.NoError(t, err)
	assert.False(t, dbToken.IsActive)
}

func TestListTokens(t *testing.T) {
	database := setupTestDB(t)
	service := NewTokenService(database)
	ctx := context.Background()

	user := createTestUser(t, database)

	// Create multiple tokens
	createTestToken(t, database, user.ID, "token-1", []string{"tunnel:create"})
	createTestToken(t, database, user.ID, "token-2", []string{"tunnel:delete"})
	createTestToken(t, database, user.ID, "token-3", []string{"tunnel:list"})

	tokens, err := service.ListTokens(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, tokens, 3)

	// Verify different user has no tokens
	otherUser := &models.User{
		Email:    "other@example.com",
		Password: "hashedpassword",
		Name:     "Other User",
		IsActive: true,
	}
	require.NoError(t, database.Create(otherUser).Error)

	otherTokens, err := service.ListTokens(ctx, otherUser.ID)
	require.NoError(t, err)
	assert.Len(t, otherTokens, 0)
}

func TestUpdateTokenLastUsed(t *testing.T) {
	database := setupTestDB(t)
	service := NewTokenService(database)
	ctx := context.Background()

	user := createTestUser(t, database)
	tokenString := "test-token-usage"
	token := createTestToken(t, database, user.ID, tokenString, []string{"tunnel:create"})

	// Initially last_used_at should be nil
	assert.Nil(t, token.LastUsedAt)

	// Validate token (should update last_used_at)
	_, err := service.ValidateToken(ctx, tokenString)
	require.NoError(t, err)

	// Check that last_used_at is now set
	var updatedToken models.AuthToken
	err = database.First(&updatedToken, token.ID).Error
	require.NoError(t, err)
	assert.NotNil(t, updatedToken.LastUsedAt)
}

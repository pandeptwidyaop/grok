package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	"github.com/pandeptwidyaop/grok/internal/server/config"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	"github.com/pandeptwidyaop/grok/internal/server/web/middleware"
	"github.com/pandeptwidyaop/grok/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestRespondJSON tests JSON response helper
func TestRespondJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"message": "test"}

	respondJSON(rec, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "test", response["message"])
}

// TestRespondError tests error response helper
func TestRespondError(t *testing.T) {
	rec := httptest.NewRecorder()
	respondError(rec, http.StatusBadRequest, "invalid input")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid input")
}

// TestIsAllowedOrigin tests origin validation
func TestIsAllowedOrigin(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{
					"http://localhost:3000",
					"http://localhost:5173",
					"http://127.0.0.1:3000",
				},
			},
		},
	}

	tests := []struct {
		name     string
		origin   string
		expected bool
	}{
		{"localhost development", "http://localhost:3000", true},
		{"localhost with port", "http://localhost:5173", true},
		{"127.0.0.1", "http://127.0.0.1:3000", true},
		{"invalid origin", "http://evil.com", false},
		{"empty origin", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.isAllowedOrigin(tt.origin)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCORSMiddleware_Preflight tests OPTIONS preflight requests
func TestCORSMiddleware_Preflight(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
			},
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called for preflight")
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "GET")
}

// TestCORSMiddleware_AllowedOrigin tests CORS with allowed origin
func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
			},
		},
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	assert.True(t, nextCalled, "Next handler should be called")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
}

// TestCORSMiddleware_DisallowedOrigin tests CORS with disallowed origin
func TestCORSMiddleware_DisallowedOrigin(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
			},
		},
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Server still processes the request, but doesn't set CORS headers
	// Browser will block the response due to missing CORS headers
	assert.True(t, nextCalled, "Next handler should be called")
	assert.Equal(t, http.StatusOK, rec.Code)
	// CORS headers should NOT be set for disallowed origin
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

// TestCORSMiddleware_NoOrigin tests request without Origin header
func TestCORSMiddleware_NoOrigin(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
			},
		},
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	assert.True(t, nextCalled, "Next handler should be called for same-origin requests")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestCORSHeaders tests all CORS headers are set correctly
func TestCORSHeaders(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:5173"},
			},
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("POST", "/api/test", bytes.NewReader([]byte("{}")))
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Check all CORS headers
	headers := rec.Header()
	assert.Equal(t, "http://localhost:5173", headers.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", headers.Get("Access-Control-Allow-Credentials"))
	assert.NotEmpty(t, headers.Get("Access-Control-Allow-Headers"))
	assert.Contains(t, headers.Get("Access-Control-Allow-Headers"), "Content-Type")
	assert.Contains(t, headers.Get("Access-Control-Allow-Headers"), "Authorization")
}

// TestCORSMiddleware_MultipleOrigins tests different allowed origins
func TestCORSMiddleware_MultipleOrigins(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{
					"http://localhost:3000",
					"http://localhost:5173",
					"http://127.0.0.1:3000",
					"http://127.0.0.1:5173",
				},
			},
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	allowedOrigins := []string{
		"http://localhost:3000",
		"http://localhost:5173",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:5173",
	}

	for _, origin := range allowedOrigins {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.Header.Set("Origin", origin)
			rec := httptest.NewRecorder()

			middleware.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, origin, rec.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

// BenchmarkCORSMiddleware benchmarks CORS middleware
func BenchmarkCORSMiddleware(b *testing.B) {
	handler := &Handler{
		config: &config.Config{
			Server: config.ServerConfig{
				AllowedOrigins: []string{"http://localhost:3000"},
			},
		},
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := handler.CORSMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)
	}
}

// Benchmark respondJSON
func BenchmarkRespondJSON(b *testing.B) {
	data := map[string]string{"message": "test", "status": "ok"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		respondJSON(rec, http.StatusOK, data)
	}
}

// createTestUser creates a test user with specified role (uses setupTestDB from organization_handler_test.go)
func createTestUser(t *testing.T, db *gorm.DB, role models.UserRole, orgID *uuid.UUID) *models.User {
	hashedPassword, err := utils.HashPassword("password123")
	require.NoError(t, err)

	user := &models.User{
		Email:          fmt.Sprintf("user-%s@test.com", uuid.New().String()[:8]),
		Name:           "Test User",
		Password:       hashedPassword,
		Role:           role,
		OrganizationID: orgID,
		IsActive:       true,
	}
	err = db.Create(user).Error
	require.NoError(t, err)
	return user
}

// createTestToken creates a test auth token for a user
func createTestAuthToken(t *testing.T, db *gorm.DB, userID uuid.UUID, name string) *models.AuthToken {
	rawToken := "grok_" + uuid.New().String()
	hashedToken := utils.HashToken(rawToken)

	token := &models.AuthToken{
		UserID:    userID,
		Name:      name,
		TokenHash: hashedToken,
		IsActive:  true,
	}
	err := db.Create(token).Error
	require.NoError(t, err)
	return token
}

// createTestTunnel creates a test tunnel
func createTestTunnel(t *testing.T, db *gorm.DB, userID uuid.UUID, orgID *uuid.UUID, subdomain string) *models.Tunnel {
	// Create a dummy token for the tunnel
	dummyToken := createTestAuthToken(t, db, userID, "Tunnel Token "+uuid.New().String()[:8])

	tunnel := &models.Tunnel{
		UserID:         userID,
		TokenID:        dummyToken.ID,
		OrganizationID: orgID,
		ClientID:       uuid.New().String(),
		Subdomain:      subdomain,
		TunnelType:     "http",
		LocalAddr:      "localhost:3000",
		PublicURL:      fmt.Sprintf("https://%s.grok.io", subdomain),
		Status:         "active",
	}
	err := db.Create(tunnel).Error
	require.NoError(t, err)
	return tunnel
}

// setupHandlerWithAuth creates a test handler with authentication middleware
func setupHandlerWithAuth(db *gorm.DB) *Handler {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret: "test-jwt-secret-for-testing-purposes-only",
		},
		Server: config.ServerConfig{
			Domain:         "grok.io",
			AllowedOrigins: []string{"http://localhost:3000"},
		},
		Tunnels: config.TunnelsConfig{
			MaxPerUser: 10,
		},
	}

	tokenService := auth.NewTokenService(db)

	// Create a tunnel manager for testing
	tunnelManager := tunnel.NewManager(
		db,
		cfg.Server.Domain,
		cfg.Tunnels.MaxPerUser,
		false, // TLS disabled for tests
		80,    // HTTP port
		443,   // HTTPS port
		10000, // TCP start port
		11000, // TCP end port
	)

	return NewHandler(db, tokenService, tunnelManager, nil, cfg)
}

// TestHealth tests the health check endpoint
func TestHealth(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	handler.health(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "healthy")
	assert.Contains(t, rec.Body.String(), "grok-server")
}

// TestGetCSRFToken tests CSRF token generation
func TestGetCSRFToken(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	req := httptest.NewRequest("GET", "/api/auth/csrf", nil)
	rec := httptest.NewRecorder()

	handler.getCSRFToken(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response["csrf_token"])
	assert.Greater(t, len(response["csrf_token"]), 40)
}

// TestDeleteToken tests token deletion with authorization
func TestDeleteToken(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	// Create test users
	superAdmin := createTestUser(t, db, models.RoleSuperAdmin, nil)
	regularUser := createTestUser(t, db, models.RoleOrgUser, nil)
	otherUser := createTestUser(t, db, models.RoleOrgUser, nil)

	// Create tokens for testing
	userToken := createTestAuthToken(t, db, regularUser.ID, "User Token")
	otherToken := createTestAuthToken(t, db, otherUser.ID, "Other Token")

	tests := []struct {
		name           string
		tokenID        string
		userID         string
		role           string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "super admin can delete any token",
			tokenID:        userToken.ID.String(),
			userID:         superAdmin.ID.String(),
			role:           string(models.RoleSuperAdmin),
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "user can delete own token",
			tokenID:        otherToken.ID.String(),
			userID:         otherUser.ID.String(),
			role:           string(models.RoleOrgUser),
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "user cannot delete other user's token",
			tokenID:        createTestAuthToken(t, db, otherUser.ID, "Another Token").ID.String(),
			userID:         regularUser.ID.String(),
			role:           string(models.RoleOrgUser),
			expectedStatus: http.StatusForbidden,
			expectError:    true,
		},
		{
			name:           "invalid token ID returns not found",
			tokenID:        uuid.New().String(),
			userID:         superAdmin.ID.String(),
			role:           string(models.RoleSuperAdmin),
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", "/api/tokens/"+tt.tokenID, nil)
			req.SetPathValue("id", tt.tokenID)

			// Set up auth context
			ctx := middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
				UserID:   tt.userID,
				Username: "testuser",
				Role:     tt.role,
			})
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			handler.deleteToken(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectError {
				assert.Contains(t, rec.Body.String(), "error")
			}
		})
	}
}

// TestToggleToken tests token toggle with authorization
func TestToggleToken(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	// Create test users
	superAdmin := createTestUser(t, db, models.RoleSuperAdmin, nil)
	regularUser := createTestUser(t, db, models.RoleOrgUser, nil)
	otherUser := createTestUser(t, db, models.RoleOrgUser, nil)

	// Create tokens
	userToken := createTestAuthToken(t, db, regularUser.ID, "User Token")
	otherToken := createTestAuthToken(t, db, otherUser.ID, "Other Token")

	tests := []struct {
		name           string
		tokenID        string
		userID         string
		role           string
		expectedStatus int
		expectToggle   bool
	}{
		{
			name:           "super admin can toggle any token",
			tokenID:        userToken.ID.String(),
			userID:         superAdmin.ID.String(),
			role:           string(models.RoleSuperAdmin),
			expectedStatus: http.StatusOK,
			expectToggle:   true,
		},
		{
			name:           "user can toggle own token",
			tokenID:        userToken.ID.String(),
			userID:         regularUser.ID.String(),
			role:           string(models.RoleOrgUser),
			expectedStatus: http.StatusOK,
			expectToggle:   true,
		},
		{
			name:           "user cannot toggle other user's token",
			tokenID:        otherToken.ID.String(),
			userID:         regularUser.ID.String(),
			role:           string(models.RoleOrgUser),
			expectedStatus: http.StatusForbidden,
			expectToggle:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get initial state
			var initialToken models.AuthToken
			db.First(&initialToken, "id = ?", tt.tokenID)
			initialState := initialToken.IsActive

			req := httptest.NewRequest("PATCH", "/api/tokens/"+tt.tokenID+"/toggle", nil)
			req.SetPathValue("id", tt.tokenID)

			ctx := middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
				UserID:   tt.userID,
				Username: "testuser",
				Role:     tt.role,
			})
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			handler.toggleToken(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.expectToggle {
				var updatedToken models.AuthToken
				db.First(&updatedToken, "id = ?", tt.tokenID)
				assert.NotEqual(t, initialState, updatedToken.IsActive)
			}
		})
	}
}

// TestGetTunnel tests getting a single tunnel with authorization
func TestGetTunnel(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	// Create organization
	org := &models.Organization{
		Name:      "Test Org",
		Subdomain: "testorg",
		IsActive:  true,
	}
	db.Create(org)

	// Create users
	superAdmin := createTestUser(t, db, models.RoleSuperAdmin, nil)
	orgAdmin := createTestUser(t, db, models.RoleOrgAdmin, &org.ID)
	orgUser := createTestUser(t, db, models.RoleOrgUser, &org.ID)
	otherUser := createTestUser(t, db, models.RoleOrgUser, nil)

	// Create tunnel owned by orgUser
	tunnel := createTestTunnel(t, db, orgUser.ID, &org.ID, "testtunnel")

	tests := []struct {
		name           string
		tunnelID       string
		userID         string
		role           string
		orgID          *string
		expectedStatus int
	}{
		{
			name:           "super admin can view any tunnel",
			tunnelID:       tunnel.ID.String(),
			userID:         superAdmin.ID.String(),
			role:           string(models.RoleSuperAdmin),
			orgID:          nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org admin can view org tunnel",
			tunnelID:       tunnel.ID.String(),
			userID:         orgAdmin.ID.String(),
			role:           string(models.RoleOrgAdmin),
			orgID:          &[]string{org.ID.String()}[0],
			expectedStatus: http.StatusOK,
		},
		{
			name:           "owner can view own tunnel",
			tunnelID:       tunnel.ID.String(),
			userID:         orgUser.ID.String(),
			role:           string(models.RoleOrgUser),
			orgID:          &[]string{org.ID.String()}[0],
			expectedStatus: http.StatusOK,
		},
		{
			name:           "other user cannot view tunnel",
			tunnelID:       tunnel.ID.String(),
			userID:         otherUser.ID.String(),
			role:           string(models.RoleOrgUser),
			orgID:          nil,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "invalid tunnel ID returns not found",
			tunnelID:       uuid.New().String(),
			userID:         superAdmin.ID.String(),
			role:           string(models.RoleSuperAdmin),
			orgID:          nil,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/tunnels/"+tt.tunnelID, nil)
			req.SetPathValue("id", tt.tunnelID)

			ctx := middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
				UserID:         tt.userID,
				Username:       "testuser",
				Role:           tt.role,
				OrganizationID: tt.orgID,
			})
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			handler.getTunnel(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

// TestCreateToken tests token creation
func TestCreateToken(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	user := createTestUser(t, db, models.RoleOrgUser, nil)

	tests := []struct {
		name           string
		body           createTokenRequest
		expectedStatus int
		expectToken    bool
	}{
		{
			name: "valid token creation",
			body: createTokenRequest{
				Name:   "Test Token",
				Scopes: []string{"tunnel:read", "tunnel:write"},
			},
			expectedStatus: http.StatusCreated,
			expectToken:    true,
		},
		{
			name: "missing token name",
			body: createTokenRequest{
				Scopes: []string{"tunnel:read"},
			},
			expectedStatus: http.StatusBadRequest,
			expectToken:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/tokens", bytes.NewReader(body))

			ctx := middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
				UserID:   user.ID.String(),
				Username: user.Email,
				Role:     string(user.Role),
			})
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			handler.createToken(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.expectToken {
				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response["token"])
				assert.Equal(t, tt.body.Name, response["name"])
			}
		})
	}
}

// TestGetStats tests stats endpoint with org filtering
func TestGetStats(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	// Create organization
	org := &models.Organization{
		Name:      "Test Org",
		Subdomain: "testorg",
		IsActive:  true,
	}
	db.Create(org)

	// Create users
	superAdmin := createTestUser(t, db, models.RoleSuperAdmin, nil)
	orgUser := createTestUser(t, db, models.RoleOrgUser, &org.ID)

	// Create tunnels
	createTestTunnel(t, db, orgUser.ID, &org.ID, "tunnel1")
	createTestTunnel(t, db, orgUser.ID, &org.ID, "tunnel2")

	tests := []struct {
		name           string
		userID         string
		role           string
		orgID          *string
		expectedStatus int
	}{
		{
			name:           "super admin gets all stats",
			userID:         superAdmin.ID.String(),
			role:           string(models.RoleSuperAdmin),
			orgID:          nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org user gets own stats",
			userID:         orgUser.ID.String(),
			role:           string(models.RoleOrgUser),
			orgID:          &[]string{org.ID.String()}[0],
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/stats", nil)

			ctx := middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
				UserID:         tt.userID,
				Username:       "testuser",
				Role:           tt.role,
				OrganizationID: tt.orgID,
			})
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			handler.getStats(rec, req)
			assert.Equal(t, tt.expectedStatus, rec.Code)

			var stats map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &stats)
			require.NoError(t, err)
			assert.Contains(t, stats, "total_tunnels")
			assert.Contains(t, stats, "active_tunnels")

			if tt.role == string(models.RoleSuperAdmin) {
				assert.Contains(t, stats, "tcp_ports")
			} else {
				assert.NotContains(t, stats, "tcp_ports")
			}
		})
	}
}

// TestGetConfig tests config endpoint
func TestGetConfig(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	user := createTestUser(t, db, models.RoleOrgUser, nil)

	req := httptest.NewRequest("GET", "/api/config", nil)
	ctx := middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
		UserID:   user.ID.String(),
		Username: user.Email,
		Role:     string(user.Role),
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.getConfig(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var config map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &config)
	require.NoError(t, err)
	assert.Equal(t, "grok.io", config["domain"])
}

// TestLogin tests login endpoint
func TestLogin(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	// Create test user with password
	hashedPassword, _ := utils.HashPassword("testpassword123")
	user := &models.User{
		Email:    "test@example.com",
		Name:     "Test User",
		Password: hashedPassword,
		Role:     models.RoleOrgUser,
		IsActive: true,
	}
	db.Create(user)

	tests := []struct {
		name           string
		body           loginRequest
		expectedStatus int
		expectToken    bool
	}{
		{
			name: "valid credentials",
			body: loginRequest{
				Username: "test@example.com",
				Password: "testpassword123",
			},
			expectedStatus: http.StatusOK,
			expectToken:    true,
		},
		{
			name: "invalid password",
			body: loginRequest{
				Username: "test@example.com",
				Password: "wrongpassword",
			},
			expectedStatus: http.StatusUnauthorized,
			expectToken:    false,
		},
		{
			name: "non-existent user",
			body: loginRequest{
				Username: "nonexistent@example.com",
				Password: "password",
			},
			expectedStatus: http.StatusUnauthorized,
			expectToken:    false,
		},
		{
			name: "missing credentials",
			body: loginRequest{
				Username: "",
				Password: "",
			},
			expectedStatus: http.StatusBadRequest,
			expectToken:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			handler.login(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.expectToken {
				var response loginResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "test@example.com", response.User)
				assert.Equal(t, string(models.RoleOrgUser), response.Role)

				// Check if auth cookie is set
				cookies := rec.Result().Cookies()
				assert.NotEmpty(t, cookies)
				found := false
				for _, cookie := range cookies {
					if cookie.Name == "auth_token" {
						found = true
						assert.NotEmpty(t, cookie.Value)
						assert.True(t, cookie.HttpOnly)
					}
				}
				assert.True(t, found, "auth_token cookie should be set")
			}
		})
	}
}

// TestListTokens tests token listing with organization filtering
func TestListTokens(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	// Create organization
	org := &models.Organization{
		Name:      "Test Org",
		Subdomain: "testorg",
		IsActive:  true,
	}
	db.Create(org)

	// Create users
	superAdmin := createTestUser(t, db, models.RoleSuperAdmin, nil)
	orgAdmin := createTestUser(t, db, models.RoleOrgAdmin, &org.ID)
	orgUser := createTestUser(t, db, models.RoleOrgUser, &org.ID)
	otherUser := createTestUser(t, db, models.RoleOrgUser, nil)

	// Create tokens
	createTestAuthToken(t, db, orgUser.ID, "Org User Token")
	createTestAuthToken(t, db, otherUser.ID, "Other User Token")
	createTestAuthToken(t, db, orgAdmin.ID, "Org Admin Token")

	tests := []struct {
		name           string
		userID         string
		role           string
		orgID          *string
		expectedStatus int
		minTokens      int
	}{
		{
			name:           "super admin sees all tokens",
			userID:         superAdmin.ID.String(),
			role:           string(models.RoleSuperAdmin),
			orgID:          nil,
			expectedStatus: http.StatusOK,
			minTokens:      3,
		},
		{
			name:           "org admin sees org tokens",
			userID:         orgAdmin.ID.String(),
			role:           string(models.RoleOrgAdmin),
			orgID:          &[]string{org.ID.String()}[0],
			expectedStatus: http.StatusOK,
			minTokens:      2, // orgUser + orgAdmin
		},
		{
			name:           "org user sees only own tokens",
			userID:         orgUser.ID.String(),
			role:           string(models.RoleOrgUser),
			orgID:          &[]string{org.ID.String()}[0],
			expectedStatus: http.StatusOK,
			minTokens:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/tokens", nil)

			ctx := middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
				UserID:         tt.userID,
				Username:       "testuser",
				Role:           tt.role,
				OrganizationID: tt.orgID,
			})
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			handler.listTokens(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			var tokens []interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &tokens)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(tokens), tt.minTokens)
		})
	}
}

// TestListTunnels tests tunnel listing with organization filtering
func TestListTunnels(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	// Create organization
	org := &models.Organization{
		Name:      "Test Org",
		Subdomain: "testorg",
		IsActive:  true,
	}
	db.Create(org)

	// Create users
	superAdmin := createTestUser(t, db, models.RoleSuperAdmin, nil)
	orgAdmin := createTestUser(t, db, models.RoleOrgAdmin, &org.ID)
	orgUser := createTestUser(t, db, models.RoleOrgUser, &org.ID)

	// Create tunnels
	createTestTunnel(t, db, orgUser.ID, &org.ID, "tunnel1")
	createTestTunnel(t, db, orgUser.ID, &org.ID, "tunnel2")
	createTestTunnel(t, db, orgAdmin.ID, &org.ID, "tunnel3")

	tests := []struct {
		name           string
		userID         string
		role           string
		orgID          *string
		expectedStatus int
		minTunnels     int
	}{
		{
			name:           "super admin sees all tunnels",
			userID:         superAdmin.ID.String(),
			role:           string(models.RoleSuperAdmin),
			orgID:          nil,
			expectedStatus: http.StatusOK,
			minTunnels:     3,
		},
		{
			name:           "org admin sees org tunnels",
			userID:         orgAdmin.ID.String(),
			role:           string(models.RoleOrgAdmin),
			orgID:          &[]string{org.ID.String()}[0],
			expectedStatus: http.StatusOK,
			minTunnels:     3,
		},
		{
			name:           "org user sees only own tunnels",
			userID:         orgUser.ID.String(),
			role:           string(models.RoleOrgUser),
			orgID:          &[]string{org.ID.String()}[0],
			expectedStatus: http.StatusOK,
			minTunnels:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/tunnels", nil)

			ctx := middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
				UserID:         tt.userID,
				Username:       "testuser",
				Role:           tt.role,
				OrganizationID: tt.orgID,
			})
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			handler.listTunnels(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			var tunnels []interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &tunnels)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(tunnels), tt.minTunnels)
		})
	}
}

// TestLogout tests logout endpoint
func TestLogout(t *testing.T) {
	db := setupTestDB(t)
	handler := setupHandlerWithAuth(db)

	user := createTestUser(t, db, models.RoleOrgUser, nil)

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	ctx := middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
		UserID:   user.ID.String(),
		Username: user.Email,
		Role:     string(user.Role),
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.logout(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Check that cookie is cleared (MaxAge = -1)
	cookies := rec.Result().Cookies()
	assert.NotEmpty(t, cookies)
	for _, cookie := range cookies {
		if cookie.Name == "auth_token" {
			assert.Equal(t, -1, cookie.MaxAge)
		}
	}
}

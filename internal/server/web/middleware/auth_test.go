package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key"

// TestNewAuthMiddleware tests middleware initialization
func TestNewAuthMiddleware(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)
	assert.NotNil(t, middleware)
	assert.Equal(t, []byte(testSecret), middleware.jwtSecret)
}

// TestGenerateToken tests JWT token generation
func TestGenerateToken(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	tests := []struct {
		name           string
		userID         string
		username       string
		role           string
		organizationID *string
	}{
		{
			name:           "super admin without org",
			userID:         "user-1",
			username:       "admin",
			role:           "super_admin",
			organizationID: nil,
		},
		{
			name:           "org admin with org",
			userID:         "user-2",
			username:       "orgadmin",
			role:           "org_admin",
			organizationID: strPtr("org-123"),
		},
		{
			name:           "org user with org",
			userID:         "user-3",
			username:       "orguser",
			role:           "org_user",
			organizationID: strPtr("org-456"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := middleware.GenerateToken(tt.userID, tt.username, tt.role, tt.organizationID)
			require.NoError(t, err)
			assert.NotEmpty(t, token)

			// Verify token can be parsed
			parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
				return middleware.jwtSecret, nil
			})
			require.NoError(t, err)
			assert.True(t, parsedToken.Valid)

			claims, ok := parsedToken.Claims.(*Claims)
			require.True(t, ok)
			assert.Equal(t, tt.userID, claims.UserID)
			assert.Equal(t, tt.username, claims.Username)
			assert.Equal(t, tt.role, claims.Role)

			if tt.organizationID != nil {
				require.NotNil(t, claims.OrganizationID)
				assert.Equal(t, *tt.organizationID, *claims.OrganizationID)
			} else {
				assert.Nil(t, claims.OrganizationID)
			}

			// Check expiration is set to 24 hours
			assert.True(t, claims.ExpiresAt.After(time.Now()))
			assert.True(t, claims.ExpiresAt.Before(time.Now().Add(25*time.Hour)))
		})
	}
}

// TestProtect_CookieAuth tests authentication via httpOnly cookie
func TestProtect_CookieAuth(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	// Generate valid token
	token, err := middleware.GenerateToken("user-1", "testuser", "org_user", strPtr("org-123"))
	require.NoError(t, err)

	// Create test handler that returns username from context
	handler := middleware.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(username))
	}))

	// Create request with auth cookie
	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: token,
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "testuser", rec.Body.String())
}

// TestProtect_BearerAuth tests authentication via Authorization header
func TestProtect_BearerAuth(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	// Generate valid token
	token, err := middleware.GenerateToken("user-1", "testuser", "org_admin", nil)
	require.NoError(t, err)

	handler := middleware.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(username))
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "testuser", rec.Body.String())
}

// TestProtect_QueryParamAuth tests authentication via query parameter (for SSE)
func TestProtect_QueryParamAuth(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	// Generate valid token
	token, err := middleware.GenerateToken("user-1", "testuser", "super_admin", nil)
	require.NoError(t, err)

	handler := middleware.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(username))
	}))

	req := httptest.NewRequest("GET", "/protected?token="+token, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "testuser", rec.Body.String())
}

// TestProtect_NoToken tests request without any authentication
func TestProtect_NoToken(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	handler := middleware.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Unauthorized")
}

// TestProtect_InvalidBearerFormat tests invalid Authorization header format
func TestProtect_InvalidBearerFormat(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	handler := middleware.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		header string
	}{
		{"missing Bearer prefix", "sometoken"},
		{"wrong prefix", "Basic sometoken"},
		{"extra parts", "Bearer token extra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

// TestProtect_InvalidToken tests invalid JWT token
func TestProtect_InvalidToken(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	handler := middleware.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name  string
		token string
	}{
		{"malformed token", "not.a.jwt"},
		{"random string", "randomstring"},
		{"wrong signature", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VybmFtZSI6InRlc3QifQ.invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.Contains(t, rec.Body.String(), "Invalid token")
		})
	}
}

// TestProtect_ExpiredToken tests expired JWT token
func TestProtect_ExpiredToken(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	// Create expired token manually
	claims := &Claims{
		Username: "testuser",
		UserID:   "user-1",
		Role:     "org_user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // Expired 1 hour ago
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-25 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(middleware.jwtSecret)
	require.NoError(t, err)

	handler := middleware.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid token")
}

// TestProtect_WrongSecret tests token signed with different secret
func TestProtect_WrongSecret(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	// Generate token with different secret
	wrongMiddleware := NewAuthMiddleware("wrong-secret")
	token, err := wrongMiddleware.GenerateToken("user-1", "testuser", "org_user", nil)
	require.NoError(t, err)

	handler := middleware.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid token")
}

// TestProtect_ClaimsInContext tests that claims are properly set in context
func TestProtect_ClaimsInContext(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	orgID := "org-123"
	token, err := middleware.GenerateToken("user-1", "testuser", "org_admin", &orgID)
	require.NoError(t, err)

	handler := middleware.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify claims are in context
		claims := GetClaimsFromContext(r.Context())
		require.NotNil(t, claims)
		assert.Equal(t, "user-1", claims.UserID)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, "org_admin", claims.Role)
		require.NotNil(t, claims.OrganizationID)
		assert.Equal(t, orgID, *claims.OrganizationID)

		// Verify legacy user context
		username := GetUserFromContext(r.Context())
		assert.Equal(t, "testuser", username)

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestGetUserFromContext tests getting user from context
func TestGetUserFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "user in context",
			ctx:      context.WithValue(context.Background(), userContextKey, "testuser"),
			expected: "testuser",
		},
		{
			name:     "no user in context",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), userContextKey, 12345),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			username := GetUserFromContext(tt.ctx)
			assert.Equal(t, tt.expected, username)
		})
	}
}

// TestProtect_CookiePriorityOverHeader tests that cookie takes priority over header
func TestProtect_CookiePriorityOverHeader(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	cookieToken, err := middleware.GenerateToken("user-1", "cookie-user", "org_user", nil)
	require.NoError(t, err)

	headerToken, err := middleware.GenerateToken("user-2", "header-user", "org_user", nil)
	require.NoError(t, err)

	handler := middleware.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(username))
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:  "auth_token",
		Value: cookieToken,
	})
	req.Header.Set("Authorization", "Bearer "+headerToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Cookie should take priority
	assert.Equal(t, "cookie-user", rec.Body.String())
}

// TestProtect_HeaderPriorityOverQuery tests that header takes priority over query
func TestProtect_HeaderPriorityOverQuery(t *testing.T) {
	middleware := NewAuthMiddleware(testSecret)

	headerToken, err := middleware.GenerateToken("user-1", "header-user", "org_user", nil)
	require.NoError(t, err)

	queryToken, err := middleware.GenerateToken("user-2", "query-user", "org_user", nil)
	require.NoError(t, err)

	handler := middleware.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(username))
	}))

	req := httptest.NewRequest("GET", "/protected?token="+queryToken, nil)
	req.Header.Set("Authorization", "Bearer "+headerToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Header should take priority
	assert.Equal(t, "header-user", rec.Body.String())
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}

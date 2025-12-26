package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

const (
	userContextKey contextKey = "user"
)

// Claims represents JWT claims with role and organization info
type Claims struct {
	Username       string  `json:"username"`
	UserID         string  `json:"user_id"`
	Role           string  `json:"role"`
	OrganizationID *string `json:"organization_id,omitempty"`
	jwt.RegisteredClaims
}

// AuthMiddleware provides JWT authentication middleware
type AuthMiddleware struct {
	jwtSecret []byte
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(jwtSecret string) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret: []byte(jwtSecret),
	}
}

// Protect wraps a handler with JWT authentication
// Supports both cookie-based (httpOnly) and header-based (Authorization) authentication
func (m *AuthMiddleware) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenString string

		// Try to get token from httpOnly cookie first (most secure)
		if cookie, err := r.Cookie("auth_token"); err == nil && cookie.Value != "" {
			tokenString = cookie.Value
		} else {
			// Fall back to Authorization header (for API clients)
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				// Extract token from header
				parts := strings.Split(authHeader, " ")
				if len(parts) != 2 || parts[0] != "Bearer" {
					http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
					return
				}
				tokenString = parts[1]
			} else {
				// Final fallback to query parameter (for SSE connections)
				tokenString = r.URL.Query().Get("token")
				if tokenString == "" {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}
		}

		// Parse and validate token
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return m.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			logger.ErrorEvent().Err(err).Msg("Invalid token")
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			return
		}

		// Add claims to context (for RBAC) and legacy user context
		ctx := SetClaimsInContext(r.Context(), claims)
		ctx = context.WithValue(ctx, userContextKey, claims.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GenerateToken generates a JWT token for a user with role and organization info
func (m *AuthMiddleware) GenerateToken(userID, username, role string, organizationID *string) (string, error) {
	claims := &Claims{
		Username:       username,
		UserID:         userID,
		Role:           role,
		OrganizationID: organizationID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.jwtSecret)
}

// GetUserFromContext retrieves username from context
func GetUserFromContext(ctx context.Context) string {
	if username, ok := ctx.Value(userContextKey).(string); ok {
		return username
	}
	return ""
}

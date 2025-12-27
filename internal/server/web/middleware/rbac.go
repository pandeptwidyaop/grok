package middleware

import (
	"context"
	"net/http"

	"github.com/pandeptwidyaop/grok/internal/db/models"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	claimsContextKey contextKey = "claims"
)

// PermissionChecker provides role-based access control middleware
type PermissionChecker struct{}

// NewPermissionChecker creates a new permission checker
func NewPermissionChecker() *PermissionChecker {
	return &PermissionChecker{}
}

// RequireRole creates middleware that checks for specific role(s)
func (pc *PermissionChecker) RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaimsFromContext(r.Context())
			if claims == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check if user has required role
			for _, role := range allowedRoles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
		})
	}
}

// RequireOrgMembership ensures user belongs to the organization in the path
func (pc *PermissionChecker) RequireOrgMembership(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaimsFromContext(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Super admin bypasses org checks
		if claims.Role == string(models.RoleSuperAdmin) {
			next.ServeHTTP(w, r)
			return
		}

		// Extract org_id from path
		orgID := r.PathValue("org_id")
		if orgID == "" {
			http.Error(w, "Bad Request: organization ID required", http.StatusBadRequest)
			return
		}

		// Verify user belongs to this organization
		if claims.OrganizationID == nil || *claims.OrganizationID != orgID {
			http.Error(w, "Forbidden: not a member of this organization", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireOrganization ensures user has an organization (for routes without org_id in path)
func (pc *PermissionChecker) RequireOrganization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaimsFromContext(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Super admin bypasses org checks
		if claims.Role == string(models.RoleSuperAdmin) {
			next.ServeHTTP(w, r)
			return
		}

		// Verify user has an organization
		if claims.OrganizationID == nil || *claims.OrganizationID == "" {
			http.Error(w, "Forbidden: organization membership required", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireOrgAdmin ensures user is admin of their organization or super admin
func (pc *PermissionChecker) RequireOrgAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaimsFromContext(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Super admin or org_admin allowed
		if claims.Role == string(models.RoleSuperAdmin) || claims.Role == string(models.RoleOrgAdmin) {
			next.ServeHTTP(w, r)
			return
		}

		http.Error(w, "Forbidden: organization admin required", http.StatusForbidden)
	})
}

// GetClaimsFromContext retrieves claims from request context
func GetClaimsFromContext(ctx context.Context) *Claims {
	if claims, ok := ctx.Value(claimsContextKey).(*Claims); ok {
		return claims
	}
	return nil
}

// SetClaimsInContext stores claims in context (used by auth middleware)
func SetClaimsInContext(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

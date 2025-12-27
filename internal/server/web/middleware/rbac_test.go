package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/stretchr/testify/assert"
)

// TestNewPermissionChecker tests permission checker initialization
func TestNewPermissionChecker(t *testing.T) {
	pc := NewPermissionChecker()
	assert.NotNil(t, pc)
}

// TestRequireRole tests role-based access control
func TestRequireRole(t *testing.T) {
	pc := NewPermissionChecker()

	tests := []struct {
		name           string
		userRole       string
		allowedRoles   []string
		hasClaims      bool
		expectedStatus int
	}{
		{
			name:           "super admin allowed",
			userRole:       string(models.RoleSuperAdmin),
			allowedRoles:   []string{string(models.RoleSuperAdmin)},
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org admin allowed",
			userRole:       string(models.RoleOrgAdmin),
			allowedRoles:   []string{string(models.RoleOrgAdmin)},
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org user allowed",
			userRole:       string(models.RoleOrgUser),
			allowedRoles:   []string{string(models.RoleOrgUser)},
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "multiple roles - match first",
			userRole:       string(models.RoleSuperAdmin),
			allowedRoles:   []string{string(models.RoleSuperAdmin), string(models.RoleOrgAdmin)},
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "multiple roles - match second",
			userRole:       string(models.RoleOrgAdmin),
			allowedRoles:   []string{string(models.RoleSuperAdmin), string(models.RoleOrgAdmin)},
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "role not allowed",
			userRole:       string(models.RoleOrgUser),
			allowedRoles:   []string{string(models.RoleSuperAdmin)},
			hasClaims:      true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "no claims in context",
			userRole:       "",
			allowedRoles:   []string{string(models.RoleOrgUser)},
			hasClaims:      false,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := pc.RequireRole(tt.allowedRoles...)
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/protected", nil)
			if tt.hasClaims {
				claims := &Claims{
					UserID:   "user-1",
					Username: "testuser",
					Role:     tt.userRole,
				}
				req = req.WithContext(SetClaimsInContext(req.Context(), claims))
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

// TestRequireOrgMembership tests organization membership validation
func TestRequireOrgMembership(t *testing.T) {
	pc := NewPermissionChecker()

	tests := []struct {
		name           string
		userRole       string
		userOrgID      *string
		pathOrgID      string
		hasClaims      bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "super admin bypasses org check",
			userRole:       string(models.RoleSuperAdmin),
			userOrgID:      nil,
			pathOrgID:      "org-123",
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org admin - matching org",
			userRole:       string(models.RoleOrgAdmin),
			userOrgID:      strPtr("org-123"),
			pathOrgID:      "org-123",
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org user - matching org",
			userRole:       string(models.RoleOrgUser),
			userOrgID:      strPtr("org-123"),
			pathOrgID:      "org-123",
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org admin - different org",
			userRole:       string(models.RoleOrgAdmin),
			userOrgID:      strPtr("org-456"),
			pathOrgID:      "org-123",
			hasClaims:      true,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "not a member of this organization",
		},
		{
			name:           "user without org",
			userRole:       string(models.RoleOrgUser),
			userOrgID:      nil,
			pathOrgID:      "org-123",
			hasClaims:      true,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "not a member of this organization",
		},
		{
			name:           "no claims",
			userRole:       "",
			userOrgID:      nil,
			pathOrgID:      "org-123",
			hasClaims:      false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing org_id in path",
			userRole:       string(models.RoleOrgAdmin),
			userOrgID:      strPtr("org-123"),
			pathOrgID:      "",
			hasClaims:      true,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "organization ID required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := pc.RequireOrgMembership(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			// Use SetPathValue to set org_id in request (Go 1.22+ routing)
			req := httptest.NewRequest("GET", "/orgs/"+tt.pathOrgID+"/users", nil)
			req.SetPathValue("org_id", tt.pathOrgID)

			if tt.hasClaims {
				claims := &Claims{
					UserID:         "user-1",
					Username:       "testuser",
					Role:           tt.userRole,
					OrganizationID: tt.userOrgID,
				}
				req = req.WithContext(SetClaimsInContext(req.Context(), claims))
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rec.Body.String(), tt.expectedBody)
			}
		})
	}
}

// TestRequireOrganization tests organization requirement
func TestRequireOrganization(t *testing.T) {
	pc := NewPermissionChecker()

	tests := []struct {
		name           string
		userRole       string
		userOrgID      *string
		hasClaims      bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "super admin bypasses org requirement",
			userRole:       string(models.RoleSuperAdmin),
			userOrgID:      nil,
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org admin with org",
			userRole:       string(models.RoleOrgAdmin),
			userOrgID:      strPtr("org-123"),
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org user with org",
			userRole:       string(models.RoleOrgUser),
			userOrgID:      strPtr("org-456"),
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org admin without org",
			userRole:       string(models.RoleOrgAdmin),
			userOrgID:      nil,
			hasClaims:      true,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "organization membership required",
		},
		{
			name:           "org user with empty org",
			userRole:       string(models.RoleOrgUser),
			userOrgID:      strPtr(""),
			hasClaims:      true,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "organization membership required",
		},
		{
			name:           "no claims",
			userRole:       "",
			userOrgID:      nil,
			hasClaims:      false,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := pc.RequireOrganization(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/tunnels", nil)
			if tt.hasClaims {
				claims := &Claims{
					UserID:         "user-1",
					Username:       "testuser",
					Role:           tt.userRole,
					OrganizationID: tt.userOrgID,
				}
				req = req.WithContext(SetClaimsInContext(req.Context(), claims))
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rec.Body.String(), tt.expectedBody)
			}
		})
	}
}

// TestRequireOrgAdmin tests organization admin requirement
func TestRequireOrgAdmin(t *testing.T) {
	pc := NewPermissionChecker()

	tests := []struct {
		name           string
		userRole       string
		hasClaims      bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "super admin allowed",
			userRole:       string(models.RoleSuperAdmin),
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org admin allowed",
			userRole:       string(models.RoleOrgAdmin),
			hasClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org user forbidden",
			userRole:       string(models.RoleOrgUser),
			hasClaims:      true,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "organization admin required",
		},
		{
			name:           "no claims",
			userRole:       "",
			hasClaims:      false,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := pc.RequireOrgAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/admin", nil)
			if tt.hasClaims {
				claims := &Claims{
					UserID:   "user-1",
					Username: "testuser",
					Role:     tt.userRole,
				}
				req = req.WithContext(SetClaimsInContext(req.Context(), claims))
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rec.Body.String(), tt.expectedBody)
			}
		})
	}
}

// TestGetClaimsFromContext tests retrieving claims from context
func TestGetClaimsFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected *Claims
	}{
		{
			name: "claims in context",
			ctx: SetClaimsInContext(context.Background(), &Claims{
				UserID:   "user-1",
				Username: "testuser",
				Role:     string(models.RoleOrgUser),
			}),
			expected: &Claims{
				UserID:   "user-1",
				Username: "testuser",
				Role:     string(models.RoleOrgUser),
			},
		},
		{
			name:     "no claims in context",
			ctx:      context.Background(),
			expected: nil,
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), claimsContextKey, "not-claims"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := GetClaimsFromContext(tt.ctx)
			if tt.expected == nil {
				assert.Nil(t, claims)
			} else {
				assert.NotNil(t, claims)
				assert.Equal(t, tt.expected.UserID, claims.UserID)
				assert.Equal(t, tt.expected.Username, claims.Username)
				assert.Equal(t, tt.expected.Role, claims.Role)
			}
		})
	}
}

// TestSetClaimsInContext tests storing claims in context
func TestSetClaimsInContext(t *testing.T) {
	claims := &Claims{
		UserID:         "user-1",
		Username:       "testuser",
		Role:           string(models.RoleOrgAdmin),
		OrganizationID: strPtr("org-123"),
	}

	ctx := SetClaimsInContext(context.Background(), claims)
	assert.NotNil(t, ctx)

	retrieved := GetClaimsFromContext(ctx)
	assert.NotNil(t, retrieved)
	assert.Equal(t, claims.UserID, retrieved.UserID)
	assert.Equal(t, claims.Username, retrieved.Username)
	assert.Equal(t, claims.Role, retrieved.Role)
	assert.Equal(t, *claims.OrganizationID, *retrieved.OrganizationID)
}

// TestMiddlewareChaining tests combining multiple RBAC middlewares
func TestMiddlewareChaining(t *testing.T) {
	pc := NewPermissionChecker()

	// Chain: RequireRole(org_admin) -> RequireOrganization
	roleMiddleware := pc.RequireRole(string(models.RoleOrgAdmin))
	orgMiddleware := pc.RequireOrganization

	handler := roleMiddleware(orgMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})))

	tests := []struct {
		name           string
		userRole       string
		userOrgID      *string
		expectedStatus int
	}{
		{
			name:           "org admin with org - allowed",
			userRole:       string(models.RoleOrgAdmin),
			userOrgID:      strPtr("org-123"),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "org admin without org - forbidden",
			userRole:       string(models.RoleOrgAdmin),
			userOrgID:      nil,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "org user with org - forbidden (wrong role)",
			userRole:       string(models.RoleOrgUser),
			userOrgID:      strPtr("org-123"),
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/admin/dashboard", nil)
			claims := &Claims{
				UserID:         "user-1",
				Username:       "testuser",
				Role:           tt.userRole,
				OrganizationID: tt.userOrgID,
			}
			req = req.WithContext(SetClaimsInContext(req.Context(), claims))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

// TestSuperAdminPrivileges tests that super admin bypasses all org checks
func TestSuperAdminPrivileges(t *testing.T) {
	pc := NewPermissionChecker()

	superAdminClaims := &Claims{
		UserID:         "user-1",
		Username:       "superadmin",
		Role:           string(models.RoleSuperAdmin),
		OrganizationID: nil, // No org
	}

	tests := []struct {
		name       string
		middleware func(http.Handler) http.Handler
	}{
		{
			name:       "RequireOrganization",
			middleware: pc.RequireOrganization,
		},
		{
			name: "RequireOrgMembership",
			middleware: func(next http.Handler) http.Handler {
				return pc.RequireOrgMembership(next)
			},
		},
		{
			name:       "RequireOrgAdmin",
			middleware: pc.RequireOrgAdmin,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := tt.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			req.SetPathValue("org_id", "org-999") // Different org
			req = req.WithContext(SetClaimsInContext(req.Context(), superAdminClaims))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Super admin should always succeed
			assert.Equal(t, http.StatusOK, rec.Code, "Super admin should bypass %s", tt.name)
		})
	}
}

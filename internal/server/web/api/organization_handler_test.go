package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/web/middleware"
	"github.com/pandeptwidyaop/grok/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate models
	err = db.AutoMigrate(&models.User{}, &models.Organization{}, &models.Tunnel{})
	require.NoError(t, err)

	return db
}

// createTestOrg creates a test organization
func createTestOrg(t *testing.T, db *gorm.DB, subdomain string) *models.Organization {
	org := &models.Organization{
		Name:        "Test Organization",
		Subdomain:   subdomain,
		Description: "Test Description",
		IsActive:    true,
	}
	err := db.Create(org).Error
	require.NoError(t, err)
	return org
}

// createTestOrgUser creates a test user in an organization
func createTestOrgUser(t *testing.T, db *gorm.DB, orgID uuid.UUID, role models.UserRole) *models.User {
	hashedPassword, err := utils.HashPassword("password123")
	require.NoError(t, err)

	user := &models.User{
		OrganizationID: &orgID,
		Email:          fmt.Sprintf("user-%s@test.com", uuid.New().String()[:8]),
		Name:           "Test User",
		Password:       hashedPassword,
		Role:           role,
		IsActive:       true,
	}
	err = db.Create(user).Error
	require.NoError(t, err)
	return user
}

// TestCreateOrganization tests organization creation
func TestCreateOrganization(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	tests := []struct {
		name           string
		body           CreateOrganizationRequest
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name: "valid organization",
			body: CreateOrganizationRequest{
				Name:        "New Org",
				Subdomain:   "neworg",
				Description: "Description",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body string) {
				var resp OrganizationResponse
				err := json.Unmarshal([]byte(body), &resp)
				require.NoError(t, err)
				assert.Equal(t, "New Org", resp.Name)
				assert.Equal(t, "neworg", resp.Subdomain)
				assert.Equal(t, "neworg.grok.io", resp.FullDomain)
				assert.True(t, resp.IsActive)
			},
		},
		{
			name: "missing name",
			body: CreateOrganizationRequest{
				Subdomain: "testorg",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Name and subdomain are required")
			},
		},
		{
			name: "invalid subdomain format",
			body: CreateOrganizationRequest{
				Name:      "Test",
				Subdomain: "ab", // Too short
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid subdomain format")
			},
		},
		{
			name: "duplicate subdomain",
			body: CreateOrganizationRequest{
				Name:      "Duplicate",
				Subdomain: "duplicate",
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "subdomain already taken")
			},
		},
	}

	// Create a duplicate org for the last test
	createTestOrg(t, db, "duplicate")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/orgs", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			handler.CreateOrganization(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec.Body.String())
			}
		})
	}
}

// TestListOrganizations tests listing organizations
func TestListOrganizations(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	// Create test orgs
	org1 := createTestOrg(t, db, "org1")
	org2 := createTestOrg(t, db, "org2")

	tests := []struct {
		name          string
		claims        *middleware.Claims
		expectedCount int
	}{
		{
			name: "super admin sees all orgs",
			claims: &middleware.Claims{
				UserID: "user-1",
				Role:   string(models.RoleSuperAdmin),
			},
			expectedCount: 2,
		},
		{
			name: "org admin sees only their org",
			claims: &middleware.Claims{
				UserID:         "user-2",
				Role:           string(models.RoleOrgAdmin),
				OrganizationID: strPtr(org1.ID.String()),
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/orgs", nil)
			req = req.WithContext(middleware.SetClaimsInContext(req.Context(), tt.claims))
			rec := httptest.NewRecorder()

			handler.ListOrganizations(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			var resp []OrganizationResponse
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Len(t, resp, tt.expectedCount)
		})
	}

	// Clean up
	db.Delete(&models.Organization{}, org1.ID)
	db.Delete(&models.Organization{}, org2.ID)
}

// TestGetOrganization tests getting a single organization
func TestGetOrganization(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "testorg")

	tests := []struct {
		name           string
		orgID          string
		expectedStatus int
	}{
		{
			name:           "existing organization",
			orgID:          org.ID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-existent organization",
			orgID:          uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "missing org id",
			orgID:          "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/orgs/"+tt.orgID, nil)
			req.SetPathValue("id", tt.orgID)
			rec := httptest.NewRecorder()

			handler.GetOrganization(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

// TestUpdateOrganization tests updating an organization
func TestUpdateOrganization(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "updatetest")

	tests := []struct {
		name           string
		orgID          string
		body           UpdateOrganizationRequest
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:  "update name",
			orgID: org.ID.String(),
			body: UpdateOrganizationRequest{
				Name: strPtr("Updated Name"),
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var resp OrganizationResponse
				json.Unmarshal([]byte(body), &resp)
				assert.Equal(t, "Updated Name", resp.Name)
			},
		},
		{
			name:  "update description",
			orgID: org.ID.String(),
			body: UpdateOrganizationRequest{
				Description: strPtr("New Description"),
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var resp OrganizationResponse
				json.Unmarshal([]byte(body), &resp)
				assert.Equal(t, "New Description", resp.Description)
			},
		},
		{
			name:  "update active status",
			orgID: org.ID.String(),
			body: UpdateOrganizationRequest{
				IsActive: boolPtr(false),
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var resp OrganizationResponse
				json.Unmarshal([]byte(body), &resp)
				assert.False(t, resp.IsActive)
			},
		},
		{
			name:           "non-existent org",
			orgID:          uuid.New().String(),
			body:           UpdateOrganizationRequest{Name: strPtr("Test")},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("PUT", "/api/orgs/"+tt.orgID, bytes.NewReader(body))
			req.SetPathValue("id", tt.orgID)
			rec := httptest.NewRecorder()

			handler.UpdateOrganization(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec.Body.String())
			}
		})
	}
}

// TestDeleteOrganization tests organization deletion
func TestDeleteOrganization(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "deletetest")

	tests := []struct {
		name           string
		orgID          string
		expectedStatus int
	}{
		{
			name:           "delete existing org",
			orgID:          org.ID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "delete non-existent org",
			orgID:          uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", "/api/orgs/"+tt.orgID, nil)
			req.SetPathValue("id", tt.orgID)
			rec := httptest.NewRecorder()

			handler.DeleteOrganization(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

// TestToggleOrganization tests toggling organization active status
func TestToggleOrganization(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "toggletest")
	initialStatus := org.IsActive

	req := httptest.NewRequest("POST", "/api/orgs/"+org.ID.String()+"/toggle", nil)
	req.SetPathValue("id", org.ID.String())
	rec := httptest.NewRecorder()

	handler.ToggleOrganization(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp OrganizationResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NotEqual(t, initialStatus, resp.IsActive)
}

// TestListOrgUsers tests listing users in an organization
func TestListOrgUsers(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "userstest")
	createTestOrgUser(t, db, org.ID, models.RoleOrgAdmin)
	createTestOrgUser(t, db, org.ID, models.RoleOrgUser)

	req := httptest.NewRequest("GET", "/api/orgs/"+org.ID.String()+"/users", nil)
	req.SetPathValue("org_id", org.ID.String())
	rec := httptest.NewRecorder()

	handler.ListOrgUsers(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var users []models.User
	err := json.Unmarshal(rec.Body.Bytes(), &users)
	require.NoError(t, err)
	assert.Equal(t, 2, len(users))
}

// TestCreateOrgUser tests creating a user in an organization
func TestCreateOrgUser(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "createusertest")

	tests := []struct {
		name           string
		orgID          string
		body           CreateOrgUserRequest
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:  "valid org admin",
			orgID: org.ID.String(),
			body: CreateOrgUserRequest{
				Email:    "admin@test.com",
				Name:     "Admin User",
				Password: "password123",
				Role:     string(models.RoleOrgAdmin),
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body string) {
				var user models.User
				json.Unmarshal([]byte(body), &user)
				assert.Equal(t, "admin@test.com", user.Email)
				assert.Equal(t, models.RoleOrgAdmin, user.Role)
				assert.Empty(t, user.Password) // Password should be redacted
			},
		},
		{
			name:  "valid org user",
			orgID: org.ID.String(),
			body: CreateOrgUserRequest{
				Email:    "user@test.com",
				Name:     "Regular User",
				Password: "password123",
				Role:     string(models.RoleOrgUser),
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:  "missing email",
			orgID: org.ID.String(),
			body: CreateOrgUserRequest{
				Name:     "User",
				Password: "password123",
				Role:     string(models.RoleOrgUser),
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Email, name, password, and role are required")
			},
		},
		{
			name:  "invalid role",
			orgID: org.ID.String(),
			body: CreateOrgUserRequest{
				Email:    "test@test.com",
				Name:     "User",
				Password: "password123",
				Role:     "invalid_role",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid role")
			},
		},
		{
			name:  "duplicate email",
			orgID: org.ID.String(),
			body: CreateOrgUserRequest{
				Email:    "admin@test.com", // Already created
				Name:     "Duplicate",
				Password: "password123",
				Role:     string(models.RoleOrgUser),
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "email already exists")
			},
		},
		{
			name:  "non-existent org",
			orgID: uuid.New().String(),
			body: CreateOrgUserRequest{
				Email:    "new@test.com",
				Name:     "User",
				Password: "password123",
				Role:     string(models.RoleOrgUser),
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/orgs/"+tt.orgID+"/users", bytes.NewReader(body))
			req.SetPathValue("org_id", tt.orgID)
			rec := httptest.NewRecorder()

			handler.CreateOrgUser(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec.Body.String())
			}
		})
	}
}

// TestGetOrgUser tests getting a user from an organization
func TestGetOrgUser(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "getusertest")
	user := createTestOrgUser(t, db, org.ID, models.RoleOrgUser)

	req := httptest.NewRequest("GET", "/api/orgs/"+org.ID.String()+"/users/"+user.ID.String(), nil)
	req.SetPathValue("org_id", org.ID.String())
	req.SetPathValue("user_id", user.ID.String())
	rec := httptest.NewRecorder()

	handler.GetOrgUser(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var respUser models.User
	json.Unmarshal(rec.Body.Bytes(), &respUser)
	assert.Equal(t, user.ID, respUser.ID)
	assert.Empty(t, respUser.Password) // Password should be redacted
}

// TestUpdateUserRole tests updating a user's role
func TestUpdateUserRole(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "updateroletest")
	user := createTestOrgUser(t, db, org.ID, models.RoleOrgUser)

	tests := []struct {
		name           string
		userID         string
		orgID          string
		body           UpdateUserRoleRequest
		expectedStatus int
		expectedRole   models.UserRole
	}{
		{
			name:           "promote to admin",
			userID:         user.ID.String(),
			orgID:          org.ID.String(),
			body:           UpdateUserRoleRequest{Role: string(models.RoleOrgAdmin)},
			expectedStatus: http.StatusOK,
			expectedRole:   models.RoleOrgAdmin,
		},
		{
			name:           "invalid role",
			userID:         user.ID.String(),
			orgID:          org.ID.String(),
			body:           UpdateUserRoleRequest{Role: "super_admin"},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "non-existent user",
			userID:         uuid.New().String(),
			orgID:          org.ID.String(),
			body:           UpdateUserRoleRequest{Role: string(models.RoleOrgAdmin)},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("PUT", "/api/orgs/"+tt.orgID+"/users/"+tt.userID+"/role", bytes.NewReader(body))
			req.SetPathValue("org_id", tt.orgID)
			req.SetPathValue("user_id", tt.userID)
			rec := httptest.NewRecorder()

			handler.UpdateUserRole(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedStatus == http.StatusOK {
				var respUser models.User
				json.Unmarshal(rec.Body.Bytes(), &respUser)
				assert.Equal(t, tt.expectedRole, respUser.Role)
			}
		})
	}
}

// TestDeleteOrgUser tests deleting a user from an organization
func TestDeleteOrgUser(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "deleteusertest")
	user := createTestOrgUser(t, db, org.ID, models.RoleOrgUser)

	tests := []struct {
		name           string
		claims         *middleware.Claims
		userID         string
		expectedStatus int
	}{
		{
			name: "delete other user",
			claims: &middleware.Claims{
				UserID: "different-user",
			},
			userID:         user.ID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name: "cannot delete self",
			claims: &middleware.Claims{
				UserID: user.ID.String(),
			},
			userID:         user.ID.String(),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", "/api/orgs/"+org.ID.String()+"/users/"+tt.userID, nil)
			req = req.WithContext(middleware.SetClaimsInContext(req.Context(), tt.claims))
			req.SetPathValue("org_id", org.ID.String())
			req.SetPathValue("user_id", tt.userID)
			rec := httptest.NewRecorder()

			handler.DeleteOrgUser(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

// TestGetOrgStats tests getting organization statistics
func TestGetOrgStats(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "statstest")
	createTestOrgUser(t, db, org.ID, models.RoleOrgAdmin)
	createTestOrgUser(t, db, org.ID, models.RoleOrgUser)

	// Create test tunnel
	tunnel := &models.Tunnel{
		OrganizationID: &org.ID,
		Subdomain:      "test",
		TunnelType:     "http",
		ClientID:       uuid.New().String(),
		Status:         "active",
		RequestsCount:  100,
		BytesIn:        1000,
		BytesOut:       2000,
	}
	db.Create(tunnel)

	req := httptest.NewRequest("GET", "/api/orgs/"+org.ID.String()+"/stats", nil)
	req.SetPathValue("org_id", org.ID.String())
	rec := httptest.NewRecorder()

	handler.GetOrgStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stats struct {
		TotalUsers    int64 `json:"total_users"`
		ActiveUsers   int64 `json:"active_users"`
		TotalTunnels  int64 `json:"total_tunnels"`
		ActiveTunnels int64 `json:"active_tunnels"`
		TotalRequests int64 `json:"total_requests"`
		TotalBytesIn  int64 `json:"total_bytes_in"`
		TotalBytesOut int64 `json:"total_bytes_out"`
	}
	json.Unmarshal(rec.Body.Bytes(), &stats)

	assert.Equal(t, int64(2), stats.TotalUsers)
	assert.Equal(t, int64(2), stats.ActiveUsers)
	assert.Equal(t, int64(1), stats.TotalTunnels)
	assert.Equal(t, int64(1), stats.ActiveTunnels)
	assert.Equal(t, int64(100), stats.TotalRequests)
	assert.Equal(t, int64(1000), stats.TotalBytesIn)
	assert.Equal(t, int64(2000), stats.TotalBytesOut)
}

// TestListOrgTunnels tests listing tunnels for an organization
func TestListOrgTunnels(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "tunnelstest")

	// Create tunnels with different statuses
	activeTunnel := &models.Tunnel{
		OrganizationID: &org.ID,
		Subdomain:      "active",
		TunnelType:     "http",
		ClientID:       uuid.New().String(),
		Status:         "active",
		ConnectedAt:    time.Now(),
	}
	db.Create(activeTunnel)

	offlineTunnel := &models.Tunnel{
		OrganizationID: &org.ID,
		Subdomain:      "offline",
		TunnelType:     "http",
		ClientID:       uuid.New().String(),
		Status:         "offline",
		ConnectedAt:    time.Now().Add(-1 * time.Hour),
	}
	db.Create(offlineTunnel)

	// This one should not be included (disconnected)
	disconnectedTunnel := &models.Tunnel{
		OrganizationID: &org.ID,
		Subdomain:      "disconnected",
		TunnelType:     "http",
		ClientID:       uuid.New().String(),
		Status:         "disconnected",
		ConnectedAt:    time.Now().Add(-2 * time.Hour),
	}
	db.Create(disconnectedTunnel)

	req := httptest.NewRequest("GET", "/api/orgs/"+org.ID.String()+"/tunnels", nil)
	req.SetPathValue("org_id", org.ID.String())
	rec := httptest.NewRecorder()

	handler.ListOrgTunnels(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var tunnels []models.Tunnel
	json.Unmarshal(rec.Body.Bytes(), &tunnels)
	assert.Equal(t, 2, len(tunnels)) // Only active and offline
}

// TestResetUserPassword tests resetting a user's password
func TestResetUserPassword(t *testing.T) {
	db := setupTestDB(t)
	handler := NewOrganizationHandler(db, "grok.io")

	org := createTestOrg(t, db, "resetpwdtest")
	user := createTestOrgUser(t, db, org.ID, models.RoleOrgUser)

	tests := []struct {
		name           string
		userID         string
		body           map[string]string
		expectedStatus int
	}{
		{
			name:   "valid password reset",
			userID: user.ID.String(),
			body: map[string]string{
				"new_password": "newpassword123",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "password too short",
			userID: user.ID.String(),
			body: map[string]string{
				"new_password": "short",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "non-existent user",
			userID: uuid.New().String(),
			body: map[string]string{
				"new_password": "newpassword123",
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/orgs/"+org.ID.String()+"/users/"+tt.userID+"/reset-password", bytes.NewReader(body))
			req.SetPathValue("org_id", org.ID.String())
			req.SetPathValue("user_id", tt.userID)
			rec := httptest.NewRecorder()

			handler.ResetUserPassword(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

// TestToResponse tests organization model to response conversion
func TestToResponse(t *testing.T) {
	handler := NewOrganizationHandler(nil, "grok.io")

	org := &models.Organization{
		ID:          uuid.New(),
		Name:        "Test Org",
		Subdomain:   "testorg",
		Description: "Test Description",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	resp := handler.toResponse(org)

	assert.Equal(t, org.ID.String(), resp.ID)
	assert.Equal(t, org.Name, resp.Name)
	assert.Equal(t, org.Subdomain, resp.Subdomain)
	assert.Equal(t, "testorg.grok.io", resp.FullDomain)
	assert.Equal(t, org.Description, resp.Description)
	assert.Equal(t, org.IsActive, resp.IsActive)
	assert.NotEmpty(t, resp.CreatedAt)
	assert.NotEmpty(t, resp.UpdatedAt)
}

// Helper functions
func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

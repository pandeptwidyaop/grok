package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	"github.com/pandeptwidyaop/grok/internal/server/web/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupWebhookTestDB creates an in-memory SQLite database for testing
func setupWebhookTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate models
	err = db.AutoMigrate(&models.Organization{}, &models.User{}, &models.WebhookApp{}, &models.WebhookRoute{})
	require.NoError(t, err)

	return db
}

// createWebhookTestApp creates a test webhook app
func createWebhookTestApp(t *testing.T, db *gorm.DB, orgID, userID uuid.UUID, name string) *models.WebhookApp {
	app := &models.WebhookApp{
		OrganizationID: orgID,
		UserID:         userID,
		Name:           name,
		Description:    "Test App",
		IsActive:       true,
	}
	err := db.Create(app).Error
	require.NoError(t, err)
	return app
}

// setupTestTunnelManager creates a test tunnel manager with default values
func setupTestTunnelManager(db *gorm.DB) *tunnel.Manager {
	return tunnel.NewManager(db, "grok.io", 5, false, 80, 443, 10000, 20000)
}

// TestCreateWebhookApp tests webhook app creation
func TestCreateWebhookApp(t *testing.T) {
	db := setupWebhookTestDB(t)
	tm := setupTestTunnelManager(db)
	handler := NewWebhookHandler(db, tm)

	// Create org and user
	org := createTestOrg(t, db, "testorg")
	user := createTestOrgUser(t, db, org.ID, models.RoleOrgUser)

	tests := []struct {
		name           string
		claims         *middleware.Claims
		body           map[string]string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name: "valid app creation",
			claims: &middleware.Claims{
				UserID:         user.ID.String(),
				OrganizationID: strPtr(org.ID.String()),
			},
			body: map[string]string{
				"name":        "myapp",
				"description": "My App",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body string) {
				var app models.WebhookApp
				json.Unmarshal([]byte(body), &app)
				assert.Equal(t, "myapp", app.Name)
				assert.True(t, app.IsActive)
			},
		},
		{
			name: "invalid app name",
			claims: &middleware.Claims{
				UserID:         user.ID.String(),
				OrganizationID: strPtr(org.ID.String()),
			},
			body: map[string]string{
				"name": "invalid name!",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "no organization",
			claims: &middleware.Claims{
				UserID:         user.ID.String(),
				OrganizationID: nil,
			},
			body: map[string]string{
				"name": "myapp",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate name",
			claims: &middleware.Claims{
				UserID:         user.ID.String(),
				OrganizationID: strPtr(org.ID.String()),
			},
			body: map[string]string{
				"name": "duplicate",
			},
			expectedStatus: http.StatusConflict,
		},
	}

	// Create a duplicate app for the last test
	createWebhookTestApp(t, db, org.ID, user.ID, "duplicate")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/api/webhooks/apps", bytes.NewReader(body))
			req = req.WithContext(middleware.SetClaimsInContext(req.Context(), tt.claims))
			rec := httptest.NewRecorder()

			handler.CreateApp(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec.Body.String())
			}
		})
	}
}

// TestListWebhookApps tests listing webhook apps
func TestListWebhookApps(t *testing.T) {
	db := setupWebhookTestDB(t)
	tm := setupTestTunnelManager(db)
	handler := NewWebhookHandler(db, tm)

	// Create test data
	org := createTestOrg(t, db, "testorg")
	user := createTestOrgUser(t, db, org.ID, models.RoleOrgUser)
	createWebhookTestApp(t, db, org.ID, user.ID, "app1")
	createWebhookTestApp(t, db, org.ID, user.ID, "app2")

	req := httptest.NewRequest("GET", "/api/webhooks/apps", nil)
	req = req.WithContext(middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
		UserID:         user.ID.String(),
		OrganizationID: strPtr(org.ID.String()),
	}))
	rec := httptest.NewRecorder()

	handler.ListApps(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var apps []models.WebhookApp
	json.Unmarshal(rec.Body.Bytes(), &apps)
	assert.Len(t, apps, 2)
}

// TestGetWebhookApp tests getting a single webhook app
func TestGetWebhookApp(t *testing.T) {
	db := setupWebhookTestDB(t)
	tm := setupTestTunnelManager(db)
	handler := NewWebhookHandler(db, tm)

	org := createTestOrg(t, db, "testorg")
	user := createTestOrgUser(t, db, org.ID, models.RoleOrgUser)
	app := createWebhookTestApp(t, db, org.ID, user.ID, "testapp")

	tests := []struct {
		name           string
		appID          string
		expectedStatus int
	}{
		{
			name:           "existing app",
			appID:          app.ID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-existent app",
			appID:          uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/webhooks/apps/"+tt.appID, nil)
			req.SetPathValue("id", tt.appID)
			req = req.WithContext(middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
				UserID:         user.ID.String(),
				OrganizationID: strPtr(org.ID.String()),
			}))
			rec := httptest.NewRecorder()

			handler.GetApp(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedStatus == http.StatusOK {
				var respApp models.WebhookApp
				json.Unmarshal(rec.Body.Bytes(), &respApp)
				assert.Equal(t, app.ID, respApp.ID)
			}
		})
	}
}

// TestUpdateWebhookApp tests updating a webhook app
func TestUpdateWebhookApp(t *testing.T) {
	db := setupWebhookTestDB(t)
	tm := setupTestTunnelManager(db)
	handler := NewWebhookHandler(db, tm)

	org := createTestOrg(t, db, "testorg")
	user := createTestOrgUser(t, db, org.ID, models.RoleOrgUser)
	app := createWebhookTestApp(t, db, org.ID, user.ID, "updateapp")

	body, _ := json.Marshal(map[string]string{
		"description": "Updated Description",
	})
	req := httptest.NewRequest("PUT", "/api/webhooks/apps/"+app.ID.String(), bytes.NewReader(body))
	req.SetPathValue("id", app.ID.String())
	req = req.WithContext(middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
		UserID:         user.ID.String(),
		OrganizationID: strPtr(org.ID.String()),
	}))
	rec := httptest.NewRecorder()

	handler.UpdateApp(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var respApp models.WebhookApp
	json.Unmarshal(rec.Body.Bytes(), &respApp)
	assert.Equal(t, "Updated Description", respApp.Description)
}

// TestDeleteWebhookApp tests deleting a webhook app
func TestDeleteWebhookApp(t *testing.T) {
	db := setupWebhookTestDB(t)
	tm := setupTestTunnelManager(db)
	handler := NewWebhookHandler(db, tm)

	org := createTestOrg(t, db, "testorg")
	user := createTestOrgUser(t, db, org.ID, models.RoleOrgUser)
	app := createWebhookTestApp(t, db, org.ID, user.ID, "deleteapp")

	req := httptest.NewRequest("DELETE", "/api/webhooks/apps/"+app.ID.String(), nil)
	req.SetPathValue("id", app.ID.String())
	req = req.WithContext(middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
		UserID:         user.ID.String(),
		OrganizationID: strPtr(org.ID.String()),
	}))
	rec := httptest.NewRecorder()

	handler.DeleteApp(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Verify app is deleted
	var deletedApp models.WebhookApp
	err := db.Where("id = ?", app.ID).First(&deletedApp).Error
	assert.Error(t, err) // Should not find the app
}

// TestToggleWebhookApp tests toggling webhook app active status
func TestToggleWebhookApp(t *testing.T) {
	db := setupWebhookTestDB(t)
	tm := setupTestTunnelManager(db)
	handler := NewWebhookHandler(db, tm)

	org := createTestOrg(t, db, "testorg")
	user := createTestOrgUser(t, db, org.ID, models.RoleOrgUser)
	app := createWebhookTestApp(t, db, org.ID, user.ID, "toggleapp")
	initialStatus := app.IsActive

	req := httptest.NewRequest("POST", "/api/webhooks/apps/"+app.ID.String()+"/toggle", nil)
	req.SetPathValue("id", app.ID.String())
	req = req.WithContext(middleware.SetClaimsInContext(req.Context(), &middleware.Claims{
		UserID:         user.ID.String(),
		OrganizationID: strPtr(org.ID.String()),
	}))
	rec := httptest.NewRecorder()

	handler.ToggleApp(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var respApp models.WebhookApp
	json.Unmarshal(rec.Body.Bytes(), &respApp)
	assert.NotEqual(t, initialStatus, respApp.IsActive)
}

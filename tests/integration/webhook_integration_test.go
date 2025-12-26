package integration

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/pandeptwidyaop/grok/internal/db/models"
)

// setupWebhookTestDB creates in-memory database for webhook tests.
func setupWebhookTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "Failed to connect to test database")

	// Auto-migrate all models
	err = db.AutoMigrate(
		&models.Organization{},
		&models.User{},
		&models.AuthToken{},
		&models.Domain{},
		&models.Tunnel{},
		&models.RequestLog{},
		&models.WebhookApp{},
		&models.WebhookRoute{},
		&models.WebhookEvent{},
	)
	require.NoError(t, err, "Failed to migrate database schema")

	return db
}

// createTestOrganization creates a test organization.
func createTestOrganization(t *testing.T, db *gorm.DB, subdomain string) *models.Organization {
	org := &models.Organization{
		ID:          uuid.New(),
		Name:        "Test Org " + subdomain,
		Subdomain:   subdomain,
		Description: "Test organization",
		IsActive:    true,
	}
	err := db.Create(org).Error
	require.NoError(t, err, "Failed to create test organization")
	return org
}

// createTestUser creates a test user.
func createTestUser(t *testing.T, db *gorm.DB, orgID uuid.UUID, email string) *models.User {
	user := &models.User{
		ID:             uuid.New(),
		Email:          email,
		Name:           "Test User",
		Password:       "hashed_password", // bcrypt hash
		Role:           "org_user",
		OrganizationID: &orgID,
		IsActive:       true,
	}
	err := db.Create(user).Error
	require.NoError(t, err, "Failed to create test user")
	return user
}

// createTestTunnel creates a test tunnel.
func createTestTunnel(t *testing.T, db *gorm.DB, orgID, userID uuid.UUID, subdomain string) *models.Tunnel {
	tokenID := uuid.New() // Create a dummy token ID
	tunnel := &models.Tunnel{
		ID:             uuid.New(),
		OrganizationID: &orgID,
		UserID:         userID,
		TokenID:        tokenID,
		TunnelType:     "http",
		Subdomain:      subdomain,
		LocalAddr:      "localhost:3000",
		PublicURL:      "https://" + subdomain + ".grok.io",
		Status:         "active",
		ClientID:       uuid.New().String(),
	}
	err := db.Create(tunnel).Error
	require.NoError(t, err, "Failed to create test tunnel")
	return tunnel
}

func TestWebhookApp_CRUD(t *testing.T) {
	db := setupWebhookTestDB(t)
	org := createTestOrganization(t, db, "trofeo")
	user := createTestUser(t, db, org.ID, "user@test.com")

	t.Run("Create WebhookApp", func(t *testing.T) {
		app := &models.WebhookApp{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Name:           "payment-app",
			Description:    "Stripe webhook receiver",
			IsActive:       true,
		}

		err := db.Create(app).Error
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, app.ID)
		assert.NotZero(t, app.CreatedAt)
		assert.NotZero(t, app.UpdatedAt)
	})

	t.Run("Read WebhookApp", func(t *testing.T) {
		// Create app
		app := &models.WebhookApp{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Name:           "test-app",
			Description:    "Test webhook app",
			IsActive:       true,
		}
		db.Create(app)

		// Read it back
		var found models.WebhookApp
		err := db.Preload("Routes").First(&found, app.ID).Error
		require.NoError(t, err)
		assert.Equal(t, app.Name, found.Name)
		assert.Equal(t, app.OrganizationID, found.OrganizationID)
	})

	t.Run("Update WebhookApp", func(t *testing.T) {
		app := &models.WebhookApp{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Name:           "update-app",
			Description:    "Original description",
			IsActive:       true,
		}
		db.Create(app)

		// Update description
		app.Description = "Updated description"
		app.IsActive = false
		err := db.Save(app).Error
		require.NoError(t, err)

		// Verify update
		var found models.WebhookApp
		db.First(&found, app.ID)
		assert.Equal(t, "Updated description", found.Description)
		assert.False(t, found.IsActive)
		assert.True(t, found.UpdatedAt.After(found.CreatedAt))
	})

	t.Run("Delete WebhookApp", func(t *testing.T) {
		app := &models.WebhookApp{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Name:           "delete-app",
			IsActive:       true,
		}
		db.Create(app)

		// Delete
		err := db.Delete(app).Error
		require.NoError(t, err)

		// Verify deletion
		var found models.WebhookApp
		err = db.First(&found, app.ID).Error
		assert.Error(t, err)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	})
}

func TestWebhookApp_UniqueConstraint(t *testing.T) {
	db := setupWebhookTestDB(t)
	org := createTestOrganization(t, db, "trofeo")
	user := createTestUser(t, db, org.ID, "user@test.com")

	t.Run("Duplicate app name in same org should fail", func(t *testing.T) {
		// Create first app
		app1 := &models.WebhookApp{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Name:           "duplicate-app",
			IsActive:       true,
		}
		err := db.Create(app1).Error
		require.NoError(t, err)

		// Try to create duplicate
		app2 := &models.WebhookApp{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Name:           "duplicate-app",
			IsActive:       true,
		}
		err = db.Create(app2).Error
		assert.Error(t, err, "Should fail on duplicate app name")
	})

	t.Run("Same app name in different org should succeed", func(t *testing.T) {
		org2 := createTestOrganization(t, db, "acme")
		user2 := createTestUser(t, db, org2.ID, "user2@test.com")

		// Create app in first org
		app1 := &models.WebhookApp{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Name:           "shared-name",
			IsActive:       true,
		}
		err := db.Create(app1).Error
		require.NoError(t, err)

		// Create app with same name in different org
		app2 := &models.WebhookApp{
			OrganizationID: org2.ID,
			UserID:         user2.ID,
			Name:           "shared-name",
			IsActive:       true,
		}
		err = db.Create(app2).Error
		assert.NoError(t, err, "Should allow same app name in different org")
	})
}

func TestWebhookRoute_CRUD(t *testing.T) {
	db := setupWebhookTestDB(t)
	org := createTestOrganization(t, db, "trofeo")
	user := createTestUser(t, db, org.ID, "user@test.com")

	// Create webhook app
	app := &models.WebhookApp{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Name:           "payment-app",
		IsActive:       true,
	}
	db.Create(app)

	t.Run("Create WebhookRoute", func(t *testing.T) {
		tunnel := createTestTunnel(t, db, org.ID, user.ID, "tunnel-create")
		route := &models.WebhookRoute{
			WebhookAppID: app.ID,
			TunnelID:     tunnel.ID,
			IsEnabled:    true,
			Priority:     100,
			HealthStatus: "unknown",
		}

		err := db.Create(route).Error
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, route.ID)
		assert.Equal(t, "unknown", route.HealthStatus)
		assert.Equal(t, 100, route.Priority)
	})

	t.Run("Read WebhookRoute with relationships", func(t *testing.T) {
		tunnel := createTestTunnel(t, db, org.ID, user.ID, "tunnel-read")
		route := &models.WebhookRoute{
			WebhookAppID: app.ID,
			TunnelID:     tunnel.ID,
			IsEnabled:    true,
			Priority:     50,
		}
		db.Create(route)

		// Read with preload
		var found models.WebhookRoute
		err := db.Preload("Tunnel").Preload("WebhookApp").First(&found, route.ID).Error
		require.NoError(t, err)
		assert.Equal(t, route.ID, found.ID)
		assert.NotNil(t, found.Tunnel)
		assert.Equal(t, tunnel.Subdomain, found.Tunnel.Subdomain)
		assert.NotNil(t, found.WebhookApp)
		assert.Equal(t, app.Name, found.WebhookApp.Name)
	})

	t.Run("Update route health status", func(t *testing.T) {
		tunnel := createTestTunnel(t, db, org.ID, user.ID, "tunnel-update")
		route := &models.WebhookRoute{
			WebhookAppID: app.ID,
			TunnelID:     tunnel.ID,
			IsEnabled:    true,
			Priority:     100,
			HealthStatus: "unknown",
		}
		db.Create(route)

		// Update health
		route.HealthStatus = "healthy"
		route.FailureCount = 0
		route.LastHealthCheck = time.Now()
		err := db.Save(route).Error
		require.NoError(t, err)

		// Verify
		var found models.WebhookRoute
		db.First(&found, route.ID)
		assert.Equal(t, "healthy", found.HealthStatus)
		assert.Equal(t, 0, found.FailureCount)
		assert.False(t, found.LastHealthCheck.IsZero())
	})

	t.Run("Toggle route enabled status", func(t *testing.T) {
		tunnel := createTestTunnel(t, db, org.ID, user.ID, "tunnel-toggle")
		route := &models.WebhookRoute{
			WebhookAppID: app.ID,
			TunnelID:     tunnel.ID,
			IsEnabled:    true,
			Priority:     100,
		}
		db.Create(route)

		// Toggle
		route.IsEnabled = false
		db.Save(route)

		var found models.WebhookRoute
		db.First(&found, route.ID)
		assert.False(t, found.IsEnabled)
	})
}

func TestWebhookRoute_UniqueConstraint(t *testing.T) {
	db := setupWebhookTestDB(t)
	org := createTestOrganization(t, db, "trofeo")
	user := createTestUser(t, db, org.ID, "user@test.com")
	tunnel := createTestTunnel(t, db, org.ID, user.ID, "tunnel1")

	app := &models.WebhookApp{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Name:           "payment-app",
		IsActive:       true,
	}
	db.Create(app)

	t.Run("Duplicate route (app + tunnel) should fail", func(t *testing.T) {
		// Create first route
		route1 := &models.WebhookRoute{
			WebhookAppID: app.ID,
			TunnelID:     tunnel.ID,
			IsEnabled:    true,
			Priority:     100,
		}
		err := db.Create(route1).Error
		require.NoError(t, err)

		// Try to create duplicate
		route2 := &models.WebhookRoute{
			WebhookAppID: app.ID,
			TunnelID:     tunnel.ID,
			IsEnabled:    true,
			Priority:     50,
		}
		err = db.Create(route2).Error
		assert.Error(t, err, "Should fail on duplicate route")
	})

	t.Run("Same tunnel in different app should succeed", func(t *testing.T) {
		app2 := &models.WebhookApp{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Name:           "notification-app",
			IsActive:       true,
		}
		db.Create(app2)

		// Create route in first app
		route1 := &models.WebhookRoute{
			WebhookAppID: app.ID,
			TunnelID:     tunnel.ID,
			IsEnabled:    true,
			Priority:     100,
		}
		db.Create(route1)

		// Create route with same tunnel in different app
		route2 := &models.WebhookRoute{
			WebhookAppID: app2.ID,
			TunnelID:     tunnel.ID,
			IsEnabled:    true,
			Priority:     100,
		}
		err := db.Create(route2).Error
		assert.NoError(t, err, "Should allow same tunnel in different app")
	})
}

func TestWebhookEvent_Create(t *testing.T) {
	db := setupWebhookTestDB(t)
	org := createTestOrganization(t, db, "trofeo")
	user := createTestUser(t, db, org.ID, "user@test.com")

	app := &models.WebhookApp{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Name:           "payment-app",
		IsActive:       true,
	}
	db.Create(app)

	t.Run("Create WebhookEvent", func(t *testing.T) {
		event := &models.WebhookEvent{
			WebhookAppID:  app.ID,
			RequestPath:   "/stripe/payment_intent",
			Method:        "POST",
			StatusCode:    200,
			DurationMs:    45,
			BytesIn:       1024,
			BytesOut:      512,
			ClientIP:      "192.168.1.1",
			RoutingStatus: "success",
			TunnelCount:   3,
			SuccessCount:  3,
		}

		err := db.Create(event).Error
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, event.ID)
		assert.NotZero(t, event.CreatedAt)
	})

	t.Run("Create failed webhook event", func(t *testing.T) {
		event := &models.WebhookEvent{
			WebhookAppID:  app.ID,
			RequestPath:   "/stripe/callback",
			Method:        "POST",
			StatusCode:    500,
			DurationMs:    120,
			BytesIn:       2048,
			BytesOut:      256,
			ClientIP:      "10.0.0.1",
			RoutingStatus: "failed",
			TunnelCount:   2,
			SuccessCount:  0,
			ErrorMessage:  "All tunnels failed",
		}

		err := db.Create(event).Error
		require.NoError(t, err)
		assert.Equal(t, "failed", event.RoutingStatus)
		assert.Equal(t, 0, event.SuccessCount)
		assert.NotEmpty(t, event.ErrorMessage)
	})
}

func TestWebhookApp_CascadeDelete(t *testing.T) {
	db := setupWebhookTestDB(t)
	org := createTestOrganization(t, db, "trofeo")
	user := createTestUser(t, db, org.ID, "user@test.com")
	tunnel := createTestTunnel(t, db, org.ID, user.ID, "tunnel1")

	// Create app with routes and events
	app := &models.WebhookApp{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Name:           "cascade-app",
		IsActive:       true,
	}
	db.Create(app)

	// Create routes
	route := &models.WebhookRoute{
		WebhookAppID: app.ID,
		TunnelID:     tunnel.ID,
		IsEnabled:    true,
		Priority:     100,
	}
	db.Create(route)

	// Create events
	event := &models.WebhookEvent{
		WebhookAppID: app.ID,
		RequestPath:  "/test",
		Method:       "POST",
		StatusCode:   200,
	}
	db.Create(event)

	t.Run("Delete app should cascade to routes and events", func(t *testing.T) {
		// Verify routes and events exist
		var routeCount int64
		db.Model(&models.WebhookRoute{}).Where("webhook_app_id = ?", app.ID).Count(&routeCount)
		assert.Equal(t, int64(1), routeCount)

		var eventCount int64
		db.Model(&models.WebhookEvent{}).Where("webhook_app_id = ?", app.ID).Count(&eventCount)
		assert.Equal(t, int64(1), eventCount)

		// Delete app
		err := db.Select("Routes", "Events").Delete(app).Error
		require.NoError(t, err)

		// Verify cascade delete
		db.Model(&models.WebhookRoute{}).Where("webhook_app_id = ?", app.ID).Count(&routeCount)
		assert.Equal(t, int64(0), routeCount, "Routes should be deleted")

		db.Model(&models.WebhookEvent{}).Where("webhook_app_id = ?", app.ID).Count(&eventCount)
		assert.Equal(t, int64(0), eventCount, "Events should be deleted")
	})
}

func TestOrganizationIsolation(t *testing.T) {
	db := setupWebhookTestDB(t)
	org1 := createTestOrganization(t, db, "org1")
	org2 := createTestOrganization(t, db, "org2")
	user1 := createTestUser(t, db, org1.ID, "user1@test.com")
	user2 := createTestUser(t, db, org2.ID, "user2@test.com")

	// Create apps in both orgs
	app1 := &models.WebhookApp{
		OrganizationID: org1.ID,
		UserID:         user1.ID,
		Name:           "org1-app",
		IsActive:       true,
	}
	db.Create(app1)

	app2 := &models.WebhookApp{
		OrganizationID: org2.ID,
		UserID:         user2.ID,
		Name:           "org2-app",
		IsActive:       true,
	}
	db.Create(app2)

	t.Run("Query apps by organization", func(t *testing.T) {
		// Query org1 apps
		var org1Apps []models.WebhookApp
		db.Where("organization_id = ?", org1.ID).Find(&org1Apps)
		assert.Len(t, org1Apps, 1)
		assert.Equal(t, "org1-app", org1Apps[0].Name)

		// Query org2 apps
		var org2Apps []models.WebhookApp
		db.Where("organization_id = ?", org2.ID).Find(&org2Apps)
		assert.Len(t, org2Apps, 1)
		assert.Equal(t, "org2-app", org2Apps[0].Name)
	})

	t.Run("Cross-org access should be prevented", func(t *testing.T) {
		// Try to find org1 app with org2 filter
		var found models.WebhookApp
		err := db.Where("organization_id = ? AND name = ?", org2.ID, "org1-app").First(&found).Error
		assert.Error(t, err)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	})
}

// Benchmark tests.
func BenchmarkWebhookApp_Create(b *testing.B) {
	db := setupWebhookTestDB(&testing.T{})
	org := createTestOrganization(&testing.T{}, db, "bench-org")
	user := createTestUser(&testing.T{}, db, org.ID, "bench@test.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app := &models.WebhookApp{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Name:           "bench-app-" + uuid.New().String()[:8],
			IsActive:       true,
		}
		db.Create(app)
	}
}

func BenchmarkWebhookEvent_Create(b *testing.B) {
	db := setupWebhookTestDB(&testing.T{})
	org := createTestOrganization(&testing.T{}, db, "bench-org")
	user := createTestUser(&testing.T{}, db, org.ID, "bench@test.com")
	app := &models.WebhookApp{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Name:           "bench-app",
		IsActive:       true,
	}
	db.Create(app)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := &models.WebhookEvent{
			WebhookAppID:  app.ID,
			RequestPath:   "/bench",
			Method:        "POST",
			StatusCode:    200,
			DurationMs:    50,
			RoutingStatus: "success",
			TunnelCount:   3,
			SuccessCount:  3,
		}
		db.Create(event)
	}
}

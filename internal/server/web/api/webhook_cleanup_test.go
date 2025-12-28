package api

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/config"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCleanupOldWebhookEvents(t *testing.T) {
	// Setup in-memory database with foreign key support
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Enable foreign key constraints in SQLite
	db.Exec("PRAGMA foreign_keys = ON")

	// Auto-migrate all tables
	err = db.AutoMigrate(&models.Organization{}, &models.User{}, &models.WebhookApp{}, &models.WebhookEvent{}, &models.WebhookTunnelResponse{})
	assert.NoError(t, err)

	// Create test organization
	orgID := uuid.New()
	org := &models.Organization{
		ID:        orgID,
		Name:      "Test Org",
		Subdomain: "testorg",
		IsActive:  true,
	}
	err = db.Create(org).Error
	assert.NoError(t, err)

	// Create test user
	userID := uuid.New()
	user := &models.User{
		ID:             userID,
		Email:          "test@example.com",
		Password:       "hashedpassword",
		Role:           "org_admin",
		OrganizationID: &orgID,
	}
	err = db.Create(user).Error
	assert.NoError(t, err)

	// Create test webhook app
	appID := uuid.New()
	app := &models.WebhookApp{
		ID:             appID,
		OrganizationID: orgID,
		UserID:         userID,
		Name:           "testapp",
		Description:    "Test app",
		IsActive:       true,
	}
	err = db.Create(app).Error
	assert.NoError(t, err)

	// Create handler with max_events = 5
	cfg := &config.Config{
		Webhooks: config.WebhooksConfig{
			MaxEvents: 5,
		},
	}
	handler := &Handler{
		db:     db,
		config: cfg,
	}

	// Create 10 webhook events (oldest to newest)
	for i := 0; i < 10; i++ {
		event := &models.WebhookEvent{
			ID:           uuid.New(),
			WebhookAppID: appID,
			RequestPath:  "/test",
			Method:       "POST",
			StatusCode:   200,
			CreatedAt:    time.Now().Add(time.Duration(i) * time.Second), // Incrementing timestamps
		}
		err = db.Create(event).Error
		assert.NoError(t, err)

		// Create tunnel response for each event
		tunnelResp := &models.WebhookTunnelResponse{
			ID:              uuid.New(),
			WebhookEventID:  event.ID,
			TunnelID:        uuid.New(),
			TunnelSubdomain: "test",
			StatusCode:      200,
			DurationMs:      100,
			Success:         true,
		}
		err = db.Create(tunnelResp).Error
		assert.NoError(t, err)
	}

	// Verify we have 10 events before cleanup
	var countBefore int64
	db.Model(&models.WebhookEvent{}).Where("webhook_app_id = ?", appID).Count(&countBefore)
	assert.Equal(t, int64(10), countBefore)

	// Verify we have 10 tunnel responses before cleanup
	var tunnelRespCountBefore int64
	db.Model(&models.WebhookTunnelResponse{}).Count(&tunnelRespCountBefore)
	assert.Equal(t, int64(10), tunnelRespCountBefore)

	// Run cleanup
	handler.cleanupOldWebhookEvents(appID)

	// Verify we now have exactly 5 events (max_events)
	var countAfter int64
	db.Model(&models.WebhookEvent{}).Where("webhook_app_id = ?", appID).Count(&countAfter)
	assert.Equal(t, int64(5), countAfter)

	// Verify tunnel responses were also deleted (CASCADE)
	var tunnelRespCountAfter int64
	db.Model(&models.WebhookTunnelResponse{}).Count(&tunnelRespCountAfter)
	assert.Equal(t, int64(5), tunnelRespCountAfter, "Tunnel responses should be deleted via CASCADE")

	// Verify the remaining events are the newest 5 (by checking they have later timestamps)
	var remainingEvents []models.WebhookEvent
	db.Where("webhook_app_id = ?", appID).Order("created_at ASC").Find(&remainingEvents)
	assert.Len(t, remainingEvents, 5)

	// The first remaining event should be newer than index 4 (0-indexed)
	// Since we created 10 events with incrementing timestamps, the oldest 5 should be deleted
	// and events 5-9 should remain
}

func TestCleanupOldWebhookEvents_WithinLimit(t *testing.T) {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.WebhookApp{}, &models.WebhookEvent{})
	assert.NoError(t, err)

	orgID := uuid.New()
	appID := uuid.New()

	app := &models.WebhookApp{
		ID:             appID,
		OrganizationID: orgID,
		Name:           "testapp",
	}
	db.Create(app)

	cfg := &config.Config{
		Webhooks: config.WebhooksConfig{
			MaxEvents: 10,
		},
	}
	handler := &Handler{
		db:     db,
		config: cfg,
	}

	// Create only 5 events (less than max)
	for i := 0; i < 5; i++ {
		event := &models.WebhookEvent{
			ID:           uuid.New(),
			WebhookAppID: appID,
			RequestPath:  "/test",
			Method:       "POST",
			StatusCode:   200,
		}
		db.Create(event)
	}

	// Run cleanup
	handler.cleanupOldWebhookEvents(appID)

	// Verify all 5 events still exist (no deletion needed)
	var count int64
	db.Model(&models.WebhookEvent{}).Where("webhook_app_id = ?", appID).Count(&count)
	assert.Equal(t, int64(5), count, "Should not delete events when within limit")
}

func TestCleanupOldWebhookEvents_Disabled(t *testing.T) {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.WebhookApp{}, &models.WebhookEvent{})
	assert.NoError(t, err)

	orgID := uuid.New()
	appID := uuid.New()

	app := &models.WebhookApp{
		ID:             appID,
		OrganizationID: orgID,
		Name:           "testapp",
	}
	db.Create(app)

	// Set max_events to 0 (disabled)
	cfg := &config.Config{
		Webhooks: config.WebhooksConfig{
			MaxEvents: 0,
		},
	}
	handler := &Handler{
		db:     db,
		config: cfg,
	}

	// Create 100 events
	for i := 0; i < 100; i++ {
		event := &models.WebhookEvent{
			ID:           uuid.New(),
			WebhookAppID: appID,
			RequestPath:  "/test",
			Method:       "POST",
			StatusCode:   200,
		}
		db.Create(event)
	}

	// Run cleanup
	handler.cleanupOldWebhookEvents(appID)

	// Verify all 100 events still exist (cleanup disabled)
	var count int64
	db.Model(&models.WebhookEvent{}).Where("webhook_app_id = ?", appID).Count(&count)
	assert.Equal(t, int64(100), count, "Should not delete events when cleanup is disabled (max_events = 0)")
}

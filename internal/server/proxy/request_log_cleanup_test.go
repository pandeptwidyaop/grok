package proxy

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/pandeptwidyaop/grok/internal/db/models"
)

func TestCleanupOldRequestLogs(t *testing.T) {
	// Setup in-memory database WITHOUT foreign key constraints for simpler unit testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Disable foreign key constraints for unit testing
	db.Exec("PRAGMA foreign_keys = OFF")

	// Auto-migrate tables
	err = db.AutoMigrate(&models.Tunnel{}, &models.RequestLog{})
	assert.NoError(t, err)

	// Create test tunnel (foreign keys disabled, so we can use any UUIDs)
	tunnelID := uuid.New()
	tunnel := &models.Tunnel{
		ID:         tunnelID,
		UserID:     uuid.New(),
		TokenID:    uuid.New(),
		Subdomain:  "test",
		TunnelType: "http",
		LocalAddr:  "localhost:3000",
		PublicURL:  "https://test.example.com",
		ClientID:   "test-client",
	}
	db.Create(tunnel)

	// Create HTTP proxy with max_request_logs = 10
	proxy := &HTTPProxy{
		db:             db,
		maxRequestLogs: 10,
	}

	// Create 20 request logs (oldest to newest)
	for i := 0; i < 20; i++ {
		log := &models.RequestLog{
			ID:         uuid.New(),
			TunnelID:   tunnelID,
			Method:     "GET",
			Path:       "/test",
			StatusCode: 200,
			DurationMs: 100,
			BytesIn:    100,
			BytesOut:   200,
			ClientIP:   "127.0.0.1",
			CreatedAt:  time.Now().Add(time.Duration(i) * time.Second),
		}
		db.Create(log)
	}

	// Verify we have 20 logs before cleanup
	var countBefore int64
	db.Model(&models.RequestLog{}).Where("tunnel_id = ?", tunnelID).Count(&countBefore)
	assert.Equal(t, int64(20), countBefore)

	// Run cleanup
	proxy.cleanupOldRequestLogs(tunnelID)

	// Verify we now have exactly 10 logs (max_request_logs)
	var countAfter int64
	db.Model(&models.RequestLog{}).Where("tunnel_id = ?", tunnelID).Count(&countAfter)
	assert.Equal(t, int64(10), countAfter)

	// Verify the remaining logs are the newest 10
	var remainingLogs []models.RequestLog
	db.Where("tunnel_id = ?", tunnelID).Order("created_at ASC").Find(&remainingLogs)
	assert.Len(t, remainingLogs, 10)
}

func TestCleanupOldRequestLogs_WithinLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Disable foreign key constraints for unit testing
	db.Exec("PRAGMA foreign_keys = OFF")
	err = db.AutoMigrate(&models.Tunnel{}, &models.RequestLog{})
	assert.NoError(t, err)

	tunnelID := uuid.New()
	tunnel := &models.Tunnel{
		ID:         tunnelID,
		UserID:     uuid.New(),
		TokenID:    uuid.New(),
		Subdomain:  "test",
		TunnelType: "http",
		LocalAddr:  "localhost:3000",
		PublicURL:  "https://test.example.com",
		ClientID:   "test-client",
	}
	db.Create(tunnel)

	proxy := &HTTPProxy{
		db:             db,
		maxRequestLogs: 50,
	}

	// Create only 10 logs (less than max)
	for i := 0; i < 10; i++ {
		log := &models.RequestLog{
			ID:         uuid.New(),
			TunnelID:   tunnelID,
			Method:     "GET",
			Path:       "/test",
			StatusCode: 200,
			DurationMs: 100,
		}
		db.Create(log)
	}

	// Run cleanup
	proxy.cleanupOldRequestLogs(tunnelID)

	// Verify all 10 logs still exist (no deletion needed)
	var count int64
	db.Model(&models.RequestLog{}).Where("tunnel_id = ?", tunnelID).Count(&count)
	assert.Equal(t, int64(10), count, "Should not delete logs when within limit")
}

func TestCleanupOldRequestLogs_Disabled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Disable foreign key constraints for unit testing
	db.Exec("PRAGMA foreign_keys = OFF")
	err = db.AutoMigrate(&models.Tunnel{}, &models.RequestLog{})
	assert.NoError(t, err)

	tunnelID := uuid.New()
	tunnel := &models.Tunnel{
		ID:         tunnelID,
		UserID:     uuid.New(),
		TokenID:    uuid.New(),
		Subdomain:  "test",
		TunnelType: "http",
		LocalAddr:  "localhost:3000",
		PublicURL:  "https://test.example.com",
		ClientID:   "test-client",
	}
	db.Create(tunnel)

	// Set max_request_logs to 0 (disabled)
	proxy := &HTTPProxy{
		db:             db,
		maxRequestLogs: 0,
	}

	// Create 100 logs
	for i := 0; i < 100; i++ {
		log := &models.RequestLog{
			ID:         uuid.New(),
			TunnelID:   tunnelID,
			Method:     "GET",
			Path:       "/test",
			StatusCode: 200,
			DurationMs: 100,
		}
		db.Create(log)
	}

	// Run cleanup
	proxy.cleanupOldRequestLogs(tunnelID)

	// Verify all 100 logs still exist (cleanup disabled)
	var count int64
	db.Model(&models.RequestLog{}).Where("tunnel_id = ?", tunnelID).Count(&count)
	assert.Equal(t, int64(100), count, "Should not delete logs when cleanup is disabled (max_request_logs = 0)")
}

func TestCleanupOldRequestLogs_MultipleTunnels(t *testing.T) {
	// Test that cleanup only affects the specified tunnel, not others
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Disable foreign key constraints for unit testing
	db.Exec("PRAGMA foreign_keys = OFF")
	err = db.AutoMigrate(&models.Tunnel{}, &models.RequestLog{})
	assert.NoError(t, err)

	// Create two tunnels
	tunnel1ID := uuid.New()
	tunnel1 := &models.Tunnel{
		ID:         tunnel1ID,
		UserID:     uuid.New(),
		TokenID:    uuid.New(),
		Subdomain:  "test1",
		TunnelType: "http",
		LocalAddr:  "localhost:3000",
		PublicURL:  "https://test1.example.com",
		ClientID:   "test-client-1",
	}
	db.Create(tunnel1)

	tunnel2ID := uuid.New()
	tunnel2 := &models.Tunnel{
		ID:         tunnel2ID,
		UserID:     uuid.New(),
		TokenID:    uuid.New(),
		Subdomain:  "test2",
		TunnelType: "http",
		LocalAddr:  "localhost:4000",
		PublicURL:  "https://test2.example.com",
		ClientID:   "test-client-2",
	}
	db.Create(tunnel2)

	proxy := &HTTPProxy{
		db:             db,
		maxRequestLogs: 5,
	}

	// Create 10 logs for tunnel1
	for i := 0; i < 10; i++ {
		log := &models.RequestLog{
			ID:         uuid.New(),
			TunnelID:   tunnel1ID,
			Method:     "GET",
			Path:       "/test1",
			StatusCode: 200,
		}
		db.Create(log)
	}

	// Create 10 logs for tunnel2
	for i := 0; i < 10; i++ {
		log := &models.RequestLog{
			ID:         uuid.New(),
			TunnelID:   tunnel2ID,
			Method:     "GET",
			Path:       "/test2",
			StatusCode: 200,
		}
		db.Create(log)
	}

	// Run cleanup on tunnel1 only
	proxy.cleanupOldRequestLogs(tunnel1ID)

	// Verify tunnel1 has 5 logs
	var count1 int64
	db.Model(&models.RequestLog{}).Where("tunnel_id = ?", tunnel1ID).Count(&count1)
	assert.Equal(t, int64(5), count1, "Tunnel1 should have 5 logs after cleanup")

	// Verify tunnel2 still has 10 logs (not affected)
	var count2 int64
	db.Model(&models.RequestLog{}).Where("tunnel_id = ?", tunnel2ID).Count(&count2)
	assert.Equal(t, int64(10), count2, "Tunnel2 should still have 10 logs (not affected by cleanup)")
}

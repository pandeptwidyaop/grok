package tunnel

import (
	"context"
	"testing"

	"github.com/google/uuid"
	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(database)
	require.NoError(t, err)

	return database
}

func TestAllocateSubdomain(t *testing.T) {
	database := setupTestDB(t)
	manager := NewManager(database, "localhost", 10)
	ctx := context.Background()
	userID := uuid.New()

	t.Run("allocate custom subdomain", func(t *testing.T) {
		subdomain, err := manager.AllocateSubdomain(ctx, userID, "myapp")
		require.NoError(t, err)
		assert.Equal(t, "myapp", subdomain)
	})

	t.Run("reject duplicate subdomain", func(t *testing.T) {
		_, err := manager.AllocateSubdomain(ctx, userID, "myapp")
		assert.Error(t, err)
	})

	t.Run("reject reserved subdomain", func(t *testing.T) {
		_, err := manager.AllocateSubdomain(ctx, userID, "api")
		assert.Error(t, err)
	})

	t.Run("generate random subdomain", func(t *testing.T) {
		subdomain, err := manager.AllocateSubdomain(ctx, userID, "")
		require.NoError(t, err)
		assert.Len(t, subdomain, 8)
	})
}

func TestRegisterTunnel(t *testing.T) {
	database := setupTestDB(t)
	manager := NewManager(database, "localhost", 10)
	ctx := context.Background()

	userID := uuid.New()
	tokenID := uuid.New()

	// Allocate subdomain first
	subdomain, err := manager.AllocateSubdomain(ctx, userID, "testapp")
	require.NoError(t, err)

	// Create tunnel
	tunnel := NewTunnel(
		userID,
		tokenID,
		subdomain,
		tunnelv1.TunnelProtocol_HTTP,
		"localhost:3000",
		"https://testapp.localhost",
		nil,
	)

	t.Run("register tunnel successfully", func(t *testing.T) {
		err := manager.RegisterTunnel(ctx, tunnel)
		require.NoError(t, err)

		// Verify in memory
		retrieved, exists := manager.GetTunnelBySubdomain(subdomain)
		assert.True(t, exists)
		assert.Equal(t, tunnel.ID, retrieved.ID)
	})

	t.Run("retrieve tunnel by ID", func(t *testing.T) {
		retrieved, exists := manager.GetTunnelByID(tunnel.ID)
		assert.True(t, exists)
		assert.Equal(t, subdomain, retrieved.Subdomain)
	})

	t.Run("unregister tunnel", func(t *testing.T) {
		err := manager.UnregisterTunnel(ctx, tunnel.ID)
		require.NoError(t, err)

		// Should not exist in memory
		_, exists := manager.GetTunnelBySubdomain(subdomain)
		assert.False(t, exists)

		// Domain should be deleted (allowing reuse)
		newSubdomain, err := manager.AllocateSubdomain(ctx, userID, subdomain)
		require.NoError(t, err)
		assert.Equal(t, subdomain, newSubdomain)
	})
}

func TestGetUserTunnels(t *testing.T) {
	database := setupTestDB(t)
	manager := NewManager(database, "localhost", 10)
	ctx := context.Background()

	userID := uuid.New()
	tokenID := uuid.New()

	// Create multiple tunnels
	tunnels := make([]*Tunnel, 3)
	for i := 0; i < 3; i++ {
		subdomain, err := manager.AllocateSubdomain(ctx, userID, "")
		require.NoError(t, err)

		tunnel := NewTunnel(
			userID,
			tokenID,
			subdomain,
			tunnelv1.TunnelProtocol_HTTP,
			"localhost:3000",
			"https://"+subdomain+".localhost",
			nil,
		)

		err = manager.RegisterTunnel(ctx, tunnel)
		require.NoError(t, err)
		tunnels[i] = tunnel
	}

	// Get all user tunnels
	userTunnels := manager.GetUserTunnels(userID)
	assert.Len(t, userTunnels, 3)

	// Verify different user has no tunnels
	otherUserTunnels := manager.GetUserTunnels(uuid.New())
	assert.Len(t, otherUserTunnels, 0)
}

func TestCountActiveTunnels(t *testing.T) {
	database := setupTestDB(t)
	manager := NewManager(database, "localhost", 10)
	ctx := context.Background()

	assert.Equal(t, 0, manager.CountActiveTunnels())

	userID := uuid.New()
	tokenID := uuid.New()

	// Register 2 tunnels
	for i := 0; i < 2; i++ {
		subdomain, err := manager.AllocateSubdomain(ctx, userID, "")
		require.NoError(t, err)

		tunnel := NewTunnel(
			userID,
			tokenID,
			subdomain,
			tunnelv1.TunnelProtocol_HTTP,
			"localhost:3000",
			"https://"+subdomain+".localhost",
			nil,
		)

		err = manager.RegisterTunnel(ctx, tunnel)
		require.NoError(t, err)
	}

	assert.Equal(t, 2, manager.CountActiveTunnels())
}

func TestBuildPublicURL(t *testing.T) {
	database := setupTestDB(t)
	manager := NewManager(database, "example.com", 10)

	tests := []struct {
		name      string
		subdomain string
		protocol  string
		expected  string
	}{
		{"HTTP", "myapp", "http", "https://myapp.example.com"},
		{"HTTPS", "secure", "https", "https://secure.example.com"},
		{"TCP", "database", "tcp", "tcp://database.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := manager.BuildPublicURL(tt.subdomain, tt.protocol)
			assert.Equal(t, tt.expected, url)
		})
	}
}

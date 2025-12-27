package tunnel

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db"
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
	manager := NewManager(database, "localhost", 10, true, 80, 443, 10000, 20000) // TLS enabled, standard ports
	ctx := context.Background()
	userID := uuid.New()

	t.Run("allocate custom subdomain", func(t *testing.T) {
		subdomain, customPart, err := manager.AllocateSubdomain(ctx, userID, nil, "myapp")
		require.NoError(t, err)
		assert.Equal(t, "myapp", subdomain)
		assert.Equal(t, "myapp", customPart)
	})

	t.Run("reject duplicate subdomain", func(t *testing.T) {
		_, _, err := manager.AllocateSubdomain(ctx, userID, nil, "myapp")
		// With persistent tunnels, domains stay reserved even after disconnect
		// So this should error because "myapp" is already allocated
		assert.Error(t, err)
	})

	t.Run("reject reserved subdomain", func(t *testing.T) {
		_, _, err := manager.AllocateSubdomain(ctx, userID, nil, "api")
		assert.Error(t, err)
	})

	t.Run("generate random subdomain", func(t *testing.T) {
		subdomain, _, err := manager.AllocateSubdomain(ctx, userID, nil, "")
		require.NoError(t, err)
		assert.Len(t, subdomain, 8)
	})
}

func TestRegisterTunnel(t *testing.T) {
	database := setupTestDB(t)
	manager := NewManager(database, "localhost", 10, true, 80, 443, 10000, 20000) // TLS enabled, standard ports
	ctx := context.Background()

	userID := uuid.New()
	tokenID := uuid.New()

	// Allocate subdomain first
	subdomain, _, err := manager.AllocateSubdomain(ctx, userID, nil, "testapp")
	require.NoError(t, err)

	// Create tunnel
	tunnel := NewTunnel(
		userID,
		tokenID,
		nil, // no orgID
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

		// Domain should still be reserved (persistent tunnels keep domains)
		_, _, err = manager.AllocateSubdomain(ctx, userID, nil, subdomain)
		assert.Error(t, err) // Should error because domain is still reserved
	})
}

func TestGetUserTunnels(t *testing.T) {
	database := setupTestDB(t)
	manager := NewManager(database, "localhost", 10, true, 80, 443, 10000, 20000) // TLS enabled, standard ports
	ctx := context.Background()

	userID := uuid.New()
	tokenID := uuid.New()

	// Create multiple tunnels
	tunnels := make([]*Tunnel, 3)
	for i := 0; i < 3; i++ {
		subdomain, _, err := manager.AllocateSubdomain(ctx, userID, nil, "")
		require.NoError(t, err)

		tunnel := NewTunnel(
			userID,
			tokenID,
			nil, // no orgID
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
	manager := NewManager(database, "localhost", 10, true, 80, 443, 10000, 20000) // TLS enabled, standard ports
	ctx := context.Background()

	assert.Equal(t, 0, manager.CountActiveTunnels())

	userID := uuid.New()
	tokenID := uuid.New()

	// Register 2 tunnels
	for i := 0; i < 2; i++ {
		subdomain, _, err := manager.AllocateSubdomain(ctx, userID, nil, "")
		require.NoError(t, err)

		tunnel := NewTunnel(
			userID,
			tokenID,
			nil, // no orgID
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

	tests := []struct {
		name       string
		tlsEnabled bool
		httpPort   int
		httpsPort  int
		subdomain  string
		protocol   string
		expected   string
	}{
		{"HTTPS default port", true, 80, 443, "myapp", "http", "https://myapp.example.com"},
		{"HTTPS custom port", true, 80, 8443, "myapp", "https", "https://myapp.example.com:8443"},
		{"HTTP default port", false, 80, 443, "myapp", "http", "http://myapp.example.com"},
		{"HTTP custom port", false, 8080, 443, "myapp", "http", "http://myapp.example.com:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(database, "example.com", 10, tt.tlsEnabled, tt.httpPort, tt.httpsPort, 10000, 20000)
			url := manager.BuildPublicURL(tt.subdomain, tt.protocol)
			assert.Equal(t, tt.expected, url)
		})
	}
}

func TestBuildTCPPublicURL(t *testing.T) {
	database := setupTestDB(t)
	manager := NewManager(database, "example.com", 10, true, 80, 443, 10000, 20000)

	tests := []struct {
		name     string
		port     int
		expected string
	}{
		{"Standard port", 12345, "tcp://example.com:12345"},
		{"Low port", 10000, "tcp://example.com:10000"},
		{"High port", 20000, "tcp://example.com:20000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := manager.BuildTCPPublicURL(tt.port)
			assert.Equal(t, tt.expected, url)
		})
	}
}

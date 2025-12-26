package tcp

import (
	"testing"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate tables
	err = database.AutoMigrate(&models.User{}, &models.AuthToken{}, &models.Tunnel{}, &models.Domain{})
	require.NoError(t, err)

	return database
}

func setupTestPortPool(t *testing.T) (*PortPool, *gorm.DB) {
	// Create in-memory database
	database := setupTestDB(t)

	// Create port pool with small range for testing
	pp, err := NewPortPool(database, 10000, 10010)
	require.NoError(t, err)

	return pp, database
}

func TestNewPortPool(t *testing.T) {
	t.Run("valid port range", func(t *testing.T) {
		database := setupTestDB(t)

		pp, err := NewPortPool(database, 10000, 10010)
		require.NoError(t, err)
		assert.NotNil(t, pp)

		stats := pp.GetStats()
		assert.Equal(t, 11, stats["total_ports"])
		assert.Equal(t, 0, stats["allocated_ports"])
		assert.Equal(t, 11, stats["available_ports"])
	})

	t.Run("invalid start port (privileged)", func(t *testing.T) {
		database := setupTestDB(t)

		_, err := NewPortPool(database, 80, 100)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "privileged")
	})

	t.Run("invalid range (end < start)", func(t *testing.T) {
		database := setupTestDB(t)

		_, err := NewPortPool(database, 10000, 9000)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "greater than start")
	})

	t.Run("invalid end port (> 65535)", func(t *testing.T) {
		database := setupTestDB(t)

		_, err := NewPortPool(database, 10000, 70000)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "65535")
	})
}

func TestPortPool_AllocatePort(t *testing.T) {
	pp, _ := setupTestPortPool(t)

	tunnelID := uuid.New()

	t.Run("successful allocation", func(t *testing.T) {
		port, err := pp.AllocatePort(tunnelID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, port, 10000)
		assert.LessOrEqual(t, port, 10010)

		// Verify port is allocated
		allocated, exists := pp.GetTunnelByPort(port)
		assert.True(t, exists)
		assert.Equal(t, tunnelID, allocated)

		// Verify stats updated
		stats := pp.GetStats()
		assert.Equal(t, 1, stats["allocated_ports"])
		assert.Equal(t, 10, stats["available_ports"])
	})

	t.Run("allocate same tunnel twice returns same port", func(t *testing.T) {
		port1, err := pp.AllocatePort(tunnelID)
		require.NoError(t, err)

		// Second allocation should return same port (idempotent)
		port2, err := pp.AllocatePort(tunnelID)
		require.NoError(t, err)
		assert.Equal(t, port1, port2)

		// Stats should not change
		stats := pp.GetStats()
		assert.Equal(t, 1, stats["allocated_ports"])
	})

	t.Run("multiple allocations", func(t *testing.T) {
		pp2, _ := setupTestPortPool(t)

		allocatedPorts := make(map[int]bool)

		// Allocate all 11 ports
		for i := 0; i < 11; i++ {
			tunnelID := uuid.New()
			port, err := pp2.AllocatePort(tunnelID)
			require.NoError(t, err)

			// Verify port is unique
			assert.False(t, allocatedPorts[port], "Port %d allocated twice", port)
			allocatedPorts[port] = true
		}

		// Verify all ports allocated
		stats := pp2.GetStats()
		assert.Equal(t, 11, stats["allocated_ports"])
		assert.Equal(t, 0, stats["available_ports"])
		assert.Equal(t, 100.0, stats["utilization"])
	})

	t.Run("no available ports", func(t *testing.T) {
		pp3, _ := setupTestPortPool(t)

		// Allocate all ports
		for i := 0; i < 11; i++ {
			_, err := pp3.AllocatePort(uuid.New())
			require.NoError(t, err)
		}

		// Try to allocate one more
		_, err := pp3.AllocatePort(uuid.New())
		assert.ErrorIs(t, err, pkgerrors.ErrNoAvailablePorts)
	})
}

func TestPortPool_ReleasePort(t *testing.T) {
	t.Run("release non-persistent tunnel port", func(t *testing.T) {
		pp, _ := setupTestPortPool(t)

		tunnelID := uuid.New()
		port, err := pp.AllocatePort(tunnelID)
		require.NoError(t, err)

		// Release port (non-persistent)
		err = pp.ReleasePort(port, false)
		require.NoError(t, err)

		// Verify port is no longer allocated
		_, exists := pp.GetTunnelByPort(port)
		assert.False(t, exists)

		// Verify port is back in available pool
		stats := pp.GetStats()
		assert.Equal(t, 0, stats["allocated_ports"])
		assert.Equal(t, 11, stats["available_ports"])

		// Verify a port can be allocated again (may or may not be the same port)
		newTunnelID := uuid.New()
		newPort, err := pp.AllocatePort(newTunnelID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, newPort, 10000)
		assert.LessOrEqual(t, newPort, 10010)

		// Verify the released port is now available
		assert.True(t, pp.IsPortAvailable(port) || newPort == port, "Released port should be available or reallocated")
	})

	t.Run("release persistent tunnel port (keep reserved)", func(t *testing.T) {
		pp, _ := setupTestPortPool(t)

		tunnelID := uuid.New()
		port, err := pp.AllocatePort(tunnelID)
		require.NoError(t, err)

		// Release port (persistent - keep reserved)
		err = pp.ReleasePort(port, true)
		require.NoError(t, err)

		// Verify port is still allocated
		allocated, exists := pp.GetTunnelByPort(port)
		assert.True(t, exists)
		assert.Equal(t, tunnelID, allocated)

		// Stats should show port still allocated
		stats := pp.GetStats()
		assert.Equal(t, 1, stats["allocated_ports"])
		assert.Equal(t, 10, stats["available_ports"])
	})

	t.Run("release port outside range", func(t *testing.T) {
		pp, _ := setupTestPortPool(t)

		err := pp.ReleasePort(9999, false) // Outside range
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "outside managed range")
	})

	t.Run("release unallocated port (no-op)", func(t *testing.T) {
		pp, _ := setupTestPortPool(t)

		// Release port that was never allocated
		err := pp.ReleasePort(10005, false)
		assert.NoError(t, err) // Should be no-op, not error
	})
}

func TestPortPool_ReallocatePortForTunnel(t *testing.T) {
	t.Run("reallocate same port for persistent tunnel", func(t *testing.T) {
		pp, _ := setupTestPortPool(t)

		tunnelID := uuid.New()

		// Allocate port initially
		port1, err := pp.AllocatePort(tunnelID)
		require.NoError(t, err)

		// Simulate disconnect (keep port reserved for persistent)
		err = pp.ReleasePort(port1, true)
		require.NoError(t, err)

		// Reallocate same port
		port2, err := pp.ReallocatePortForTunnel(tunnelID, port1)
		require.NoError(t, err)
		assert.Equal(t, port1, port2)

		// Verify port is still allocated to same tunnel
		allocated, exists := pp.GetTunnelByPort(port2)
		assert.True(t, exists)
		assert.Equal(t, tunnelID, allocated)
	})

	t.Run("reallocate port allocated to different tunnel", func(t *testing.T) {
		pp, _ := setupTestPortPool(t)

		tunnelID1 := uuid.New()
		tunnelID2 := uuid.New()

		// Allocate port to tunnel 1
		port, err := pp.AllocatePort(tunnelID1)
		require.NoError(t, err)

		// Try to reallocate same port for tunnel 2 (should fail)
		_, err = pp.ReallocatePortForTunnel(tunnelID2, port)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already allocated to different tunnel")
	})
}

func TestPortPool_LoadAllocatedPorts(t *testing.T) {
	t.Run("restore active tunnels from database", func(t *testing.T) {
		database := setupTestDB(t)

		// Create active tunnel with allocated port
		userID := uuid.New()
		tokenID := uuid.New()
		remotePort := 10005

		tunnel := &models.Tunnel{
			ID:         uuid.New(),
			UserID:     userID,
			TokenID:    tokenID,
			TunnelType: "tcp",
			RemotePort: &remotePort,
			LocalAddr:  "localhost:22",
			PublicURL:  "tcp://localhost:10005",
			ClientID:   uuid.New().String(),
			Status:     "active",
		}

		err := database.Create(tunnel).Error
		require.NoError(t, err)

		// Create port pool - should load allocated port from database
		pp, err := NewPortPool(database, 10000, 10010)
		require.NoError(t, err)

		// Verify port is allocated
		allocatedTunnelID, exists := pp.GetTunnelByPort(10005)
		assert.True(t, exists)
		assert.Equal(t, tunnel.ID, allocatedTunnelID)

		// Verify stats
		stats := pp.GetStats()
		assert.Equal(t, 1, stats["allocated_ports"])
		assert.Equal(t, 10, stats["available_ports"])
	})

	t.Run("restore persistent offline tunnels from database", func(t *testing.T) {
		database := setupTestDB(t)

		// Create persistent offline tunnel with allocated port
		userID := uuid.New()
		tokenID := uuid.New()
		remotePort := 10007
		savedName := "ssh-tunnel"

		tunnel := &models.Tunnel{
			ID:           uuid.New(),
			UserID:       userID,
			TokenID:      tokenID,
			TunnelType:   "tcp",
			RemotePort:   &remotePort,
			LocalAddr:    "localhost:22",
			PublicURL:    "tcp://localhost:10007",
			ClientID:     uuid.New().String(),
			SavedName:    &savedName,
			IsPersistent: true,
			Status:       "offline",
		}

		err := database.Create(tunnel).Error
		require.NoError(t, err)

		// Create port pool - should load allocated port from database
		pp, err := NewPortPool(database, 10000, 10010)
		require.NoError(t, err)

		// Verify port is still allocated (reserved for persistent tunnel)
		allocatedTunnelID, exists := pp.GetTunnelByPort(10007)
		assert.True(t, exists)
		assert.Equal(t, tunnel.ID, allocatedTunnelID)

		// Verify stats
		stats := pp.GetStats()
		assert.Equal(t, 1, stats["allocated_ports"])
		assert.Equal(t, 10, stats["available_ports"])
	})

	t.Run("ignore non-persistent offline tunnels", func(t *testing.T) {
		database := setupTestDB(t)

		// Create non-persistent offline tunnel (should be ignored)
		userID := uuid.New()
		tokenID := uuid.New()
		remotePort := 10008

		tunnel := &models.Tunnel{
			ID:           uuid.New(),
			UserID:       userID,
			TokenID:      tokenID,
			TunnelType:   "tcp",
			RemotePort:   &remotePort,
			LocalAddr:    "localhost:22",
			PublicURL:    "tcp://localhost:10008",
			ClientID:     uuid.New().String(),
			IsPersistent: false,
			Status:       "offline",
		}

		err := database.Create(tunnel).Error
		require.NoError(t, err)

		// Create port pool - should NOT load port from non-persistent offline tunnel
		pp, err := NewPortPool(database, 10000, 10010)
		require.NoError(t, err)

		// Verify port is NOT allocated
		_, exists := pp.GetTunnelByPort(10008)
		assert.False(t, exists)

		// Verify stats - all ports available
		stats := pp.GetStats()
		assert.Equal(t, 0, stats["allocated_ports"])
		assert.Equal(t, 11, stats["available_ports"])
	})
}

func TestPortPool_IsPortAvailable(t *testing.T) {
	pp, _ := setupTestPortPool(t)

	tunnelID := uuid.New()
	port, err := pp.AllocatePort(tunnelID)
	require.NoError(t, err)

	t.Run("allocated port not available", func(t *testing.T) {
		assert.False(t, pp.IsPortAvailable(port))
	})

	t.Run("unallocated port in range available", func(t *testing.T) {
		assert.True(t, pp.IsPortAvailable(10001))
	})

	t.Run("port outside range not available", func(t *testing.T) {
		assert.False(t, pp.IsPortAvailable(9999))
		assert.False(t, pp.IsPortAvailable(10011))
	})
}

func TestPortPool_GetAllocatedPort(t *testing.T) {
	pp, _ := setupTestPortPool(t)

	tunnelID := uuid.New()

	t.Run("get port for tunnel", func(t *testing.T) {
		port, err := pp.AllocatePort(tunnelID)
		require.NoError(t, err)

		retrievedPort, exists := pp.GetAllocatedPort(tunnelID)
		assert.True(t, exists)
		assert.Equal(t, port, retrievedPort)
	})

	t.Run("get port for non-existent tunnel", func(t *testing.T) {
		_, exists := pp.GetAllocatedPort(uuid.New())
		assert.False(t, exists)
	})
}

func TestPortPool_Concurrent(t *testing.T) {
	t.Run("concurrent allocations", func(t *testing.T) {
		pp, _ := setupTestPortPool(t)

		// Use all 11 ports concurrently
		numGoroutines := 11
		results := make(chan int, numGoroutines)
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				tunnelID := uuid.New()
				port, err := pp.AllocatePort(tunnelID)
				if err != nil {
					errors <- err
					return
				}
				results <- port
			}()
		}

		// Collect results
		ports := make(map[int]bool)
		for i := 0; i < numGoroutines; i++ {
			select {
			case port := <-results:
				assert.False(t, ports[port], "Port %d allocated twice concurrently", port)
				ports[port] = true
			case err := <-errors:
				t.Fatalf("Concurrent allocation failed: %v", err)
			}
		}

		// Verify all ports unique
		assert.Equal(t, numGoroutines, len(ports))
	})
}

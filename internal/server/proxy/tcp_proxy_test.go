package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
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

func TestNewTCPProxy(t *testing.T) {
	database := setupTestDB(t)
	manager := tunnel.NewManager(database, "localhost", 10, true, 80, 443, 10000, 20000)

	proxy := NewTCPProxy(manager)
	require.NotNil(t, proxy)
	assert.NotNil(t, proxy.tunnelManager)
	assert.NotNil(t, proxy.listeners)
}

func TestTCPProxy_StartStopListener(t *testing.T) {
	database := setupTestDB(t)
	manager := tunnel.NewManager(database, "localhost", 10, true, 80, 443, 10000, 20000)

	proxy := NewTCPProxy(manager)
	defer proxy.Shutdown()

	tunnelID := uuid.New()
	port := 15000

	// Start listener
	err := proxy.StartListener(port, tunnelID)
	require.NoError(t, err)

	// Verify listener is active
	activePorts := proxy.GetActiveListeners()
	assert.Contains(t, activePorts, port)

	// Try to start duplicate listener (should fail)
	err = proxy.StartListener(port, tunnelID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listener already exists")

	// Stop listener
	err = proxy.StopListener(port)
	require.NoError(t, err)

	// Verify listener is removed
	activePorts = proxy.GetActiveListeners()
	assert.NotContains(t, activePorts, port)
}

func TestTCPProxy_ShutdownCleansUpListeners(t *testing.T) {
	database := setupTestDB(t)
	manager := tunnel.NewManager(database, "localhost", 10, true, 80, 443, 10000, 20000)

	proxy := NewTCPProxy(manager)

	// Start multiple listeners
	tunnelID1 := uuid.New()
	tunnelID2 := uuid.New()

	err := proxy.StartListener(15001, tunnelID1)
	require.NoError(t, err)

	err = proxy.StartListener(15002, tunnelID2)
	require.NoError(t, err)

	// Verify listeners are active
	activePorts := proxy.GetActiveListeners()
	assert.Len(t, activePorts, 2)

	// Shutdown
	proxy.Shutdown()

	// Give it a moment to clean up
	time.Sleep(100 * time.Millisecond)

	// Verify all listeners are removed
	activePorts = proxy.GetActiveListeners()
	assert.Len(t, activePorts, 0)
}

func TestTCPProxy_Integration(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	manager := tunnel.NewManager(database, "localhost", 10, true, 80, 443, 10000, 20000)

	proxy := NewTCPProxy(manager)
	defer proxy.Shutdown()

	// Set the TCP proxy on the manager
	manager.SetTCPProxy(proxy)

	userID := uuid.New()
	tokenID := uuid.New()

	// Allocate subdomain
	subdomain, _, err := manager.AllocateSubdomain(ctx, userID, nil, "")
	require.NoError(t, err)

	// Create TCP tunnel
	tun := tunnel.NewTunnel(
		userID,
		tokenID,
		nil,
		subdomain,
		tunnelv1.TunnelProtocol_TCP,
		"localhost:22",
		"tcp://localhost:12000", // Will be updated by RegisterTunnel
		nil,
	)

	// Register tunnel (should allocate port and start listener)
	err = manager.RegisterTunnel(ctx, tun)
	require.NoError(t, err)

	// Verify port was allocated
	assert.NotNil(t, tun.RemotePort)
	allocatedPort := *tun.RemotePort

	// Verify listener is active
	activePorts := proxy.GetActiveListeners()
	assert.Contains(t, activePorts, allocatedPort)

	// Unregister tunnel (should stop listener and release port)
	err = manager.UnregisterTunnel(ctx, tun.ID)
	require.NoError(t, err)

	// Give it a moment to clean up
	time.Sleep(100 * time.Millisecond)

	// Verify listener is stopped
	activePorts = proxy.GetActiveListeners()
	assert.NotContains(t, activePorts, allocatedPort)
}

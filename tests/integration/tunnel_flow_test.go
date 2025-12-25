package integration

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	grpcserver "github.com/pandeptwidyaop/grok/internal/server/grpc"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	"github.com/pandeptwidyaop/grok/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const bufSize = 1024 * 1024

// setupTestServer creates a test gRPC server with in-memory database
func setupTestServer(t *testing.T) (*grpc.Server, *gorm.DB, *tunnel.Manager, *auth.TokenService, *bufconn.Listener) {
	// Create in-memory SQLite database for testing
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(database)
	require.NoError(t, err)

	// Create test user
	testUser := &models.User{
		Email:    "test@example.com",
		Password: "hashedpassword",
		Name:     "Test User",
		IsActive: true,
	}
	require.NoError(t, database.Create(testUser).Error)

	// Create test token
	tokenHash := utils.HashToken("test-token-12345")

	testToken := &models.AuthToken{
		UserID:    testUser.ID,
		TokenHash: tokenHash,
		Name:      "Test Token",
		Scopes:    datatypes.JSON(`["tunnel:create"]`),
		IsActive:  true,
	}
	require.NoError(t, database.Create(testToken).Error)

	// Create services
	tokenService := auth.NewTokenService(database)
	tunnelManager := tunnel.NewManager(database, "localhost", 10)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	tunnelService := grpcserver.NewTunnelService(tunnelManager, tokenService)
	tunnelv1.RegisterTunnelServiceServer(grpcServer, tunnelService)

	// Create bufconn listener for testing
	lis := bufconn.Listen(bufSize)

	// Start server in goroutine
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	return grpcServer, database, tunnelManager, tokenService, lis
}

// TestCompleteTunnelFlow tests the complete tunnel creation and data persistence
func TestCompleteTunnelFlow(t *testing.T) {
	grpcServer, database, tunnelManager, _, lis := setupTestServer(t)
	defer grpcServer.Stop()

	// Create client connection
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := tunnelv1.NewTunnelServiceClient(conn)

	// Test data
	testSubdomain := "mytest"
	testLocalAddr := "localhost:8080"
	testToken := "test-token-12345"

	// Step 1: Create tunnel via CreateTunnel RPC
	createResp, err := client.CreateTunnel(ctx, &tunnelv1.CreateTunnelRequest{
		AuthToken:    testToken,
		Protocol:     tunnelv1.TunnelProtocol_HTTP,
		LocalAddress: testLocalAddr,
		Subdomain:    testSubdomain,
	})
	require.NoError(t, err, "CreateTunnel should succeed")
	assert.NotEmpty(t, createResp.PublicUrl, "Public URL should not be empty")
	assert.Equal(t, testSubdomain, createResp.Subdomain, "Subdomain should match")
	assert.Equal(t, tunnelv1.TunnelStatus_ACTIVE, createResp.Status)

	// Step 2: Connect ProxyStream and register tunnel
	stream, err := client.ProxyStream(ctx)
	require.NoError(t, err, "ProxyStream should connect")

	// Send registration message
	regMsg := &tunnelv1.ProxyMessage{
		Message: &tunnelv1.ProxyMessage_Control{
			Control: &tunnelv1.ControlMessage{
				Type:     tunnelv1.ControlMessage_UNKNOWN,
				TunnelId: testSubdomain + "|" + testToken + "|" + testLocalAddr + "|" + createResp.PublicUrl,
			},
		},
	}
	err = stream.Send(regMsg)
	require.NoError(t, err, "Should send registration message")

	// Wait for tunnel to be registered
	time.Sleep(100 * time.Millisecond)

	// Step 3: Verify tunnel data in database
	var dbTunnel models.Tunnel
	err = database.Where("subdomain = ?", testSubdomain).First(&dbTunnel).Error
	require.NoError(t, err, "Tunnel should exist in database")

	// Assert all fields are populated correctly
	assert.Equal(t, testSubdomain, dbTunnel.Subdomain, "Subdomain should match")
	assert.Equal(t, testLocalAddr, dbTunnel.LocalAddr, "LocalAddr should be saved")
	assert.Equal(t, createResp.PublicUrl, dbTunnel.PublicURL, "PublicURL should be saved")
	assert.Equal(t, "HTTPS", dbTunnel.TunnelType, "TunnelType should be HTTPS")
	assert.Equal(t, "active", dbTunnel.Status, "Status should be active")
	assert.NotEmpty(t, dbTunnel.ClientID, "ClientID should not be empty")
	assert.NotZero(t, dbTunnel.ConnectedAt, "ConnectedAt should be set")

	// Step 4: Verify tunnel in memory
	tun, exists := tunnelManager.GetTunnelBySubdomain(testSubdomain)
	assert.True(t, exists, "Tunnel should exist in memory")
	assert.Equal(t, testLocalAddr, tun.LocalAddr, "In-memory LocalAddr should match")
	assert.Equal(t, createResp.PublicUrl, tun.PublicURL, "In-memory PublicURL should match")

	// Step 5: Close stream and verify cleanup
	stream.CloseSend()
	time.Sleep(100 * time.Millisecond)

	// Verify tunnel is unregistered from memory
	_, exists = tunnelManager.GetTunnelBySubdomain(testSubdomain)
	assert.False(t, exists, "Tunnel should be removed from memory after disconnect")

	// Verify domain reservation is cleaned up
	var domainCount int64
	database.Model(&models.Domain{}).Where("subdomain = ?", testSubdomain).Count(&domainCount)
	assert.Equal(t, int64(0), domainCount, "Domain reservation should be deleted after disconnect")

	// Verify tunnel status is updated in database
	err = database.Where("subdomain = ?", testSubdomain).First(&dbTunnel).Error
	require.NoError(t, err)
	assert.Equal(t, "disconnected", dbTunnel.Status, "Status should be updated to disconnected")
}

// TestSubdomainAllocation tests subdomain allocation and validation
func TestSubdomainAllocation(t *testing.T) {
	grpcServer, database, tunnelManager, _, _ := setupTestServer(t)
	defer grpcServer.Stop()

	ctx := context.Background()
	userID := uuid.New()

	t.Run("allocate custom subdomain", func(t *testing.T) {
		subdomain, err := tunnelManager.AllocateSubdomain(ctx, userID, "custom")
		require.NoError(t, err)
		assert.Equal(t, "custom", subdomain)

		// Verify in database
		var domain models.Domain
		err = database.Where("subdomain = ?", "custom").First(&domain).Error
		require.NoError(t, err)
	})

	t.Run("reject reserved subdomain", func(t *testing.T) {
		_, err := tunnelManager.AllocateSubdomain(ctx, userID, "api")
		assert.Error(t, err, "Should reject reserved subdomain 'api'")
	})

	t.Run("reject duplicate subdomain", func(t *testing.T) {
		_, err := tunnelManager.AllocateSubdomain(ctx, userID, "custom")
		assert.Error(t, err, "Should reject duplicate subdomain")
	})

	t.Run("generate random subdomain", func(t *testing.T) {
		subdomain, err := tunnelManager.AllocateSubdomain(ctx, userID, "")
		require.NoError(t, err)
		assert.Len(t, subdomain, 8, "Random subdomain should be 8 characters")
		assert.Regexp(t, "^[a-z0-9]+$", subdomain, "Should contain only alphanumeric")
	})
}

// TestDomainCleanupOnDisconnect specifically tests the cleanup bug fix
func TestDomainCleanupOnDisconnect(t *testing.T) {
	_, database, tunnelManager, _, _ := setupTestServer(t)

	ctx := context.Background()
	userID := uuid.New()
	tokenID := uuid.New()

	// Create and register a tunnel
	subdomain, err := tunnelManager.AllocateSubdomain(ctx, userID, "cleanup-test")
	require.NoError(t, err)

	tun := tunnel.NewTunnel(
		userID,
		tokenID,
		subdomain,
		tunnelv1.TunnelProtocol_HTTP,
		"localhost:3000",
		"https://cleanup-test.localhost",
		nil,
	)

	err = tunnelManager.RegisterTunnel(ctx, tun)
	require.NoError(t, err)

	// Verify domain exists
	var domainCount int64
	database.Model(&models.Domain{}).Where("subdomain = ?", subdomain).Count(&domainCount)
	assert.Equal(t, int64(1), domainCount, "Domain should exist")

	// Unregister tunnel
	err = tunnelManager.UnregisterTunnel(ctx, tun.ID)
	require.NoError(t, err)

	// Verify domain is deleted (THIS WAS THE BUG!)
	database.Model(&models.Domain{}).Where("subdomain = ?", subdomain).Count(&domainCount)
	assert.Equal(t, int64(0), domainCount, "Domain should be deleted after unregister")

	// Verify we can reuse the subdomain immediately
	subdomain2, err := tunnelManager.AllocateSubdomain(ctx, userID, "cleanup-test")
	require.NoError(t, err, "Should be able to reuse subdomain after cleanup")
	assert.Equal(t, "cleanup-test", subdomain2)
}

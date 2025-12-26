package integration

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	grpcserver "github.com/pandeptwidyaop/grok/internal/server/grpc"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/pandeptwidyaop/grok/pkg/utils"
)

const bufSize = 1024 * 1024

// setupTestServer creates a test gRPC server with in-memory database.
func setupTestServer(t *testing.T) (*grpc.Server, *gorm.DB, *tunnel.Manager, *auth.TokenService, *bufconn.Listener) {
	// Create in-memory SQLite database for testing
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(database)
	require.NoError(t, err)

	// Create test user (without organization for basic tests)
	testUser := &models.User{
		Email:          "test@example.com",
		Password:       "hashedpassword",
		Name:           "Test User",
		IsActive:       true,
		Role:           models.RoleOrgUser,
		OrganizationID: nil, // No organization for basic tests
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
	tunnelManager := tunnel.NewManager(database, "localhost", 10, true, 80, 443, 10000, 20000) // TLS enabled, standard ports

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

// TestCompleteTunnelFlow tests the complete tunnel creation and data persistence.
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

	// Verify domain reservation is KEPT (persistent tunnels keep domains)
	var domainCount int64
	database.Model(&models.Domain{}).Where("subdomain = ?", testSubdomain).Count(&domainCount)
	assert.Equal(t, int64(1), domainCount, "Domain reservation should be kept for persistent tunnels")

	// Verify tunnel status is updated to offline in database
	err = database.Where("subdomain = ?", testSubdomain).First(&dbTunnel).Error
	require.NoError(t, err)
	assert.Equal(t, "offline", dbTunnel.Status, "Status should be updated to offline")
}

// TestSubdomainAllocation tests subdomain allocation and validation.
func TestSubdomainAllocation(t *testing.T) {
	grpcServer, database, tunnelManager, _, _ := setupTestServer(t)
	defer grpcServer.Stop()

	ctx := context.Background()
	userID := uuid.New()

	// Create test organization for duplicate testing
	testOrg := &models.Organization{
		Name:      "TestOrg",
		Subdomain: "testorg",
		IsActive:  true,
	}
	require.NoError(t, database.Create(testOrg).Error)

	t.Run("allocate custom subdomain", func(t *testing.T) {
		subdomain, customPart, err := tunnelManager.AllocateSubdomain(ctx, userID, nil, "custom")
		require.NoError(t, err)
		assert.Equal(t, "custom", subdomain)
		assert.Equal(t, "custom", customPart)

		// Verify in database
		var domain models.Domain
		err = database.Where("subdomain = ?", "custom").First(&domain).Error
		require.NoError(t, err)
	})

	t.Run("reject reserved subdomain", func(t *testing.T) {
		_, _, err := tunnelManager.AllocateSubdomain(ctx, userID, nil, "api")
		assert.Error(t, err, "Should reject reserved subdomain 'api'")
	})

	t.Run("reject duplicate subdomain with organization", func(t *testing.T) {
		// Allocate first time with organization
		_, _, err := tunnelManager.AllocateSubdomain(ctx, userID, &testOrg.ID, "duplicate-test")
		require.NoError(t, err, "First allocation should succeed")

		// Try to allocate again with same org - should fail
		_, _, err = tunnelManager.AllocateSubdomain(ctx, userID, &testOrg.ID, "duplicate-test")
		assert.Error(t, err, "Should reject duplicate subdomain within same org")
	})

	t.Run("generate random subdomain", func(t *testing.T) {
		subdomain, customPart, err := tunnelManager.AllocateSubdomain(ctx, userID, nil, "")
		require.NoError(t, err)
		assert.Len(t, subdomain, 8, "Random subdomain should be 8 characters")
		assert.Equal(t, subdomain, customPart, "Without org, subdomain should equal custom part")
		assert.Regexp(t, "^[a-z0-9]+$", subdomain, "Should contain only alphanumeric")
	})
}

// TestDomainPersistenceOnDisconnect tests that domains are kept reserved for persistent tunnels.
func TestDomainPersistenceOnDisconnect(t *testing.T) {
	_, database, tunnelManager, _, _ := setupTestServer(t)

	ctx := context.Background()
	userID := uuid.New()
	tokenID := uuid.New()

	// Create and register a tunnel
	subdomain, _, err := tunnelManager.AllocateSubdomain(ctx, userID, nil, "persistent-test")
	require.NoError(t, err)

	tun := tunnel.NewTunnel(
		userID,
		tokenID,
		nil, // No organization
		subdomain,
		tunnelv1.TunnelProtocol_HTTP,
		"localhost:3000",
		"https://persistent-test.localhost",
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

	// Verify domain is still reserved (persistent tunnels keep their domains)
	database.Model(&models.Domain{}).Where("subdomain = ?", subdomain).Count(&domainCount)
	assert.Equal(t, int64(1), domainCount, "Domain should still be reserved after unregister")

	// Verify we CANNOT reuse the subdomain (it's still reserved)
	_, _, err = tunnelManager.AllocateSubdomain(ctx, userID, nil, "persistent-test")
	assert.Error(t, err, "Should not be able to reuse reserved subdomain")
	assert.Equal(t, pkgerrors.ErrSubdomainTaken, err, "Should return ErrSubdomainTaken")
}

// TestOrganizationSubdomainAllocation tests subdomain allocation with organizations.
func TestOrganizationSubdomainAllocation(t *testing.T) {
	_, database, tunnelManager, _, _ := setupTestServer(t)

	ctx := context.Background()

	// Create two organizations
	org1 := &models.Organization{
		Name:      "Trofeo",
		Subdomain: "trofeo",
		IsActive:  true,
	}
	require.NoError(t, database.Create(org1).Error)

	org2 := &models.Organization{
		Name:      "ACME Corp",
		Subdomain: "acme",
		IsActive:  true,
	}
	require.NoError(t, database.Create(org2).Error)

	// Create users in different organizations
	user1 := &models.User{
		Email:          "user1@trofeo.com",
		Password:       "hashedpass",
		Name:           "User 1",
		IsActive:       true,
		Role:           models.RoleOrgUser,
		OrganizationID: &org1.ID,
	}
	require.NoError(t, database.Create(user1).Error)

	user2 := &models.User{
		Email:          "user2@acme.com",
		Password:       "hashedpass",
		Name:           "User 2",
		IsActive:       true,
		Role:           models.RoleOrgUser,
		OrganizationID: &org2.ID,
	}
	require.NoError(t, database.Create(user2).Error)

	t.Run("allocate subdomain with organization prefix", func(t *testing.T) {
		// User in "trofeo" org creates tunnel "app1"
		subdomain, customPart, err := tunnelManager.AllocateSubdomain(ctx, user1.ID, user1.OrganizationID, "app1")
		require.NoError(t, err)
		assert.Equal(t, "app1-trofeo", subdomain, "Subdomain should have org prefix")
		assert.Equal(t, "app1", customPart, "Custom part should be stored separately")

		// Verify in database
		var domain models.Domain
		err = database.Where("subdomain = ?", "app1-trofeo").First(&domain).Error
		require.NoError(t, err)
		assert.Equal(t, org1.ID, *domain.OrganizationID)
	})

	t.Run("different orgs can use same custom part", func(t *testing.T) {
		// User in "acme" org also creates tunnel "app1"
		subdomain, customPart, err := tunnelManager.AllocateSubdomain(ctx, user2.ID, user2.OrganizationID, "app1")
		require.NoError(t, err)
		assert.Equal(t, "app1-acme", subdomain, "Subdomain should have different org prefix")
		assert.Equal(t, "app1", customPart)

		// Verify both domains exist
		var domainCount int64
		database.Model(&models.Domain{}).Where("subdomain IN ?", []string{"app1-trofeo", "app1-acme"}).Count(&domainCount)
		assert.Equal(t, int64(2), domainCount, "Both domains should exist")
	})

	t.Run("duplicate within same org should fail", func(t *testing.T) {
		// Try to create "app1" again in trofeo org
		_, _, err := tunnelManager.AllocateSubdomain(ctx, user1.ID, user1.OrganizationID, "app1")
		assert.Error(t, err, "Should reject duplicate subdomain within same org")
	})

	t.Run("random subdomain with org", func(t *testing.T) {
		subdomain, customPart, err := tunnelManager.AllocateSubdomain(ctx, user1.ID, user1.OrganizationID, "")
		require.NoError(t, err)
		assert.Contains(t, subdomain, "-trofeo", "Random subdomain should have org suffix")
		assert.Len(t, customPart, 8, "Random custom part should be 8 characters")
		assert.Equal(t, customPart+"-trofeo", subdomain)
	})
}

// TestOrganizationTunnelIsolation tests that tunnels are properly isolated by organization.
func TestOrganizationTunnelIsolation(t *testing.T) {
	_, database, tunnelManager, _, _ := setupTestServer(t)

	ctx := context.Background()

	// Create organization
	org := &models.Organization{
		Name:      "TestOrg",
		Subdomain: "testorg",
		IsActive:  true,
	}
	require.NoError(t, database.Create(org).Error)

	// Create user in org
	user := &models.User{
		Email:          "user@testorg.com",
		Password:       "hashedpass",
		Name:           "Test User",
		IsActive:       true,
		Role:           models.RoleOrgUser,
		OrganizationID: &org.ID,
	}
	require.NoError(t, database.Create(user).Error)

	// Allocate subdomain and create tunnel
	subdomain, _, err := tunnelManager.AllocateSubdomain(ctx, user.ID, user.OrganizationID, "myapi")
	require.NoError(t, err)
	assert.Equal(t, "myapi-testorg", subdomain)

	tun := tunnel.NewTunnel(
		user.ID,
		uuid.New(),
		user.OrganizationID,
		subdomain,
		tunnelv1.TunnelProtocol_HTTPS,
		"localhost:3000",
		"https://myapi-testorg.localhost",
		nil,
	)

	err = tunnelManager.RegisterTunnel(ctx, tun)
	require.NoError(t, err)

	// Verify tunnel has organization_id
	var dbTunnel models.Tunnel
	err = database.Where("subdomain = ?", subdomain).First(&dbTunnel).Error
	require.NoError(t, err)
	assert.NotNil(t, dbTunnel.OrganizationID)
	assert.Equal(t, org.ID, *dbTunnel.OrganizationID)

	// Cleanup
	err = tunnelManager.UnregisterTunnel(ctx, tun.ID)
	require.NoError(t, err)
}

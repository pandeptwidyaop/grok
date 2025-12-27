package grpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	tunnelv1 "github.com/pandeptwidyaop/grok/gen/proto/tunnel/v1"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	"github.com/pandeptwidyaop/grok/internal/server/tunnel"
	pkgerrors "github.com/pandeptwidyaop/grok/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate all models
	err = db.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.AuthToken{},
		&models.Domain{},
		&models.Tunnel{},
	)
	require.NoError(t, err)

	return db
}

// setupTestTunnelService creates a test tunnel service with all dependencies
func setupTestTunnelService(t *testing.T) (*TunnelService, *gorm.DB, *auth.TokenService, *tunnel.Manager) {
	db := setupTestDB(t)

	// Create token service
	tokenService := auth.NewTokenService(db)

	// Create tunnel manager
	tm := tunnel.NewManager(db, "grok.io", 5, false, 80, 443, 10000, 20000)

	// Create tunnel service
	service := NewTunnelService(tm, tokenService)

	return service, db, tokenService, tm
}

// createTestUser creates a test user in the database
func createTestUser(t *testing.T, db *gorm.DB, username string, orgID *uuid.UUID) *models.User {
	user := &models.User{
		Name:           username,
		Email:          username + "@test.com",
		Password:       "hashedpassword",
		Role:           models.RoleOrgUser,
		OrganizationID: orgID,
	}
	err := db.Create(user).Error
	require.NoError(t, err)
	return user
}

// createTestToken creates a test auth token for a user
func createTestToken(t *testing.T, db *gorm.DB, tokenService *auth.TokenService, userID uuid.UUID, name string) (string, *models.AuthToken) {
	token, tokenString, err := tokenService.CreateToken(context.Background(), userID, name, nil)
	require.NoError(t, err)
	return tokenString, token
}

// TestNewTunnelService tests tunnel service initialization
func TestNewTunnelService(t *testing.T) {
	service, _, tokenService, tm := setupTestTunnelService(t)

	assert.NotNil(t, service)
	assert.Equal(t, tm, service.tunnelManager)
	assert.Equal(t, tokenService, service.tokenService)
}

// TestParseRegistrationData tests registration data parsing
func TestParseRegistrationData(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		expectError bool
		expected    *registrationData
	}{
		{
			name: "valid 4 parts",
			data: "myapp|grok_abc123|localhost:3000|http://myapp.grok.io",
			expected: &registrationData{
				subdomain: "myapp",
				authToken: "grok_abc123",
				localAddr: "localhost:3000",
				publicURL: "http://myapp.grok.io",
				savedName: "",
			},
		},
		{
			name: "valid 5 parts with saved name",
			data: "myapp|grok_abc123|localhost:3000|http://myapp.grok.io|my-tunnel",
			expected: &registrationData{
				subdomain: "myapp",
				authToken: "grok_abc123",
				localAddr: "localhost:3000",
				publicURL: "http://myapp.grok.io",
				savedName: "my-tunnel",
			},
		},
		{
			name: "valid 5 parts with empty saved name",
			data: "myapp|grok_abc123|localhost:3000|http://myapp.grok.io|",
			expected: &registrationData{
				subdomain: "myapp",
				authToken: "grok_abc123",
				localAddr: "localhost:3000",
				publicURL: "http://myapp.grok.io",
				savedName: "",
			},
		},
		{
			name:        "invalid - too few parts",
			data:        "myapp|grok_abc123|localhost:3000",
			expectError: true,
		},
		{
			name:        "invalid - too many parts",
			data:        "myapp|grok_abc123|localhost:3000|http://myapp.grok.io|name|extra",
			expectError: true,
		},
		{
			name:        "invalid - empty string",
			data:        "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := parseRegistrationData(tt.data)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, reg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.subdomain, reg.subdomain)
				assert.Equal(t, tt.expected.authToken, reg.authToken)
				assert.Equal(t, tt.expected.localAddr, reg.localAddr)
				assert.Equal(t, tt.expected.publicURL, reg.publicURL)
				assert.Equal(t, tt.expected.savedName, reg.savedName)
			}
		})
	}
}

// TestDetermineProtocolFromURL tests URL protocol detection
func TestDetermineProtocolFromURL(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		expected  tunnelv1.TunnelProtocol
	}{
		{
			name:      "https URL",
			publicURL: "https://myapp.grok.io",
			expected:  tunnelv1.TunnelProtocol_HTTPS,
		},
		{
			name:      "http URL",
			publicURL: "http://myapp.grok.io",
			expected:  tunnelv1.TunnelProtocol_HTTP,
		},
		{
			name:      "tcp URL",
			publicURL: "tcp://myapp.grok.io:10000",
			expected:  tunnelv1.TunnelProtocol_TCP,
		},
		{
			name:      "empty URL defaults to HTTP",
			publicURL: "",
			expected:  tunnelv1.TunnelProtocol_HTTP,
		},
		{
			name:      "invalid URL defaults to HTTP",
			publicURL: "ftp://myapp.grok.io",
			expected:  tunnelv1.TunnelProtocol_HTTP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineProtocolFromURL(tt.publicURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDetermineProtocol tests protocol determination from request
func TestDetermineProtocol(t *testing.T) {
	service, _, _, _ := setupTestTunnelService(t)

	tests := []struct {
		name        string
		reqProtocol tunnelv1.TunnelProtocol
		expected    string
	}{
		{
			name:        "HTTP protocol",
			reqProtocol: tunnelv1.TunnelProtocol_HTTP,
			expected:    "http",
		},
		{
			name:        "HTTPS protocol",
			reqProtocol: tunnelv1.TunnelProtocol_HTTPS,
			expected:    "https",
		},
		{
			name:        "TCP protocol",
			reqProtocol: tunnelv1.TunnelProtocol_TCP,
			expected:    "tcp",
		},
		{
			name:        "unspecified defaults to HTTP (TLS disabled)",
			reqProtocol: tunnelv1.TunnelProtocol_TUNNEL_PROTOCOL_UNSPECIFIED,
			expected:    "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.determineProtocol(tt.reqProtocol)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCreateTunnel tests tunnel creation
func TestCreateTunnel(t *testing.T) {
	service, db, tokenService, _ := setupTestTunnelService(t)

	// Create test user
	user := createTestUser(t, db, "testuser", nil)
	tokenString, _ := createTestToken(t, db, tokenService, user.ID, "test-token")

	tests := []struct {
		name          string
		req           *tunnelv1.CreateTunnelRequest
		expectedError codes.Code
		checkResponse func(t *testing.T, resp *tunnelv1.CreateTunnelResponse)
	}{
		{
			name: "valid HTTP tunnel",
			req: &tunnelv1.CreateTunnelRequest{
				AuthToken:    tokenString,
				Protocol:     tunnelv1.TunnelProtocol_HTTP,
				LocalAddress: "localhost:3000",
			},
			checkResponse: func(t *testing.T, resp *tunnelv1.CreateTunnelResponse) {
				assert.NotEmpty(t, resp.PublicUrl)
				assert.NotEmpty(t, resp.Subdomain)
				assert.Equal(t, tunnelv1.TunnelStatus_ACTIVE, resp.Status)
				assert.Contains(t, resp.PublicUrl, "http://")
				assert.Contains(t, resp.PublicUrl, ".grok.io")
			},
		},
		{
			name: "valid HTTPS tunnel (falls back to HTTP when TLS disabled)",
			req: &tunnelv1.CreateTunnelRequest{
				AuthToken:    tokenString,
				Protocol:     tunnelv1.TunnelProtocol_HTTPS,
				LocalAddress: "localhost:8080",
			},
			checkResponse: func(t *testing.T, resp *tunnelv1.CreateTunnelResponse) {
				assert.NotEmpty(t, resp.PublicUrl)
				// Since TLS is disabled in test, HTTPS falls back to HTTP
				assert.Contains(t, resp.PublicUrl, "http://")
			},
		},
		{
			name: "valid TCP tunnel",
			req: &tunnelv1.CreateTunnelRequest{
				AuthToken:    tokenString,
				Protocol:     tunnelv1.TunnelProtocol_TCP,
				LocalAddress: "localhost:22",
			},
			checkResponse: func(t *testing.T, resp *tunnelv1.CreateTunnelResponse) {
				assert.NotEmpty(t, resp.PublicUrl)
				assert.Contains(t, resp.PublicUrl, "tcp://")
			},
		},
		{
			name: "custom subdomain",
			req: &tunnelv1.CreateTunnelRequest{
				AuthToken:    tokenString,
				Protocol:     tunnelv1.TunnelProtocol_HTTP,
				LocalAddress: "localhost:3000",
				Subdomain:    "myapp",
			},
			checkResponse: func(t *testing.T, resp *tunnelv1.CreateTunnelResponse) {
				assert.Equal(t, "myapp", resp.Subdomain)
				assert.Contains(t, resp.PublicUrl, "http://myapp.grok.io")
			},
		},
		{
			name: "invalid token",
			req: &tunnelv1.CreateTunnelRequest{
				AuthToken:    "invalid_token",
				Protocol:     tunnelv1.TunnelProtocol_HTTP,
				LocalAddress: "localhost:3000",
			},
			expectedError: codes.Unauthenticated,
		},
		{
			name: "missing protocol",
			req: &tunnelv1.CreateTunnelRequest{
				AuthToken:    tokenString,
				Protocol:     tunnelv1.TunnelProtocol_TUNNEL_PROTOCOL_UNSPECIFIED,
				LocalAddress: "localhost:3000",
			},
			expectedError: codes.InvalidArgument,
		},
		{
			name: "missing local address",
			req: &tunnelv1.CreateTunnelRequest{
				AuthToken:    tokenString,
				Protocol:     tunnelv1.TunnelProtocol_HTTP,
				LocalAddress: "",
			},
			expectedError: codes.InvalidArgument,
		},
		{
			name: "invalid subdomain format",
			req: &tunnelv1.CreateTunnelRequest{
				AuthToken:    tokenString,
				Protocol:     tunnelv1.TunnelProtocol_HTTP,
				LocalAddress: "localhost:3000",
				Subdomain:    "invalid_subdomain!",
			},
			expectedError: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.CreateTunnel(context.Background(), tt.req)

			if tt.expectedError != codes.OK {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedError, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				}
			}
		})
	}
}

// TestCreateTunnel_SubdomainTaken tests subdomain conflict handling
func TestCreateTunnel_SubdomainTaken(t *testing.T) {
	service, db, tokenService, _ := setupTestTunnelService(t)

	// Create two users
	user1 := createTestUser(t, db, "user1", nil)
	user2 := createTestUser(t, db, "user2", nil)

	token1, _ := createTestToken(t, db, tokenService, user1.ID, "token1")
	token2, _ := createTestToken(t, db, tokenService, user2.ID, "token2")

	// User1 creates first tunnel with subdomain
	resp1, err := service.CreateTunnel(context.Background(), &tunnelv1.CreateTunnelRequest{
		AuthToken:    token1,
		Protocol:     tunnelv1.TunnelProtocol_HTTP,
		LocalAddress: "localhost:3000",
		Subdomain:    "myapp",
	})
	require.NoError(t, err)
	assert.Equal(t, "myapp", resp1.Subdomain)

	// User2 should NOT be able to use the same subdomain
	resp2, err := service.CreateTunnel(context.Background(), &tunnelv1.CreateTunnelRequest{
		AuthToken:    token2,
		Protocol:     tunnelv1.TunnelProtocol_HTTP,
		LocalAddress: "localhost:8080",
		Subdomain:    "myapp",
	})
	require.Error(t, err)
	assert.Nil(t, resp2)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, st.Code())
}

// TestAllocateSubdomainForTunnel tests subdomain allocation logic
func TestAllocateSubdomainForTunnel(t *testing.T) {
	service, db, _, _ := setupTestTunnelService(t)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser", nil)

	tests := []struct {
		name               string
		setup              func()
		requestedSubdomain string
		expectError        bool
		checkResult        func(t *testing.T, fullSubdomain, customPart string)
	}{
		{
			name:               "random subdomain generation",
			requestedSubdomain: "",
			setup:              func() {},
			checkResult: func(t *testing.T, fullSubdomain, customPart string) {
				assert.NotEmpty(t, fullSubdomain)
				// When no subdomain requested, customPart equals fullSubdomain (both random)
				assert.Equal(t, fullSubdomain, customPart)
				assert.Len(t, fullSubdomain, 8) // Random subdomains are 8 chars
			},
		},
		{
			name:               "custom subdomain - new",
			requestedSubdomain: "customapp",
			setup:              func() {},
			checkResult: func(t *testing.T, fullSubdomain, customPart string) {
				assert.Equal(t, "customapp", fullSubdomain)
				assert.Equal(t, "customapp", customPart)
			},
		},
		{
			name:               "reserved subdomain",
			requestedSubdomain: "api",
			setup:              func() {},
			expectError:        true,
		},
		{
			name:               "invalid subdomain characters",
			requestedSubdomain: "invalid_name!",
			setup:              func() {},
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			fullSubdomain, customPart, err := service.allocateSubdomainForTunnel(ctx, user.ID, nil, tt.requestedSubdomain)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.checkResult != nil {
					tt.checkResult(t, fullSubdomain, customPart)
				}
			}

			// Note: Cleanup would require UnregisterTunnel with tunnel ID
			// Skipping cleanup as this is just for testing subdomain allocation
		})
	}
}

// TestAllocateSubdomainForTunnel_OfflineTunnelReuse tests offline tunnel subdomain reuse
func TestAllocateSubdomainForTunnel_OfflineTunnelReuse(t *testing.T) {
	service, db, _, _ := setupTestTunnelService(t)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser", nil)

	// Create an offline tunnel with saved name "myapp"
	offlineTunnel := &models.Tunnel{
		UserID:       user.ID,
		TokenID:      uuid.New(),
		Subdomain:    "abc123def",
		SavedName:    strPtr("myapp"),
		TunnelType:   "http",
		LocalAddr:    "localhost:3000",
		PublicURL:    "http://abc123def.grok.io",
		Status:       "offline",
		IsPersistent: true,
		ClientID:     uuid.New().String(),
	}
	err := db.Create(offlineTunnel).Error
	require.NoError(t, err)

	// Request subdomain with saved name "myapp" should reuse offline tunnel's subdomain
	fullSubdomain, customPart, err := service.allocateSubdomainForTunnel(ctx, user.ID, nil, "myapp")
	require.NoError(t, err)
	assert.Equal(t, "abc123def", fullSubdomain)
	assert.Equal(t, "myapp", customPart)
}

// TestCreateTunnel_InvalidToken tests handling of invalid tokens
func TestCreateTunnel_InvalidToken(t *testing.T) {
	service, _, _, _ := setupTestTunnelService(t)

	// Use a token that doesn't exist in database
	resp, err := service.CreateTunnel(context.Background(), &tunnelv1.CreateTunnelRequest{
		AuthToken:    "grok_nonexistenttoken123456789012",
		Protocol:     tunnelv1.TunnelProtocol_HTTP,
		LocalAddress: "localhost:3000",
	})

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "invalid authentication token")
}

// TestCreateTunnel_WithOrganization tests tunnel creation for org users
func TestCreateTunnel_WithOrganization(t *testing.T) {
	service, db, tokenService, _ := setupTestTunnelService(t)

	// Create organization
	org := &models.Organization{
		Name:        "Test Org",
		Subdomain:   "testorg",
		Description: "Test",
	}
	err := db.Create(org).Error
	require.NoError(t, err)

	// Create user in organization
	user := createTestUser(t, db, "orguser", &org.ID)
	tokenString, _ := createTestToken(t, db, tokenService, user.ID, "org-token")

	resp, err := service.CreateTunnel(context.Background(), &tunnelv1.CreateTunnelRequest{
		AuthToken:    tokenString,
		Protocol:     tunnelv1.TunnelProtocol_HTTP,
		LocalAddress: "localhost:3000",
		Subdomain:    "orgapp",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	// Organization subdomains are namespaced: {custom}-{org}
	assert.Equal(t, "orgapp-testorg", resp.Subdomain)
	assert.Contains(t, resp.PublicUrl, "orgapp-testorg.grok.io")
}

// BenchmarkCreateTunnel benchmarks tunnel creation
func BenchmarkCreateTunnel(b *testing.B) {
	service, db, tokenService, _ := setupTestTunnelService(&testing.T{})
	user := createTestUser(&testing.T{}, db, "benchuser", nil)
	tokenString, _ := createTestToken(&testing.T{}, db, tokenService, user.ID, "bench-token")

	req := &tunnelv1.CreateTunnelRequest{
		AuthToken:    tokenString,
		Protocol:     tunnelv1.TunnelProtocol_HTTP,
		LocalAddress: "localhost:3000",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.CreateTunnel(context.Background(), req)
	}
}

// BenchmarkParseRegistrationData benchmarks registration parsing
func BenchmarkParseRegistrationData(b *testing.B) {
	data := "myapp|grok_abc123|localhost:3000|http://myapp.grok.io|my-tunnel"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseRegistrationData(data)
	}
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}

// TestDetermineProtocol_WithTLS tests protocol determination with TLS enabled
func TestDetermineProtocol_WithTLS(t *testing.T) {
	db := setupTestDB(t)
	tokenService := auth.NewTokenService(db)

	// Create manager with TLS enabled
	tm := tunnel.NewManager(db, "grok.io", 5, true, 80, 443, 10000, 20000)
	service := NewTunnelService(tm, tokenService)

	// Unspecified protocol should default to HTTPS when TLS is enabled
	result := service.determineProtocol(tunnelv1.TunnelProtocol_TUNNEL_PROTOCOL_UNSPECIFIED)
	assert.Equal(t, "https", result)
}

// TestAllocateSubdomainForTunnel_ReservedSubdomains tests reserved subdomain rejection
func TestAllocateSubdomainForTunnel_ReservedSubdomains(t *testing.T) {
	service, db, _, _ := setupTestTunnelService(t)
	ctx := context.Background()

	user := createTestUser(t, db, "edgeuser", nil)

	// Only test subdomains that are actually reserved
	reservedSubdomains := []string{"api", "admin", "www"}

	for _, subdomain := range reservedSubdomains {
		t.Run("reserved_"+subdomain, func(t *testing.T) {
			_, _, err := service.allocateSubdomainForTunnel(ctx, user.ID, nil, subdomain)
			assert.Error(t, err)
			assert.ErrorIs(t, err, pkgerrors.ErrInvalidSubdomain)
		})
	}
}

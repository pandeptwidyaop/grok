package interceptors

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/pandeptwidyaop/grok/internal/db/models"
	"github.com/pandeptwidyaop/grok/internal/server/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate models
	err = db.AutoMigrate(&models.User{}, &models.AuthToken{})
	require.NoError(t, err)

	return db
}

// createTestUser creates a test user
func createTestUser(t *testing.T, db *gorm.DB) *models.User {
	user := &models.User{
		Email:    "test@example.com",
		Password: "hashedpassword",
		Name:     "Test User",
		Role:     models.RoleOrgUser,
	}
	err := db.Create(user).Error
	require.NoError(t, err)
	return user
}

// createTestToken creates a test auth token
func createTestToken(t *testing.T, db *gorm.DB, tokenService *auth.TokenService, userID uuid.UUID) (string, *models.AuthToken) {
	token, authToken, err := tokenService.CreateToken(context.Background(), userID, "test-token", nil)
	require.NoError(t, err)
	return authToken, token
}

// mockHandler is a mock gRPC handler
func mockHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return "success", nil
}

// mockStreamHandler is a mock gRPC stream handler
func mockStreamHandler(srv interface{}, stream grpc.ServerStream) error {
	return nil
}

// mockServerStream implements grpc.ServerStream for testing
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

// TestAuthInterceptor_ValidToken tests auth interceptor with valid token
func TestAuthInterceptor_ValidToken(t *testing.T) {
	db := setupTestDB(t)
	tokenService := auth.NewTokenService(db)
	user := createTestUser(t, db)
	tokenString, authToken := createTestToken(t, db, tokenService, user.ID)

	interceptor := AuthInterceptor(tokenService)

	// Create context with auth metadata
	md := metadata.New(map[string]string{
		authorizationKey: tokenString,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	// Call interceptor
	resp, err := interceptor(ctx, nil, info, mockHandler)

	require.NoError(t, err)
	assert.Equal(t, "success", resp)

	// Verify user ID was added to context (interceptor calls handler with modified context)
	// We can't directly access the context passed to handler in this test,
	// but we verified no error occurred
	_ = authToken
}

// TestAuthInterceptor_MissingToken tests auth interceptor without token
func TestAuthInterceptor_MissingToken(t *testing.T) {
	db := setupTestDB(t)
	tokenService := auth.NewTokenService(db)

	interceptor := AuthInterceptor(tokenService)

	// Create context without auth metadata
	ctx := context.Background()

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	// Call interceptor
	resp, err := interceptor(ctx, nil, info, mockHandler)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "missing or invalid authentication token")
}

// TestAuthInterceptor_InvalidToken tests auth interceptor with invalid token
func TestAuthInterceptor_InvalidToken(t *testing.T) {
	db := setupTestDB(t)
	tokenService := auth.NewTokenService(db)

	interceptor := AuthInterceptor(tokenService)

	// Create context with invalid token
	md := metadata.New(map[string]string{
		authorizationKey: "grok_invalidtoken123456789012345",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	// Call interceptor
	resp, err := interceptor(ctx, nil, info, mockHandler)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "invalid authentication token")
}

// TestAuthInterceptor_SkipAuth tests that health check skips auth
func TestAuthInterceptor_SkipAuth(t *testing.T) {
	db := setupTestDB(t)
	tokenService := auth.NewTokenService(db)

	interceptor := AuthInterceptor(tokenService)

	// Create context without auth metadata
	ctx := context.Background()

	info := &grpc.UnaryServerInfo{
		FullMethod: "/grpc.health.v1.Health/Check",
	}

	// Call interceptor
	resp, err := interceptor(ctx, nil, info, mockHandler)

	// Should succeed without auth token
	require.NoError(t, err)
	assert.Equal(t, "success", resp)
}

// TestExtractToken tests token extraction from metadata
func TestExtractToken(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		expectError bool
		expectToken string
	}{
		{
			name: "valid token in metadata",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				authorizationKey: "grok_token123",
			})),
			expectError: false,
			expectToken: "grok_token123",
		},
		{
			name:        "missing metadata",
			ctx:         context.Background(),
			expectError: true,
		},
		{
			name: "missing authorization header",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"other-header": "value",
			})),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := extractToken(tt.ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectToken, token)
			}
		})
	}
}

// TestSkipAuth tests skip auth logic
func TestSkipAuth(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		expected bool
	}{
		{
			name:     "health check - should skip",
			method:   "/grpc.health.v1.Health/Check",
			expected: true,
		},
		{
			name:     "regular method - should not skip",
			method:   "/test.Service/Method",
			expected: false,
		},
		{
			name:     "other method - should not skip",
			method:   "/grok.tunnel.v1.TunnelService/CreateTunnel",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := skipAuth(tt.method)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestStreamAuthInterceptor_ValidToken tests stream auth with valid token
func TestStreamAuthInterceptor_ValidToken(t *testing.T) {
	db := setupTestDB(t)
	tokenService := auth.NewTokenService(db)
	user := createTestUser(t, db)
	tokenString, _ := createTestToken(t, db, tokenService, user.ID)

	interceptor := StreamAuthInterceptor(tokenService)

	// Create context with auth metadata
	md := metadata.New(map[string]string{
		authorizationKey: tokenString,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	stream := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{
		FullMethod: "/test.Service/StreamMethod",
	}

	// Call interceptor
	err := interceptor(nil, stream, info, mockStreamHandler)

	require.NoError(t, err)
}

// TestStreamAuthInterceptor_MissingToken tests stream auth without token
func TestStreamAuthInterceptor_MissingToken(t *testing.T) {
	db := setupTestDB(t)
	tokenService := auth.NewTokenService(db)

	interceptor := StreamAuthInterceptor(tokenService)

	// Create context without auth metadata
	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{
		FullMethod: "/test.Service/StreamMethod",
	}

	// Call interceptor
	err := interceptor(nil, stream, info, mockStreamHandler)

	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

// TestStreamAuthInterceptor_SkipAuth tests stream skips auth for health check
func TestStreamAuthInterceptor_SkipAuth(t *testing.T) {
	db := setupTestDB(t)
	tokenService := auth.NewTokenService(db)

	interceptor := StreamAuthInterceptor(tokenService)

	// Create context without auth metadata
	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{
		FullMethod: "/grpc.health.v1.Health/Check",
	}

	// Call interceptor
	err := interceptor(nil, stream, info, mockStreamHandler)

	// Should succeed without auth token
	require.NoError(t, err)
}

// TestAuthServerStream_Context tests wrapped stream context
func TestAuthServerStream_Context(t *testing.T) {
	originalCtx := context.Background()
	wrappedCtx := context.WithValue(originalCtx, userIDKey, uuid.New())

	stream := &authServerStream{
		ctx: wrappedCtx,
	}

	assert.Equal(t, wrappedCtx, stream.Context())
}

// TestAuthInterceptor_ContextValues tests that user ID is added to context
func TestAuthInterceptor_ContextValues(t *testing.T) {
	db := setupTestDB(t)
	tokenService := auth.NewTokenService(db)
	user := createTestUser(t, db)
	tokenString, authToken := createTestToken(t, db, tokenService, user.ID)

	interceptor := AuthInterceptor(tokenService)

	// Create context with auth metadata
	md := metadata.New(map[string]string{
		authorizationKey: tokenString,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	// Custom handler that checks context values
	var capturedCtx context.Context
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		capturedCtx = ctx
		return "success", nil
	}

	// Call interceptor
	_, err := interceptor(ctx, nil, info, handler)
	require.NoError(t, err)

	// Verify context values
	require.NotNil(t, capturedCtx)
	userID := capturedCtx.Value(userIDKey)
	tokenID := capturedCtx.Value(tokenIDKey)

	assert.Equal(t, user.ID, userID)
	assert.Equal(t, authToken.ID, tokenID)
}

// BenchmarkAuthInterceptor benchmarks auth interceptor
func BenchmarkAuthInterceptor(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.User{}, &models.AuthToken{})

	tokenService := auth.NewTokenService(db)
	user := &models.User{
		Email:    "bench@example.com",
		Password: "hashedpassword",
		Name:     "Bench User",
		Role:     models.RoleOrgUser,
	}
	db.Create(user)

	authToken, tokenString, _ := tokenService.CreateToken(context.Background(), user.ID, "bench-token", nil)
	_ = authToken

	interceptor := AuthInterceptor(tokenService)

	md := metadata.New(map[string]string{
		authorizationKey: tokenString,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interceptor(ctx, nil, info, mockHandler)
	}
}

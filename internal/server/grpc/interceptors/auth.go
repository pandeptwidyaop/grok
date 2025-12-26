package interceptors

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/pandeptwidyaop/grok/internal/server/auth"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

const (
	authorizationKey = "authorization"
)

// AuthInterceptor validates authentication tokens.
func AuthInterceptor(tokenService *auth.TokenService) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip auth for certain methods (e.g., health checks)
		if skipAuth(info.FullMethod) {
			return handler(ctx, req)
		}

		// Extract token from metadata
		token, err := extractToken(ctx)
		if err != nil {
			logger.WarnEvent().
				Err(err).
				Str("method", info.FullMethod).
				Msg("Missing or invalid auth token")
			return nil, status.Error(codes.Unauthenticated, "missing or invalid authentication token")
		}

		// Validate token
		authToken, err := tokenService.ValidateToken(ctx, token)
		if err != nil {
			logger.WarnEvent().
				Err(err).
				Str("method", info.FullMethod).
				Msg("Token validation failed")
			return nil, status.Error(codes.Unauthenticated, "invalid authentication token")
		}

		// Add user info to context
		ctx = context.WithValue(ctx, "user_id", authToken.UserID)
		ctx = context.WithValue(ctx, "token_id", authToken.ID)

		return handler(ctx, req)
	}
}

// StreamAuthInterceptor validates authentication tokens for streams.
func StreamAuthInterceptor(tokenService *auth.TokenService) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := stream.Context()

		// Skip auth for certain methods
		if skipAuth(info.FullMethod) {
			return handler(srv, stream)
		}

		// Extract token from metadata
		token, err := extractToken(ctx)
		if err != nil {
			logger.WarnEvent().
				Err(err).
				Str("method", info.FullMethod).
				Msg("Missing or invalid auth token in stream")
			return status.Error(codes.Unauthenticated, "missing or invalid authentication token")
		}

		// Validate token
		authToken, err := tokenService.ValidateToken(ctx, token)
		if err != nil {
			logger.WarnEvent().
				Err(err).
				Str("method", info.FullMethod).
				Msg("Token validation failed in stream")
			return status.Error(codes.Unauthenticated, "invalid authentication token")
		}

		// Create wrapped stream with auth context
		wrappedStream := &authServerStream{
			ServerStream: stream,
			ctx:          context.WithValue(ctx, "user_id", authToken.UserID),
		}

		return handler(srv, wrappedStream)
	}
}

// authServerStream wraps grpc.ServerStream with authentication context.
type authServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context.
func (s *authServerStream) Context() context.Context {
	return s.ctx
}

// extractToken extracts the authentication token from gRPC metadata.
func extractToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	values := md.Get(authorizationKey)
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	return values[0], nil
}

// skipAuth returns true if the method should skip authentication.
func skipAuth(method string) bool {
	// Add methods that don't require authentication
	skipMethods := map[string]bool{
		"/grpc.health.v1.Health/Check": true,
		// Add other public methods here
	}

	return skipMethods[method]
}

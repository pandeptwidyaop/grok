package interceptors

import (
	"context"
	"time"

	"github.com/pandeptwidyaop/grok/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggingInterceptor logs gRPC requests and responses
func LoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Call handler
		resp, err := handler(ctx, req)

		// Log request
		duration := time.Since(start)
		code := status.Code(err)

		event := logger.InfoEvent().
			Str("method", info.FullMethod).
			Dur("duration", duration).
			Str("code", code.String())

		if err != nil {
			event.Err(err)
		}

		event.Msg("gRPC request completed")

		return resp, err
	}
}

// StreamLoggingInterceptor logs gRPC streaming requests
func StreamLoggingInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		logger.InfoEvent().
			Str("method", info.FullMethod).
			Bool("is_client_stream", info.IsClientStream).
			Bool("is_server_stream", info.IsServerStream).
			Msg("gRPC stream started")

		// Call handler
		err := handler(srv, stream)

		// Log completion
		duration := time.Since(start)
		code := status.Code(err)

		event := logger.InfoEvent().
			Str("method", info.FullMethod).
			Dur("duration", duration).
			Str("code", code.String())

		if err != nil {
			event.Err(err)
		}

		event.Msg("gRPC stream completed")

		return err
	}
}

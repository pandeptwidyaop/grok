package interceptors

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// slowHandler is a handler that introduces delay.
func slowHandler(delay time.Duration) grpc.UnaryHandler {
	return func(_ context.Context, _ interface{}) (interface{}, error) {
		time.Sleep(delay)
		return "success", nil
	}
}

// errorHandler returns an error with specific code.
func errorHandler(code codes.Code, msg string) grpc.UnaryHandler {
	return func(_ context.Context, _ interface{}) (interface{}, error) {
		return nil, status.Error(code, msg)
	}
}

// slowStreamHandler is a stream handler that introduces delay.
func slowStreamHandler(delay time.Duration) grpc.StreamHandler {
	return func(_ interface{}, _ grpc.ServerStream) error {
		time.Sleep(delay)
		return nil
	}
}

// errorStreamHandler returns an error with specific code.
func errorStreamHandler(code codes.Code, msg string) grpc.StreamHandler {
	return func(_ interface{}, _ grpc.ServerStream) error {
		return status.Error(code, msg)
	}
}

// TestLoggingInterceptor_SuccessfulRequest tests logging for successful request.
func TestLoggingInterceptor_SuccessfulRequest(t *testing.T) {
	interceptor := LoggingInterceptor()

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	// Call interceptor with successful handler
	resp, err := interceptor(ctx, nil, info, mockHandler)

	require.NoError(t, err)
	assert.Equal(t, "success", resp)
}

// TestLoggingInterceptor_FailedRequest tests logging for failed request.
func TestLoggingInterceptor_FailedRequest(t *testing.T) {
	interceptor := LoggingInterceptor()

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/FailingMethod",
	}

	// Call interceptor with error handler
	handler := errorHandler(codes.Internal, "internal error")
	resp, err := interceptor(ctx, nil, info, handler)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "internal error", st.Message())
}

// TestLoggingInterceptor_VariousErrorCodes tests different error codes.
func TestLoggingInterceptor_VariousErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		code codes.Code
		msg  string
	}{
		{
			name: "unauthenticated",
			code: codes.Unauthenticated,
			msg:  "not authenticated",
		},
		{
			name: "permission denied",
			code: codes.PermissionDenied,
			msg:  "permission denied",
		},
		{
			name: "not found",
			code: codes.NotFound,
			msg:  "not found",
		},
		{
			name: "invalid argument",
			code: codes.InvalidArgument,
			msg:  "invalid argument",
		},
		{
			name: "deadline exceeded",
			code: codes.DeadlineExceeded,
			msg:  "deadline exceeded",
		},
	}

	interceptor := LoggingInterceptor()
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := errorHandler(tt.code, tt.msg)
			_, err := interceptor(ctx, nil, info, handler)

			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.code, st.Code())
			assert.Equal(t, tt.msg, st.Message())
		})
	}
}

// TestLoggingInterceptor_DurationTracking tests duration is tracked.
func TestLoggingInterceptor_DurationTracking(t *testing.T) {
	interceptor := LoggingInterceptor()

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/SlowMethod",
	}

	// Use a handler that introduces delay
	delay := 50 * time.Millisecond
	handler := slowHandler(delay)

	start := time.Now()
	resp, err := interceptor(ctx, nil, info, handler)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "success", resp)

	// Verify that actual elapsed time is at least the delay
	assert.GreaterOrEqual(t, elapsed, delay)
}

// TestLoggingInterceptor_MethodName tests different method names are logged.
func TestLoggingInterceptor_MethodName(t *testing.T) {
	tests := []struct {
		name       string
		fullMethod string
	}{
		{
			name:       "create tunnel",
			fullMethod: "/grok.tunnel.v1.TunnelService/CreateTunnel",
		},
		{
			name:       "proxy stream",
			fullMethod: "/grok.tunnel.v1.TunnelService/ProxyStream",
		},
		{
			name:       "health check",
			fullMethod: "/grpc.health.v1.Health/Check",
		},
	}

	interceptor := LoggingInterceptor()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &grpc.UnaryServerInfo{
				FullMethod: tt.fullMethod,
			}

			resp, err := interceptor(ctx, nil, info, mockHandler)

			require.NoError(t, err)
			assert.Equal(t, "success", resp)
		})
	}
}

// TestLoggingInterceptor_PreservesError tests error is preserved.
func TestLoggingInterceptor_PreservesError(t *testing.T) {
	interceptor := LoggingInterceptor()

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	expectedErr := errors.New("custom error")
	handler := func(_ context.Context, _ interface{}) (interface{}, error) {
		return nil, expectedErr
	}

	_, err := interceptor(ctx, nil, info, handler)

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

// TestStreamLoggingInterceptor_SuccessfulStream tests stream logging for success.
func TestStreamLoggingInterceptor_SuccessfulStream(t *testing.T) {
	interceptor := StreamLoggingInterceptor()

	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/StreamMethod",
		IsClientStream: true,
		IsServerStream: true,
	}

	err := interceptor(nil, stream, info, mockStreamHandler)

	require.NoError(t, err)
}

// TestStreamLoggingInterceptor_FailedStream tests stream logging for failed stream.
func TestStreamLoggingInterceptor_FailedStream(t *testing.T) {
	interceptor := StreamLoggingInterceptor()

	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/StreamMethod",
		IsClientStream: true,
		IsServerStream: true,
	}

	handler := errorStreamHandler(codes.Internal, "stream error")
	err := interceptor(nil, stream, info, handler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "stream error", st.Message())
}

// TestStreamLoggingInterceptor_StreamTypes tests different stream types.
func TestStreamLoggingInterceptor_StreamTypes(t *testing.T) {
	tests := []struct {
		name           string
		isClientStream bool
		isServerStream bool
	}{
		{
			name:           "unary (neither)",
			isClientStream: false,
			isServerStream: false,
		},
		{
			name:           "client stream",
			isClientStream: true,
			isServerStream: false,
		},
		{
			name:           "server stream",
			isClientStream: false,
			isServerStream: true,
		},
		{
			name:           "bidirectional stream",
			isClientStream: true,
			isServerStream: true,
		},
	}

	interceptor := StreamLoggingInterceptor()
	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &grpc.StreamServerInfo{
				FullMethod:     "/test.Service/StreamMethod",
				IsClientStream: tt.isClientStream,
				IsServerStream: tt.isServerStream,
			}

			err := interceptor(nil, stream, info, mockStreamHandler)
			require.NoError(t, err)
		})
	}
}

// TestStreamLoggingInterceptor_DurationTracking tests stream duration tracking.
func TestStreamLoggingInterceptor_DurationTracking(t *testing.T) {
	interceptor := StreamLoggingInterceptor()

	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/SlowStream",
		IsClientStream: true,
		IsServerStream: true,
	}

	delay := 50 * time.Millisecond
	handler := slowStreamHandler(delay)

	start := time.Now()
	err := interceptor(nil, stream, info, handler)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, delay)
}

// TestStreamLoggingInterceptor_VariousErrorCodes tests different error codes.
func TestStreamLoggingInterceptor_VariousErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		code codes.Code
		msg  string
	}{
		{
			name: "canceled",
			code: codes.Canceled,
			msg:  "operation canceled",
		},
		{
			name: "resource exhausted",
			code: codes.ResourceExhausted,
			msg:  "resource exhausted",
		},
		{
			name: "unavailable",
			code: codes.Unavailable,
			msg:  "service unavailable",
		},
	}

	interceptor := StreamLoggingInterceptor()
	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/StreamMethod",
		IsClientStream: true,
		IsServerStream: true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := errorStreamHandler(tt.code, tt.msg)
			err := interceptor(nil, stream, info, handler)

			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.code, st.Code())
			assert.Equal(t, tt.msg, st.Message())
		})
	}
}

// TestStreamLoggingInterceptor_PreservesError tests error is preserved.
func TestStreamLoggingInterceptor_PreservesError(t *testing.T) {
	interceptor := StreamLoggingInterceptor()

	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/StreamMethod",
		IsClientStream: true,
		IsServerStream: true,
	}

	expectedErr := errors.New("custom stream error")
	handler := func(_ interface{}, _ grpc.ServerStream) error {
		return expectedErr
	}

	err := interceptor(nil, stream, info, handler)

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

// BenchmarkLoggingInterceptor benchmarks logging interceptor.
func BenchmarkLoggingInterceptor(b *testing.B) {
	interceptor := LoggingInterceptor()
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interceptor(ctx, nil, info, mockHandler)
	}
}

// BenchmarkStreamLoggingInterceptor benchmarks stream logging interceptor.
func BenchmarkStreamLoggingInterceptor(b *testing.B) {
	interceptor := StreamLoggingInterceptor()
	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/StreamMethod",
		IsClientStream: true,
		IsServerStream: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interceptor(nil, stream, info, mockStreamHandler)
	}
}

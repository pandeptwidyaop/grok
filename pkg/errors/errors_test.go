package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPredefinedErrors tests that all predefined errors are defined.
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrUnauthorized", ErrUnauthorized, "unauthorized"},
		{"ErrInvalidToken", ErrInvalidToken, "invalid token"},
		{"ErrTokenExpired", ErrTokenExpired, "token expired"},
		{"ErrUserNotFound", ErrUserNotFound, "user not found"},
		{"ErrTunnelNotFound", ErrTunnelNotFound, "tunnel not found"},
		{"ErrSubdomainTaken", ErrSubdomainTaken, "subdomain already taken"},
		{"ErrInvalidSubdomain", ErrInvalidSubdomain, "invalid subdomain format"},
		{"ErrSubdomainAllocationFailed", ErrSubdomainAllocationFailed, "subdomain allocation failed"},
		{"ErrMaxTunnelsReached", ErrMaxTunnelsReached, "maximum tunnels per user reached"},
		{"ErrLocalServiceUnreachable", ErrLocalServiceUnreachable, "local service unreachable"},
		{"ErrRequestTimeout", ErrRequestTimeout, "request timeout"},
		{"ErrRateLimited", ErrRateLimited, "rate limited"},
		{"ErrInvalidProtocol", ErrInvalidProtocol, "invalid protocol"},
		{"ErrNoAvailablePorts", ErrNoAvailablePorts, "no available ports in pool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err)
			assert.Equal(t, tt.msg, tt.err.Error())
		})
	}
}

// TestPredefinedErrorsAreUnique tests that predefined errors are unique instances.
func TestPredefinedErrorsAreUnique(t *testing.T) {
	// Each error should be a unique instance
	assert.NotEqual(t, ErrUnauthorized, ErrInvalidToken)
	assert.NotEqual(t, ErrTokenExpired, ErrUserNotFound)
	assert.NotEqual(t, ErrTunnelNotFound, ErrSubdomainTaken)
}

// TestPredefinedErrorsWithErrorsIs tests using errors.Is with predefined errors.
func TestPredefinedErrorsWithErrorsIs(t *testing.T) {
	// Wrap a predefined error
	wrappedErr := fmt.Errorf("context: %w", ErrUnauthorized)

	// errors.Is should find the wrapped error
	assert.True(t, errors.Is(wrappedErr, ErrUnauthorized))
	assert.False(t, errors.Is(wrappedErr, ErrInvalidToken))

	// Direct comparison with errors.Is
	assert.True(t, errors.Is(ErrTokenExpired, ErrTokenExpired))
	assert.False(t, errors.Is(ErrTokenExpired, ErrUserNotFound))
}

// TestAppError_Error tests AppError.Error() method.
func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appErr   *AppError
		expected string
	}{
		{
			name: "with underlying error",
			appErr: &AppError{
				Code:    "AUTH_001",
				Message: "authentication failed",
				Err:     errors.New("invalid credentials"),
			},
			expected: "AUTH_001: authentication failed: invalid credentials",
		},
		{
			name: "without underlying error",
			appErr: &AppError{
				Code:    "TUNNEL_001",
				Message: "tunnel creation failed",
				Err:     nil,
			},
			expected: "TUNNEL_001: tunnel creation failed",
		},
		{
			name: "with predefined error",
			appErr: &AppError{
				Code:    "AUTH_002",
				Message: "token validation failed",
				Err:     ErrInvalidToken,
			},
			expected: "AUTH_002: token validation failed: invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.appErr.Error())
		})
	}
}

// TestAppError_Unwrap tests AppError.Unwrap() method.
func TestAppError_Unwrap(t *testing.T) {
	underlyingErr := errors.New("underlying error")
	appErr := &AppError{
		Code:    "TEST_001",
		Message: "test error",
		Err:     underlyingErr,
	}

	// Unwrap should return the underlying error
	unwrapped := appErr.Unwrap()
	assert.Equal(t, underlyingErr, unwrapped)

	// errors.Is should work with wrapped errors
	assert.True(t, errors.Is(appErr, underlyingErr))
}

// TestAppError_UnwrapNil tests AppError.Unwrap() with no underlying error.
func TestAppError_UnwrapNil(t *testing.T) {
	appErr := &AppError{
		Code:    "TEST_002",
		Message: "test error without underlying",
		Err:     nil,
	}

	// Unwrap should return nil
	unwrapped := appErr.Unwrap()
	assert.Nil(t, unwrapped)
}

// TestNewAppError tests NewAppError constructor.
func TestNewAppError(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
		err     error
	}{
		{
			name:    "with underlying error",
			code:    "ERR_001",
			message: "operation failed",
			err:     errors.New("network error"),
		},
		{
			name:    "without underlying error",
			code:    "ERR_002",
			message: "validation failed",
			err:     nil,
		},
		{
			name:    "with predefined error",
			code:    "ERR_003",
			message: "authentication error",
			err:     ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr := NewAppError(tt.code, tt.message, tt.err)

			require.NotNil(t, appErr)
			assert.Equal(t, tt.code, appErr.Code)
			assert.Equal(t, tt.message, appErr.Message)
			assert.Equal(t, tt.err, appErr.Err)

			// Verify Error() method works
			errStr := appErr.Error()
			assert.Contains(t, errStr, tt.code)
			assert.Contains(t, errStr, tt.message)
		})
	}
}

// TestWrap tests Wrap function.
func TestWrap(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		message     string
		expectNil   bool
		expectInMsg string
	}{
		{
			name:        "wrap error",
			err:         errors.New("original error"),
			message:     "additional context",
			expectNil:   false,
			expectInMsg: "additional context: original error",
		},
		{
			name:        "wrap nil",
			err:         nil,
			message:     "this should not appear",
			expectNil:   true,
			expectInMsg: "",
		},
		{
			name:        "wrap predefined error",
			err:         ErrUnauthorized,
			message:     "user not authenticated",
			expectNil:   false,
			expectInMsg: "user not authenticated: unauthorized",
		},
		{
			name:        "wrap AppError",
			err:         NewAppError("TEST", "test error", nil),
			message:     "wrapped context",
			expectNil:   false,
			expectInMsg: "wrapped context: TEST: test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := Wrap(tt.err, tt.message)

			if tt.expectNil {
				assert.Nil(t, wrapped)
			} else {
				require.NotNil(t, wrapped)
				assert.Equal(t, tt.expectInMsg, wrapped.Error())

				// Verify errors.Is works with wrapped errors
				if tt.err != nil {
					assert.True(t, errors.Is(wrapped, tt.err))
				}
			}
		})
	}
}

// TestWrapChain tests wrapping errors multiple times.
func TestWrapChain(t *testing.T) {
	original := errors.New("original")
	wrapped1 := Wrap(original, "level 1")
	wrapped2 := Wrap(wrapped1, "level 2")
	wrapped3 := Wrap(wrapped2, "level 3")

	// All should be non-nil
	assert.NotNil(t, wrapped1)
	assert.NotNil(t, wrapped2)
	assert.NotNil(t, wrapped3)

	// errors.Is should find the original error
	assert.True(t, errors.Is(wrapped3, original))

	// Error message should contain all contexts
	msg := wrapped3.Error()
	assert.Contains(t, msg, "level 3")
	assert.Contains(t, msg, "level 2")
	assert.Contains(t, msg, "level 1")
	assert.Contains(t, msg, "original")
}

// TestAppErrorAsError tests using AppError as a regular error.
func TestAppErrorAsError(t *testing.T) {
	appErr := NewAppError("TEST", "test message", nil)

	// Should satisfy error interface
	var err error = appErr
	assert.NotNil(t, err)
	assert.Equal(t, "TEST: test message", err.Error())

	// errors.As should work
	var targetErr *AppError
	assert.True(t, errors.As(err, &targetErr))
	assert.Equal(t, "TEST", targetErr.Code)
	assert.Equal(t, "test message", targetErr.Message)
}

// TestAppErrorWithPredefinedErrors tests combining AppError with predefined errors.
func TestAppErrorWithPredefinedErrors(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		message       string
		predefinedErr error
	}{
		{"unauthorized", "AUTH_FAILED", "user authentication failed", ErrUnauthorized},
		{"invalid token", "TOKEN_INVALID", "token validation failed", ErrInvalidToken},
		{"token expired", "TOKEN_EXPIRED", "token has expired", ErrTokenExpired},
		{"tunnel not found", "TUNNEL_404", "requested tunnel does not exist", ErrTunnelNotFound},
		{"subdomain taken", "SUBDOMAIN_CONFLICT", "subdomain is already in use", ErrSubdomainTaken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr := NewAppError(tt.code, tt.message, tt.predefinedErr)

			// errors.Is should find the predefined error
			assert.True(t, errors.Is(appErr, tt.predefinedErr))

			// Error message should contain both
			msg := appErr.Error()
			assert.Contains(t, msg, tt.code)
			assert.Contains(t, msg, tt.message)
			assert.Contains(t, msg, tt.predefinedErr.Error())
		})
	}
}

// TestErrorComposition tests complex error composition.
func TestErrorComposition(t *testing.T) {
	// Start with a predefined error
	baseErr := ErrLocalServiceUnreachable

	// Wrap it with context
	wrappedErr := Wrap(baseErr, "failed to connect to localhost:3000")

	// Create an AppError with the wrapped error
	appErr := NewAppError("PROXY_ERROR", "proxy request failed", wrappedErr)

	// Wrap the AppError again
	finalErr := Wrap(appErr, "tunnel request processing error")

	// errors.Is should still find the original predefined error
	assert.True(t, errors.Is(finalErr, ErrLocalServiceUnreachable))

	// errors.As should find the AppError
	var targetAppErr *AppError
	assert.True(t, errors.As(finalErr, &targetAppErr))
	assert.Equal(t, "PROXY_ERROR", targetAppErr.Code)

	// Error message should contain all contexts
	msg := finalErr.Error()
	assert.Contains(t, msg, "tunnel request processing error")
	assert.Contains(t, msg, "PROXY_ERROR")
	assert.Contains(t, msg, "proxy request failed")
	assert.Contains(t, msg, "failed to connect to localhost:3000")
	assert.Contains(t, msg, "local service unreachable")
}

// BenchmarkNewAppError benchmarks AppError creation.
func BenchmarkNewAppError(b *testing.B) {
	baseErr := errors.New("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewAppError("CODE", "message", baseErr)
	}
}

// BenchmarkWrap benchmarks Wrap function.
func BenchmarkWrap(b *testing.B) {
	baseErr := errors.New("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Wrap(baseErr, "context")
	}
}

// BenchmarkAppErrorError benchmarks Error() method.
func BenchmarkAppErrorError(b *testing.B) {
	appErr := NewAppError("CODE", "message", errors.New("test"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = appErr.Error()
	}
}

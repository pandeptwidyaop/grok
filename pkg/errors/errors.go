package errors

import (
	"errors"
	"fmt"
)

// Common errors
var (
	ErrUnauthorized              = errors.New("unauthorized")
	ErrInvalidToken              = errors.New("invalid token")
	ErrTokenExpired              = errors.New("token expired")
	ErrUserNotFound              = errors.New("user not found")
	ErrTunnelNotFound            = errors.New("tunnel not found")
	ErrSubdomainTaken            = errors.New("subdomain already taken")
	ErrInvalidSubdomain          = errors.New("invalid subdomain format")
	ErrSubdomainAllocationFailed = errors.New("subdomain allocation failed")
	ErrMaxTunnelsReached         = errors.New("maximum tunnels per user reached")
	ErrLocalServiceUnreachable   = errors.New("local service unreachable")
	ErrRequestTimeout            = errors.New("request timeout")
	ErrRateLimited               = errors.New("rate limited")
	ErrInvalidProtocol           = errors.New("invalid protocol")
	ErrNoAvailablePorts          = errors.New("no available ports in pool")
)

// AppError represents an application error with context
type AppError struct {
	Code    string
	Message string
	Err     error
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new application error
func NewAppError(code, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Wrap wraps an error with additional context
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

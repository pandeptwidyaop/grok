package utils

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var (
	// ErrInvalidWebhookAppName is returned when webhook app name is invalid.
	ErrInvalidWebhookAppName = errors.New("invalid webhook app name")

	// ErrInvalidWebhookPath is returned when webhook path is invalid.
	ErrInvalidWebhookPath = errors.New("invalid webhook path")

	// ErrReservedWebhookAppName is returned when webhook app name is reserved.
	ErrReservedWebhookAppName = errors.New("webhook app name is reserved")

	// Reserved webhook app names that cannot be used.
	reservedWebhookAppNames = []string{
		"api", "admin", "webhook", "webhooks", "dashboard", "status",
		"health", "metrics", "docs", "apps", "system", "config",
		"www", "blog", "support", "help",
	}

	// Rejects: payment--app, payment_app, PaymentApp, -payment, payment-.
	webhookAppNameRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// - Cannot be a reserved name (case-sensitive).
func IsValidWebhookAppName(name string) bool {
	// Check length
	if len(name) < 3 || len(name) > 50 {
		return false
	}

	// Check against reserved names (case-sensitive)
	if IsReservedWebhookAppName(name) {
		return false
	}

	// Check format with regex (strict: must already be lowercase)
	return webhookAppNameRegex.MatchString(name)
}

// NormalizeWebhookAppName normalizes a webhook app name to lowercase and trims whitespace.
func NormalizeWebhookAppName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// Case-sensitive check (only lowercase reserved names match).
func IsReservedWebhookAppName(name string) bool {
	for _, reserved := range reservedWebhookAppNames {
		if name == reserved {
			return true
		}
	}
	return false
}

// - No null bytes or backslashes.
func ValidateWebhookPath(path string) error {
	// Check if path starts with /
	if !strings.HasPrefix(path, "/") {
		return errors.New("webhook path must start with /")
	}

	// Check max length
	if len(path) > 1024 {
		return errors.New("webhook path too long (max 1024 characters)")
	}

	// URL decode to catch encoded path traversal attempts (%2e%2e = ..)
	decodedPath, err := url.QueryUnescape(path)
	if err != nil {
		// If decoding fails, check the original path
		decodedPath = path
	}

	// Check for path traversal attempts in both original and decoded
	if strings.Contains(path, "..") || strings.Contains(decodedPath, "..") {
		return errors.New("webhook path contains invalid sequence '..'")
	}

	// Check for backslashes (Windows-style path traversal)
	if strings.Contains(path, "\\") || strings.Contains(decodedPath, "\\") {
		return errors.New("webhook path contains backslash")
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") || strings.Contains(decodedPath, "\x00") {
		return errors.New("webhook path contains null byte")
	}

	return nil
}

// SanitizeWebhookPath sanitizes a webhook path by removing dangerous patterns.
func SanitizeWebhookPath(path string) string {
	// Ensure it starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Remove any null bytes
	path = strings.ReplaceAll(path, "\x00", "")

	// Remove path traversal sequences
	path = strings.ReplaceAll(path, "../", "")
	path = strings.ReplaceAll(path, "/..", "")

	// Normalize multiple slashes to single slash
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	return path
}

// ValidateWebhookAppNameOrError validates webhook app name and returns error if invalid.
func ValidateWebhookAppNameOrError(name string) error {
	if len(name) < 3 {
		return errors.New("webhook app name must be at least 3 characters")
	}

	if len(name) > 50 {
		return errors.New("webhook app name must be at most 50 characters")
	}

	normalized := NormalizeWebhookAppName(name)

	if IsReservedWebhookAppName(normalized) {
		return ErrReservedWebhookAppName
	}

	if !webhookAppNameRegex.MatchString(normalized) {
		return errors.New("webhook app name must contain only lowercase letters, numbers, hyphens, and underscores")
	}

	return nil
}

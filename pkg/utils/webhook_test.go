package utils

import (
	"strings"
	"testing"
)

func TestIsValidWebhookAppName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid cases
		{"valid lowercase", "payment-app", true},
		{"valid with numbers", "app123", true},
		{"valid with hyphens", "my-webhook-app", true},
		{"valid minimum length", "abc", true},
		{"valid maximum length", strings.Repeat("a", 50), true},

		// Invalid cases
		{"too short", "ab", false},
		{"too long", strings.Repeat("a", 51), false},
		{"uppercase letters", "PaymentApp", false},
		{"underscore", "payment_app", false},
		{"special characters", "payment@app", false},
		{"starts with hyphen", "-payment", false},
		{"ends with hyphen", "payment-", false},
		{"double hyphen", "payment--app", false},
		{"spaces", "payment app", false},
		{"empty string", "", false},
		{"reserved name", "admin", false},
		{"reserved name", "api", false},
		{"reserved name", "webhook", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidWebhookAppName(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidWebhookAppName(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsReservedWebhookAppName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"reserved: admin", "admin", true},
		{"reserved: api", "api", true},
		{"reserved: webhook", "webhook", true},
		{"reserved: www", "www", true},
		{"reserved: status", "status", true},
		{"not reserved", "payment-app", false},
		{"not reserved", "my-app", false},
		{"case sensitive", "Admin", false}, // lowercase check
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsReservedWebhookAppName(tt.input)
			if result != tt.expected {
				t.Errorf("IsReservedWebhookAppName(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateWebhookPath(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldErr bool
	}{
		// Valid cases
		{"simple path", "/stripe/webhook", false},
		{"path with multiple segments", "/api/v1/webhooks/stripe", false},
		{"path with query params", "/webhook?token=abc", false},
		{"root path", "/", false},
		{"path with numbers", "/webhook123", false},
		{"path with hyphens", "/stripe-webhook", false},
		{"path with underscores", "/stripe_webhook", false},

		// Invalid cases - path traversal
		{"parent directory", "/stripe/../admin", true},
		{"parent at start", "/../etc/passwd", true},
		{"encoded parent", "/stripe/%2e%2e/admin", true},
		{"backslash", "/stripe\\..\\admin", true},
		{"multiple parents", "/a/../../etc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWebhookPath(tt.input)
			if tt.shouldErr && err == nil {
				t.Errorf("ValidateWebhookPath(%q) should return error but got nil", tt.input)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("ValidateWebhookPath(%q) should not return error but got: %v", tt.input, err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkIsValidWebhookAppName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IsValidWebhookAppName("payment-app-123")
	}
}

func BenchmarkValidateWebhookPath(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ValidateWebhookPath("/stripe/payment_intent")
	}
}

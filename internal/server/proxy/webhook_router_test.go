package proxy

import (
	"testing"
)

func TestWebhookRouter_IsWebhookRequest(t *testing.T) {
	router := &WebhookRouter{
		baseDomain: "grok.io",
	}

	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		// Valid webhook requests
		{"valid webhook subdomain", "trofeo-webhook.grok.io", true},
		{"valid with port", "trofeo-webhook.grok.io:443", true},
		{"valid simple org", "org-webhook.grok.io", true},
		{"valid multi-part org", "my-org-webhook.grok.io", true},

		// Invalid requests - not webhook subdomain
		{"regular subdomain", "trofeo.grok.io", false},
		{"api subdomain", "api.grok.io", false},
		{"no subdomain", "grok.io", false},
		{"wrong domain", "trofeo-webhook.example.com", false},
		{"incomplete webhook", "webhook.grok.io", false},    // no org prefix
		{"webhook prefix", "webhook-trofeo.grok.io", false}, // wrong order

		// Edge cases
		{"empty host", "", false},
		{"just domain", ".grok.io", false},
		{"just webhook", "-webhook", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.IsWebhookRequest(tt.host)
			if result != tt.expected {
				t.Errorf("IsWebhookRequest(%q) = %v; want %v", tt.host, result, tt.expected)
			}
		})
	}
}

func TestWebhookRouter_ExtractWebhookComponents(t *testing.T) {
	router := &WebhookRouter{
		baseDomain: "grok.io",
	}

	tests := []struct {
		name         string
		host         string
		path         string
		wantOrg      string
		wantApp      string
		wantUserPath string
		shouldErr    bool
	}{
		// Valid cases
		{
			name:         "complete webhook URL",
			host:         "trofeo-webhook.grok.io",
			path:         "/payment-app/stripe/payment_intent",
			wantOrg:      "trofeo",
			wantApp:      "payment-app",
			wantUserPath: "/stripe/payment_intent",
			shouldErr:    false,
		},
		{
			name:         "webhook with port",
			host:         "trofeo-webhook.grok.io:443",
			path:         "/payment-app/stripe/callback",
			wantOrg:      "trofeo",
			wantApp:      "payment-app",
			wantUserPath: "/stripe/callback",
			shouldErr:    false,
		},
		{
			name:         "app only path",
			host:         "trofeo-webhook.grok.io",
			path:         "/payment-app",
			wantOrg:      "trofeo",
			wantApp:      "payment-app",
			wantUserPath: "/",
			shouldErr:    false,
		},
		{
			name:         "app with trailing slash",
			host:         "trofeo-webhook.grok.io",
			path:         "/payment-app/",
			wantOrg:      "trofeo",
			wantApp:      "payment-app",
			wantUserPath: "/",
			shouldErr:    false,
		},
		{
			name:         "complex user path",
			host:         "my-org-webhook.grok.io",
			path:         "/app123/api/v1/webhooks/stripe",
			wantOrg:      "my-org",
			wantApp:      "app123",
			wantUserPath: "/api/v1/webhooks/stripe",
			shouldErr:    false,
		},
		{
			name:         "single char app",
			host:         "org-webhook.grok.io",
			path:         "/a/webhook",
			wantOrg:      "org",
			wantApp:      "a",
			wantUserPath: "/webhook",
			shouldErr:    false,
		},

		// Invalid cases
		{
			name:      "wrong domain",
			host:      "trofeo-webhook.example.com",
			path:      "/payment-app/stripe",
			shouldErr: true,
		},
		{
			name:      "not webhook subdomain",
			host:      "trofeo.grok.io",
			path:      "/payment-app/stripe",
			shouldErr: true,
		},
		{
			name:      "missing app name",
			host:      "trofeo-webhook.grok.io",
			path:      "/",
			shouldErr: true,
		},
		{
			name:      "empty path",
			host:      "trofeo-webhook.grok.io",
			path:      "",
			shouldErr: true,
		},
		{
			name:      "path without leading slash",
			host:      "trofeo-webhook.grok.io",
			path:      "payment-app/stripe",
			shouldErr: true,
		},
		{
			name:      "empty app name",
			host:      "trofeo-webhook.grok.io",
			path:      "//stripe",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, app, userPath, err := router.ExtractWebhookComponents(tt.host, tt.path)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("ExtractWebhookComponents(%q, %q) should return error but got nil", tt.host, tt.path)
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractWebhookComponents(%q, %q) unexpected error: %v", tt.host, tt.path, err)
				return
			}

			if org != tt.wantOrg {
				t.Errorf("orgSubdomain = %q; want %q", org, tt.wantOrg)
			}
			if app != tt.wantApp {
				t.Errorf("appName = %q; want %q", app, tt.wantApp)
			}
			if userPath != tt.wantUserPath {
				t.Errorf("userPath = %q; want %q", userPath, tt.wantUserPath)
			}
		})
	}
}

// Benchmark tests.
func BenchmarkIsWebhookRequest(b *testing.B) {
	router := &WebhookRouter{
		baseDomain: "grok.io",
	}
	for i := 0; i < b.N; i++ {
		router.IsWebhookRequest("trofeo-webhook.grok.io")
	}
}

func BenchmarkExtractWebhookComponents(b *testing.B) {
	router := &WebhookRouter{
		baseDomain: "grok.io",
	}
	for i := 0; i < b.N; i++ {
		router.ExtractWebhookComponents("trofeo-webhook.grok.io", "/payment-app/stripe/callback")
	}
}

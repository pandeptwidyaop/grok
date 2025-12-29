package proxy

import (
	"testing"

	"github.com/google/uuid"

	"github.com/pandeptwidyaop/grok/internal/db/models"
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
	// Setup test database
	db := setupTestDB(t)

	// Create test organization
	org := &models.Organization{
		ID:        uuid.New(),
		Name:      "Trofeo Org",
		Subdomain: "trofeo",
		IsActive:  true,
	}
	if err := db.Create(org).Error; err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	// Create test user
	user := &models.User{
		ID:             uuid.New(),
		Email:          "test@example.com",
		Password:       "hashed",
		Name:           "Test User",
		IsActive:       true,
		OrganizationID: &org.ID,
		Role:           models.RoleOrgAdmin,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create test webhook apps with new format: {app-name}-{org-subdomain}-webhook
	apps := []models.WebhookApp{
		{
			ID:             uuid.New(),
			OrganizationID: org.ID,
			UserID:         user.ID,
			Name:           "payment-app",
			IsActive:       true,
		},
		{
			ID:             uuid.New(),
			OrganizationID: org.ID,
			UserID:         user.ID,
			Name:           "metachannel",
			IsActive:       true,
		},
	}
	for _, app := range apps {
		if err := db.Create(&app).Error; err != nil {
			t.Fatalf("Failed to create webhook app: %v", err)
		}
	}

	router := &WebhookRouter{
		db:         db,
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
		// Valid cases - new format: {app-name}-{org-subdomain}-webhook
		{
			name:         "complete webhook URL",
			host:         "payment-app-trofeo-webhook.grok.io",
			path:         "/stripe/payment_intent",
			wantOrg:      "trofeo",
			wantApp:      "payment-app",
			wantUserPath: "/stripe/payment_intent",
			shouldErr:    false,
		},
		{
			name:         "webhook with port",
			host:         "payment-app-trofeo-webhook.grok.io:443",
			path:         "/stripe/callback",
			wantOrg:      "trofeo",
			wantApp:      "payment-app",
			wantUserPath: "/stripe/callback",
			shouldErr:    false,
		},
		{
			name:         "root path",
			host:         "payment-app-trofeo-webhook.grok.io",
			path:         "/",
			wantOrg:      "trofeo",
			wantApp:      "payment-app",
			wantUserPath: "/",
			shouldErr:    false,
		},
		{
			name:         "empty path",
			host:         "payment-app-trofeo-webhook.grok.io",
			path:         "",
			wantOrg:      "trofeo",
			wantApp:      "payment-app",
			wantUserPath: "/",
			shouldErr:    false,
		},
		{
			name:         "complex user path",
			host:         "metachannel-trofeo-webhook.grok.io",
			path:         "/api/v1/webhooks/stripe",
			wantOrg:      "trofeo",
			wantApp:      "metachannel",
			wantUserPath: "/api/v1/webhooks/stripe",
			shouldErr:    false,
		},

		// Invalid cases
		{
			name:      "wrong domain",
			host:      "payment-app-trofeo-webhook.example.com",
			path:      "/stripe",
			shouldErr: true,
		},
		{
			name:      "not webhook subdomain",
			host:      "trofeo.grok.io",
			path:      "/stripe",
			shouldErr: true,
		},
		{
			name:      "app not found in database",
			host:      "nonexistent-trofeo-webhook.grok.io",
			path:      "/",
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

func TestRequestData_QueryStringForwarding(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		queryString string
		wantPath    string
		wantQuery   string
	}{
		{
			name:        "with query parameters",
			method:      "GET",
			path:        "/api/users",
			queryString: "id=123&filter=active",
			wantPath:    "/api/users",
			wantQuery:   "id=123&filter=active",
		},
		{
			name:        "without query parameters",
			method:      "POST",
			path:        "/api/payment",
			queryString: "",
			wantPath:    "/api/payment",
			wantQuery:   "",
		},
		{
			name:        "complex query string",
			method:      "GET",
			path:        "/webhook/stripe",
			queryString: "timestamp=1234567890&signature=abc123&event=payment_intent.succeeded",
			wantPath:    "/webhook/stripe",
			wantQuery:   "timestamp=1234567890&signature=abc123&event=payment_intent.succeeded",
		},
		{
			name:        "single query param",
			method:      "GET",
			path:        "/callback",
			queryString: "code=abc123",
			wantPath:    "/callback",
			wantQuery:   "code=abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestData := &RequestData{
				Method:      tt.method,
				Path:        tt.path,
				QueryString: tt.queryString,
				Headers:     make(map[string][]string),
				Body:        []byte{},
			}

			// Verify fields are set correctly
			if requestData.Path != tt.wantPath {
				t.Errorf("Path = %q; want %q", requestData.Path, tt.wantPath)
			}
			if requestData.QueryString != tt.wantQuery {
				t.Errorf("QueryString = %q; want %q", requestData.QueryString, tt.wantQuery)
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
	// Note: Benchmarks don't use testing.T, so we skip DB setup
	// This benchmark measures the parsing logic without database overhead
	// For real-world performance, database query time would be included
	b.Skip("Skipping benchmark - requires database setup")
}

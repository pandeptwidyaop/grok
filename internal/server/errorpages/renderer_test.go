package errorpages

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderErrorPage_404_HTML(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	RenderErrorPage(w, req, http.StatusNotFound, &ErrorPageData{
		Subdomain: "test-tunnel",
	})

	// Verify status code
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected HTML content type, got %s", contentType)
	}

	// Verify body contains expected content
	body := w.Body.String()
	if !strings.Contains(body, "404") {
		t.Error("Expected body to contain '404'")
	}
	if !strings.Contains(body, "test-tunnel") {
		t.Error("Expected body to contain subdomain 'test-tunnel'")
	}
	if !strings.Contains(body, "Tunnel Not Found") {
		t.Error("Expected body to contain 'Tunnel Not Found'")
	}
}

func TestRenderErrorPage_404_JSON(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	RenderErrorPage(w, req, http.StatusNotFound, &ErrorPageData{
		Subdomain: "test-tunnel",
	})

	// Verify status code
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	// Verify JSON response
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if response["error"] != "Tunnel not found" {
		t.Errorf("Expected error 'Tunnel not found', got %v", response["error"])
	}
	if response["status"] != float64(404) {
		t.Errorf("Expected status 404, got %v", response["status"])
	}
	if !strings.Contains(response["details"].(string), "test-tunnel") {
		t.Error("Expected details to contain subdomain")
	}
}

func TestRenderErrorPage_400_JSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/webhook", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	RenderErrorPage(w, req, http.StatusBadRequest, &ErrorPageData{
		URL: "invalid.webhook.url/path",
	})

	// Verify status code
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	// Verify JSON response
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if response["error"] != "Invalid webhook URL" {
		t.Errorf("Expected error 'Invalid webhook URL', got %v", response["error"])
	}
}

func TestRenderErrorPage_NilData(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic with nil data
	RenderErrorPage(w, req, http.StatusNotFound, nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	// Should still render template successfully
	body := w.Body.String()
	if !strings.Contains(body, "404") {
		t.Error("Expected body to contain '404' even with nil data")
	}
}

func TestRenderErrorPage_UnsupportedStatus(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Should fallback to plain text for unmapped status codes
	RenderErrorPage(w, req, http.StatusInternalServerError, nil)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	// Should be plain text fallback
	body := w.Body.String()
	if strings.Contains(body, "<html>") {
		t.Error("Expected plain text fallback, got HTML")
	}
}

func TestTunnelNotFound_HTML(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	TunnelNotFound(w, req, "my-app")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	// Should be HTML
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected HTML content type, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "my-app") {
		t.Error("Expected body to contain subdomain")
	}
}

func TestTunnelNotFound_JSON(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	TunnelNotFound(w, req, "my-app")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	// Should be JSON
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if !strings.Contains(response["details"].(string), "my-app") {
		t.Error("Expected details to contain subdomain")
	}
}

func TestInvalidWebhookURL_AlwaysJSON(t *testing.T) {
	w := httptest.NewRecorder()

	InvalidWebhookURL(w, "webhook.example.com/bad/path")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Should always be JSON for webhooks
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if response["error"] != "Invalid webhook URL" {
		t.Errorf("Expected error 'Invalid webhook URL', got %v", response["error"])
	}
	if !strings.Contains(response["details"].(string), "webhook.example.com/bad/path") {
		t.Error("Expected details to contain URL")
	}
}

func TestTemplateCaching(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	// First render initializes cache
	w1 := httptest.NewRecorder()
	RenderErrorPage(w1, req, http.StatusNotFound, nil)

	// Second render should use cached template
	w2 := httptest.NewRecorder()
	RenderErrorPage(w2, req, http.StatusNotFound, nil)

	// Both should succeed
	if w1.Code != http.StatusNotFound || w2.Code != http.StatusNotFound {
		t.Error("Template caching should not affect rendering")
	}

	// Verify cache was populated
	cacheMu.RLock()
	cacheSize := len(templateCache)
	cacheMu.RUnlock()

	if cacheSize == 0 {
		t.Error("Expected template cache to be populated")
	}
}

func TestAcceptsJSON(t *testing.T) {
	tests := []struct {
		name   string
		accept string
		want   bool
	}{
		{"JSON explicit", "application/json", true},
		{"JSON with charset", "application/json; charset=utf-8", true},
		{"HTML", "text/html", false},
		{"Any", "*/*", false},
		{"Empty", "", false},
		{"Multiple with JSON", "text/html, application/json", true},
		{"Multiple with spaces", "text/html , application/json , */*", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			got := acceptsJSON(req)
			if got != tt.want {
				t.Errorf("acceptsJSON() = %v, want %v for Accept: %s", got, tt.want, tt.accept)
			}
		})
	}
}

func BenchmarkRenderErrorPage_HTML(b *testing.B) {
	// Initialize templates once
	initTemplates()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "text/html")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		RenderErrorPage(w, req, http.StatusNotFound, &ErrorPageData{
			Subdomain: "benchmark-tunnel",
		})
	}
}

func BenchmarkRenderErrorPage_JSON(b *testing.B) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "application/json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		RenderErrorPage(w, req, http.StatusNotFound, &ErrorPageData{
			Subdomain: "benchmark-tunnel",
		})
	}
}

package errorpages

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderErrorPage_404(t *testing.T) {
	w := httptest.NewRecorder()

	RenderErrorPage(w, http.StatusNotFound, &ErrorPageData{
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

func TestRenderErrorPage_400(t *testing.T) {
	w := httptest.NewRecorder()

	RenderErrorPage(w, http.StatusBadRequest, &ErrorPageData{
		URL: "invalid.webhook.url/path",
	})

	// Verify status code
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Verify body contains expected content
	body := w.Body.String()
	if !strings.Contains(body, "400") {
		t.Error("Expected body to contain '400'")
	}
	if !strings.Contains(body, "invalid.webhook.url/path") {
		t.Error("Expected body to contain URL")
	}
	if !strings.Contains(body, "Invalid Webhook URL") {
		t.Error("Expected body to contain 'Invalid Webhook URL'")
	}
}

func TestRenderErrorPage_NilData(t *testing.T) {
	w := httptest.NewRecorder()

	// Should not panic with nil data
	RenderErrorPage(w, http.StatusNotFound, nil)

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
	w := httptest.NewRecorder()

	// Should fallback to plain text for unmapped status codes
	RenderErrorPage(w, http.StatusInternalServerError, nil)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	// Should be plain text fallback
	body := w.Body.String()
	if strings.Contains(body, "<html>") {
		t.Error("Expected plain text fallback, got HTML")
	}
}

func TestTunnelNotFound(t *testing.T) {
	w := httptest.NewRecorder()

	TunnelNotFound(w, "my-app")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "my-app") {
		t.Error("Expected body to contain subdomain")
	}
}

func TestInvalidWebhookURL(t *testing.T) {
	w := httptest.NewRecorder()

	InvalidWebhookURL(w, "webhook.example.com/bad/path")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "webhook.example.com/bad/path") {
		t.Error("Expected body to contain URL")
	}
}

func TestTemplateCaching(t *testing.T) {
	// First render initializes cache
	w1 := httptest.NewRecorder()
	RenderErrorPage(w1, http.StatusNotFound, nil)

	// Second render should use cached template
	w2 := httptest.NewRecorder()
	RenderErrorPage(w2, http.StatusNotFound, nil)

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

func BenchmarkRenderErrorPage(b *testing.B) {
	// Initialize templates once
	initTemplates()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		RenderErrorPage(w, http.StatusNotFound, &ErrorPageData{
			Subdomain: "benchmark-tunnel",
		})
	}
}

package errorpages

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"sync"

	"github.com/pandeptwidyaop/grok/pkg/logger"
)

var (
	// Template cache for performance (compiled once at startup).
	templateCache = make(map[string]*template.Template)
	cacheMu       sync.RWMutex
	initOnce      sync.Once
)

// ErrorPageData holds dynamic data for error templates.
type ErrorPageData struct {
	Subdomain string // For 404 errors
	URL       string // For 400 errors
}

// initTemplates compiles all templates on first use.
func initTemplates() {
	initOnce.Do(func() {
		// Parse all templates
		templates := []string{"404.html", "400.html"}

		for _, tmplName := range templates {
			tmpl, err := template.ParseFS(templatesFS, "templates/"+tmplName)
			if err != nil {
				logger.ErrorEvent().
					Err(err).
					Str("template", tmplName).
					Msg("Failed to parse error template")
				continue
			}

			cacheMu.Lock()
			templateCache[tmplName] = tmpl
			cacheMu.Unlock()

			logger.InfoEvent().
				Str("template", tmplName).
				Msg("Error template loaded successfully")
		}
	})
}

// acceptsJSON checks if the client accepts JSON responses.
func acceptsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	// Check if client explicitly accepts JSON
	return strings.Contains(accept, "application/json") || strings.Contains(accept, "*/json")
}

// renderJSON renders a JSON error response.
func renderJSON(w http.ResponseWriter, statusCode int, message string, details string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"error":   message,
		"status":  statusCode,
		"message": http.StatusText(statusCode),
	}

	if details != "" {
		response["details"] = details
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WarnEvent().
			Err(err).
			Msg("Failed to encode JSON error response")
	}
}

// RenderErrorPage renders an error page with optional data.
// Supports content negotiation: returns JSON if Accept header contains application/json,
// otherwise returns HTML.
func RenderErrorPage(w http.ResponseWriter, r *http.Request, statusCode int, data *ErrorPageData) {
	// Ensure data is not nil
	if data == nil {
		data = &ErrorPageData{}
	}

	// Check if client accepts JSON
	if acceptsJSON(r) {
		var message, details string
		switch statusCode {
		case http.StatusNotFound:
			message = "Tunnel not found"
			if data.Subdomain != "" {
				details = "Subdomain: " + data.Subdomain
			}
		case http.StatusBadRequest:
			message = "Invalid webhook URL"
			if data.URL != "" {
				details = "URL: " + data.URL
			}
		default:
			message = http.StatusText(statusCode)
		}
		renderJSON(w, statusCode, message, details)
		return
	}

	// Initialize templates on first call for HTML responses
	initTemplates()

	// Map status codes to template files
	var templateName string
	switch statusCode {
	case http.StatusNotFound:
		templateName = "404.html"
	case http.StatusBadRequest:
		templateName = "400.html"
	default:
		// Fallback to plain text for unmapped errors
		http.Error(w, http.StatusText(statusCode), statusCode)
		return
	}

	// Get template from cache
	cacheMu.RLock()
	tmpl, ok := templateCache[templateName]
	cacheMu.RUnlock()

	if !ok {
		logger.ErrorEvent().
			Str("template", templateName).
			Msg("Error template not found in cache")
		http.Error(w, http.StatusText(statusCode), statusCode)
		return
	}

	// Render template to buffer (avoid partial writes on error)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		logger.ErrorEvent().
			Err(err).
			Str("template", templateName).
			Msg("Failed to execute error template")
		http.Error(w, http.StatusText(statusCode), statusCode)
		return
	}

	// Write response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	if _, err := buf.WriteTo(w); err != nil {
		logger.WarnEvent().
			Err(err).
			Msg("Failed to write error page to response")
	}
}

// TunnelNotFound renders 404 error page for missing tunnels.
// Supports content negotiation: returns JSON for API clients, HTML for browsers.
func TunnelNotFound(w http.ResponseWriter, r *http.Request, subdomain string) {
	RenderErrorPage(w, r, http.StatusNotFound, &ErrorPageData{
		Subdomain: subdomain,
	})
}

// InvalidWebhookURL renders 400 error for invalid webhook URLs.
// Always returns JSON since webhooks are API-to-API communication.
func InvalidWebhookURL(w http.ResponseWriter, url string) {
	renderJSON(w, http.StatusBadRequest, "Invalid webhook URL", "URL: "+url)
}

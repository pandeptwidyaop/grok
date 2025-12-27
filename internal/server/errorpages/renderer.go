package errorpages

import (
	"bytes"
	"html/template"
	"net/http"
	"sync"

	"github.com/pandeptwidyaop/grok/pkg/logger"
)

var (
	// Template cache for performance (compiled once at startup)
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

// RenderErrorPage renders an error page with optional data.
func RenderErrorPage(w http.ResponseWriter, statusCode int, data *ErrorPageData) {
	// Initialize templates on first call
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

	// Ensure data is not nil
	if data == nil {
		data = &ErrorPageData{}
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
func TunnelNotFound(w http.ResponseWriter, subdomain string) {
	RenderErrorPage(w, http.StatusNotFound, &ErrorPageData{
		Subdomain: subdomain,
	})
}

// InvalidWebhookURL renders 400 error page for invalid webhook URLs.
func InvalidWebhookURL(w http.ResponseWriter, url string) {
	RenderErrorPage(w, http.StatusBadRequest, &ErrorPageData{
		URL: url,
	})
}

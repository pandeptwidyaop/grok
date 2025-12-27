package dashboard

import (
	"embed"
	"io/fs"
)

// Embed the React dashboard build output.
//
//go:embed web/dist/*
var dashboardFS embed.FS

// GetDashboardFS returns the embedded filesystem for the dashboard.
// This contains the React app build output (index.html, JS, CSS, etc.).
func GetDashboardFS() (fs.FS, error) {
	// Strip the "web/dist" prefix to serve files from root
	return fs.Sub(dashboardFS, "web/dist")
}

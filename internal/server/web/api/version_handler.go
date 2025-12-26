package api

import (
	"net/http"

	"github.com/pandeptwidyaop/grok/internal/version"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// VersionHandler handles version-related API requests
type VersionHandler struct{}

// NewVersionHandler creates a new version handler
func NewVersionHandler() *VersionHandler {
	return &VersionHandler{}
}

// GetVersion returns the current version information
func (vh *VersionHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	info := version.GetVersion()
	respondJSON(w, http.StatusOK, info)
}

// CheckUpdates checks for available updates from GitHub
func (vh *VersionHandler) CheckUpdates(w http.ResponseWriter, r *http.Request) {
	updateInfo, err := version.CheckForUpdates("pandeptwidyaop", "grok")
	if err != nil {
		logger.ErrorEvent().Err(err).Msg("Failed to check for updates")
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to check for updates",
		})
		return
	}

	respondJSON(w, http.StatusOK, updateInfo)
}

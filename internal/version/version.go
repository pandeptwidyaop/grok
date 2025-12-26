package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pandeptwidyaop/grok/pkg/logger"
)

var (
	// Version is the current version of the application
	// This can be set at build time with -ldflags "-X internal/version.Version=v1.0.0"
	Version = "dev"

	// GitCommit is the git commit hash
	GitCommit = "unknown"

	// BuildDate is the build date
	BuildDate = "unknown"
)

// Info represents version information
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
}

// GetVersion returns the current version information
func GetVersion() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	}
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
}

// UpdateInfo represents update availability information
type UpdateInfo struct {
	CurrentVersion string         `json:"current_version"`
	LatestVersion  string         `json:"latest_version"`
	UpdateAvailable bool          `json:"update_available"`
	ReleaseURL     string         `json:"release_url,omitempty"`
	ReleaseNotes   string         `json:"release_notes,omitempty"`
}

// CheckForUpdates checks GitHub for the latest release
func CheckForUpdates(owner, repo string) (*UpdateInfo, error) {
	// Get latest release from GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent header (required by GitHub API)
	req.Header.Set("User-Agent", "Grok-Tunnel")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.WarnEvent().
			Int("status_code", resp.StatusCode).
			Str("response", string(body)).
			Msg("GitHub API returned non-200 status")
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Compare versions
	updateAvailable := isNewerVersion(Version, release.TagName)

	return &UpdateInfo{
		CurrentVersion:  Version,
		LatestVersion:   release.TagName,
		UpdateAvailable: updateAvailable,
		ReleaseURL:      release.HTMLURL,
		ReleaseNotes:    release.Body,
	}, nil
}

// isNewerVersion compares two semantic version strings
// Returns true if latestVersion is newer than currentVersion
func isNewerVersion(currentVersion, latestVersion string) bool {
	// Skip check if running dev version
	if currentVersion == "dev" {
		return false
	}

	// Remove 'v' prefix if present
	current := strings.TrimPrefix(currentVersion, "v")
	latest := strings.TrimPrefix(latestVersion, "v")

	// Simple string comparison (works for semantic versioning)
	// For more robust comparison, consider using a semver library
	return latest > current
}

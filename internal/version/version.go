package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/pandeptwidyaop/grok/pkg/logger"
)

var (
	// This can be set at build time with -ldflags "-X internal/version.Version=v1.0.0".
	Version = "dev"

	// GitCommit is the git commit hash.
	GitCommit = "unknown"

	// BuildDate is the build date.
	BuildDate = "unknown"
)

// Info represents version information.
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
}

// GetVersion returns the current version information.
func GetVersion() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	}
}

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	Prerelease  bool      `json:"prerelease"`
	Assets      []Asset   `json:"assets"`
}

// Asset represents a downloadable binary asset from a GitHub release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Release is an alias for GitHubRelease for backward compatibility.
type Release = GitHubRelease

// ReleaseListOptions configures release fetching behavior.
type ReleaseListOptions struct {
	IncludePrerelease bool
	Limit             int // Default: 10
}

// UpdateInfo represents update availability information.
type UpdateInfo struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url,omitempty"`
	ReleaseNotes    string `json:"release_notes,omitempty"`
}

// CheckForUpdates checks GitHub for the latest release.
func CheckForUpdates(owner, repo string) (*UpdateInfo, error) {
	// Get latest release from GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

// Returns true if latestVersion is newer than currentVersion.
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

// FetchReleases retrieves a list of releases from GitHub.
func FetchReleases(owner, repo string, opts ReleaseListOptions) ([]Release, error) {
	if opts.Limit == 0 {
		opts.Limit = 10
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=%d",
		owner, repo, opts.Limit)

	client := &http.Client{Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Grok-Tunnel")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s",
			resp.StatusCode, string(body))
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases: %w", err)
	}

	// Filter out pre-releases if not requested
	if !opts.IncludePrerelease {
		filtered := make([]Release, 0, len(releases))
		for _, r := range releases {
			if !r.Prerelease {
				filtered = append(filtered, r)
			}
		}
		releases = filtered
	}

	// Limit to requested count
	if len(releases) > opts.Limit {
		releases = releases[:opts.Limit]
	}

	return releases, nil
}

// FetchReleaseByTag retrieves a specific release by its tag name.
func FetchReleaseByTag(owner, repo, tag string) (Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s",
		owner, repo, tag)

	client := &http.Client{Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Release{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Grok-Tunnel")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Release{}, fmt.Errorf("GitHub API returned status %d: %s",
			resp.StatusCode, string(body))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return Release{}, fmt.Errorf("failed to decode release: %w", err)
	}

	return release, nil
}

// ValidateReleaseAssets checks if a release has the binary for the current platform.
func ValidateReleaseAssets(release Release, binaryName string) error {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Normalize architecture to match release naming convention
	arch := goarch
	if goarch == "amd64" {
		arch = "x86_64"
	} else if goarch == "arm64" {
		arch = "aarch64"
	} else if goarch == "386" {
		arch = "i386"
	}

	// Expected filename: {binary}_{version}_{os}_{arch}.tar.gz
	// Strip 'v' prefix from tag name for filename matching
	version := strings.TrimPrefix(release.TagName, "v")
	expectedFile := fmt.Sprintf("%s_%s_%s_%s.tar.gz",
		binaryName, version, goos, arch)

	// Check if asset exists
	for _, asset := range release.Assets {
		if asset.Name == expectedFile {
			return nil
		}
	}

	// List available platforms for error message
	available := make([]string, 0, len(release.Assets))
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".tar.gz") &&
			!strings.Contains(asset.Name, "checksums") {
			available = append(available, asset.Name)
		}
	}

	return fmt.Errorf("no binary for %s/%s. Available: %s",
		goos, arch, strings.Join(available, ", "))
}

// TruncateReleaseNotes truncates release notes to a maximum number of lines.
func TruncateReleaseNotes(body string, maxLines int) string {
	if body == "" {
		return ""
	}

	lines := strings.Split(body, "\n")
	if len(lines) <= maxLines {
		return body
	}

	truncated := strings.Join(lines[:maxLines], "\n")
	return truncated + "\n\n... (read more at release URL)"
}

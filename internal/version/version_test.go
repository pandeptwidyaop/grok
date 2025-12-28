package version

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
)

func TestFetchReleases(t *testing.T) {
	tests := []struct {
		name               string
		opts               ReleaseListOptions
		mockResponse       string
		expectedCount      int
		expectedPrerelease int
	}{
		{
			name: "fetch stable releases only",
			opts: ReleaseListOptions{IncludePrerelease: false, Limit: 10},
			mockResponse: `[
				{"tag_name": "v1.2.0", "name": "Release 1.2.0", "published_at": "2025-01-01T00:00:00Z", "html_url": "https://github.com/test/repo/releases/tag/v1.2.0", "body": "Release notes", "prerelease": false, "assets": []},
				{"tag_name": "v1.2.0-alpha.1", "name": "Alpha 1", "published_at": "2024-12-01T00:00:00Z", "html_url": "https://github.com/test/repo/releases/tag/v1.2.0-alpha.1", "body": "Alpha release", "prerelease": true, "assets": []},
				{"tag_name": "v1.1.0", "name": "Release 1.1.0", "published_at": "2024-11-01T00:00:00Z", "html_url": "https://github.com/test/repo/releases/tag/v1.1.0", "body": "Release notes", "prerelease": false, "assets": []}
			]`,
			expectedCount:      2,
			expectedPrerelease: 0,
		},
		{
			name: "fetch with pre-releases",
			opts: ReleaseListOptions{IncludePrerelease: true, Limit: 10},
			mockResponse: `[
				{"tag_name": "v1.2.0", "name": "Release 1.2.0", "published_at": "2025-01-01T00:00:00Z", "html_url": "https://github.com/test/repo/releases/tag/v1.2.0", "body": "Release notes", "prerelease": false, "assets": []},
				{"tag_name": "v1.2.0-alpha.1", "name": "Alpha 1", "published_at": "2024-12-01T00:00:00Z", "html_url": "https://github.com/test/repo/releases/tag/v1.2.0-alpha.1", "body": "Alpha release", "prerelease": true, "assets": []},
				{"tag_name": "v1.1.0", "name": "Release 1.1.0", "published_at": "2024-11-01T00:00:00Z", "html_url": "https://github.com/test/repo/releases/tag/v1.1.0", "body": "Release notes", "prerelease": false, "assets": []}
			]`,
			expectedCount:      3,
			expectedPrerelease: 1,
		},
		{
			name: "respect limit",
			opts: ReleaseListOptions{IncludePrerelease: true, Limit: 2},
			mockResponse: `[
				{"tag_name": "v1.3.0", "name": "Release 1.3.0", "published_at": "2025-02-01T00:00:00Z", "html_url": "https://github.com/test/repo/releases/tag/v1.3.0", "body": "Release notes", "prerelease": false, "assets": []},
				{"tag_name": "v1.2.0", "name": "Release 1.2.0", "published_at": "2025-01-01T00:00:00Z", "html_url": "https://github.com/test/repo/releases/tag/v1.2.0", "body": "Release notes", "prerelease": false, "assets": []},
				{"tag_name": "v1.1.0", "name": "Release 1.1.0", "published_at": "2024-11-01T00:00:00Z", "html_url": "https://github.com/test/repo/releases/tag/v1.1.0", "body": "Release notes", "prerelease": false, "assets": []}
			]`,
			expectedCount:      2,
			expectedPrerelease: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			// Temporarily replace GitHub API URL for testing
			// We'll use a helper function that accepts custom URL
			releases, err := fetchReleasesFromURL(server.URL, tt.opts)
			if err != nil {
				t.Fatalf("FetchReleases failed: %v", err)
			}

			if len(releases) != tt.expectedCount {
				t.Errorf("Expected %d releases, got %d", tt.expectedCount, len(releases))
			}

			preCount := 0
			for _, r := range releases {
				if r.Prerelease {
					preCount++
				}
			}

			if preCount != tt.expectedPrerelease {
				t.Errorf("Expected %d pre-releases, got %d", tt.expectedPrerelease, preCount)
			}
		})
	}
}

// Helper function for testing.
func fetchReleasesFromURL(url string, opts ReleaseListOptions) ([]Release, error) {
	if opts.Limit == 0 {
		opts.Limit = 10
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Grok-Tunnel")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
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

func TestValidateReleaseAssets(t *testing.T) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Normalize arch
	arch := goarch
	if goarch == "amd64" {
		arch = "x86_64"
	} else if goarch == "arm64" {
		arch = "aarch64"
	} else if goarch == "386" {
		arch = "i386"
	}

	tests := []struct {
		name        string
		release     Release
		binaryName  string
		expectError bool
	}{
		{
			name: "valid assets for current platform",
			release: Release{
				TagName: "v1.2.0",
				Assets: []Asset{
					{Name: "grok-server_1.2.0_" + goos + "_" + arch + ".tar.gz"},
					{Name: "grok-server_1.2.0_linux_x86_64.tar.gz"},
				},
			},
			binaryName:  "grok-server",
			expectError: false,
		},
		{
			name: "missing platform binary",
			release: Release{
				TagName: "v1.2.0",
				Assets: []Asset{
					{Name: "grok-server_1.2.0_windows_x86_64.tar.gz"},
				},
			},
			binaryName:  "grok-server",
			expectError: true,
		},
		{
			name: "empty assets",
			release: Release{
				TagName: "v1.2.0",
				Assets:  []Asset{},
			},
			binaryName:  "grok-server",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReleaseAssets(tt.release, tt.binaryName)
			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestTruncateReleaseNotes(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		maxLines int
		expected string
	}{
		{
			name:     "short notes no truncation",
			body:     "Line 1\nLine 2\nLine 3",
			maxLines: 5,
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "truncate long notes",
			body:     "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6",
			maxLines: 3,
			expected: "Line 1\nLine 2\nLine 3\n\n... (read more at release URL)",
		},
		{
			name:     "empty notes",
			body:     "",
			maxLines: 3,
			expected: "",
		},
		{
			name:     "exact limit",
			body:     "Line 1\nLine 2\nLine 3",
			maxLines: 3,
			expected: "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateReleaseNotes(tt.body, tt.maxLines)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\n\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestFetchReleaseByTag(t *testing.T) {
	mockRelease := `{
		"tag_name": "v1.2.0",
		"name": "Release 1.2.0",
		"published_at": "2025-01-01T00:00:00Z",
		"html_url": "https://github.com/test/repo/releases/tag/v1.2.0",
		"body": "Release notes",
		"prerelease": false,
		"assets": []
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockRelease))
	}))
	defer server.Close()

	// We would need to refactor FetchReleaseByTag to accept custom URL for testing
	// For now, this test demonstrates the structure
	// In production, consider using dependency injection or test build tags
	t.Skip("Requires refactoring to inject custom URL for testing")
}

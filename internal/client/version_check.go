package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	versionpkg "github.com/pandeptwidyaop/grok/internal/version"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// VersionChecker checks version compatibility between client and server.
type VersionChecker struct {
	serverAddr string
	httpClient *http.Client
}

// NewVersionChecker creates a new version checker for the given server address.
func NewVersionChecker(serverAddr string) *VersionChecker {
	return &VersionChecker{
		serverAddr: serverAddr,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// VersionMismatch contains information about client-server version differences.
type VersionMismatch struct {
	ClientVersion string
	ServerVersion string
	Mismatch      bool
}

// CheckVersion queries server version and compares with client version.
func (vc *VersionChecker) CheckVersion(ctx context.Context) (*VersionMismatch, error) {
	serverVersion, err := vc.GetServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}

	clientVersion := versionpkg.Version
	mismatch := clientVersion != serverVersion

	return &VersionMismatch{
		ClientVersion: clientVersion,
		ServerVersion: serverVersion,
		Mismatch:      mismatch,
	}, nil
}

// GetServerVersion retrieves the server version string.
func (vc *VersionChecker) GetServerVersion(ctx context.Context) (string, error) {
	// Construct API URL (assume server runs on HTTP for version check)
	// The serverAddr format is typically "server.example.com:443" or "localhost:8080"
	url := fmt.Sprintf("https://%s/api/version", vc.serverAddr)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := vc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var versionInfo versionpkg.Info
	if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return versionInfo.Version, nil
}

// DisplayWarning displays a warning banner if versions mismatch.
func (vm *VersionMismatch) DisplayWarning() {
	if !vm.Mismatch {
		return
	}

	logger.WarnEvent().
		Str("client_version", vm.ClientVersion).
		Str("server_version", vm.ServerVersion).
		Msg("Version mismatch detected")

	// Display warning banner
	fmt.Println("┌─────────────────────────────────────────────────────────┐")
	fmt.Printf("│ ⚠️  VERSION MISMATCH WARNING                            │\n")
	fmt.Printf("│ Client: %-47s │\n", vm.ClientVersion)
	fmt.Printf("│ Server: %-47s │\n", vm.ServerVersion)
	fmt.Println("│                                                         │")
	fmt.Println("│ Some operations may not be supported.                  │")
	fmt.Println("│ Update client: grok update --match-server              │")
	fmt.Println("└─────────────────────────────────────────────────────────┘")
	fmt.Println()
}

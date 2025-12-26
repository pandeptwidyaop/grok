package updater

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pandeptwidyaop/grok/internal/version"
	"github.com/pandeptwidyaop/grok/pkg/logger"
)

const (
	downloadTimeout = 5 * time.Minute
	githubOwner     = "pandeptwidyaop"
	githubRepo      = "grok"
)

// Updater handles binary updates from GitHub releases.
type Updater struct {
	currentVersion string
	binaryName     string
	binaryPath     string
}

// NewUpdater creates a new updater instance.
func NewUpdater(binaryName string) (*Updater, error) {
	// Get current binary path
	binaryPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	return &Updater{
		currentVersion: version.Version,
		binaryName:     binaryName,
		binaryPath:     binaryPath,
	}, nil
}

// CheckForUpdates checks if a new version is available.
func (u *Updater) CheckForUpdates() (*version.UpdateInfo, error) {
	return version.CheckForUpdates(githubOwner, githubRepo)
}

// Update downloads and installs the latest version.
func (u *Updater) Update(updateInfo *version.UpdateInfo) error {
	if !updateInfo.UpdateAvailable {
		return fmt.Errorf("no update available")
	}

	logger.InfoEvent().
		Str("current_version", updateInfo.CurrentVersion).
		Str("latest_version", updateInfo.LatestVersion).
		Msg("Starting update")

	// Determine download URL based on OS and architecture
	downloadURL, err := u.getDownloadURL(updateInfo.LatestVersion)
	if err != nil {
		return fmt.Errorf("failed to determine download URL: %w", err)
	}

	logger.InfoEvent().Str("url", downloadURL).Msg("Downloading update")

	// Download the new binary
	tmpFile, err := u.downloadBinary(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer os.Remove(tmpFile)

	logger.InfoEvent().Msg("Extracting binary")

	// Extract binary from tar.gz
	extractedBinary, err := u.extractBinary(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}
	defer os.Remove(extractedBinary)

	logger.InfoEvent().Msg("Replacing current binary")

	// Replace current binary
	if err := u.replaceBinary(extractedBinary); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	logger.InfoEvent().
		Str("version", updateInfo.LatestVersion).
		Msg("Update completed successfully")

	return nil
}

// getDownloadURL constructs the download URL for the current platform.
func (u *Updater) getDownloadURL(version string) (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Normalize architecture names
	arch := goarch
	if goarch == "amd64" {
		arch = "x86_64"
	} else if goarch == "arm64" {
		arch = "aarch64"
	} else if goarch == "386" {
		arch = "i386"
	}

	// Normalize OS names
	osName := goos
	if goos == "darwin" {
		osName = "darwin"
	} else if goos == "linux" {
		osName = "linux"
	} else if goos == "windows" {
		osName = "windows"
	}

	// Construct filename based on binary name
	// Client: grok_1.0.0_linux_x86_64.tar.gz
	// Server: grok-server_1.0.0_linux_x86_64.tar.gz
	filename := fmt.Sprintf("%s_%s_%s_%s.tar.gz", u.binaryName, version, osName, arch)

	// Construct full URL
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s",
		githubOwner, githubRepo, version, filename)

	return url, nil
}

// downloadBinary downloads the binary to a temporary file.
func (u *Updater) downloadBinary(url string) (string, error) {
	client := &http.Client{
		Timeout: downloadTimeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "grok-update-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	// Copy response body to temp file
	written, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	logger.InfoEvent().
		Int64("bytes", written).
		Msg("Downloaded binary")

	return tmpFile.Name(), nil
}

// extractBinary extracts the binary from the tar.gz archive.
func (u *Updater) extractBinary(tarGzPath string) (string, error) {
	// Open the tar.gz file
	file, err := os.Open(tarGzPath)
	if err != nil {
		return "", fmt.Errorf("failed to open tar.gz: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract the binary
	tmpDir := os.TempDir()
	var extractedPath string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar: %w", err)
		}

		// Look for the binary file
		if header.Typeflag == tar.TypeReg && strings.Contains(header.Name, u.binaryName) {
			extractedPath = filepath.Join(tmpDir, filepath.Base(header.Name))

			outFile, err := os.Create(extractedPath)
			if err != nil {
				return "", fmt.Errorf("failed to create extracted file: %w", err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return "", fmt.Errorf("failed to extract binary: %w", err)
			}
			outFile.Close()

			// Make it executable
			if err := os.Chmod(extractedPath, 0755); err != nil {
				return "", fmt.Errorf("failed to make binary executable: %w", err)
			}

			break
		}
	}

	if extractedPath == "" {
		return "", fmt.Errorf("binary not found in archive")
	}

	return extractedPath, nil
}

// replaceBinary replaces the current binary with the new one.
func (u *Updater) replaceBinary(newBinaryPath string) error {
	// Backup current binary
	backupPath := u.binaryPath + ".bak"
	if err := os.Rename(u.binaryPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Copy new binary to current location
	if err := copyFile(newBinaryPath, u.binaryPath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, u.binaryPath)
		return fmt.Errorf("failed to copy new binary: %w", err)
	}

	// Make it executable
	if err := os.Chmod(u.binaryPath, 0755); err != nil {
		// Restore backup on failure
		os.Remove(u.binaryPath)
		os.Rename(backupPath, u.binaryPath)
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Remove backup
	os.Remove(backupPath)

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

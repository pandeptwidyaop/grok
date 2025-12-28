package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	versionpkg "github.com/pandeptwidyaop/grok/internal/version"
)

func TestNewVersionChecker(t *testing.T) {
	checker := NewVersionChecker("example.com:443")
	if checker == nil {
		t.Fatal("Expected non-nil VersionChecker")
	}
	if checker.serverAddr != "example.com:443" {
		t.Errorf("Expected serverAddr 'example.com:443', got '%s'", checker.serverAddr)
	}
	if checker.httpClient == nil {
		t.Error("Expected non-nil httpClient")
	}
}

func TestGetServerVersion(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse interface{}
		statusCode     int
		expectError    bool
		expectedVer    string
	}{
		{
			name: "successful version fetch",
			serverResponse: versionpkg.Info{
				Version:   "v1.2.0",
				GitCommit: "abc123",
				BuildDate: "2025-01-01",
			},
			statusCode:  http.StatusOK,
			expectError: false,
			expectedVer: "v1.2.0",
		},
		{
			name:           "server error",
			serverResponse: nil,
			statusCode:     http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name:           "invalid json",
			serverResponse: "invalid json",
			statusCode:     http.StatusOK,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.serverResponse != nil {
					if str, ok := tt.serverResponse.(string); ok {
						_, _ = w.Write([]byte(str))
					} else {
						_ = json.NewEncoder(w).Encode(tt.serverResponse)
					}
				}
			}))
			defer server.Close()

			// Extract host from test server URL (remove https://)
			serverAddr := strings.TrimPrefix(server.URL, "https://")

			// Create checker with custom HTTP client that trusts test server
			checker := NewVersionChecker(serverAddr)
			checker.httpClient = server.Client()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			version, err := checker.GetServerVersion(ctx)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if version != tt.expectedVer {
					t.Errorf("Expected version '%s', got '%s'", tt.expectedVer, version)
				}
			}
		})
	}
}

func TestCheckVersion(t *testing.T) {
	// Save original version and restore after test
	originalVersion := versionpkg.Version
	defer func() {
		versionpkg.Version = originalVersion
	}()

	tests := []struct {
		name           string
		clientVersion  string
		serverVersion  string
		expectMismatch bool
	}{
		{
			name:           "versions match",
			clientVersion:  "v1.2.0",
			serverVersion:  "v1.2.0",
			expectMismatch: false,
		},
		{
			name:           "versions mismatch",
			clientVersion:  "v1.1.0",
			serverVersion:  "v1.2.0",
			expectMismatch: true,
		},
		{
			name:           "dev client, release server",
			clientVersion:  "dev",
			serverVersion:  "v1.2.0",
			expectMismatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set client version
			versionpkg.Version = tt.clientVersion

			// Create test server
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(versionpkg.Info{
					Version:   tt.serverVersion,
					GitCommit: "test",
					BuildDate: "test",
				})
			}))
			defer server.Close()

			serverAddr := strings.TrimPrefix(server.URL, "https://")

			checker := NewVersionChecker(serverAddr)
			checker.httpClient = server.Client()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			mismatch, err := checker.CheckVersion(ctx)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if mismatch.ClientVersion != tt.clientVersion {
				t.Errorf("Expected client version '%s', got '%s'", tt.clientVersion, mismatch.ClientVersion)
			}

			if mismatch.ServerVersion != tt.serverVersion {
				t.Errorf("Expected server version '%s', got '%s'", tt.serverVersion, mismatch.ServerVersion)
			}

			if mismatch.Mismatch != tt.expectMismatch {
				t.Errorf("Expected mismatch=%v, got mismatch=%v", tt.expectMismatch, mismatch.Mismatch)
			}
		})
	}
}

func TestDisplayWarning(t *testing.T) {
	tests := []struct {
		name           string
		mismatch       VersionMismatch
		expectOutput   bool
		expectedString string
	}{
		{
			name: "display warning on mismatch",
			mismatch: VersionMismatch{
				ClientVersion: "v1.1.0",
				ServerVersion: "v1.2.0",
				Mismatch:      true,
			},
			expectOutput:   true,
			expectedString: "VERSION MISMATCH WARNING",
		},
		{
			name: "no output when versions match",
			mismatch: VersionMismatch{
				ClientVersion: "v1.2.0",
				ServerVersion: "v1.2.0",
				Mismatch:      false,
			},
			expectOutput: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			tt.mismatch.DisplayWarning()

			// Restore stdout
			_ = w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			if tt.expectOutput {
				if output == "" {
					t.Error("Expected output, got empty string")
				}
				if !strings.Contains(output, tt.expectedString) {
					t.Errorf("Expected output to contain '%s', got:\n%s", tt.expectedString, output)
				}
				if !strings.Contains(output, tt.mismatch.ClientVersion) {
					t.Errorf("Expected output to contain client version '%s'", tt.mismatch.ClientVersion)
				}
				if !strings.Contains(output, tt.mismatch.ServerVersion) {
					t.Errorf("Expected output to contain server version '%s'", tt.mismatch.ServerVersion)
				}
			} else {
				if output != "" {
					t.Errorf("Expected no output, got: %s", output)
				}
			}
		})
	}
}

func TestCheckVersionWithTimeout(t *testing.T) {
	// Create slow server that exceeds context timeout
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverAddr := strings.TrimPrefix(server.URL, "https://")
	checker := NewVersionChecker(serverAddr)
	checker.httpClient = server.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := checker.CheckVersion(ctx)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadDefaults tests loading configuration with defaults
func TestLoadDefaults(t *testing.T) {
	// Create a temporary directory for config
	tmpDir := t.TempDir()

	// Set HOME to temp dir to avoid touching real config
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Load config (no file exists, should use defaults)
	cfg, err := Load("")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify defaults
	assert.Equal(t, "localhost:4443", cfg.Server.Addr)
	assert.False(t, cfg.Server.TLS)
	assert.Empty(t, cfg.Server.TLSCertFile)
	assert.False(t, cfg.Server.TLSInsecure)
	assert.Empty(t, cfg.Server.TLSServerName)

	assert.Empty(t, cfg.Auth.Token)

	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)

	assert.True(t, cfg.Reconnect.Enabled)
	assert.Equal(t, 1, cfg.Reconnect.InitialDelay)
	assert.Equal(t, 30, cfg.Reconnect.MaxDelay)
	assert.Equal(t, 2, cfg.Reconnect.BackoffFactor)
	assert.Equal(t, 0, cfg.Reconnect.MaxAttempts)
}

// TestLoadFromFile tests loading configuration from a file
func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create a test config file
	configContent := `
server:
  addr: "tunnel.example.com:443"
  tls: true
  tls_cert_file: "/path/to/cert.pem"
  tls_insecure: false
  tls_server_name: "tunnel.example.com"

auth:
  token: "grok_test123456"

logging:
  level: "debug"
  format: "json"

reconnect:
  enabled: false
  initial_delay: 5
  max_delay: 60
  backoff_factor: 3
  max_attempts: 10
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config from file
	cfg, err := Load(configFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify values from file
	assert.Equal(t, "tunnel.example.com:443", cfg.Server.Addr)
	assert.True(t, cfg.Server.TLS)
	assert.Equal(t, "/path/to/cert.pem", cfg.Server.TLSCertFile)
	assert.False(t, cfg.Server.TLSInsecure)
	assert.Equal(t, "tunnel.example.com", cfg.Server.TLSServerName)

	assert.Equal(t, "grok_test123456", cfg.Auth.Token)

	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)

	assert.False(t, cfg.Reconnect.Enabled)
	assert.Equal(t, 5, cfg.Reconnect.InitialDelay)
	assert.Equal(t, 60, cfg.Reconnect.MaxDelay)
	assert.Equal(t, 3, cfg.Reconnect.BackoffFactor)
	assert.Equal(t, 10, cfg.Reconnect.MaxAttempts)
}

// TestLoadPartialConfig tests loading with partial configuration
func TestLoadPartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create a test config file with only some fields
	configContent := `
server:
  addr: "custom.example.com:8443"
  tls: true

auth:
  token: "grok_partial"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config from file
	cfg, err := Load(configFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify values from file
	assert.Equal(t, "custom.example.com:8443", cfg.Server.Addr)
	assert.True(t, cfg.Server.TLS)
	assert.Equal(t, "grok_partial", cfg.Auth.Token)

	// Verify defaults for unspecified fields
	assert.Empty(t, cfg.Server.TLSCertFile)
	assert.False(t, cfg.Server.TLSInsecure)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.True(t, cfg.Reconnect.Enabled)
}

// TestLoadInvalidConfigFile tests loading from an invalid config file
func TestLoadInvalidConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")

	// Create an invalid config file
	configContent := `
this is not valid yaml: [
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config should fail
	_, err = Load(configFile)
	assert.Error(t, err)
}

// TestSaveToken tests saving auth token
func TestSaveToken(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Save token
	err := SaveToken("grok_newsavedtoken")
	require.NoError(t, err)

	// Verify config file was created
	configFile := filepath.Join(tmpDir, ".grok", "config.yaml")
	assert.FileExists(t, configFile)

	// Load config and verify token
	cfg, err := Load(configFile)
	require.NoError(t, err)
	assert.Equal(t, "grok_newsavedtoken", cfg.Auth.Token)
}

// TestSaveTokenMultipleTimes tests saving token multiple times
func TestSaveTokenMultipleTimes(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Save first token
	err := SaveToken("grok_first")
	require.NoError(t, err)

	// Save second token (should overwrite)
	err = SaveToken("grok_second")
	require.NoError(t, err)

	// Load config and verify latest token
	configFile := filepath.Join(tmpDir, ".grok", "config.yaml")
	cfg, err := Load(configFile)
	require.NoError(t, err)
	assert.Equal(t, "grok_second", cfg.Auth.Token)
}

// TestSaveServer tests saving server address
func TestSaveServer(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Save server address
	err := SaveServer("custom.server.com:9443")
	require.NoError(t, err)

	// Load config and verify server
	configFile := filepath.Join(tmpDir, ".grok", "config.yaml")
	cfg, err := Load(configFile)
	require.NoError(t, err)
	assert.Equal(t, "custom.server.com:9443", cfg.Server.Addr)
}

// TestSetTLSCert tests setting TLS certificate
func TestSetTLSCert(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Set TLS cert
	err := SetTLSCert("/path/to/my/cert.pem")
	require.NoError(t, err)

	// Load config and verify TLS settings
	configFile := filepath.Join(tmpDir, ".grok", "config.yaml")
	cfg, err := Load(configFile)
	require.NoError(t, err)
	assert.True(t, cfg.Server.TLS)
	assert.Equal(t, "/path/to/my/cert.pem", cfg.Server.TLSCertFile)
	assert.False(t, cfg.Server.TLSInsecure)
}

// TestSetTLSInsecure tests setting TLS insecure mode
func TestSetTLSInsecure(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Set TLS insecure
	err := SetTLSInsecure(true)
	require.NoError(t, err)

	// Load config and verify TLS settings
	configFile := filepath.Join(tmpDir, ".grok", "config.yaml")
	cfg, err := Load(configFile)
	require.NoError(t, err)
	assert.True(t, cfg.Server.TLS)
	assert.True(t, cfg.Server.TLSInsecure)
	assert.Empty(t, cfg.Server.TLSCertFile) // Cert file should be cleared
}

// TestEnableTLS tests enabling TLS
func TestEnableTLS(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Enable TLS
	err := EnableTLS()
	require.NoError(t, err)

	// Load config and verify TLS settings
	configFile := filepath.Join(tmpDir, ".grok", "config.yaml")
	cfg, err := Load(configFile)
	require.NoError(t, err)
	assert.True(t, cfg.Server.TLS)
	assert.Empty(t, cfg.Server.TLSCertFile)
	assert.False(t, cfg.Server.TLSInsecure)
}

// TestDisableTLS tests disabling TLS
func TestDisableTLS(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// First enable TLS
	err := EnableTLS()
	require.NoError(t, err)

	// Then disable TLS
	err = DisableTLS()
	require.NoError(t, err)

	// Load config and verify TLS is disabled
	configFile := filepath.Join(tmpDir, ".grok", "config.yaml")
	cfg, err := Load(configFile)
	require.NoError(t, err)
	assert.False(t, cfg.Server.TLS)
}

// TestConfigPersistence tests that config changes persist
func TestConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Save multiple settings
	err := SaveToken("grok_persist")
	require.NoError(t, err)

	err = SaveServer("persist.server.com:443")
	require.NoError(t, err)

	err = SetTLSCert("/path/to/cert.pem")
	require.NoError(t, err)

	// Load config and verify all settings were persisted
	configFile := filepath.Join(tmpDir, ".grok", "config.yaml")
	cfg, err := Load(configFile)
	require.NoError(t, err)

	assert.Equal(t, "grok_persist", cfg.Auth.Token)
	assert.Equal(t, "persist.server.com:443", cfg.Server.Addr)
	assert.True(t, cfg.Server.TLS)
	assert.Equal(t, "/path/to/cert.pem", cfg.Server.TLSCertFile)
}

// TestConfigDefaultDirectory tests that config directory is created
func TestConfigDefaultDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Save a setting (should create directory)
	err := SaveToken("grok_testdir")
	require.NoError(t, err)

	// Verify directory was created with correct permissions
	configDir := filepath.Join(tmpDir, ".grok")
	info, err := os.Stat(configDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestLoadFromMultipleSources tests config loading from file
func TestLoadFromMultipleSources(t *testing.T) {
	tmpDir := t.TempDir()

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create a config file
	configFile := filepath.Join(tmpDir, ".grok", "config.yaml")
	err := os.MkdirAll(filepath.Dir(configFile), 0755)
	require.NoError(t, err)

	configContent := `
server:
  addr: "file.server.com:443"
  tls: true
auth:
  token: "grok_fromfile"
`
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config from file
	cfg, err := Load("")
	require.NoError(t, err)

	// Values from file
	assert.Equal(t, "grok_fromfile", cfg.Auth.Token)
	assert.Equal(t, "file.server.com:443", cfg.Server.Addr)
	assert.True(t, cfg.Server.TLS)

	// Defaults for unspecified values
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.True(t, cfg.Reconnect.Enabled)
}

// BenchmarkLoad benchmarks config loading
func BenchmarkLoad(b *testing.B) {
	tmpDir := b.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  addr: "bench.server.com:443"
  tls: true
auth:
  token: "grok_bench"
`
	os.WriteFile(configFile, []byte(configContent), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Load(configFile)
	}
}

// BenchmarkSaveToken benchmarks token saving
func BenchmarkSaveToken(b *testing.B) {
	tmpDir := b.TempDir()

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SaveToken("grok_bench")
	}
}

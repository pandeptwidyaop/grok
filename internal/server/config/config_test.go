package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoad_ValidConfig tests loading a valid configuration
func TestLoad_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  grpc_port: 4443
  http_port: 80
  https_port: 443
  api_port: 4040
  domain: "example.com"
  tcp_port_start: 10000
  tcp_port_end: 20000
  allowed_origins:
    - "http://localhost:5173"

database:
  driver: "sqlite"
  database: "test.db"

tls:
  auto_cert: true
  cert_dir: "/tmp/certs"

auth:
  jwt_secret: "this-is-a-very-secure-jwt-secret-with-at-least-32-characters"
  admin_username: "admin"
  admin_password: "secure_password"

tunnels:
  max_per_user: 10
  idle_timeout: "15m"
  heartbeat_interval: "30s"

logging:
  level: "debug"
  format: "json"
  output: "stdout"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify server config
	assert.Equal(t, 4443, cfg.Server.GRPCPort)
	assert.Equal(t, 80, cfg.Server.HTTPPort)
	assert.Equal(t, 443, cfg.Server.HTTPSPort)
	assert.Equal(t, 4040, cfg.Server.APIPort)
	assert.Equal(t, "example.com", cfg.Server.Domain)
	assert.Equal(t, 10000, cfg.Server.TCPPortStart)
	assert.Equal(t, 20000, cfg.Server.TCPPortEnd)
	assert.Contains(t, cfg.Server.AllowedOrigins, "http://localhost:5173")

	// Verify database config
	assert.Equal(t, "sqlite", cfg.Database.Driver)
	assert.Equal(t, "test.db", cfg.Database.Database)

	// Verify TLS config
	assert.True(t, cfg.TLS.AutoCert)
	assert.Equal(t, "/tmp/certs", cfg.TLS.CertDir)

	// Verify auth config
	assert.Equal(t, "this-is-a-very-secure-jwt-secret-with-at-least-32-characters", cfg.Auth.JWTSecret)
	assert.Equal(t, "admin", cfg.Auth.AdminUsername)
	assert.Equal(t, "secure_password", cfg.Auth.AdminPassword)

	// Verify tunnels config
	assert.Equal(t, 10, cfg.Tunnels.MaxPerUser)
	assert.Equal(t, "15m", cfg.Tunnels.IdleTimeout)
	assert.Equal(t, "30s", cfg.Tunnels.HeartbeatInterval)

	// Verify logging config
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "stdout", cfg.Logging.Output)
}

// TestLoad_WithDefaults tests loading config with defaults
func TestLoad_WithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Minimal config with only required fields
	configContent := `
auth:
  jwt_secret: "this-is-a-very-secure-jwt-secret-with-at-least-32-characters-long"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify defaults are applied
	assert.Equal(t, 4443, cfg.Server.GRPCPort)
	assert.Equal(t, 80, cfg.Server.HTTPPort)
	assert.Equal(t, "grok.io", cfg.Server.Domain)
	assert.Equal(t, "sqlite", cfg.Database.Driver)
	assert.Equal(t, "grok.db", cfg.Database.Database)
	assert.Equal(t, 5, cfg.Tunnels.MaxPerUser)
	assert.Equal(t, "info", cfg.Logging.Level)
}

// TestLoad_MissingJWTSecret tests loading config without JWT secret
func TestLoad_MissingJWTSecret(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  domain: "example.com"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "jwt_secret is required")
}

// TestLoad_DefaultJWTSecret tests loading config with default JWT secret
func TestLoad_DefaultJWTSecret(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
auth:
  jwt_secret: "change-this-to-a-secure-random-string"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "must be changed from default value")
}

// TestLoad_ShortJWTSecret tests loading config with short JWT secret
func TestLoad_ShortJWTSecret(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
auth:
  jwt_secret: "short"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "at least 32 characters")
}

// TestLoad_InvalidYAML tests loading invalid YAML
func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
this is not valid yaml: [
  - broken
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

// TestLoad_NonExistentFile tests loading non-existent config file
func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

// TestLoad_PostgreSQLConfig tests loading PostgreSQL configuration
func TestLoad_PostgreSQLConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
auth:
  jwt_secret: "this-is-a-very-secure-jwt-secret-with-at-least-32-characters"

database:
  driver: "postgres"
  host: "db.example.com"
  port: 5432
  database: "grok_production"
  username: "grok_user"
  password: "secure_db_password"
  ssl_mode: "require"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	require.NoError(t, err)

	assert.Equal(t, "postgres", cfg.Database.Driver)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "grok_production", cfg.Database.Database)
	assert.Equal(t, "grok_user", cfg.Database.Username)
	assert.Equal(t, "secure_db_password", cfg.Database.Password)
	assert.Equal(t, "require", cfg.Database.SSLMode)
}

// TestLoad_TLSConfig tests loading TLS configuration
func TestLoad_TLSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
auth:
  jwt_secret: "this-is-a-very-secure-jwt-secret-with-at-least-32-characters"

tls:
  auto_cert: false
  cert_file: "/etc/grok/tls/cert.pem"
  key_file: "/etc/grok/tls/key.pem"
  email: "admin@example.com"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	require.NoError(t, err)

	assert.False(t, cfg.TLS.AutoCert)
	assert.Equal(t, "/etc/grok/tls/cert.pem", cfg.TLS.CertFile)
	assert.Equal(t, "/etc/grok/tls/key.pem", cfg.TLS.KeyFile)
	assert.Equal(t, "admin@example.com", cfg.TLS.Email)
}

// TestLoad_LoggingConfig tests loading logging configuration
func TestLoad_LoggingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
auth:
  jwt_secret: "this-is-a-very-secure-jwt-secret-with-at-least-32-characters"

logging:
  level: "debug"
  format: "text"
  output: "file"
  file: "/var/log/grok/server.log"
  sql_log_level: "info"
  http_log_level: "info"
  sse_log_level: "info"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	require.NoError(t, err)

	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
	assert.Equal(t, "file", cfg.Logging.Output)
	assert.Equal(t, "/var/log/grok/server.log", cfg.Logging.File)
	assert.Equal(t, "info", cfg.Logging.SQLLogLevel)
	assert.Equal(t, "info", cfg.Logging.HTTPLogLevel)
	assert.Equal(t, "info", cfg.Logging.SSELogLevel)
}

// TestLoad_TunnelsConfig tests loading tunnels configuration
func TestLoad_TunnelsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
auth:
  jwt_secret: "this-is-a-very-secure-jwt-secret-with-at-least-32-characters"

tunnels:
  max_per_user: 20
  idle_timeout: "30m"
  heartbeat_interval: "60s"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	require.NoError(t, err)

	assert.Equal(t, 20, cfg.Tunnels.MaxPerUser)
	assert.Equal(t, "30m", cfg.Tunnels.IdleTimeout)
	assert.Equal(t, "60s", cfg.Tunnels.HeartbeatInterval)
}

// TestLoad_AllowedOrigins tests loading CORS allowed origins
func TestLoad_AllowedOrigins(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
auth:
  jwt_secret: "this-is-a-very-secure-jwt-secret-with-at-least-32-characters"

server:
  allowed_origins:
    - "https://example.com"
    - "https://app.example.com"
    - "http://localhost:3000"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	require.NoError(t, err)

	assert.Len(t, cfg.Server.AllowedOrigins, 3)
	assert.Contains(t, cfg.Server.AllowedOrigins, "https://example.com")
	assert.Contains(t, cfg.Server.AllowedOrigins, "https://app.example.com")
	assert.Contains(t, cfg.Server.AllowedOrigins, "http://localhost:3000")
}

// TestLoad_EnvironmentVariables tests environment variable override
func TestLoad_EnvironmentVariables(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
auth:
  jwt_secret: "this-is-a-very-secure-jwt-secret-with-at-least-32-characters"

server:
  domain: "config-file-domain.com"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set environment variable
	os.Setenv("GROK_SERVER_DOMAIN", "env-override-domain.com")
	defer os.Unsetenv("GROK_SERVER_DOMAIN")

	cfg, err := Load(configFile)
	require.NoError(t, err)

	// Env var should override config file
	assert.Equal(t, "env-override-domain.com", cfg.Server.Domain)
}

// TestLoad_EnvironmentVariableJWTSecret tests JWT secret from env var
func TestLoad_EnvironmentVariableJWTSecret(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Config with a JWT secret that will be overridden by env var
	configContent := `
server:
  domain: "example.com"
auth:
  jwt_secret: "config-file-jwt-secret-32-chars-minimum!!"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set JWT secret via environment variable (should override config file)
	os.Setenv("GROK_AUTH_JWT_SECRET", "environment-jwt-secret-that-is-long-enough-to-pass-validation")
	defer os.Unsetenv("GROK_AUTH_JWT_SECRET")

	cfg, err := Load(configFile)
	require.NoError(t, err)

	// Environment variable should override config file value
	assert.Equal(t, "environment-jwt-secret-that-is-long-enough-to-pass-validation", cfg.Auth.JWTSecret)
}

// TestValidateConfig tests config validation
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			cfg: &Config{
				Auth: AuthConfig{
					JWTSecret: "this-is-a-very-secure-jwt-secret-with-at-least-32-characters",
				},
			},
			expectError: false,
		},
		{
			name: "empty JWT secret",
			cfg: &Config{
				Auth: AuthConfig{
					JWTSecret: "",
				},
			},
			expectError: true,
			errorMsg:    "jwt_secret is required",
		},
		{
			name: "default JWT secret",
			cfg: &Config{
				Auth: AuthConfig{
					JWTSecret: "change-this-to-a-secure-random-string",
				},
			},
			expectError: true,
			errorMsg:    "must be changed from default value",
		},
		{
			name: "short JWT secret",
			cfg: &Config{
				Auth: AuthConfig{
					JWTSecret: "tooshort",
				},
			},
			expectError: true,
			errorMsg:    "at least 32 characters",
		},
		{
			name: "exactly 32 characters",
			cfg: &Config{
				Auth: AuthConfig{
					JWTSecret: "12345678901234567890123456789012", // exactly 32
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestLoad_CompleteProductionConfig tests complete production-like config
func TestLoad_CompleteProductionConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "production.yaml")

	configContent := `
server:
  grpc_port: 4443
  http_port: 80
  https_port: 443
  api_port: 4040
  domain: "tunnel.production.com"
  tcp_port_start: 10000
  tcp_port_end: 20000
  allowed_origins:
    - "https://dashboard.production.com"
    - "https://api.production.com"

database:
  driver: "postgres"
  host: "prod-db.internal"
  port: 5432
  database: "grok_prod"
  username: "grok_prod_user"
  password: "very_secure_password_123"
  ssl_mode: "require"

tls:
  auto_cert: false
  cert_file: "/etc/tls/production.crt"
  key_file: "/etc/tls/production.key"
  email: "ops@production.com"

auth:
  jwt_secret: "production-jwt-secret-with-high-entropy-and-at-least-32-chars"
  admin_username: "prod_admin"
  admin_password: "super_secure_admin_password"

tunnels:
  max_per_user: 50
  idle_timeout: "1h"
  heartbeat_interval: "45s"

logging:
  level: "info"
  format: "json"
  output: "file"
  file: "/var/log/grok/server.log"
  sql_log_level: "error"
  http_log_level: "info"
  sse_log_level: "warn"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify all sections are loaded correctly
	assert.Equal(t, "tunnel.production.com", cfg.Server.Domain)
	assert.Equal(t, "postgres", cfg.Database.Driver)
	assert.Equal(t, "prod-db.internal", cfg.Database.Host)
	assert.False(t, cfg.TLS.AutoCert)
	assert.Equal(t, 50, cfg.Tunnels.MaxPerUser)
	assert.Equal(t, "info", cfg.Logging.Level)
}

// BenchmarkLoad benchmarks config loading
func BenchmarkLoad(b *testing.B) {
	tmpDir := b.TempDir()
	configFile := filepath.Join(tmpDir, "bench.yaml")

	configContent := `
auth:
  jwt_secret: "benchmark-jwt-secret-that-is-long-enough-32-chars"

server:
  domain: "bench.example.com"
`

	os.WriteFile(configFile, []byte(configContent), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Load(configFile)
	}
}

// BenchmarkValidateConfig benchmarks config validation
func BenchmarkValidateConfig(b *testing.B) {
	cfg := &Config{
		Auth: AuthConfig{
			JWTSecret: "benchmark-jwt-secret-that-is-long-enough-32-chars",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validateConfig(cfg)
	}
}

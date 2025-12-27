package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the client configuration.
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Reconnect ReconnectConfig `mapstructure:"reconnect"`
	Dashboard DashboardConfig `mapstructure:"dashboard"`
}

// ServerConfig holds server connection settings.
type ServerConfig struct {
	Addr          string `mapstructure:"addr"`
	TLS           bool   `mapstructure:"tls"`
	TLSCertFile   string `mapstructure:"tls_cert_file"`   // Optional: custom CA cert
	TLSInsecure   bool   `mapstructure:"tls_insecure"`    // Skip cert verification (dev only)
	TLSServerName string `mapstructure:"tls_server_name"` // Override server name for verification
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Token string `mapstructure:"token"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// ReconnectConfig holds reconnection settings.
type ReconnectConfig struct {
	Enabled       bool `mapstructure:"enabled"`
	InitialDelay  int  `mapstructure:"initial_delay"`  // seconds
	MaxDelay      int  `mapstructure:"max_delay"`      // seconds
	BackoffFactor int  `mapstructure:"backoff_factor"` // multiplier
	MaxAttempts   int  `mapstructure:"max_attempts"`   // 0 = infinite
}

// DashboardConfig holds dashboard settings.
type DashboardConfig struct {
	Enabled     bool  `mapstructure:"enabled"`       // Enable dashboard
	Port        int   `mapstructure:"port"`          // Dashboard port (default: 4041)
	MaxRequests int   `mapstructure:"max_requests"`  // Max requests to store (default: 1000)
	MaxBodySize int64 `mapstructure:"max_body_size"` // Max body size to capture in bytes (default: 102400 = 100KB)
}

// Load loads configuration from file.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults first
	setDefaults(v)

	// If config path is provided, use it
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Look for config in default locations
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home dir: %w", err)
		}

		configDir := filepath.Join(home, ".grok")
		v.AddConfigPath(configDir)
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	// Read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found, use defaults
	}

	// Read from environment variables
	v.SetEnvPrefix("GROK")
	v.AutomaticEnv()

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.addr", "localhost:4443")
	v.SetDefault("server.tls", false)
	v.SetDefault("server.tls_cert_file", "")
	v.SetDefault("server.tls_insecure", false)
	v.SetDefault("server.tls_server_name", "")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "text")

	// Reconnect defaults
	v.SetDefault("reconnect.enabled", true)
	v.SetDefault("reconnect.initial_delay", 1)
	v.SetDefault("reconnect.max_delay", 30)
	v.SetDefault("reconnect.backoff_factor", 2)
	v.SetDefault("reconnect.max_attempts", 0) // infinite

	// Dashboard defaults
	v.SetDefault("dashboard.enabled", true)
	v.SetDefault("dashboard.port", 4041)
	v.SetDefault("dashboard.max_requests", 1000)
	v.SetDefault("dashboard.max_body_size", 102400) // 100KB
}

// SaveToken saves auth token to config file.
func SaveToken(token string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}

	configDir := filepath.Join(home, ".grok")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory if not exists
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configFile)

	// Try to read existing config
	_ = v.ReadInConfig()

	// Set token
	v.Set("auth.token", token)

	// Write config
	if err := v.WriteConfig(); err != nil {
		// If file doesn't exist, create it
		if err := v.SafeWriteConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	return nil
}

// SaveServer saves server address to config file.
func SaveServer(addr string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}

	configDir := filepath.Join(home, ".grok")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory if not exists
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configFile)

	// Try to read existing config
	_ = v.ReadInConfig()

	// Set server address
	v.Set("server.addr", addr)

	// Write config
	if err := v.WriteConfig(); err != nil {
		// If file doesn't exist, create it
		if err := v.SafeWriteConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	return nil
}

// SetTLSCert sets TLS certificate file and enables TLS.
func SetTLSCert(certPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}

	configDir := filepath.Join(home, ".grok")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory if not exists
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configFile)

	// Try to read existing config
	_ = v.ReadInConfig()

	// Enable TLS and set cert file
	v.Set("server.tls", true)
	v.Set("server.tls_cert_file", certPath)
	v.Set("server.tls_insecure", false) // Disable insecure when using cert

	// Write config
	if err := v.WriteConfig(); err != nil {
		// If file doesn't exist, create it
		if err := v.SafeWriteConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	return nil
}

// SetTLSInsecure sets TLS insecure mode.
func SetTLSInsecure(insecure bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}

	configDir := filepath.Join(home, ".grok")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory if not exists
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configFile)

	// Try to read existing config
	_ = v.ReadInConfig()

	// Enable TLS and set insecure mode
	v.Set("server.tls", true)
	v.Set("server.tls_insecure", insecure)
	if insecure {
		// Clear cert file when using insecure mode
		v.Set("server.tls_cert_file", "")
	}

	// Write config
	if err := v.WriteConfig(); err != nil {
		// If file doesn't exist, create it
		if err := v.SafeWriteConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	return nil
}

// EnableTLS enables TLS with system CA pool.
func EnableTLS() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}

	configDir := filepath.Join(home, ".grok")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory if not exists
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configFile)

	// Try to read existing config
	_ = v.ReadInConfig()

	// Enable TLS with system CA (no custom cert)
	v.Set("server.tls", true)
	v.Set("server.tls_cert_file", "")
	v.Set("server.tls_insecure", false)

	// Write config
	if err := v.WriteConfig(); err != nil {
		// If file doesn't exist, create it
		if err := v.SafeWriteConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	return nil
}

// DisableTLS disables TLS.
func DisableTLS() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}

	configDir := filepath.Join(home, ".grok")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory if not exists
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configFile)

	// Try to read existing config
	_ = v.ReadInConfig()

	// Disable TLS
	v.Set("server.tls", false)

	// Write config
	if err := v.WriteConfig(); err != nil {
		// If file doesn't exist, create it
		if err := v.SafeWriteConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	return nil
}

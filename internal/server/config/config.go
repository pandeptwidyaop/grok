package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the server configuration.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	TLS      TLSConfig      `mapstructure:"tls"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Tunnels  TunnelsConfig  `mapstructure:"tunnels"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// ServerConfig holds server settings.
type ServerConfig struct {
	GRPCPort       int      `mapstructure:"grpc_port"`
	HTTPPort       int      `mapstructure:"http_port"`
	HTTPSPort      int      `mapstructure:"https_port"`
	APIPort        int      `mapstructure:"api_port"`
	Domain         string   `mapstructure:"domain"`
	TCPPortStart   int      `mapstructure:"tcp_port_start"`
	TCPPortEnd     int      `mapstructure:"tcp_port_end"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

// TLSConfig holds TLS settings.
type TLSConfig struct {
	AutoCert bool   `mapstructure:"auto_cert"`
	CertDir  string `mapstructure:"cert_dir"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	Email    string `mapstructure:"email"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret     string `mapstructure:"jwt_secret"`
	AdminUsername string `mapstructure:"admin_username"`
	AdminPassword string `mapstructure:"admin_password"`
}

// TunnelsConfig holds tunnel settings.
type TunnelsConfig struct {
	MaxPerUser        int    `mapstructure:"max_per_user"`
	IdleTimeout       string `mapstructure:"idle_timeout"`
	HeartbeatInterval string `mapstructure:"heartbeat_interval"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level        string `mapstructure:"level"`
	Format       string `mapstructure:"format"`
	Output       string `mapstructure:"output"`
	File         string `mapstructure:"file"`
	SQLLogLevel  string `mapstructure:"sql_log_level"`  // GORM SQL query log level: silent, error, warn, info
	HTTPLogLevel string `mapstructure:"http_log_level"` // HTTP request log level: silent, error, warn, info
	SSELogLevel  string `mapstructure:"sse_log_level"`  // SSE connection log level: silent, warn, info
}

// Load loads configuration from file.
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Enable environment variable support
	// Environment variables like GROK_AUTH_JWT_SECRET override config file values
	viper.SetEnvPrefix("GROK")
	viper.AutomaticEnv()
	// Replace dots with underscores in env vars (auth.jwt_secret -> AUTH_JWT_SECRET)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate critical security settings
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// validateConfig ensures critical security settings are properly configured.
func validateConfig(cfg *Config) error {
	// JWT secret must be set and not be the default value
	if cfg.Auth.JWTSecret == "" {
		return fmt.Errorf("auth.jwt_secret is required (set via config file or GROK_AUTH_JWT_SECRET environment variable)")
	}
	if cfg.Auth.JWTSecret == "change-this-to-a-secure-random-string" {
		return fmt.Errorf("auth.jwt_secret must be changed from default value (use a secure random string or set GROK_AUTH_JWT_SECRET)")
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		return fmt.Errorf("auth.jwt_secret must be at least 32 characters long for security")
	}

	return nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.grpc_port", 4443)
	viper.SetDefault("server.http_port", 80)
	viper.SetDefault("server.https_port", 443)
	viper.SetDefault("server.api_port", 4040)
	viper.SetDefault("server.domain", "grok.io")
	viper.SetDefault("server.tcp_port_start", 10000)
	viper.SetDefault("server.tcp_port_end", 20000)
	// CORS defaults - localhost for development
	viper.SetDefault("server.allowed_origins", []string{
		"http://localhost:5173", // Vite dev server
		"http://localhost:4040", // Dashboard API port
	})

	// Database defaults (SQLite for easier local development)
	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.database", "grok.db")
	// PostgreSQL defaults (if driver is set to postgres)
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.username", "grok")
	viper.SetDefault("database.ssl_mode", "disable")

	// TLS defaults
	viper.SetDefault("tls.auto_cert", true)
	viper.SetDefault("tls.cert_dir", "/var/lib/grok/certs")

	// Auth defaults
	viper.SetDefault("auth.admin_username", "admin")
	viper.SetDefault("auth.admin_password", "admin123") // Change in production!

	// Tunnel defaults
	viper.SetDefault("tunnels.max_per_user", 5)
	viper.SetDefault("tunnels.idle_timeout", "10m")
	viper.SetDefault("tunnels.heartbeat_interval", "30s")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.sql_log_level", "silent") // silent = no SQL query logs (use "info" to enable)
	viper.SetDefault("logging.http_log_level", "error") // error = only log errors (use "info" to log all requests)
	viper.SetDefault("logging.sse_log_level", "warn")   // warn = only log disconnects (use "info" to log all connects)
}

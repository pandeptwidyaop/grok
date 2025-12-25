package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config represents the server configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	TLS      TLSConfig      `mapstructure:"tls"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Tunnels  TunnelsConfig  `mapstructure:"tunnels"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// ServerConfig holds server settings
type ServerConfig struct {
	GRPCPort      int    `mapstructure:"grpc_port"`
	HTTPPort      int    `mapstructure:"http_port"`
	HTTPSPort     int    `mapstructure:"https_port"`
	APIPort       int    `mapstructure:"api_port"`
	Domain        string `mapstructure:"domain"`
}

// DatabaseConfig holds database settings
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

// TLSConfig holds TLS settings
type TLSConfig struct {
	AutoCert bool   `mapstructure:"auto_cert"`
	CertDir  string `mapstructure:"cert_dir"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	Email    string `mapstructure:"email"`
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	JWTSecret     string `mapstructure:"jwt_secret"`
	AdminUsername string `mapstructure:"admin_username"`
	AdminPassword string `mapstructure:"admin_password"`
}

// TunnelsConfig holds tunnel settings
type TunnelsConfig struct {
	MaxPerUser        int    `mapstructure:"max_per_user"`
	IdleTimeout       string `mapstructure:"idle_timeout"`
	HeartbeatInterval string `mapstructure:"heartbeat_interval"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
	File   string `mapstructure:"file"`
}

// Load loads configuration from file
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

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

	return &cfg, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.grpc_port", 4443)
	viper.SetDefault("server.http_port", 80)
	viper.SetDefault("server.https_port", 443)
	viper.SetDefault("server.api_port", 4040)
	viper.SetDefault("server.domain", "grok.io")

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
}

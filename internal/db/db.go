package db

import (
	"fmt"
	"strings"

	"github.com/pandeptwidyaop/grok/internal/db/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds database configuration
type Config struct {
	Driver   string // "postgres" or "sqlite"
	Host     string // for postgres
	Port     int    // for postgres
	Database string // database name for postgres, file path for sqlite
	Username string // for postgres
	Password string // for postgres
	SSLMode  string // for postgres
}

// Connect establishes a connection to the database
func Connect(cfg Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch strings.ToLower(cfg.Driver) {
	case "sqlite":
		// SQLite connection
		// cfg.Database should be file path, e.g., "grok.db" or ":memory:" for in-memory
		dialector = sqlite.Open(cfg.Database)

	case "postgres", "postgresql":
		// PostgreSQL connection
		dsn := fmt.Sprintf(
			"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.Database, cfg.Username, cfg.Password, cfg.SSLMode,
		)
		dialector = postgres.Open(dsn)

	default:
		return nil, fmt.Errorf("unsupported database driver: %s (supported: sqlite, postgres)", cfg.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// AutoMigrate runs automatic migrations for all models
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.AuthToken{},
		&models.Domain{},
		&models.Tunnel{},
		&models.RequestLog{},
	)
}

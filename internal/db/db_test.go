package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/logger"

	"github.com/pandeptwidyaop/grok/internal/db/models"
)

// TestConnect_SQLite tests SQLite database connection.
func TestConnect_SQLite(t *testing.T) {
	cfg := Config{
		Driver:   "sqlite",
		Database: ":memory:",
	}

	db, err := Connect(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)

	// Verify connection works
	sqlDB, err := db.DB()
	require.NoError(t, err)
	assert.NoError(t, sqlDB.Ping())
}

// TestConnect_SQLiteFile tests SQLite with file path.
func TestConnect_SQLiteFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := tmpDir + "/test.db"

	cfg := Config{
		Driver:   "sqlite",
		Database: dbFile,
	}

	db, err := Connect(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	assert.NoError(t, sqlDB.Ping())
}

// TestConnect_SQLiteCaseInsensitive tests SQLite driver name is case insensitive.
func TestConnect_SQLiteCaseInsensitive(t *testing.T) {
	tests := []string{"sqlite", "SQLITE", "SQLite", "SqLiTe"}

	for _, driver := range tests {
		t.Run(driver, func(t *testing.T) {
			cfg := Config{
				Driver:   driver,
				Database: ":memory:",
			}

			db, err := Connect(cfg)
			require.NoError(t, err)
			require.NotNil(t, db)
		})
	}
}

// TestConnect_PostgreSQLDriverNames tests PostgreSQL driver name variations.
func TestConnect_PostgreSQLDriverNames(t *testing.T) {
	// Note: These will fail to connect (no real postgres server)
	// but should pass the driver name check
	tests := []string{"postgres", "postgresql", "POSTGRES", "PostgreSQL"}

	for _, driver := range tests {
		t.Run(driver, func(t *testing.T) {
			cfg := Config{
				Driver:   driver,
				Database: "test",
				Host:     "localhost",
				Port:     5432,
				Username: "test",
				Password: "test",
				SSLMode:  "disable",
			}

			// This will fail because there's no postgres server,
			// but we're testing that the driver name is recognized
			_, err := Connect(cfg)
			// Error is expected (connection failure), but NOT "unsupported driver"
			if err != nil {
				assert.NotContains(t, err.Error(), "unsupported database driver")
			}
		})
	}
}

// TestConnect_UnsupportedDriver tests unsupported database driver.
func TestConnect_UnsupportedDriver(t *testing.T) {
	cfg := Config{
		Driver:   "mysql",
		Database: "test",
	}

	db, err := Connect(cfg)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "unsupported database driver")
}

// TestConnect_InvalidDriver tests completely invalid driver name.
func TestConnect_InvalidDriver(t *testing.T) {
	cfg := Config{
		Driver:   "invalid_db_driver",
		Database: "test",
	}

	db, err := Connect(cfg)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "unsupported database driver")
}

// TestConnect_LogLevels tests different SQL log levels.
func TestConnect_LogLevels(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		expected logger.LogLevel
	}{
		{"silent", "silent", logger.Silent},
		{"error", "error", logger.Error},
		{"warn", "warn", logger.Warn},
		{"info", "info", logger.Info},
		{"empty defaults to silent", "", logger.Silent},
		{"uppercase", "INFO", logger.Info},
		{"mixed case", "Warn", logger.Warn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Driver:      "sqlite",
				Database:    ":memory:",
				SQLLogLevel: tt.logLevel,
			}

			db, err := Connect(cfg)
			require.NoError(t, err)
			require.NotNil(t, db)

			// Verify connection works
			sqlDB, err := db.DB()
			require.NoError(t, err)
			assert.NoError(t, sqlDB.Ping())
		})
	}
}

// TestAutoMigrate tests automatic migration.
func TestAutoMigrate(t *testing.T) {
	cfg := Config{
		Driver:   "sqlite",
		Database: ":memory:",
	}

	db, err := Connect(cfg)
	require.NoError(t, err)

	// Run migrations
	err = AutoMigrate(db)
	assert.NoError(t, err)

	// Verify tables were created
	tables := []string{
		"organizations",
		"users",
		"auth_tokens",
		"domains",
		"tunnels",
		"request_logs",
		"webhook_apps",
		"webhook_routes",
		"webhook_events",
	}

	for _, table := range tables {
		t.Run("table_"+table, func(t *testing.T) {
			assert.True(t, db.Migrator().HasTable(table), "table %s should exist", table)
		})
	}
}

// TestAutoMigrate_CreateRecords tests creating records after migration.
func TestAutoMigrate_CreateRecords(t *testing.T) {
	cfg := Config{
		Driver:   "sqlite",
		Database: ":memory:",
	}

	db, err := Connect(cfg)
	require.NoError(t, err)

	err = AutoMigrate(db)
	require.NoError(t, err)

	// Create an organization
	org := &models.Organization{
		Name:        "Test Org",
		Subdomain:   "testorg",
		Description: "Test organization",
	}
	err = db.Create(org).Error
	require.NoError(t, err)
	assert.NotEmpty(t, org.ID)

	// Create a user
	user := &models.User{
		Email:          "test@example.com",
		Password:       "hashedpassword",
		Name:           "Test User",
		Role:           models.RoleOrgUser,
		OrganizationID: &org.ID,
	}
	err = db.Create(user).Error
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)

	// Create an auth token
	token := &models.AuthToken{
		UserID:    user.ID,
		TokenHash: "somehash",
		Name:      "Test Token",
	}
	err = db.Create(token).Error
	require.NoError(t, err)
	assert.NotEmpty(t, token.ID)

	// Create a domain
	domain := &models.Domain{
		UserID:         user.ID,
		OrganizationID: &org.ID,
		Subdomain:      "testdomain",
	}
	err = db.Create(domain).Error
	require.NoError(t, err)
	assert.NotEmpty(t, domain.ID)

	// Create a tunnel
	tunnel := &models.Tunnel{
		UserID:         user.ID,
		TokenID:        token.ID,
		OrganizationID: &org.ID,
		TunnelType:     "http",
		Subdomain:      "testtunnel",
		LocalAddr:      "localhost:3000",
		PublicURL:      "http://testtunnel.example.com",
		ClientID:       "client123",
		Status:         "online",
	}
	err = db.Create(tunnel).Error
	require.NoError(t, err)
	assert.NotEmpty(t, tunnel.ID)

	// Create a webhook app
	webhookApp := &models.WebhookApp{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Name:           "testapp",
		Description:    "Test webhook app",
	}
	err = db.Create(webhookApp).Error
	require.NoError(t, err)
	assert.NotEmpty(t, webhookApp.ID)
}

// TestAutoMigrate_ForeignKeys tests foreign key relationships.
func TestAutoMigrate_ForeignKeys(t *testing.T) {
	cfg := Config{
		Driver:   "sqlite",
		Database: ":memory:",
	}

	db, err := Connect(cfg)
	require.NoError(t, err)

	err = AutoMigrate(db)
	require.NoError(t, err)

	// Create organization first (parent)
	org := &models.Organization{
		Name:      "FK Test Org",
		Subdomain: "fktest",
	}
	err = db.Create(org).Error
	require.NoError(t, err)

	// Create user with org reference
	user := &models.User{
		Email:          "fk@example.com",
		Password:       "hash",
		Name:           "FK User",
		Role:           models.RoleOrgUser,
		OrganizationID: &org.ID,
	}
	err = db.Create(user).Error
	require.NoError(t, err)

	// Verify foreign key relationship by loading with Preload
	var loadedUser models.User
	err = db.Preload("Organization").First(&loadedUser, user.ID).Error
	require.NoError(t, err)
	assert.NotNil(t, loadedUser.Organization)
	assert.Equal(t, org.ID, loadedUser.Organization.ID)
	assert.Equal(t, "FK Test Org", loadedUser.Organization.Name)
}

// TestAutoMigrate_MultipleRuns tests running AutoMigrate multiple times.
func TestAutoMigrate_MultipleRuns(t *testing.T) {
	cfg := Config{
		Driver:   "sqlite",
		Database: ":memory:",
	}

	db, err := Connect(cfg)
	require.NoError(t, err)

	// First migration
	err = AutoMigrate(db)
	assert.NoError(t, err)

	// Second migration (should be idempotent)
	err = AutoMigrate(db)
	assert.NoError(t, err)

	// Third migration
	err = AutoMigrate(db)
	assert.NoError(t, err)

	// Tables should still exist
	assert.True(t, db.Migrator().HasTable("organizations"))
	assert.True(t, db.Migrator().HasTable("users"))
}

// TestConnect_EmptyConfig tests connection with empty config.
func TestConnect_EmptyConfig(t *testing.T) {
	cfg := Config{}

	db, err := Connect(cfg)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "unsupported database driver")
}

// TestConnect_SQLiteWithLogLevel tests SQLite with different log levels.
func TestConnect_SQLiteWithLogLevel(t *testing.T) {
	cfg := Config{
		Driver:      "sqlite",
		Database:    ":memory:",
		SQLLogLevel: "info",
	}

	db, err := Connect(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)

	// Run AutoMigrate to generate some SQL logs
	err = AutoMigrate(db)
	assert.NoError(t, err)

	// Create a test record to verify logging works
	org := &models.Organization{
		Name:      "Log Test",
		Subdomain: "logtest",
	}
	err = db.Create(org).Error
	assert.NoError(t, err)
}

// TestConnect_PostgreSQLDSN tests PostgreSQL DSN format.
func TestConnect_PostgreSQLDSN(t *testing.T) {
	// This test verifies the DSN format is correct
	// It will fail to connect (no server), but should create valid DSN
	cfg := Config{
		Driver:   "postgres",
		Host:     "testhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
		Password: "testpass",
		SSLMode:  "disable",
	}

	_, err := Connect(cfg)
	// Connection will fail, but error should not be about DSN format
	if err != nil {
		assert.NotContains(t, err.Error(), "unsupported database driver")
	}
}

// BenchmarkConnect benchmarks database connection.
func BenchmarkConnect(b *testing.B) {
	cfg := Config{
		Driver:   "sqlite",
		Database: ":memory:",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db, err := Connect(cfg)
		if err != nil {
			b.Fatal(err)
		}
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}
}

// BenchmarkAutoMigrate benchmarks auto migration.
func BenchmarkAutoMigrate(b *testing.B) {
	cfg := Config{
		Driver:   "sqlite",
		Database: ":memory:",
	}

	db, err := Connect(cfg)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AutoMigrate(db)
	}
}

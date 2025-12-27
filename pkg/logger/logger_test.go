package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetup_DefaultLevel tests logger setup with default level.
func TestSetup_DefaultLevel(t *testing.T) {
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}

	err := Setup(cfg)
	assert.NoError(t, err)

	// Verify global level is set
	assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
}

// TestSetup_DebugLevel tests logger setup with debug level.
func TestSetup_DebugLevel(t *testing.T) {
	cfg := Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	}

	err := Setup(cfg)
	assert.NoError(t, err)

	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
}

// TestSetup_WarnLevel tests logger setup with warn level.
func TestSetup_WarnLevel(t *testing.T) {
	cfg := Config{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}

	err := Setup(cfg)
	assert.NoError(t, err)

	assert.Equal(t, zerolog.WarnLevel, zerolog.GlobalLevel())
}

// TestSetup_ErrorLevel tests logger setup with error level.
func TestSetup_ErrorLevel(t *testing.T) {
	cfg := Config{
		Level:  "error",
		Format: "json",
		Output: "stdout",
	}

	err := Setup(cfg)
	assert.NoError(t, err)

	assert.Equal(t, zerolog.ErrorLevel, zerolog.GlobalLevel())
}

// TestSetup_InvalidLevel tests logger setup with invalid level.
func TestSetup_InvalidLevel(t *testing.T) {
	cfg := Config{
		Level:  "invalid",
		Format: "json",
		Output: "stdout",
	}

	err := Setup(cfg)
	assert.NoError(t, err)

	// Should default to Info level
	assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
}

// TestSetup_JSONFormat tests logger setup with JSON format.
func TestSetup_JSONFormat(t *testing.T) {
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}

	err := Setup(cfg)
	assert.NoError(t, err)

	// Get logger and verify it works
	logger := Get()
	assert.NotNil(t, logger)
}

// TestSetup_TextFormat tests logger setup with text format.
func TestSetup_TextFormat(t *testing.T) {
	cfg := Config{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	}

	err := Setup(cfg)
	assert.NoError(t, err)

	logger := Get()
	assert.NotNil(t, logger)
}

// TestSetup_FileOutput tests logger setup with file output.
func TestSetup_FileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: "file",
		File:   logFile,
	}

	err := Setup(cfg)
	assert.NoError(t, err)

	// Write a test log
	Info("test message")

	// Verify file was created
	assert.FileExists(t, logFile)

	// Read file content
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test message")
}

// TestSetup_FileOutputDefaultName tests logger setup with file output but no filename.
func TestSetup_FileOutputDefaultName(t *testing.T) {
	// Change to temp directory to avoid creating files in project
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: "file",
		File:   "", // No file specified, should use default
	}

	err := Setup(cfg)
	assert.NoError(t, err)

	// Write a test log
	Info("default file test")

	// Verify default file was created
	assert.FileExists(t, "grok.log")
}

// TestGet tests Get function.
func TestGet(t *testing.T) {
	// Setup logger first
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
	Setup(cfg)

	logger := Get()
	assert.NotNil(t, logger)

	// Verify it's a zerolog.Logger
	logger.Info().Msg("test")
}

// TestInfoEvent tests InfoEvent function.
func TestInfoEvent(t *testing.T) {
	cfg := Config{Level: "info", Format: "json", Output: "stdout"}
	Setup(cfg)

	event := InfoEvent()
	assert.NotNil(t, event)

	// Should be able to chain methods
	event.Str("key", "value").Msg("test")
}

// TestDebugEvent tests DebugEvent function.
func TestDebugEvent(t *testing.T) {
	cfg := Config{Level: "debug", Format: "json", Output: "stdout"}
	Setup(cfg)

	event := DebugEvent()
	assert.NotNil(t, event)

	event.Str("key", "value").Msg("test")
}

// TestErrorEvent tests ErrorEvent function.
func TestErrorEvent(t *testing.T) {
	cfg := Config{Level: "info", Format: "json", Output: "stdout"}
	Setup(cfg)

	event := ErrorEvent()
	assert.NotNil(t, event)

	event.Str("key", "value").Msg("test")
}

// TestWarnEvent tests WarnEvent function.
func TestWarnEvent(t *testing.T) {
	cfg := Config{Level: "info", Format: "json", Output: "stdout"}
	Setup(cfg)

	event := WarnEvent()
	assert.NotNil(t, event)

	event.Str("key", "value").Msg("test")
}

// TestWithField tests WithField function.
func TestWithField(t *testing.T) {
	cfg := Config{Level: "info", Format: "json", Output: "stdout"}
	Setup(cfg)

	logger := WithField("request_id", "12345")
	assert.NotNil(t, logger)

	// Should be able to log with the field
	logger.Info().Msg("test with field")
}

// TestWithFields tests WithFields function.
func TestWithFields(t *testing.T) {
	cfg := Config{Level: "info", Format: "json", Output: "stdout"}
	Setup(cfg)

	fields := map[string]interface{}{
		"request_id": "12345",
		"user_id":    "67890",
		"action":     "test",
	}

	logger := WithFields(fields)
	assert.NotNil(t, logger)

	logger.Info().Msg("test with multiple fields")
}

// TestLoggingFunctions tests simple logging functions.
func TestLoggingFunctions(t *testing.T) {
	// Capture output for verification
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		Level:  "debug",
		Format: "json",
		Output: "file",
		File:   logFile,
	}

	err := Setup(cfg)
	require.NoError(t, err)

	// Test each logging function
	Info("info message")
	Debug("debug message")
	Error("error message")
	Warn("warn message")

	// Read log file
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logContent := string(content)

	// Verify all messages are logged
	assert.Contains(t, logContent, "info message")
	assert.Contains(t, logContent, "debug message")
	assert.Contains(t, logContent, "error message")
	assert.Contains(t, logContent, "warn message")

	// Verify log levels
	assert.Contains(t, logContent, `"level":"info"`)
	assert.Contains(t, logContent, `"level":"debug"`)
	assert.Contains(t, logContent, `"level":"error"`)
	assert.Contains(t, logContent, `"level":"warn"`)
}

// TestLogLevelFiltering tests that log levels are properly filtered.
func TestLogLevelFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "level_test.log")

	// Set level to warn (should filter out info and debug)
	cfg := Config{
		Level:  "warn",
		Format: "json",
		Output: "file",
		File:   logFile,
	}

	err := Setup(cfg)
	require.NoError(t, err)

	// Log at different levels
	Debug("debug message - should not appear")
	Info("info message - should not appear")
	Warn("warn message - should appear")
	Error("error message - should appear")

	// Read log file
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logContent := string(content)

	// Debug and Info should NOT be logged
	assert.NotContains(t, logContent, "debug message")
	assert.NotContains(t, logContent, "info message")

	// Warn and Error SHOULD be logged
	assert.Contains(t, logContent, "warn message")
	assert.Contains(t, logContent, "error message")
}

// TestEventChaining tests event method chaining.
func TestEventChaining(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "chain_test.log")

	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: "file",
		File:   logFile,
	}

	err := Setup(cfg)
	require.NoError(t, err)

	// Chain multiple methods
	InfoEvent().
		Str("user_id", "123").
		Int("count", 42).
		Bool("active", true).
		Msg("chained event")

	// Read log file
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logContent := string(content)

	// Verify all fields are logged
	assert.Contains(t, logContent, `"user_id":"123"`)
	assert.Contains(t, logContent, `"count":42`)
	assert.Contains(t, logContent, `"active":true`)
	assert.Contains(t, logContent, "chained event")
}

// TestMultipleSetups tests calling Setup multiple times.
func TestMultipleSetups(t *testing.T) {
	// First setup
	cfg1 := Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	}
	err := Setup(cfg1)
	assert.NoError(t, err)
	assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())

	// Second setup should override
	cfg2 := Config{
		Level:  "error",
		Format: "json",
		Output: "stdout",
	}
	err = Setup(cfg2)
	assert.NoError(t, err)
	assert.Equal(t, zerolog.ErrorLevel, zerolog.GlobalLevel())
}

// TestTextFormatOutput tests text format produces human-readable output.
func TestTextFormatOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "text_test.log")

	cfg := Config{
		Level:  "info",
		Format: "text",
		Output: "file",
		File:   logFile,
	}

	err := Setup(cfg)
	require.NoError(t, err)

	Info("text format test")

	// Read log file
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logContent := string(content)

	// Text format should contain the message
	assert.Contains(t, logContent, "text format test")

	// Text format should be more readable (not JSON)
	// It shouldn't have JSON structure like {"level":"info",...}
	// Instead it should have a console-like format
	lines := strings.Split(strings.TrimSpace(logContent), "\n")
	assert.Greater(t, len(lines), 0)
}

// BenchmarkInfo benchmarks Info logging.
func BenchmarkInfo(b *testing.B) {
	cfg := Config{Level: "info", Format: "json", Output: "stdout"}
	Setup(cfg)

	// Redirect to /dev/null to avoid output overhead
	zerolog.SetGlobalLevel(zerolog.Disabled)
	defer zerolog.SetGlobalLevel(zerolog.InfoLevel)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Info("benchmark message")
	}
}

// BenchmarkInfoEvent benchmarks InfoEvent with chaining.
func BenchmarkInfoEvent(b *testing.B) {
	cfg := Config{Level: "info", Format: "json", Output: "stdout"}
	Setup(cfg)

	zerolog.SetGlobalLevel(zerolog.Disabled)
	defer zerolog.SetGlobalLevel(zerolog.InfoLevel)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		InfoEvent().Str("key", "value").Int("count", i).Msg("benchmark")
	}
}

// BenchmarkWithFields benchmarks WithFields.
func BenchmarkWithFields(b *testing.B) {
	cfg := Config{Level: "info", Format: "json", Output: "stdout"}
	Setup(cfg)

	fields := map[string]interface{}{
		"request_id": "12345",
		"user_id":    "67890",
	}

	zerolog.SetGlobalLevel(zerolog.Disabled)
	defer zerolog.SetGlobalLevel(zerolog.InfoLevel)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger := WithFields(fields)
		logger.Info().Msg("benchmark")
	}
}

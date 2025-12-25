package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config holds logger configuration
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
	Output string // stdout, file
	File   string // file path if Output is "file"
}

// Setup initializes the global logger
func Setup(cfg Config) error {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set output writer
	var writer io.Writer
	switch cfg.Output {
	case "file":
		if cfg.File == "" {
			cfg.File = "grok.log"
		}
		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		writer = file
	default:
		writer = os.Stdout
	}

	// Set format
	if cfg.Format == "text" {
		writer = zerolog.ConsoleWriter{
			Out:        writer,
			TimeFormat: time.RFC3339,
		}
	}

	// Configure global logger
	log.Logger = zerolog.New(writer).With().Timestamp().Caller().Logger()

	return nil
}

// Get returns the global logger
func Get() *zerolog.Logger {
	return &log.Logger
}

// Info logs an info message
func Info(msg string) {
	log.Info().Msg(msg)
}

// Debug logs a debug message
func Debug(msg string) {
	log.Debug().Msg(msg)
}

// Error logs an error message
func Error(msg string) {
	log.Error().Msg(msg)
}

// Warn logs a warning message
func Warn(msg string) {
	log.Warn().Msg(msg)
}

// Fatal logs a fatal message and exits
func Fatal(msg string) {
	log.Fatal().Msg(msg)
}

// InfoEvent returns an info event for chaining
func InfoEvent() *zerolog.Event {
	return log.Info()
}

// DebugEvent returns a debug event for chaining
func DebugEvent() *zerolog.Event {
	return log.Debug()
}

// ErrorEvent returns an error event for chaining
func ErrorEvent() *zerolog.Event {
	return log.Error()
}

// WarnEvent returns a warning event for chaining
func WarnEvent() *zerolog.Event {
	return log.Warn()
}

// WithField returns a logger with additional field
func WithField(key string, value interface{}) *zerolog.Logger {
	logger := log.With().Interface(key, value).Logger()
	return &logger
}

// WithFields returns a logger with multiple fields
func WithFields(fields map[string]interface{}) *zerolog.Logger {
	logger := log.With().Fields(fields).Logger()
	return &logger
}

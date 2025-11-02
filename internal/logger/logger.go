package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LogConfig holds logging configuration
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`     // "json" or "console"
	TimeFormat string `mapstructure:"timeformat"` // "unix", "rfc3339", or custom format
}

// Logger wraps zerolog.Logger with additional context
type Logger struct {
	zerolog.Logger
}

// NewLogger creates a new logger instance with the given configuration
func NewLogger(config LogConfig) *Logger {
	// Set global log level
	level := parseLogLevel(config.Level)
	zerolog.SetGlobalLevel(level)

	// Configure time format
	switch strings.ToLower(config.TimeFormat) {
	case "unix":
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	case "rfc3339":
		zerolog.TimeFieldFormat = time.RFC3339
	case "":
		zerolog.TimeFieldFormat = time.RFC3339 // default
	default:
		zerolog.TimeFieldFormat = config.TimeFormat
	}

	var logger zerolog.Logger

	// Configure output format
	switch strings.ToLower(config.Format) {
	case "console":
		// Pretty console output for development
		output := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
		}
		logger = zerolog.New(output).With().Timestamp().Logger()
	case "json", "":
		// JSON output for production (default)
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	default:
		// Default to JSON
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	// Add service information
	logger = logger.With().
		Str("service", "places-api").
		Str("version", getVersion()).
		Logger()

	// Set as global logger
	log.Logger = logger

	return &Logger{Logger: logger}
}

// parseLogLevel converts string log level to zerolog.Level
func parseLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info", "":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	case "disabled":
		return zerolog.Disabled
	default:
		return zerolog.InfoLevel
	}
}

// getVersion returns the application version (can be set via build flags)
func getVersion() string {
	version := os.Getenv("APP_VERSION")
	if version == "" {
		return "dev"
	}
	return version
}

// WithComponent creates a logger with component context
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With().Str("component", component).Logger(),
	}
}

// WithRequestID creates a logger with request ID context
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{
		Logger: l.Logger.With().Str("request_id", requestID).Logger(),
	}
}

// WithUserID creates a logger with user ID context
func (l *Logger) WithUserID(userID string) *Logger {
	return &Logger{
		Logger: l.Logger.With().Str("user_id", userID).Logger(),
	}
}

// WithArea creates a logger with area context
func (l *Logger) WithArea(areaKey string) *Logger {
	return &Logger{
		Logger: l.Logger.With().Str("area_key", areaKey).Logger(),
	}
}

// WithJob creates a logger with job context
func (l *Logger) WithJob(jobID string, jobType string) *Logger {
	return &Logger{
		Logger: l.Logger.With().
			Str("job_id", jobID).
			Str("job_type", jobType).
			Logger(),
	}
}

// WithError creates a logger with error context
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		Logger: l.Logger.With().Err(err).Logger(),
	}
}

// WithDuration creates a logger with duration context
func (l *Logger) WithDuration(duration time.Duration) *Logger {
	return &Logger{
		Logger: l.Logger.With().Dur("duration", duration).Logger(),
	}
}

// LogHTTPRequest logs HTTP request information
func (l *Logger) LogHTTPRequest(method, path, userAgent, clientIP string, statusCode int, duration time.Duration) {
	l.Info().
		Str("method", method).
		Str("path", path).
		Str("user_agent", userAgent).
		Str("client_ip", clientIP).
		Int("status_code", statusCode).
		Dur("duration", duration).
		Msg("HTTP request")
}

// LogAPICall logs external API calls
func (l *Logger) LogAPICall(service, endpoint, method string, statusCode int, duration time.Duration) {
	durationInSeconds := int(duration.Seconds())
	l.Info().
		Str("external_service", service).
		Str("endpoint", endpoint).
		Str("method", method).
		Int("status_code", statusCode).
		Dur("duration", duration). // Convert to seconds
		Int("duration_seconds", durationInSeconds).
		Msg("External API call")
}

// LogJobProgress logs job progress updates
func (l *Logger) LogJobProgress(jobID, jobType, step string, percentage int, message string) {
	l.Info().
		Str("job_id", jobID).
		Str("job_type", jobType).
		Str("step", step).
		Int("percentage", percentage).
		Str("message", message).
		Msg("Job progress")
}

// LogDatabaseQuery logs database operations
func (l *Logger) LogDatabaseQuery(operation, table string, duration time.Duration, rowsAffected int64) {
	l.Debug().
		Str("db_operation", operation).
		Str("table", table).
		Dur("duration", duration).
		Int64("rows_affected", rowsAffected).
		Msg("Database query")
}

// Global logger instance (will be initialized in main)
var Global *Logger

// InitGlobal initializes the global logger
func InitGlobal(config LogConfig) {
	Global = NewLogger(config)
}

// Helper functions for global logger
func Info() *zerolog.Event {
	return Global.Info()
}

func Debug() *zerolog.Event {
	return Global.Debug()
}

func Warn() *zerolog.Event {
	return Global.Warn()
}

func Error() *zerolog.Event {
	return Global.Error()
}

func Fatal() *zerolog.Event {
	return Global.Fatal()
}

func WithComponent(component string) *Logger {
	return Global.WithComponent(component)
}

func WithRequestID(requestID string) *Logger {
	return Global.WithRequestID(requestID)
}

func WithArea(areaKey string) *Logger {
	return Global.WithArea(areaKey)
}

func WithJob(jobID, jobType string) *Logger {
	return Global.WithJob(jobID, jobType)
}

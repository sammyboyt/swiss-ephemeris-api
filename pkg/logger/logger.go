package logger

import (
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger with additional context methods
type Logger struct {
	*zap.Logger
	serviceName string
}

// LogConfig holds logger configuration
type LogConfig struct {
	Level       string `yaml:"level" env:"LOG_LEVEL" default:"info"`
	Format      string `yaml:"format" env:"LOG_FORMAT" default:"json"`
	ServiceName string `yaml:"service_name" env:"SERVICE_NAME" default:"astral-backend"`
	Environment string `yaml:"environment" env:"ENV" default:"development"`
}

// NewLogger creates a new structured logger with the given configuration
func NewLogger(config LogConfig) (*Logger, error) {
	// Parse log level
	level, err := parseLevel(config.Level)
	if err != nil {
		return nil, err
	}

	// Configure encoder
	encoderConfig := createEncoderConfig()
	var encoder zapcore.Encoder

	if config.Format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Configure output
	writeSyncer, err := createWriteSyncer(config)
	if err != nil {
		return nil, err
	}

	// Create core
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// Create logger
	zapLogger := zap.New(core, getLoggerOptions(config)...)

	return &Logger{
		Logger:      zapLogger.Named(config.ServiceName),
		serviceName: config.ServiceName,
	}, nil
}

// WithRequestID returns a logger with request ID context
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{
		Logger:      l.Logger.With(zap.String("request_id", requestID)),
		serviceName: l.serviceName,
	}
}

// WithUserID returns a logger with user ID context
func (l *Logger) WithUserID(userID string) *Logger {
	return &Logger{
		Logger:      l.Logger.With(zap.String("user_id", userID)),
		serviceName: l.serviceName,
	}
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields ...zap.Field) *Logger {
	return &Logger{
		Logger:      l.Logger.With(fields...),
		serviceName: l.serviceName,
	}
}

// RequestLogger logs HTTP request details
func (l *Logger) RequestLogger(method, path, userAgent, remoteAddr string) {
	l.Info("HTTP request",
		zap.String("method", method),
		zap.String("path", path),
		zap.String("user_agent", userAgent),
		zap.String("remote_addr", remoteAddr),
	)
}

// ResponseLogger logs HTTP response details
func (l *Logger) ResponseLogger(statusCode int, duration time.Duration, responseSize int) {
	level := zap.InfoLevel
	if statusCode >= 400 {
		level = zap.WarnLevel
	}
	if statusCode >= 500 {
		level = zap.ErrorLevel
	}

	l.Log(level, "HTTP response",
		zap.Int("status_code", statusCode),
		zap.Duration("duration", duration),
		zap.Int("response_size", responseSize),
	)
}

// ErrorLogger logs application errors with context
func (l *Logger) ErrorLogger(err error, message string, fields ...zap.Field) {
	allFields := append(fields,
		zap.Error(err),
		zap.String("error_type", getErrorType(err)),
	)

	l.Error(message, allFields...)
}

// SecurityLogger logs security-related events
func (l *Logger) SecurityLogger(event, details string, fields ...zap.Field) {
	allFields := append(fields,
		zap.String("security_event", event),
		zap.String("details", details),
	)

	l.Warn("Security event", allFields...)
}

// PerformanceLogger logs performance metrics
func (l *Logger) PerformanceLogger(operation string, duration int64, fields ...zap.Field) {
	allFields := append(fields,
		zap.String("operation", operation),
		zap.Int64("duration_ms", duration),
	)

	l.Info("Performance metric", allFields...)
}

// Helper functions

func parseLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, nil // Default to info
	}
}

func createEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "service",
		CallerKey:      "caller",
		FunctionKey:    "function",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

func createWriteSyncer(config LogConfig) (zapcore.WriteSyncer, error) {
	if config.Environment == "development" || config.Environment == "test" {
		return zapcore.AddSync(os.Stdout), nil
	}

	// Production: try to write to file, fallback to stdout if permission denied
	logFile := "/var/log/" + config.ServiceName + ".log"
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// If we can't write to file, just use stdout (for testing/containerized environments)
		return zapcore.AddSync(os.Stdout), nil
	}

	return zapcore.NewMultiWriteSyncer(
		zapcore.AddSync(os.Stdout),
		zapcore.AddSync(file),
	), nil
}

func getLoggerOptions(config LogConfig) []zap.Option {
	options := []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	}

	if config.Environment != "production" {
		options = append(options, zap.Development())
	}

	return options
}

func getErrorType(err error) string {
	if err == nil {
		return "none"
	}

	// Check for common error types by inspecting error message
	errorType := "unknown"
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "connection refused"):
		errorType = "connection_error"
	case strings.Contains(errStr, "timeout"):
		errorType = "timeout_error"
	case strings.Contains(errStr, "not found"):
		errorType = "not_found_error"
	case strings.Contains(errStr, "validation"):
		errorType = "validation_error"
	case strings.Contains(errStr, "unauthorized"):
		errorType = "auth_error"
	case strings.Contains(errStr, "INTERNAL_ERROR"):
		errorType = "internal_error"
	case strings.Contains(errStr, "VALIDATION_ERROR"):
		errorType = "validation_error"
	case strings.Contains(errStr, "AUTHENTICATION_ERROR"):
		errorType = "auth_error"
	case strings.Contains(errStr, "AUTHORIZATION_ERROR"):
		errorType = "auth_error"
	case strings.Contains(errStr, "RATE_LIMIT_EXCEEDED"):
		errorType = "rate_limit_error"
	case strings.Contains(errStr, "EPHEMERIS_ERROR"):
		errorType = "ephemeris_error"
	}

	return errorType
}

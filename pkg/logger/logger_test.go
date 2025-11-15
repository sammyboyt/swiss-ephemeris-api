package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewLogger(t *testing.T) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test-service",
		Environment: "test",
	}

	logger, err := NewLogger(config)
	assert.NoError(t, err)
	assert.NotNil(t, logger)
	assert.Equal(t, "test-service", logger.serviceName)
	assert.NotNil(t, logger.Logger)
}

func TestNewLoggerWithInvalidLevel(t *testing.T) {
	config := LogConfig{
		Level:       "invalid",
		Format:      "json",
		ServiceName: "test-service",
		Environment: "test",
	}

	logger, err := NewLogger(config)
	assert.NoError(t, err) // Should default to info level
	assert.NotNil(t, logger)
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zapcore.Level
		hasError bool
	}{
		{"debug", zapcore.DebugLevel, false},
		{"info", zapcore.InfoLevel, false},
		{"warn", zapcore.WarnLevel, false},
		{"warning", zapcore.WarnLevel, false},
		{"error", zapcore.ErrorLevel, false},
		{"fatal", zapcore.FatalLevel, false},
		{"invalid", zapcore.InfoLevel, false}, // Should default to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := parseLevel(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, level)
			}
		})
	}
}

func TestLoggerWithRequestID(t *testing.T) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test-service",
		Environment: "test",
	}

	logger, _ := NewLogger(config)
	contextLogger := logger.WithRequestID("req-123")

	assert.NotEqual(t, logger, contextLogger)
	assert.Equal(t, logger.serviceName, contextLogger.serviceName)
}

func TestLoggerWithUserID(t *testing.T) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test-service",
		Environment: "test",
	}

	logger, _ := NewLogger(config)
	contextLogger := logger.WithUserID("user-456")

	assert.NotEqual(t, logger, contextLogger)
	assert.Equal(t, logger.serviceName, contextLogger.serviceName)
}

func TestLoggerWithFields(t *testing.T) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test-service",
		Environment: "test",
	}

	logger, _ := NewLogger(config)
	contextLogger := logger.WithFields(
		zap.String("operation", "test"),
		zap.Int("count", 42),
	)

	assert.NotEqual(t, logger, contextLogger)
	assert.Equal(t, logger.serviceName, contextLogger.serviceName)
}

func TestRequestLogger(t *testing.T) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test-service",
		Environment: "test",
	}

	// This test would need a custom core to capture output
	// For now, just test that the method doesn't panic
	logger, _ := NewLogger(config)

	assert.NotPanics(t, func() {
		logger.RequestLogger("GET", "/api/test", "test-agent", "127.0.0.1")
	})
}

func TestResponseLogger(t *testing.T) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test-service",
		Environment: "test",
	}

	logger, _ := NewLogger(config)

	// Test different status codes
	assert.NotPanics(t, func() {
		logger.ResponseLogger(200, 150, 1024)
	})

	assert.NotPanics(t, func() {
		logger.ResponseLogger(404, 50, 256)
	})

	assert.NotPanics(t, func() {
		logger.ResponseLogger(500, 2000, 512)
	})
}

func TestErrorLogger(t *testing.T) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test-service",
		Environment: "test",
	}

	logger, _ := NewLogger(config)

	testErr := &testError{message: "test error", code: "TEST_ERROR"}

	assert.NotPanics(t, func() {
		logger.ErrorLogger(testErr, "Test error occurred",
			zap.String("component", "test"),
			zap.Int("attempt", 3),
		)
	})
}

func TestSecurityLogger(t *testing.T) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test-service",
		Environment: "test",
	}

	logger, _ := NewLogger(config)

	assert.NotPanics(t, func() {
		logger.SecurityLogger("invalid_api_key", "Key xyz attempted access",
			zap.String("ip", "192.168.1.1"),
			zap.String("endpoint", "/api/secret"),
		)
	})
}

func TestPerformanceLogger(t *testing.T) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test-service",
		Environment: "test",
	}

	logger, _ := NewLogger(config)

	assert.NotPanics(t, func() {
		logger.PerformanceLogger("ephemeris_calculation", 150,
			zap.String("algorithm", "swiss_ephemeris"),
			zap.Int("bodies", 10),
		)
	})
}

func TestGetErrorType(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"nil error", nil, "none"},
		{"connection error", &testError{message: "connection refused"}, "connection_error"},
		{"timeout error", &testError{message: "timeout occurred"}, "timeout_error"},
		{"not found error", &testError{message: "resource not found"}, "not_found_error"},
		{"validation error", &testError{message: "validation failed"}, "validation_error"},
		{"unauthorized error", &testError{message: "unauthorized access"}, "auth_error"},
		{"internal error", &testError{message: "INTERNAL_ERROR: something"}, "internal_error"},
		{"validation error code", &testError{message: "VALIDATION_ERROR: bad input"}, "validation_error"},
		{"auth error code", &testError{message: "AUTHENTICATION_ERROR: bad key"}, "auth_error"},
		{"rate limit error", &testError{message: "RATE_LIMIT_EXCEEDED: too fast"}, "rate_limit_error"},
		{"ephemeris error", &testError{message: "EPHEMERIS_ERROR: calc failed"}, "ephemeris_error"},
		{"unknown error", &testError{message: "some random error"}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getErrorType(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateEncoderConfig(t *testing.T) {
	config := createEncoderConfig()

	assert.Equal(t, "timestamp", config.TimeKey)
	assert.Equal(t, "level", config.LevelKey)
	assert.Equal(t, "service", config.NameKey)
	assert.Equal(t, "caller", config.CallerKey)
	assert.Equal(t, "function", config.FunctionKey)
	assert.Equal(t, "message", config.MessageKey)
	assert.Equal(t, "stacktrace", config.StacktraceKey)

	assert.NotNil(t, config.EncodeLevel)
	assert.NotNil(t, config.EncodeTime)
	assert.NotNil(t, config.EncodeDuration)
	assert.NotNil(t, config.EncodeCaller)
}

func TestGetLoggerOptions(t *testing.T) {
	// Development environment
	devConfig := LogConfig{Environment: "development"}
	devOptions := getLoggerOptions(devConfig)

	assert.Len(t, devOptions, 3) // AddCaller, AddCallerSkip, Development

	// Production environment
	prodConfig := LogConfig{Environment: "production"}
	prodOptions := getLoggerOptions(prodConfig)

	assert.Len(t, prodOptions, 2) // AddCaller, AddCallerSkip (no Development)
}

// Benchmark tests
func BenchmarkLoggerInfo(b *testing.B) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "bench-service",
		Environment: "test",
	}

	logger, _ := NewLogger(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message",
			zap.String("key", "value"),
			zap.Int("count", i),
		)
	}
}

func BenchmarkLoggerWithContext(b *testing.B) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "bench-service",
		Environment: "test",
	}

	logger, _ := NewLogger(config)
	contextLogger := logger.WithRequestID("req-123").WithUserID("user-456")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contextLogger.Info("contextual message",
			zap.String("operation", "test"),
			zap.Int("iteration", i),
		)
	}
}

func BenchmarkErrorLogger(b *testing.B) {
	config := LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "bench-service",
		Environment: "test",
	}

	logger, _ := NewLogger(config)
	testErr := &testError{message: "benchmark error"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.ErrorLogger(testErr, "Benchmark error occurred",
			zap.Int("iteration", i),
		)
	}
}

// Helper types for testing
type testError struct {
	message string
	code    string
}

func (e *testError) Error() string {
	return e.message
}

func (e *testError) Code() string {
	return e.code
}

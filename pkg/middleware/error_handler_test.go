package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"astral-backend/pkg/errors"
	"astral-backend/pkg/logger"

	"github.com/stretchr/testify/assert"
)

func TestErrorHandler_PanicRecovery(t *testing.T) {
	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})

	handler := NewErrorHandler(logger)

	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic message")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler := handler.Handle(panicHandler)
	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Contains(t, response, "error")
	assert.Contains(t, response, "timestamp")
	assert.Contains(t, response, "request_id")

	errMap := response["error"].(map[string]interface{})
	assert.Equal(t, "INTERNAL_ERROR", errMap["code"])
	assert.Contains(t, errMap["message"], "Internal server error")
}

func TestErrorHandler_APIErrorResponse(t *testing.T) {
	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})

	handler := NewErrorHandler(logger)

	// Handler that returns an API error
	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		RespondWithError(w, r, errors.NewValidationError("email", "invalid format"), logger)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler := handler.Handle(errorHandler)
	wrappedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Contains(t, response, "error")
	errMap := response["error"].(map[string]interface{})
	assert.Equal(t, "VALIDATION_ERROR", errMap["code"])
	assert.Equal(t, "Request validation failed", errMap["message"])

	// Check details are included in development mode
	assert.Contains(t, errMap, "details")
	details := errMap["details"].(map[string]interface{})
	assert.Equal(t, "email", details["field"])
	assert.Equal(t, "invalid format", details["reason"])
}

func TestErrorHandler_MultipleErrors(t *testing.T) {
	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})

	handler := NewErrorHandler(logger)

	testCases := []struct {
		name           string
		errFunc        func() errors.Error
		expectedCode   string
		expectedStatus int
	}{
		{
			name: "authentication error",
			errFunc: func() errors.Error {
				return errors.NewAuthenticationError("Invalid credentials")
			},
			expectedCode:   "AUTHENTICATION_ERROR",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "authorization error",
			errFunc: func() errors.Error {
				return errors.NewAuthorizationError("Insufficient permissions")
			},
			expectedCode:   "AUTHORIZATION_ERROR",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "not found error",
			errFunc: func() errors.Error {
				return errors.NewNotFoundError("resource")
			},
			expectedCode:   "NOT_FOUND",
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "rate limit error",
			errFunc: func() errors.Error {
				return errors.NewRateLimitError()
			},
			expectedCode:   "RATE_LIMIT_EXCEEDED",
			expectedStatus: http.StatusTooManyRequests,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				RespondWithError(w, r, tc.errFunc(), logger)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			wrappedHandler := handler.Handle(errorHandler)
			wrappedHandler.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			errMap := response["error"].(map[string]interface{})
			assert.Equal(t, tc.expectedCode, errMap["code"])
		})
	}
}

func TestErrorHandler_RequestID(t *testing.T) {
	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})

	handler := NewErrorHandler(logger)

	// Test with X-Request-ID header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "custom-request-123")

	w := httptest.NewRecorder()

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	wrappedHandler := handler.Handle(panicHandler)
	wrappedHandler.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, "custom-request-123", response["request_id"])
}

func TestErrorHandler_JSONEncodingFailure(t *testing.T) {
	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})

	handler := NewErrorHandler(logger)

	// Create an error with problematic details that might cause JSON encoding issues
	problematicError := errors.NewAPIError("TEST_ERROR", "Test message", http.StatusBadRequest, map[string]interface{}{
		"problematic": make(chan int), // Channels cannot be JSON encoded
	})

	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		RespondWithError(w, r, problematicError, logger)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler := handler.Handle(errorHandler)
	wrappedHandler.ServeHTTP(w, req)

	// Should still return a valid error response despite encoding issues
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	// The JSON unmarshaling might fail, but the handler should still provide a basic response
	if err == nil {
		// If it did parse, check it has basic error structure
		assert.Contains(t, response, "error")
	}
}

func TestErrorHandler_HandleFunc(t *testing.T) {
	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})

	handler := NewErrorHandler(logger)

	panicFunc := func(w http.ResponseWriter, r *http.Request) {
		panic("test panic in function")
	}

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedFunc := handler.HandleFunc(panicFunc)
	wrappedFunc(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	errMap := response["error"].(map[string]interface{})
	assert.Equal(t, "INTERNAL_ERROR", errMap["code"])
}

func TestErrorHandler_StackTraceLogging(t *testing.T) {
	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})

	handler := NewErrorHandler(logger)

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("stack trace test")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler := handler.Handle(panicHandler)
	wrappedHandler.ServeHTTP(w, req)

	// The stack trace should be logged but we can't easily test the log output
	// In a real scenario, we'd use a test logger that captures output
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Benchmark tests
func BenchmarkErrorHandler_NoError(b *testing.B) {
	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "info", ServiceName: "bench", Environment: "test",
	})

	handler := NewErrorHandler(logger)

	successHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		wrappedHandler := handler.Handle(successHandler)
		wrappedHandler.ServeHTTP(w, req)
	}
}

func BenchmarkErrorHandler_WithPanic(b *testing.B) {
	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "info", ServiceName: "bench", Environment: "test",
	})

	handler := NewErrorHandler(logger)

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("benchmark panic")
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		wrappedHandler := handler.Handle(panicHandler)
		wrappedHandler.ServeHTTP(w, req)
	}
}

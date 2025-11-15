package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"astral-backend/pkg/logger"

	"github.com/stretchr/testify/assert"
)

func TestRequestIDMiddleware_GenerateRequestID(t *testing.T) {
	testLogger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})
	middleware := NewRequestIDMiddleware(testLogger)

	// RED: Should generate unique request IDs
	id1 := middleware.generateRequestID()
	id2 := middleware.generateRequestID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Len(t, id1, 32) // 16 bytes * 2 hex chars
}

func TestRequestIDMiddleware_AddsHeader(t *testing.T) {
	testLogger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})
	middleware := NewRequestIDMiddleware(testLogger)

	// RED: Should add X-Request-ID header to response
	handler := middleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get(RequestIDHeader))
}

func TestRequestIDMiddleware_ContextPropagation(t *testing.T) {
	testLogger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})
	middleware := NewRequestIDMiddleware(testLogger)

	// RED: Should add request ID to context
	var capturedRequestID string
	handler := middleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = GetRequestID(r.Context())
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.NotEmpty(t, capturedRequestID)
	assert.Equal(t, w.Header().Get(RequestIDHeader), capturedRequestID)
}

func TestRequestIDMiddleware_ExistingHeader(t *testing.T) {
	testLogger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})
	middleware := NewRequestIDMiddleware(testLogger)

	// RED: Should use existing request ID header
	const existingID = "existing-request-id"

	var capturedRequestID string
	handler := middleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = GetRequestID(r.Context())
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(RequestIDHeader, existingID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, existingID, capturedRequestID)
	assert.Equal(t, existingID, w.Header().Get(RequestIDHeader))
}

func TestRequestIDMiddleware_LoggerIntegration(t *testing.T) {
	testLogger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})
	middleware := NewRequestIDMiddleware(testLogger)

	// RED: Should provide logger with request ID in context
	handler := middleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxLogger := GetLoggerFromContext(r.Context())
		assert.NotNil(t, ctxLogger)
		// Verify logger has request ID field
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetRequestID_EmptyContext(t *testing.T) {
	// RED: Should return empty string for missing request ID
	ctx := context.Background()
	requestID := GetRequestID(ctx)
	assert.Empty(t, requestID)
}

func TestGetLoggerFromContext_EmptyContext(t *testing.T) {
	// RED: Should return nil for missing logger
	ctx := context.Background()
	ctxLogger := GetLoggerFromContext(ctx)
	assert.Nil(t, ctxLogger)
}

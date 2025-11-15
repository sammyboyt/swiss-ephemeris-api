package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"

	"astral-backend/pkg/logger"

	"go.uber.org/zap"
)

type RequestIDKey struct{}

type loggerContextKey struct{}

const RequestIDHeader = "X-Request-ID"

// RequestIDMiddleware generates and propagates request IDs
type RequestIDMiddleware struct {
	logger *logger.Logger
}

// NewRequestIDMiddleware creates a new request ID middleware
func NewRequestIDMiddleware(logger *logger.Logger) *RequestIDMiddleware {
	return &RequestIDMiddleware{
		logger: logger,
	}
}

// Middleware is the main middleware handler
func (m *RequestIDMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)

		// Generate new request ID if not present
		if requestID == "" {
			requestID = m.generateRequestID()
		}

		// Add to response header
		w.Header().Set(RequestIDHeader, requestID)

		// Add to request context
		ctx := context.WithValue(r.Context(), RequestIDKey{}, requestID)

		// Add logger with request ID to context
		loggerWithID := m.logger.WithFields(zap.String("request_id", requestID))
		ctx = context.WithValue(ctx, loggerContextKey{}, loggerWithID)

		// Call next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateRequestID generates a unique request ID
func (m *RequestIDMiddleware) generateRequestID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%x", bytes)
}

// GetRequestID extracts the request ID from context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey{}).(string); ok {
		return requestID
	}
	return ""
}

// GetLoggerFromContext gets the logger with request ID from context
func GetLoggerFromContext(ctx context.Context) *logger.Logger {
	if logger, ok := ctx.Value(loggerContextKey{}).(*logger.Logger); ok {
		return logger
	}
	return nil
}

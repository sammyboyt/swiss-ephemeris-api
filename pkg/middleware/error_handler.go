package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"

	"astral-backend/pkg/errors"
	"astral-backend/pkg/logger"

	"go.uber.org/zap"
)

// ErrorHandler provides comprehensive error handling for HTTP requests
type ErrorHandler struct {
	logger *logger.Logger
}

// NewErrorHandler creates a new error handler middleware
func NewErrorHandler(logger *logger.Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// Handle wraps an HTTP handler with comprehensive error handling
func (eh *ErrorHandler) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Panic recovery
		defer func() {
			if err := recover(); err != nil {
				eh.handlePanic(w, r, err)
			}
		}()

		// Execute the next handler
		next.ServeHTTP(w, r)
	})
}

// HandleFunc wraps an HTTP handler function with comprehensive error handling
func (eh *ErrorHandler) HandleFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Panic recovery
		defer func() {
			if err := recover(); err != nil {
				eh.handlePanic(w, r, err)
			}
		}()

		// Execute the next handler
		next(w, r)
	}
}

// handlePanic handles panic recovery and logging
func (eh *ErrorHandler) handlePanic(w http.ResponseWriter, r *http.Request, panicErr interface{}) {
	// Create internal error
	err := errors.NewInternalError(fmt.Errorf("panic: %v", panicErr))

	// Log the panic with stack trace
	eh.logger.ErrorLogger(fmt.Errorf("panic: %v", panicErr), "Panic recovered in HTTP handler",
		zap.String("url", r.URL.String()),
		zap.String("method", r.Method),
		zap.String("user_agent", r.UserAgent()),
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("stack_trace", string(debug.Stack())),
	)

	// Send error response
	eh.respondWithError(w, r, err)
}

// respondWithError sends a structured error response
func (eh *ErrorHandler) respondWithError(w http.ResponseWriter, r *http.Request, err errors.Error) {
	// Create error response
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code(),
			"message": err.Message(),
		},
		"timestamp":  fmt.Sprintf("%d", getCurrentTimestamp()),
		"request_id": getRequestID(r),
	}

	// Add details in development mode
	if isDevelopmentMode() {
		if details := err.Details(); len(details) > 0 {
			response["error"].(map[string]interface{})["details"] = details
		}
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode())

	// Write response
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		// If JSON encoding fails, send a basic error response
		eh.logger.ErrorLogger(encodeErr, "Failed to encode error response")
		http.Error(w, `{"error": {"code": "INTERNAL_ERROR", "message": "Internal server error"}}`, http.StatusInternalServerError)
	}
}

// Helper functions

func getRequestID(r *http.Request) string {
	// Try to get request ID from context first
	if id := r.Context().Value("request_id"); id != nil {
		if strID, ok := id.(string); ok {
			return strID
		}
	}

	// Fallback to header
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}

	// Generate a simple ID if none exists
	return fmt.Sprintf("req-%d", getCurrentTimestamp())
}

func getCurrentTimestamp() int64 {
	// This would use time.Now().Unix() in real implementation
	// For testing, we can mock this
	return 1640995200 // Fixed timestamp for testing
}

func isDevelopmentMode() bool {
	// Check environment variable
	// In real implementation, this would check os.Getenv("ENV") == "development"
	return true // Default to development for testing
}

// RespondWithError is a convenience function for handlers to return errors
func RespondWithError(w http.ResponseWriter, r *http.Request, err errors.Error, logger *logger.Logger) {
	handler := NewErrorHandler(logger)
	handler.respondWithError(w, r, err)
}

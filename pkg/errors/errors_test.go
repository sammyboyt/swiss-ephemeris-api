package errors

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAPIError(t *testing.T) {
	details := map[string]interface{}{
		"field":  "email",
		"reason": "invalid format",
	}

	err := NewAPIError("VALIDATION_ERROR", "Invalid input", http.StatusBadRequest, details)

	assert.Equal(t, "VALIDATION_ERROR", err.Code())
	assert.Equal(t, "Invalid input", err.Message())
	assert.Equal(t, http.StatusBadRequest, err.StatusCode())
	assert.Equal(t, details, err.Details())
	assert.Contains(t, err.Error(), "VALIDATION_ERROR: Invalid input")
}

func TestNewAPIErrorWithoutDetails(t *testing.T) {
	err := NewAPIError("INTERNAL_ERROR", "Something went wrong", http.StatusInternalServerError, nil)

	assert.Equal(t, "INTERNAL_ERROR", err.Code())
	assert.Equal(t, "Something went wrong", err.Message())
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode())
	assert.Nil(t, err.Details())
}

func TestErrorWrapping(t *testing.T) {
	originalErr := errors.New("database connection failed")
	apiErr := NewInternalError(originalErr)

	assert.Contains(t, apiErr.Error(), "database connection failed")
	assert.Equal(t, "INTERNAL_ERROR", apiErr.Code())
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode())
	assert.NotNil(t, apiErr.cause)
	assert.Equal(t, originalErr, apiErr.cause)
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("email", "invalid format")

	assert.Equal(t, "VALIDATION_ERROR", err.Code())
	assert.Equal(t, "Request validation failed", err.Message())
	assert.Equal(t, http.StatusBadRequest, err.StatusCode())

	details := err.Details()
	assert.NotNil(t, details)
	assert.Equal(t, "email", details["field"])
	assert.Equal(t, "invalid format", details["reason"])
}

func TestNewAuthenticationError(t *testing.T) {
	err := NewAuthenticationError("Invalid API key")

	assert.Equal(t, "AUTHENTICATION_ERROR", err.Code())
	assert.Equal(t, "Authentication failed", err.Message())
	assert.Equal(t, http.StatusUnauthorized, err.StatusCode())

	details := err.Details()
	assert.NotNil(t, details)
	assert.Equal(t, "Invalid API key", details["reason"])
}

func TestNewAuthorizationError(t *testing.T) {
	err := NewAuthorizationError("Missing required scope")

	assert.Equal(t, "AUTHORIZATION_ERROR", err.Code())
	assert.Equal(t, "Authorization failed", err.Message())
	assert.Equal(t, http.StatusForbidden, err.StatusCode())

	details := err.Details()
	assert.NotNil(t, details)
	assert.Equal(t, "Missing required scope", details["reason"])
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("API key")

	assert.Equal(t, "NOT_FOUND", err.Code())
	assert.Equal(t, "API key not found", err.Message())
	assert.Equal(t, http.StatusNotFound, err.StatusCode())
	assert.Nil(t, err.Details())
}

func TestNewRateLimitError(t *testing.T) {
	err := NewRateLimitError()

	assert.Equal(t, "RATE_LIMIT_EXCEEDED", err.Code())
	assert.Equal(t, "Rate limit exceeded", err.Message())
	assert.Equal(t, http.StatusTooManyRequests, err.StatusCode())
}

func TestNewEphemerisError(t *testing.T) {
	err := NewEphemerisError("Swiss Ephemeris data not found")

	assert.Equal(t, "EPHEMERIS_ERROR", err.Code())
	assert.Equal(t, "Ephemeris calculation failed", err.Message())
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode())

	details := err.Details()
	assert.NotNil(t, details)
	assert.Equal(t, "Swiss Ephemeris data not found", details["reason"])
}

func TestIsAPIError(t *testing.T) {
	apiErr := NewValidationError("test", "reason")
	regularErr := errors.New("regular error")

	assert.True(t, IsAPIError(apiErr))
	assert.False(t, IsAPIError(regularErr))
	assert.False(t, IsAPIError(nil))
}

func TestGetAPIError(t *testing.T) {
	apiErr := NewValidationError("test", "reason")
	regularErr := errors.New("regular error")

	extracted, ok := GetAPIError(apiErr)
	assert.True(t, ok)
	assert.Equal(t, apiErr, extracted)

	extracted, ok = GetAPIError(regularErr)
	assert.False(t, ok)
	assert.Nil(t, extracted)
}

func TestErrorChaining(t *testing.T) {
	// Test that errors can wrap other API errors
	innerErr := NewValidationError("field", "invalid")
	outerErr := NewInternalError(innerErr)

	assert.Contains(t, outerErr.Error(), "VALIDATION_ERROR")
	assert.Contains(t, outerErr.Error(), "INTERNAL_ERROR")
	assert.Equal(t, http.StatusInternalServerError, outerErr.StatusCode())
}

func TestErrorImplementsInterface(t *testing.T) {
	var err Error = NewValidationError("test", "reason")

	// Should implement all interface methods
	assert.NotEmpty(t, err.Code())
	assert.NotEmpty(t, err.Message())
	assert.NotNil(t, err.Details())
	assert.NotZero(t, err.StatusCode())
	assert.NotEmpty(t, err.Error())
}

// Benchmark tests for performance
func BenchmarkNewAPIError(b *testing.B) {
	details := map[string]interface{}{"field": "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewAPIError("TEST_ERROR", "Test message", http.StatusBadRequest, details)
	}
}

func BenchmarkErrorWrapping(b *testing.B) {
	originalErr := errors.New("original error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewInternalError(originalErr)
	}
}

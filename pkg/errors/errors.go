package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Error represents an API error interface
type Error interface {
	error
	Code() string
	Message() string
	Details() map[string]interface{}
	StatusCode() int
}

// APIError implements the Error interface for structured API errors
type APIError struct {
	code       string
	message    string
	details    map[string]interface{}
	statusCode int
	cause      error
}

// Error returns the error message
func (e *APIError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.code, e.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

// Code returns the error code
func (e *APIError) Code() string {
	return e.code
}

// Message returns the error message
func (e *APIError) Message() string {
	return e.message
}

// Details returns additional error details
func (e *APIError) Details() map[string]interface{} {
	return e.details
}

// StatusCode returns the HTTP status code
func (e *APIError) StatusCode() int {
	return e.statusCode
}

// NewAPIError creates a new API error
func NewAPIError(code, message string, statusCode int, details map[string]interface{}) *APIError {
	return &APIError{
		code:       code,
		message:    message,
		statusCode: statusCode,
		details:    details,
	}
}

// NewValidationError creates a validation error
func NewValidationError(field, reason string) *APIError {
	return &APIError{
		code:       "VALIDATION_ERROR",
		message:    "Request validation failed",
		statusCode: http.StatusBadRequest,
		details: map[string]interface{}{
			"field":  field,
			"reason": reason,
		},
	}
}

// NewAuthenticationError creates an authentication error
func NewAuthenticationError(reason string) *APIError {
	return &APIError{
		code:       "AUTHENTICATION_ERROR",
		message:    "Authentication failed",
		statusCode: http.StatusUnauthorized,
		details: map[string]interface{}{
			"reason": reason,
		},
	}
}

// NewAuthorizationError creates an authorization error
func NewAuthorizationError(reason string) *APIError {
	return &APIError{
		code:       "AUTHORIZATION_ERROR",
		message:    "Authorization failed",
		statusCode: http.StatusForbidden,
		details: map[string]interface{}{
			"reason": reason,
		},
	}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string) *APIError {
	return &APIError{
		code:       "NOT_FOUND",
		message:    fmt.Sprintf("%s not found", resource),
		statusCode: http.StatusNotFound,
	}
}

// NewInternalError creates an internal server error
func NewInternalError(err error) *APIError {
	return &APIError{
		code:       "INTERNAL_ERROR",
		message:    "Internal server error",
		statusCode: http.StatusInternalServerError,
		cause:      err,
	}
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError() *APIError {
	return &APIError{
		code:       "RATE_LIMIT_EXCEEDED",
		message:    "Rate limit exceeded",
		statusCode: http.StatusTooManyRequests,
	}
}

// NewEphemerisError creates an ephemeris calculation error
func NewEphemerisError(reason string) *APIError {
	return &APIError{
		code:       "EPHEMERIS_ERROR",
		message:    "Ephemeris calculation failed",
		statusCode: http.StatusInternalServerError,
		details: map[string]interface{}{
			"reason": reason,
		},
	}
}

// IsAPIError checks if an error is an API error
func IsAPIError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr)
}

// GetAPIError extracts API error from an error
func GetAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	ok := errors.As(err, &apiErr)
	return apiErr, ok
}

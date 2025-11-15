package middleware

import (
	"context"
	"net/http"
	"strings"

	"astral-backend/pkg/auth"
	"astral-backend/pkg/errors"
	"astral-backend/pkg/logger"
	"astral-backend/pkg/models"

	"go.uber.org/zap"
)

// AuthMiddleware handles API key authentication
type AuthMiddleware struct {
	apiKeyService auth.APIKeyServiceInterface
	logger        *logger.Logger
	requiredScope string // Optional: require specific scope for all routes
}

// APIKeyContextKey is the key used to store API key info in request context
type APIKeyContextKey struct{}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(apiKeyService auth.APIKeyServiceInterface, logger *logger.Logger, requiredScope string) *AuthMiddleware {
	return &AuthMiddleware{
		apiKeyService: apiKeyService,
		logger:        logger,
		requiredScope: requiredScope,
	}
}

// RequireScope returns a new middleware that requires a specific scope
func (m *AuthMiddleware) RequireScope(scope string) *AuthMiddleware {
	return &AuthMiddleware{
		apiKeyService: m.apiKeyService,
		logger:        m.logger,
		requiredScope: scope,
	}
}

// Authenticate is the main authentication middleware handler
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := getRequestIDFromContext(r.Context())
		if requestID == "" {
			requestID = "unknown"
		}

		m.logger.Info("Authenticating request",
			zap.String("request_id", requestID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		)

		// Extract API key from request
		apiKey, err := m.extractAPIKey(r)
		if err != nil {
			m.logger.Warn("Failed to extract API key from request",
				zap.String("request_id", requestID),
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
			)
			m.handleAuthError(w, err)
			return
		}

		// Validate API key
		validatedKey, err := m.apiKeyService.ValidateAPIKey(r.Context(), apiKey)
		if err != nil {
			m.logger.Warn("API key validation failed",
				zap.String("request_id", requestID),
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
			)
			m.handleAuthError(w, err)
			return
		}

		// Check required scope if specified
		if m.requiredScope != "" && !validatedKey.HasScope(m.requiredScope) {
			m.logger.Warn("API key missing required scope",
				zap.String("request_id", requestID),
				zap.String("key_name", validatedKey.Name),
				zap.String("required_scope", m.requiredScope),
				zap.Strings("key_scopes", validatedKey.Scopes),
			)
			m.handleAuthError(w, errors.NewAuthorizationError("Insufficient permissions"))
			return
		}

		// Add API key to request context
		ctx := context.WithValue(r.Context(), APIKeyContextKey{}, validatedKey)
		r = r.WithContext(ctx)

		m.logger.Info("Request authenticated successfully",
			zap.String("request_id", requestID),
			zap.String("key_name", validatedKey.Name),
			zap.Strings("key_scopes", validatedKey.Scopes),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		)

		// Call next handler
		next.ServeHTTP(w, r)
	})
}

// extractAPIKey extracts the API key from the HTTP request
// Priority order: Authorization header > X-API-Key header > api_key query parameter
func (m *AuthMiddleware) extractAPIKey(r *http.Request) (string, error) {
	// Try Authorization header first (Bearer token format)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey := strings.TrimPrefix(authHeader, "Bearer ")
			if apiKey != "" {
				return strings.TrimSpace(apiKey), nil
			}
		}
		// Also support direct API key in Authorization header
		if !strings.Contains(authHeader, " ") {
			return strings.TrimSpace(authHeader), nil
		}
	}

	// Try X-API-Key header
	apiKeyHeader := r.Header.Get("X-API-Key")
	if apiKeyHeader != "" {
		return strings.TrimSpace(apiKeyHeader), nil
	}

	// Try api_key query parameter
	apiKeyQuery := r.URL.Query().Get("api_key")
	if apiKeyQuery != "" {
		return strings.TrimSpace(apiKeyQuery), nil
	}

	return "", errors.NewAuthenticationError("No API key provided")
}

// handleAuthError handles authentication/authorization errors
func (m *AuthMiddleware) handleAuthError(w http.ResponseWriter, err error) {
	var apiErr *errors.APIError
	if errors.IsAPIError(err) {
		apiErr, _ = errors.GetAPIError(err)
	} else {
		apiErr = errors.NewInternalError(err)
	}

	// Don't log internal errors as they might be sensitive
	if apiErr.Code() != "INTERNAL_ERROR" {
		m.logger.Warn("Authentication error",
			zap.String("error_code", apiErr.Code()),
			zap.String("error_message", apiErr.Message()),
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.StatusCode())

	// Return error response
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    apiErr.Code(),
			"message": apiErr.Message(),
		},
	}

	if details := apiErr.Details(); details != nil {
		response["error"].(map[string]interface{})["details"] = details
	}

	// Note: In a real implementation, you'd use json.Marshal here
	// For now, we'll use a simple response since we don't want to add more dependencies
	w.Write([]byte(`{"error":{"code":"` + apiErr.Code() + `","message":"` + apiErr.Message() + `"}}`))
}

// GetAPIKeyFromContext extracts the API key from the request context
func GetAPIKeyFromContext(ctx context.Context) (*models.APIKey, bool) {
	apiKey, ok := ctx.Value(APIKeyContextKey{}).(*models.APIKey)
	return apiKey, ok
}

// RequireAuthentication is a convenience function to create authenticated routes
func RequireAuthentication(apiKeyService auth.APIKeyServiceInterface, logger *logger.Logger) func(http.Handler) http.Handler {
	middleware := NewAuthMiddleware(apiKeyService, logger, "")
	return middleware.Authenticate
}

// RequireScope is a convenience function to create scope-protected routes
func RequireScope(apiKeyService auth.APIKeyServiceInterface, logger *logger.Logger, scope string) func(http.Handler) http.Handler {
	middleware := NewAuthMiddleware(apiKeyService, logger, scope)
	return middleware.Authenticate
}

// getRequestIDFromContext extracts request ID from context (assuming it's set by another middleware)
func getRequestIDFromContext(ctx context.Context) string {
	// This would typically be set by a request ID middleware
	// For now, return empty string
	return ""
}

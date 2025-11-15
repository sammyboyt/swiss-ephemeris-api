package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"astral-backend/pkg/auth"
	"astral-backend/pkg/errors"
	"astral-backend/pkg/logger"
	"astral-backend/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock API Key Service
type mockAPIKeyService struct {
	mock.Mock
}

func (m *mockAPIKeyService) CreateAPIKey(ctx context.Context, req models.CreateAPIKeyRequest, clientIP string) (*models.CreateAPIKeyResponse, error) {
	args := m.Called(ctx, req, clientIP)
	return args.Get(0).(*models.CreateAPIKeyResponse), args.Error(1)
}

func (m *mockAPIKeyService) ValidateAPIKey(ctx context.Context, apiKey string) (*models.APIKey, error) {
	args := m.Called(ctx, apiKey)
	return args.Get(0).(*models.APIKey), args.Error(1)
}

func (m *mockAPIKeyService) RevokeAPIKey(ctx context.Context, keyID string) error {
	args := m.Called(ctx, keyID)
	return args.Error(0)
}

func (m *mockAPIKeyService) GetAPIKey(ctx context.Context, keyID string) (models.APIKeyResponse, error) {
	args := m.Called(ctx, keyID)
	return args.Get(0).(models.APIKeyResponse), args.Error(1)
}

func (m *mockAPIKeyService) ListAPIKeys(ctx context.Context) ([]models.APIKeyResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.APIKeyResponse), args.Error(1)
}

// Helper to create test middleware
func setupAuthMiddleware(t *testing.T, service auth.APIKeyServiceInterface) *AuthMiddleware {
	logger, err := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test-auth", Environment: "test",
	})
	assert.NoError(t, err)

	return NewAuthMiddleware(service, logger, "")
}

// Helper to create test request with API key
func createTestRequest(method, url, apiKey string) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return req
}

// Test API key extraction from different sources
func TestAuthMiddleware_ExtractAPIKey(t *testing.T) {
	tests := []struct {
		name         string
		setupRequest func() *http.Request
		expectedKey  string
		expectError  bool
	}{
		{
			name: "Bearer token in Authorization header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer test-key-123")
				return req
			},
			expectedKey: "test-key-123",
			expectError: false,
		},
		{
			name: "Direct API key in Authorization header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "direct-api-key")
				return req
			},
			expectedKey: "direct-api-key",
			expectError: false,
		},
		{
			name: "X-API-Key header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Key", "x-api-key-value")
				return req
			},
			expectedKey: "x-api-key-value",
			expectError: false,
		},
		{
			name: "api_key query parameter",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test?api_key=query-api-key", nil)
				return req
			},
			expectedKey: "query-api-key",
			expectError: false,
		},
		{
			name: "no API key provided",
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			expectedKey: "",
			expectError: true,
		},
		{
			name: "priority order - Authorization header takes precedence",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test?api_key=query-key", nil)
				req.Header.Set("Authorization", "Bearer auth-key")
				req.Header.Set("X-API-Key", "header-key")
				return req
			},
			expectedKey: "auth-key",
			expectError: false,
		},
		{
			name: "priority order - X-API-Key over query param",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test?api_key=query-key", nil)
				req.Header.Set("X-API-Key", "header-key")
				return req
			},
			expectedKey: "header-key",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := setupAuthMiddleware(t, &mockAPIKeyService{})
			req := tt.setupRequest()

			key, err := middleware.extractAPIKey(req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "AUTHENTICATION_ERROR")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKey, key)
			}
		})
	}
}

// Test successful authentication
func TestAuthMiddleware_SuccessfulAuthentication(t *testing.T) {
	service := &mockAPIKeyService{}
	middleware := setupAuthMiddleware(t, service)

	// Mock successful validation
	validKey := &models.APIKey{
		Name:     "Test Key",
		Scopes:   []string{"read:ephemeris"},
		IsActive: true,
	}
	service.On("ValidateAPIKey", mock.Anything, "test-key").Return(validKey, nil)

	// Create test handler that checks context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey, ok := GetAPIKeyFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, validKey, apiKey)
		assert.Equal(t, "Test Key", apiKey.Name)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Create middleware chain
	handler := middleware.Authenticate(testHandler)

	// Create request
	req := createTestRequest("GET", "/test", "test-key")
	w := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
	service.AssertExpectations(t)
}

// Test authentication failure
func TestAuthMiddleware_AuthenticationFailure(t *testing.T) {
	service := &mockAPIKeyService{}
	middleware := setupAuthMiddleware(t, service)

	// Mock validation failure with authentication error
	service.On("ValidateAPIKey", mock.Anything, "invalid-key").Return((*models.APIKey)(nil), errors.NewAuthenticationError("Invalid key"))

	// Create middleware chain
	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	}))

	// Create request
	req := createTestRequest("GET", "/test", "invalid-key")
	w := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "AUTHENTICATION_ERROR")
	service.AssertExpectations(t)
}

// Test missing API key
func TestAuthMiddleware_MissingAPIKey(t *testing.T) {
	service := &mockAPIKeyService{}
	middleware := setupAuthMiddleware(t, service)

	// Create middleware chain
	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	}))

	// Create request without API key
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "AUTHENTICATION_ERROR")
	assert.Contains(t, w.Body.String(), "Authentication failed")
}

// Test scope requirement
func TestAuthMiddleware_RequireScope(t *testing.T) {
	service := &mockAPIKeyService{}
	middleware := setupAuthMiddleware(t, service).RequireScope("admin")

	// Mock key with insufficient scope
	validKey := &models.APIKey{
		Name:     "Test Key",
		Scopes:   []string{"read:ephemeris"}, // Missing admin scope
		IsActive: true,
	}
	service.On("ValidateAPIKey", mock.Anything, "test-key").Return(validKey, nil)

	// Create middleware chain
	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	}))

	// Create request
	req := createTestRequest("GET", "/test", "test-key")
	w := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "AUTHORIZATION_ERROR")
	service.AssertExpectations(t)
}

// Test scope requirement success
func TestAuthMiddleware_RequireScopeSuccess(t *testing.T) {
	service := &mockAPIKeyService{}
	middleware := setupAuthMiddleware(t, service).RequireScope("admin")

	// Mock key with required scope
	validKey := &models.APIKey{
		Name:     "Admin Key",
		Scopes:   []string{"read:ephemeris", "admin"},
		IsActive: true,
	}
	service.On("ValidateAPIKey", mock.Anything, "admin-key").Return(validKey, nil)

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey, ok := GetAPIKeyFromContext(r.Context())
		assert.True(t, ok)
		assert.True(t, apiKey.HasScope("admin"))
		w.WriteHeader(http.StatusOK)
	})

	// Create middleware chain
	handler := middleware.Authenticate(testHandler)

	// Create request
	req := createTestRequest("GET", "/test", "admin-key")
	w := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	service.AssertExpectations(t)
}

// Test context extraction
func TestGetAPIKeyFromContext(t *testing.T) {
	// Test with API key in context
	validKey := &models.APIKey{Name: "Test Key"}
	ctx := context.WithValue(context.Background(), APIKeyContextKey{}, validKey)

	apiKey, ok := GetAPIKeyFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, validKey, apiKey)

	// Test without API key in context
	ctx2 := context.Background()
	apiKey2, ok2 := GetAPIKeyFromContext(ctx2)
	assert.False(t, ok2)
	assert.Nil(t, apiKey2)
}

// Test convenience functions
func TestConvenienceFunctions(t *testing.T) {
	service := &mockAPIKeyService{}
	logger, _ := logger.NewLogger(logger.LogConfig{Level: "info", ServiceName: "test", Environment: "test"})

	// Test RequireAuthentication
	authFunc := RequireAuthentication(service, logger)
	assert.NotNil(t, authFunc)

	// Test RequireScope
	scopeFunc := RequireScope(service, logger, "admin")
	assert.NotNil(t, scopeFunc)

	// They should return middleware functions
	assert.NotNil(t, authFunc)
	assert.NotNil(t, scopeFunc)
}

// Test error handling
func TestAuthMiddleware_ErrorHandling(t *testing.T) {
	service := &mockAPIKeyService{}
	middleware := setupAuthMiddleware(t, service)

	// Test internal error handling
	service.On("ValidateAPIKey", mock.Anything, "error-key").Return((*models.APIKey)(nil), errors.NewInternalError(assert.AnError))

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	}))

	req := createTestRequest("GET", "/test", "error-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

// Benchmark authentication middleware
func BenchmarkAuthMiddleware_Authenticate(b *testing.B) {
	service := &mockAPIKeyService{}
	middleware := setupAuthMiddleware(&testing.T{}, service)

	// Mock successful validation
	validKey := &models.APIKey{
		Name:     "Bench Key",
		Scopes:   []string{"read:ephemeris"},
		IsActive: true,
	}
	service.On("ValidateAPIKey", mock.Anything, "bench-key").Return(validKey, nil).Maybe()

	// Simple handler
	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := createTestRequest("GET", "/benchmark", "bench-key")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

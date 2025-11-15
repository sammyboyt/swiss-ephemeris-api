package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"astral-backend/eph"
	"astral-backend/pkg/errors"
	"astral-backend/pkg/logger"
	"astral-backend/pkg/middleware"
	"astral-backend/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock API Key Service
type mockAPIKeyServiceForHandlers struct {
	mock.Mock
}

func (m *mockAPIKeyServiceForHandlers) CreateAPIKey(ctx context.Context, req models.CreateAPIKeyRequest, clientIP string) (*models.CreateAPIKeyResponse, error) {
	args := m.Called(ctx, req, clientIP)
	return args.Get(0).(*models.CreateAPIKeyResponse), args.Error(1)
}

func (m *mockAPIKeyServiceForHandlers) ValidateAPIKey(ctx context.Context, apiKey string) (*models.APIKey, error) {
	args := m.Called(ctx, apiKey)
	return args.Get(0).(*models.APIKey), args.Error(1)
}

func (m *mockAPIKeyServiceForHandlers) RevokeAPIKey(ctx context.Context, keyID string) error {
	args := m.Called(ctx, keyID)
	return args.Error(0)
}

func (m *mockAPIKeyServiceForHandlers) GetAPIKey(ctx context.Context, keyID string) (models.APIKeyResponse, error) {
	args := m.Called(ctx, keyID)
	return args.Get(0).(models.APIKeyResponse), args.Error(1)
}

func (m *mockAPIKeyServiceForHandlers) ListAPIKeys(ctx context.Context) ([]models.APIKeyResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.APIKeyResponse), args.Error(1)
}

// Test setup helpers
func setupEphemerisHandler(t *testing.T) (*EphemerisHandler, *mockAPIKeyServiceForHandlers, func()) {
	t.Helper()

	logger, err := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
	})
	require.NoError(t, err)

	service := &mockAPIKeyServiceForHandlers{}
	// Create a mock ephemeris service for testing
	mockEphService := &mockEphemerisService{}
	handler := NewEphemerisHandler(mockEphService, logger)

	return handler, service, func() {}
}

// createAuthenticatedRequest creates a request with API key authentication
func createAuthenticatedRequest(method, url, apiKey string) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return req
}

// createAuthenticatedContext creates a request context with an authenticated API key
func createAuthenticatedContext(req *http.Request, apiKey *models.APIKey) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.APIKeyContextKey{}, apiKey)
	return req.WithContext(ctx)
}

// Test successful planet calculation
func TestEphemerisHandler_GetPlanets_Success(t *testing.T) {
	handler, service, cleanup := setupEphemerisHandler(t)
	defer cleanup()

	// Mock successful authentication
	apiKey := &models.APIKey{
		Name:   "Test Key",
		Scopes: []string{"read:ephemeris"},
	}
	service.On("ValidateAPIKey", mock.Anything, "test-key").Return(apiKey, nil)

	// Create authenticated request
	req := createAuthenticatedRequest("GET", "/planets?year=2023&month=12&day=25&ut=12.0", "test-key")
	req = createAuthenticatedContext(req, apiKey)

	// Create middleware chain with authentication
	authMiddleware := middleware.NewAuthMiddleware(service, handler.logger, "")
	authenticatedHandler := authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets))

	w := httptest.NewRecorder()

	// Execute request
	authenticatedHandler.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response PlanetResponse
	err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&response)
	assert.NoError(t, err)
	assert.NotEmpty(t, response.Planets)

	// Verify we got planets (Swiss Ephemeris integration)
	assert.True(t, len(response.Planets) > 0)
	for _, planet := range response.Planets {
		assert.NotEmpty(t, planet.Name)
		assert.NotNil(t, planet.Longitude)
		assert.True(t, planet.Longitude >= 0 && planet.Longitude <= 360)
	}

	service.AssertExpectations(t)
}

// Test successful house calculation
func TestEphemerisHandler_GetHouses_Success(t *testing.T) {
	handler, service, cleanup := setupEphemerisHandler(t)
	defer cleanup()

	// Mock successful authentication
	apiKey := &models.APIKey{
		Name:   "Test Key",
		Scopes: []string{"read:ephemeris"},
	}
	service.On("ValidateAPIKey", mock.Anything, "test-key").Return(apiKey, nil)

	// Create authenticated request
	req := createAuthenticatedRequest("GET", "/houses?year=2023&month=12&day=25&ut=12.0&lat=40.7128&lng=-74.0060", "test-key")
	req = createAuthenticatedContext(req, apiKey)

	// Create middleware chain with authentication
	authMiddleware := middleware.NewAuthMiddleware(service, handler.logger, "")
	authenticatedHandler := authMiddleware.Authenticate(http.HandlerFunc(handler.GetHouses))

	w := httptest.NewRecorder()

	// Execute request
	authenticatedHandler.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var houses []map[string]interface{}
	err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&houses)
	assert.NoError(t, err)
	assert.True(t, len(houses) > 0)

	service.AssertExpectations(t)
}

// Test successful chart calculation
func TestEphemerisHandler_GetChart_Success(t *testing.T) {
	handler, service, cleanup := setupEphemerisHandler(t)
	defer cleanup()

	// Mock successful authentication
	apiKey := &models.APIKey{
		Name:   "Test Key",
		Scopes: []string{"read:ephemeris"},
	}
	service.On("ValidateAPIKey", mock.Anything, "test-key").Return(apiKey, nil)

	// Create authenticated request
	req := createAuthenticatedRequest("GET", "/chart?year=2023&month=12&day=25&ut=12.0&lat=40.7128&lng=-74.0060", "test-key")
	req = createAuthenticatedContext(req, apiKey)

	// Create middleware chain with authentication
	authMiddleware := middleware.NewAuthMiddleware(service, handler.logger, "")
	authenticatedHandler := authMiddleware.Authenticate(http.HandlerFunc(handler.GetChart))

	w := httptest.NewRecorder()

	// Execute request
	authenticatedHandler.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response ChartResponse
	err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&response)
	assert.NoError(t, err)
	assert.NotEmpty(t, response.Planets)
	assert.NotEmpty(t, response.Houses)

	service.AssertExpectations(t)
}

// Test authentication failure
func TestEphemerisHandler_AuthenticationFailure(t *testing.T) {
	handler, service, cleanup := setupEphemerisHandler(t)
	defer cleanup()

	// Mock authentication failure
	service.On("ValidateAPIKey", mock.Anything, "invalid-key").Return((*models.APIKey)(nil), errors.NewAuthenticationError("Invalid key"))

	// Create request with invalid API key
	req := createAuthenticatedRequest("GET", "/planets?year=2023&month=12&day=25&ut=12.0", "invalid-key")

	// Create middleware chain with authentication
	authMiddleware := middleware.NewAuthMiddleware(service, handler.logger, "")
	authenticatedHandler := authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets))

	w := httptest.NewRecorder()

	// Execute request
	authenticatedHandler.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var errorResponse map[string]interface{}
	err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&errorResponse)
	assert.NoError(t, err)
	assert.Contains(t, errorResponse, "error")

	service.AssertExpectations(t)
}

// Test missing authentication
func TestEphemerisHandler_MissingAuthentication(t *testing.T) {
	handler, service, cleanup := setupEphemerisHandler(t)
	defer cleanup()

	// Create request without API key
	req := httptest.NewRequest("GET", "/planets?year=2023&month=12&day=25&ut=12.0", nil)

	// Create middleware chain with authentication
	authMiddleware := middleware.NewAuthMiddleware(service, handler.logger, "")
	authenticatedHandler := authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets))

	w := httptest.NewRecorder()

	// Execute request
	authenticatedHandler.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var errorResponse map[string]interface{}
	err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&errorResponse)
	assert.NoError(t, err)
	assert.Contains(t, errorResponse, "error")

	service.AssertNotCalled(t, "ValidateAPIKey", mock.Anything, mock.Anything)
}

// Test authorization failure (insufficient scope)
func TestEphemerisHandler_AuthorizationFailure(t *testing.T) {
	handler, service, cleanup := setupEphemerisHandler(t)
	defer cleanup()

	// Mock successful authentication but insufficient scope
	apiKey := &models.APIKey{
		Name:   "Limited Key",
		Scopes: []string{"write:ephemeris"}, // Missing read:ephemeris
	}
	service.On("ValidateAPIKey", mock.Anything, "limited-key").Return(apiKey, nil)

	// Create authenticated request
	req := createAuthenticatedRequest("GET", "/planets?year=2023&month=12&day=25&ut=12.0", "limited-key")
	req = createAuthenticatedContext(req, apiKey)

	// Create middleware chain with scope requirement
	authMiddleware := middleware.NewAuthMiddleware(service, handler.logger, "read:ephemeris")
	authenticatedHandler := authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets))

	w := httptest.NewRecorder()

	// Execute request
	authenticatedHandler.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusForbidden, w.Code)

	var errorResponse map[string]interface{}
	err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&errorResponse)
	assert.NoError(t, err)
	assert.Contains(t, errorResponse, "error")

	service.AssertExpectations(t)
}

// Test invalid request parameters
func TestEphemerisHandler_InvalidParameters(t *testing.T) {
	handler, service, cleanup := setupEphemerisHandler(t)
	defer cleanup()

	// Mock successful authentication
	apiKey := &models.APIKey{
		Name:   "Test Key",
		Scopes: []string{"read:ephemeris"},
	}
	service.On("ValidateAPIKey", mock.Anything, "test-key").Return(apiKey, nil)

	testCases := []struct {
		name         string
		url          string
		expectedCode int
	}{
		{
			name:         "missing year",
			url:          "/planets?month=12&day=25&ut=12.0",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "invalid month",
			url:          "/planets?year=2023&month=13&day=25&ut=12.0",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "invalid day",
			url:          "/planets?year=2023&month=12&day=32&ut=12.0",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "invalid UT",
			url:          "/planets?year=2023&month=12&day=25&ut=25.0",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "missing coordinates for houses",
			url:          "/houses?year=2023&month=12&day=25&ut=12.0",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "invalid latitude",
			url:          "/houses?year=2023&month=12&day=25&ut=12.0&lat=91&lng=0",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "invalid longitude",
			url:          "/houses?year=2023&month=12&day=25&ut=12.0&lat=0&lng=181",
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create authenticated request
			req := createAuthenticatedRequest("GET", tc.url, "test-key")
			req = createAuthenticatedContext(req, apiKey)

			// Create middleware chain with authentication
			authMiddleware := middleware.NewAuthMiddleware(service, handler.logger, "")
			var authenticatedHandler http.Handler

			if strings.Contains(tc.url, "/houses") {
				authenticatedHandler = authMiddleware.Authenticate(http.HandlerFunc(handler.GetHouses))
			} else {
				authenticatedHandler = authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets))
			}

			w := httptest.NewRecorder()

			// Execute request
			authenticatedHandler.ServeHTTP(w, req)

			// Assert response
			assert.Equal(t, tc.expectedCode, w.Code)

			if tc.expectedCode == http.StatusBadRequest {
				var errorResponse map[string]interface{}
				err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&errorResponse)
				assert.NoError(t, err)
				assert.Contains(t, errorResponse, "error")
			}
		})
	}

	service.AssertExpectations(t)
}

// Test parameter parsing edge cases
func TestEphemerisHandler_ParameterParsing(t *testing.T) {
	handler, _, cleanup := setupEphemerisHandler(t)
	defer cleanup()

	testCases := []struct {
		name        string
		query       map[string]string
		expectError bool
	}{
		{
			name: "valid parameters",
			query: map[string]string{
				"year":  "2023",
				"month": "12",
				"day":   "25",
				"ut":    "12.5",
			},
			expectError: false,
		},
		{
			name: "NaN UT",
			query: map[string]string{
				"year":  "2023",
				"month": "12",
				"day":   "25",
				"ut":    "NaN",
			},
			expectError: true,
		},
		{
			name: "empty year",
			query: map[string]string{
				"month": "12",
				"day":   "25",
				"ut":    "12.0",
			},
			expectError: true,
		},
		{
			name: "non-numeric year",
			query: map[string]string{
				"year":  "invalid",
				"month": "12",
				"day":   "25",
				"ut":    "12.0",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock request
			req := httptest.NewRequest("GET", "/planets", nil)
			q := req.URL.Query()
			for k, v := range tc.query {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()

			_, err := handler.parsePlanetRequest(req)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test response formatting
func TestEphemerisHandler_ResponseFormatting(t *testing.T) {
	handler, service, cleanup := setupEphemerisHandler(t)
	defer cleanup()

	// Mock successful authentication
	apiKey := &models.APIKey{
		Name:   "Test Key",
		Scopes: []string{"read:ephemeris"},
	}
	service.On("ValidateAPIKey", mock.Anything, "test-key").Return(apiKey, nil)

	// Create authenticated request
	req := createAuthenticatedRequest("GET", "/planets?year=2023&month=12&day=25&ut=12.0", "test-key")
	req = createAuthenticatedContext(req, apiKey)

	// Create middleware chain with authentication
	authMiddleware := middleware.NewAuthMiddleware(service, handler.logger, "")
	authenticatedHandler := authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets))

	w := httptest.NewRecorder()

	// Execute request
	authenticatedHandler.ServeHTTP(w, req)

	// Assert response format
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response PlanetResponse
	err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&response)
	assert.NoError(t, err)

	// Verify JSON structure
	assert.NotNil(t, response.Planets)
	assert.IsType(t, []eph.Planet{}, response.Planets)

	service.AssertExpectations(t)
}

// Benchmark ephemeris calculations
func BenchmarkEphemerisHandler_GetPlanets(b *testing.B) {
	handler, service, _ := setupEphemerisHandler(&testing.T{})

	// Mock successful authentication
	apiKey := &models.APIKey{
		Name:   "Bench Key",
		Scopes: []string{"read:ephemeris"},
	}
	service.On("ValidateAPIKey", mock.Anything, "bench-key").Return(apiKey, nil).Maybe()

	// Create authenticated request
	req := createAuthenticatedRequest("GET", "/planets?year=2023&month=12&day=25&ut=12.0", "bench-key")
	req = createAuthenticatedContext(req, apiKey)

	// Create middleware chain with authentication
	authMiddleware := middleware.NewAuthMiddleware(service, handler.logger, "")
	authenticatedHandler := authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		authenticatedHandler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkEphemerisHandler_GetChart(b *testing.B) {
	handler, service, _ := setupEphemerisHandler(&testing.T{})

	// Mock successful authentication
	apiKey := &models.APIKey{
		Name:   "Bench Key",
		Scopes: []string{"read:ephemeris"},
	}
	service.On("ValidateAPIKey", mock.Anything, "bench-key").Return(apiKey, nil).Maybe()

	// Create authenticated request
	req := createAuthenticatedRequest("GET", "/chart?year=2023&month=12&day=25&ut=12.0&lat=40.7128&lng=-74.0060", "bench-key")
	req = createAuthenticatedContext(req, apiKey)

	// Create middleware chain with authentication
	authMiddleware := middleware.NewAuthMiddleware(service, handler.logger, "")
	authenticatedHandler := authMiddleware.Authenticate(http.HandlerFunc(handler.GetChart))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		authenticatedHandler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// mockEphemerisService implements EphemerisService for testing
type mockEphemerisService struct{}

func (m *mockEphemerisService) GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]eph.Planet, error) {
	return []eph.Planet{
		{Name: "Sun", Longitude: 45.5, Retrograde: false},
		{Name: "Moon", Longitude: 120.3, Retrograde: false},
	}, nil
}

func (m *mockEphemerisService) GetHousesCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]eph.House, error) {
	houses := make([]eph.House, 12)
	for i := 0; i < 12; i++ {
		houses[i] = eph.House{
			ID:        i + 1,
			Longitude: float64(i * 30),
			Hsys:      "P",
		}
	}
	return houses, nil
}

func (m *mockEphemerisService) GetChartCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]eph.Planet, []eph.House, error) {
	planets, _ := m.GetPlanetsCached(ctx, yr, mon, day, ut)
	houses, _ := m.GetHousesCached(ctx, yr, mon, day, ut, lat, lng)
	return planets, houses, nil
}

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	var response CelestialBodyResponse
	err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&response)
	assert.NoError(t, err)
	assert.NotEmpty(t, response.Bodies)

	// Verify we got planets (Swiss Ephemeris integration)
	assert.True(t, len(response.Bodies) > 0)
	for _, body := range response.Bodies {
		assert.NotEmpty(t, body.Name)
		assert.NotNil(t, body.Longitude)
		assert.True(t, body.Longitude >= 0 && body.Longitude <= 360)
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
	assert.NotEmpty(t, response.Bodies)
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

	var response CelestialBodyResponse
	err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&response)
	assert.NoError(t, err)

	// Verify JSON structure
	assert.NotNil(t, response.Bodies)
	assert.IsType(t, []eph.CelestialBody{}, response.Bodies)

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
type mockEphemerisService struct {
	mock.Mock
}

func (m *mockEphemerisService) GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]eph.CelestialBody, error) {
	// Always return default mock data for backward compatibility
	// This legacy test doesn't set up expectations, so just provide mock data
	return []eph.CelestialBody{
		{ID: 0, Name: "Sun", Type: eph.TypePlanet, Longitude: 45.5},
		{ID: 1, Name: "Moon", Type: eph.TypePlanet, Longitude: 120.3},
	}, nil
}

func (m *mockEphemerisService) GetHousesCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]eph.House, error) {
	// Always return default mock data for backward compatibility
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

func (m *mockEphemerisService) GetChartCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]eph.CelestialBody, []eph.House, error) {
	// Always return default mock data for backward compatibility
	// This legacy test doesn't set up expectations, so just provide mock data
	planets := []eph.CelestialBody{
		{ID: 0, Name: "Sun", Type: eph.TypePlanet, Longitude: 45.5},
		{ID: 1, Name: "Moon", Type: eph.TypePlanet, Longitude: 120.3},
	}
	houses := make([]eph.House, 12)
	for i := 0; i < 12; i++ {
		houses[i] = eph.House{
			ID:        i + 1,
			Longitude: float64(i * 30),
			Hsys:      "P",
		}
	}
	return planets, houses, nil
}

func (m *mockEphemerisService) CalculateBodies(ctx context.Context, time eph.AstroTimeRequest, config eph.EphemerisConfig) (*eph.EphemerisResult, error) {
	args := m.Called(ctx, time, config)
	return args.Get(0).(*eph.EphemerisResult), args.Error(1)
}

func (m *mockEphemerisService) CalculateBodiesCached(ctx context.Context, time eph.AstroTimeRequest, config eph.EphemerisConfig) (*eph.EphemerisResult, error) {
	args := m.Called(ctx, time, config)
	return args.Get(0).(*eph.EphemerisResult), args.Error(1)
}

func (m *mockEphemerisService) GetTraditionalBodies(ctx context.Context, time eph.AstroTimeRequest) ([]eph.CelestialBody, error) {
	args := m.Called(ctx, time)
	return args.Get(0).([]eph.CelestialBody), args.Error(1)
}

func (m *mockEphemerisService) GetExtendedBodies(ctx context.Context, time eph.AstroTimeRequest, types []eph.CelestialBodyType) ([]eph.CelestialBody, error) {
	args := m.Called(ctx, time, types)
	return args.Get(0).([]eph.CelestialBody), args.Error(1)
}

func (m *mockEphemerisService) GetFixedStars(ctx context.Context, time eph.AstroTimeRequest, constellations []string) ([]eph.Constellation, error) {
	args := m.Called(ctx, time, constellations)
	return args.Get(0).([]eph.Constellation), args.Error(1)
}

func (m *mockEphemerisService) GetFullChart(ctx context.Context, time eph.AstroTimeRequest, lat, lng float64) (*eph.EphemerisResult, error) {
	args := m.Called(ctx, time, lat, lng)
	return args.Get(0).(*eph.EphemerisResult), args.Error(1)
}

// New handler tests for CelestialBody-based endpoints

func TestEphemerisHandler_GetBodies(t *testing.T) {
	t.Run("success_with_traditional_bodies", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{
			Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
		})
		require.NoError(t, err)
		require.NotNil(t, testLogger)
		t.Logf("Logger created successfully: %p", testLogger)
		handler := NewEphemerisHandler(mockService, testLogger)
		require.NotNil(t, handler.logger)
		t.Logf("Handler created with logger: %p", handler.logger)

		expectedResult := &eph.EphemerisResult{
			Bodies: []eph.CelestialBody{
				{ID: 0, Name: "Sun", Type: eph.TypePlanet, Longitude: 280.45},
				{ID: 1, Name: "Moon", Type: eph.TypePlanet, Longitude: 123.67},
			},
			Metadata: eph.CalculationMetadata{
				BodiesCalculated: 2,
				Cached:           false,
			},
		}

		mockService.On("CalculateBodies", mock.Anything, mock.Anything, mock.Anything).Return(expectedResult, nil)

		// Create request with request ID middleware to set up logger context
		req := httptest.NewRequest("GET", "/api/v1/bodies?year=2024&month=1&day=1&traditional=true", nil)
		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetBodies(w, r)
		})).ServeHTTP(w, req)

		// Verify
		assert.Equal(t, http.StatusOK, w.Code)

		var response CelestialBodyResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Len(t, response.Bodies, 2)
		assert.Equal(t, "Sun", response.Bodies[0].Name)
		assert.Equal(t, "Moon", response.Bodies[1].Name)

		mockService.AssertExpectations(t)
	})

	t.Run("success_with_extended_bodies", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{
			Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
		})
		require.NoError(t, err)
		require.NotNil(t, testLogger)
		handler := NewEphemerisHandler(mockService, testLogger)

		expectedResult := &eph.EphemerisResult{
			Bodies: []eph.CelestialBody{
				{ID: 17, Name: "Ceres", Type: eph.TypeAsteroid, Longitude: 123.45},
				{ID: 15, Name: "Chiron", Type: eph.TypeCentaur, Longitude: 234.56},
			},
			Metadata: eph.CalculationMetadata{
				BodiesCalculated: 2,
				Cached:           true,
			},
		}

		mockService.On("CalculateBodies", mock.Anything, mock.Anything, mock.Anything).Return(expectedResult, nil)

		// Create request with request ID middleware to set up logger context
		req := httptest.NewRequest("GET", "/api/v1/bodies?year=2024&month=1&day=1&asteroids=true&centaurs=true", nil)
		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetBodies(w, r)
		})).ServeHTTP(w, req)

		// Verify
		assert.Equal(t, http.StatusOK, w.Code)

		var response CelestialBodyResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Len(t, response.Bodies, 2)
		assert.Equal(t, eph.TypeAsteroid, response.Bodies[0].Type)
		assert.Equal(t, eph.TypeCentaur, response.Bodies[1].Type)

		mockService.AssertExpectations(t)
	})

	t.Run("invalid_time_parameters", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{
			Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
		})
		require.NoError(t, err)
		require.NotNil(t, testLogger)
		handler := NewEphemerisHandler(mockService, testLogger)

		// Create request with invalid year
		req := httptest.NewRequest("GET", "/api/v1/bodies?year=invalid&month=1&day=1", nil)
		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetBodies(w, r)
		})).ServeHTTP(w, req)

		// Verify
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errorResp map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&errorResp)
		assert.NoError(t, err)
		assert.Contains(t, errorResp, "error")
	})
}

func TestEphemerisHandler_GetTraditionalBodies(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{
			Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
		})
		require.NoError(t, err)
		require.NotNil(t, testLogger)
		handler := NewEphemerisHandler(mockService, testLogger)

		expectedBodies := []eph.CelestialBody{
			{ID: 0, Name: "Sun", Type: eph.TypePlanet},
			{ID: 1, Name: "Moon", Type: eph.TypePlanet},
			{ID: 2, Name: "Mercury", Type: eph.TypePlanet},
			{ID: 3, Name: "Venus", Type: eph.TypePlanet},
			{ID: 4, Name: "Mars", Type: eph.TypePlanet},
			{ID: 5, Name: "Jupiter", Type: eph.TypePlanet},
			{ID: 6, Name: "Saturn", Type: eph.TypePlanet},
			{ID: 7, Name: "Uranus", Type: eph.TypePlanet},
			{ID: 8, Name: "Neptune", Type: eph.TypePlanet},
			{ID: 9, Name: "Pluto", Type: eph.TypePlanet},
		}

		mockService.On("GetTraditionalBodies", mock.Anything, mock.Anything).Return(expectedBodies, nil)

		// Create request with request ID middleware to set up logger context
		req := httptest.NewRequest("GET", "/api/v1/traditional?year=2024&month=1&day=1", nil)
		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetTraditionalBodies(w, r)
		})).ServeHTTP(w, req)

		// Verify
		assert.Equal(t, http.StatusOK, w.Code)

		var response CelestialBodyResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Len(t, response.Bodies, 10)

		// Check first few bodies
		assert.Equal(t, "Sun", response.Bodies[0].Name)
		assert.Equal(t, "Moon", response.Bodies[1].Name)
		assert.Equal(t, "Saturn", response.Bodies[6].Name)

		mockService.AssertExpectations(t)
	})
}

func TestEphemerisHandler_GetExtendedBodies(t *testing.T) {
	t.Run("success_with_multiple_types", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{
			Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
		})
		require.NoError(t, err)
		require.NotNil(t, testLogger)
		handler := NewEphemerisHandler(mockService, testLogger)

		expectedBodies := []eph.CelestialBody{
			{ID: 10, Name: "Mean Node", Type: eph.TypeNode},
			{ID: 17, Name: "Ceres", Type: eph.TypeAsteroid},
			{ID: 15, Name: "Chiron", Type: eph.TypeCentaur},
		}

		mockService.On("GetExtendedBodies", mock.Anything, mock.Anything, mock.AnythingOfType("[]eph.CelestialBodyType")).Return(expectedBodies, nil)

		// Create request with request ID middleware to set up logger context
		req := httptest.NewRequest("GET", "/api/v1/extended?year=2024&month=1&day=1&types=node,asteroid,centaur", nil)
		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetExtendedBodies(w, r)
		})).ServeHTTP(w, req)

		// Verify
		assert.Equal(t, http.StatusOK, w.Code)

		var response CelestialBodyResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Len(t, response.Bodies, 3)

		// Check body types
		bodyTypes := make(map[eph.CelestialBodyType]bool)
		for _, body := range response.Bodies {
			bodyTypes[body.Type] = true
		}
		assert.True(t, bodyTypes[eph.TypeNode])
		assert.True(t, bodyTypes[eph.TypeAsteroid])
		assert.True(t, bodyTypes[eph.TypeCentaur])

		mockService.AssertExpectations(t)
	})

	t.Run("default_to_all_extended_types", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{
			Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
		})
		require.NoError(t, err)
		require.NotNil(t, testLogger)
		handler := NewEphemerisHandler(mockService, testLogger)

		expectedBodies := []eph.CelestialBody{
			{ID: 10, Name: "Mean Node", Type: eph.TypeNode},
			{ID: 17, Name: "Ceres", Type: eph.TypeAsteroid},
		}

		mockService.On("GetExtendedBodies", mock.Anything, mock.Anything, mock.AnythingOfType("[]eph.CelestialBodyType")).Return(expectedBodies, nil)

		// Create request with request ID middleware to set up logger context
		req := httptest.NewRequest("GET", "/api/v1/extended?year=2024&month=1&day=1", nil)
		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetExtendedBodies(w, r)
		})).ServeHTTP(w, req)

		// Verify
		assert.Equal(t, http.StatusOK, w.Code)

		var response CelestialBodyResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.NotEmpty(t, response.Bodies)

		mockService.AssertExpectations(t)
	})
}

func TestEphemerisHandler_GetFixedStars(t *testing.T) {
	t.Run("success_with_single_constellation", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{
			Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
		})
		require.NoError(t, err)
		require.NotNil(t, testLogger)
		handler := NewEphemerisHandler(mockService, testLogger)

		expectedConstellations := []eph.Constellation{
			{
				Name:      "Leo",
				Abbrev:    "Leo",
				LatinName: "Leo",
				Stars: []eph.CelestialBody{
					{
						ID:            -1,
						Name:          "Regulus",
						Type:          eph.TypeFixedStar,
						Constellation: stringPtr("Leo"),
						Longitude:     150.23,
						Magnitude:     floatPtr(1.35),
					},
				},
				StarCount: 1,
			},
		}

		mockService.On("GetFixedStars", mock.Anything, mock.Anything, mock.AnythingOfType("[]string")).Return(expectedConstellations, nil)

		// Create request with request ID middleware to set up logger context
		req := httptest.NewRequest("GET", "/api/v1/fixed-stars?year=2024&month=1&day=1&constellations=Leo", nil)
		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetFixedStars(w, r)
		})).ServeHTTP(w, req)

		// Verify
		assert.Equal(t, http.StatusOK, w.Code)

		var response eph.EphemerisResult
		err = json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Len(t, response.Constellations, 1)
		assert.Equal(t, "Leo", response.Constellations[0].Abbrev)
		assert.Len(t, response.Constellations[0].Stars, 1)
		assert.Equal(t, "Regulus", response.Constellations[0].Stars[0].Name)
		assert.Equal(t, eph.TypeFixedStar, response.Constellations[0].Stars[0].Type)

		mockService.AssertExpectations(t)
	})

	t.Run("success_with_multiple_constellations", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{
			Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
		})
		require.NoError(t, err)
		require.NotNil(t, testLogger)
		handler := NewEphemerisHandler(mockService, testLogger)

		expectedConstellations := []eph.Constellation{
			{Name: "Leo", Abbrev: "Leo", StarCount: 5},
			{Name: "Virgo", Abbrev: "Vir", StarCount: 3},
		}

		mockService.On("GetFixedStars", mock.Anything, mock.Anything, mock.AnythingOfType("[]string")).Return(expectedConstellations, nil)

		// Create request with request ID middleware to set up logger context
		req := httptest.NewRequest("GET", "/api/v1/fixed-stars?year=2024&month=1&day=1&constellations=Leo,Virgo", nil)
		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetFixedStars(w, r)
		})).ServeHTTP(w, req)

		// Verify
		assert.Equal(t, http.StatusOK, w.Code)

		var response eph.EphemerisResult
		err = json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Len(t, response.Constellations, 2)

		// Check constellation names
		names := make(map[string]bool)
		for _, constell := range response.Constellations {
			names[constell.Abbrev] = true
		}
		assert.True(t, names["Leo"])
		assert.True(t, names["Vir"])

		mockService.AssertExpectations(t)
	})
}

func TestEphemerisHandler_GetFullChart(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{
			Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
		})
		require.NoError(t, err)
		require.NotNil(t, testLogger)
		handler := NewEphemerisHandler(mockService, testLogger)

		expectedResult := &eph.EphemerisResult{
			Bodies: []eph.CelestialBody{
				{ID: 0, Name: "Sun", Type: eph.TypePlanet, Longitude: 280.45},
				{ID: 1, Name: "Moon", Type: eph.TypePlanet, Longitude: 123.67},
			},
			Houses: []eph.House{
				{ID: 1, Longitude: 123.45, Hsys: "P"},
				{ID: 2, Longitude: 153.67, Hsys: "P"},
				{ID: 3, Longitude: 183.89, Hsys: "P"},
				{ID: 4, Longitude: 214.12, Hsys: "P"},
				{ID: 5, Longitude: 244.34, Hsys: "P"},
				{ID: 6, Longitude: 274.56, Hsys: "P"},
				{ID: 7, Longitude: 304.78, Hsys: "P"},
				{ID: 8, Longitude: 335.01, Hsys: "P"},
				{ID: 9, Longitude: 5.23, Hsys: "P"},
				{ID: 10, Longitude: 35.45, Hsys: "P"},
				{ID: 11, Longitude: 65.67, Hsys: "P"},
				{ID: 12, Longitude: 95.89, Hsys: "P"},
			},
			Metadata: eph.CalculationMetadata{
				BodiesCalculated: 2, // Only celestial bodies, not houses
				Cached:           false,
			},
		}

		mockService.On("GetFullChart", mock.Anything, mock.Anything, 40.7128, -74.0060).Return(expectedResult, nil)

		// Create request with request ID middleware to set up logger context
		req := httptest.NewRequest("GET", "/api/v1/full-chart?year=2024&month=1&day=1&lat=40.7128&lng=-74.0060", nil)
		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetFullChart(w, r)
		})).ServeHTTP(w, req)

		// Verify
		assert.Equal(t, http.StatusOK, w.Code)

		var response eph.EphemerisResult
		err = json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.NotEmpty(t, response.Bodies)
		assert.NotEmpty(t, response.Houses, "Should have houses in separate field")
		assert.Greater(t, response.Metadata.BodiesCalculated, 0)

		// Should contain celestial bodies and houses in separate fields
		hasBodies := false
		for _, body := range response.Bodies {
			if body.Type == eph.TypePlanet {
				hasBodies = true
			}
		}
		assert.True(t, hasBodies, "Should have celestial bodies")
		assert.Len(t, response.Houses, 12, "Should have 12 houses")

		// Verify houses have correct IDs and properties
		for _, house := range response.Houses {
			assert.True(t, house.ID >= 1 && house.ID <= 12, "House ID should be between 1 and 12")
			assert.True(t, house.Longitude >= 0 && house.Longitude < 360, "House longitude should be valid")
			assert.Equal(t, "P", house.Hsys, "House system should be Placidus")
		}

		mockService.AssertExpectations(t)
	})
}

// TestEphemerisHandler_GetFullChart_CompleteValidation tests comprehensive validation of full chart response
func TestEphemerisHandler_GetFullChart_CompleteValidation(t *testing.T) {
	t.Run("response_structure_validation", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{
			Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
		})
		require.NoError(t, err)
		require.NotNil(t, testLogger)
		handler := NewEphemerisHandler(mockService, testLogger)

		expectedResult := &eph.EphemerisResult{
			Bodies: []eph.CelestialBody{
				{ID: 0, Name: "Sun", Type: eph.TypePlanet, Longitude: 280.45},
				{ID: 1, Name: "Moon", Type: eph.TypePlanet, Longitude: 123.67},
				{ID: 2, Name: "Mercury", Type: eph.TypePlanet, Longitude: 275.12},
			},
			Houses: []eph.House{
				{ID: 1, Longitude: 123.45, Hsys: "P"},
				{ID: 2, Longitude: 153.67, Hsys: "P"},
				{ID: 3, Longitude: 183.89, Hsys: "P"},
				{ID: 4, Longitude: 214.12, Hsys: "P"},
				{ID: 5, Longitude: 244.34, Hsys: "P"},
				{ID: 6, Longitude: 274.56, Hsys: "P"},
				{ID: 7, Longitude: 304.78, Hsys: "P"},
				{ID: 8, Longitude: 335.01, Hsys: "P"},
				{ID: 9, Longitude: 5.23, Hsys: "P"},
				{ID: 10, Longitude: 35.45, Hsys: "P"},
				{ID: 11, Longitude: 65.67, Hsys: "P"},
				{ID: 12, Longitude: 95.89, Hsys: "P"},
			},
			Metadata: eph.CalculationMetadata{
				BodiesCalculated: 3,
				Cached:           false,
			},
			Timestamp: time.Now(),
		}

		mockService.On("GetFullChart", mock.Anything, mock.Anything, 40.7128, -74.0060).Return(expectedResult, nil)

		// Create request with request ID middleware
		req := httptest.NewRequest("GET", "/api/v1/full-chart?year=2024&month=1&day=1&lat=40.7128&lng=-74.0060", nil)
		w := httptest.NewRecorder()

		// Apply request ID middleware
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.GetFullChart(w, r)
		})).ServeHTTP(w, req)

		// Verify response
		assert.Equal(t, http.StatusOK, w.Code)

		var response eph.EphemerisResult
		err = json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)

		// Validate response structure
		assert.NotEmpty(t, response.Bodies, "Should have bodies")
		assert.NotEmpty(t, response.Houses, "Should have houses")
		assert.Len(t, response.Houses, 12, "Should have exactly 12 houses")

		// Validate bodies are only celestial bodies, not houses
		for _, body := range response.Bodies {
			assert.NotEqual(t, "house", body.Type, "Bodies should not contain house objects")
			assert.Contains(t, []string{"planet", "asteroid", "centaur"}, string(body.Type), "Body should be celestial object")
		}

		// Validate houses are properly structured
		for _, house := range response.Houses {
			assert.True(t, house.ID >= 1 && house.ID <= 12, "House ID should be 1-12")
			assert.True(t, house.Longitude >= 0 && house.Longitude < 360, "House longitude should be valid")
			assert.Equal(t, "P", house.Hsys, "House system should be Placidus")
		}

		// Validate all house IDs 1-12 are present
		houseIDs := make(map[int]bool)
		for _, house := range response.Houses {
			houseIDs[house.ID] = true
		}
		for i := 1; i <= 12; i++ {
			assert.True(t, houseIDs[i], "House ID %d should be present", i)
		}

		mockService.AssertExpectations(t)
	})

	t.Run("error_handling", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{
			Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
		})
		require.NoError(t, err)
		handler := NewEphemerisHandler(mockService, testLogger)

		mockService.On("GetFullChart", mock.Anything, mock.Anything, 40.7128, -74.0060).Return((*eph.EphemerisResult)(nil), fmt.Errorf("calculation failed"))

		// Create request
		req := httptest.NewRequest("GET", "/api/v1/full-chart?year=2024&month=1&day=1&lat=40.7128&lng=-74.0060", nil)
		w := httptest.NewRecorder()

		// Apply request ID middleware
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.GetFullChart(w, r)
		})).ServeHTTP(w, req)

		// Verify error response
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var errorResp map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&errorResp)
		assert.NoError(t, err)
		assert.Contains(t, errorResp, "error")
		assert.Equal(t, "EPHEMERIS_ERROR", errorResp["error"].(map[string]interface{})["code"])

		mockService.AssertExpectations(t)
	})
}

func TestEphemerisHandler_GetFullChart_MissingLocation(t *testing.T) {
	t.Run("missing_location_parameters", func(t *testing.T) {
		// Setup
		mockService := &mockEphemerisService{}
		testLogger, err := logger.NewLogger(logger.LogConfig{Level: "info"})
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}
		if testLogger == nil {
			t.Fatal("Logger should not be nil")
		}
		handler := NewEphemerisHandler(mockService, testLogger)

		// Create request without lat/lng
		req := httptest.NewRequest("GET", "/api/v1/full-chart?year=2024&month=1&day=1", nil)
		w := httptest.NewRecorder()

		// Execute
		handler.GetFullChart(w, req)

		// Verify
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errorResp map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&errorResp)
		assert.NoError(t, err)
		assert.Contains(t, errorResp, "error")
	})
}

// TestEphemerisHandler_GetFixedStars_ZodiacParameter tests Zodiac constellation API support
func TestEphemerisHandler_GetFixedStars_ZodiacParameter(t *testing.T) {
	t.Run("zodiac_parameter_expansion", func(t *testing.T) {
		mockService := &mockEphemerisService{}
		testLogger := createTestLogger(t)
		handler := NewEphemerisHandler(mockService, testLogger)

		// Mock service to return stars from all zodiac constellations
		expectedResult := &eph.EphemerisResult{
			Constellations: []eph.Constellation{
				{Name: "Leo", Abbrev: "Leo", Stars: []eph.CelestialBody{
					{Name: "Regulus", Type: eph.TypeFixedStar, Constellation: stringPtr("Leo")},
				}},
				{Name: "Virgo", Abbrev: "Vir", Stars: []eph.CelestialBody{
					{Name: "Spica", Type: eph.TypeFixedStar, Constellation: stringPtr("Vir")},
				}},
			},
		}

		// Expect the service to be called with all zodiac member constellations
		zodiacMembers := []string{"Ari", "Tau", "Gem", "Cnc", "Leo", "Vir", "Lib", "Sco", "Sgr", "Cap", "Aqr", "Psc"}
		mockService.On("GetFixedStars", mock.Anything, mock.Anything, mock.MatchedBy(func(consts []string) bool {
			// Check that all zodiac members are included
			for _, member := range zodiacMembers {
				if !containsString(consts, member) {
					return false
				}
			}
			return true
		})).Return(expectedResult.Constellations, nil)

		// Test API call with Zodiac parameter
		req := createAuthenticatedRequest("GET", "/api/v1/fixed-stars?year=2024&month=1&day=1&constellations=Zodiac", "test-key")
		req = createAuthenticatedContext(req, &models.APIKey{Name: "Test Key", Scopes: []string{"read:ephemeris"}})

		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetFixedStars(w, r)
		})).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response eph.EphemerisResult
		err := json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.NotEmpty(t, response.Constellations)

		mockService.AssertExpectations(t)
	})

	t.Run("zodiac_plus_additional_constellations", func(t *testing.T) {
		mockService := &mockEphemerisService{}
		testLogger := createTestLogger(t)
		handler := NewEphemerisHandler(mockService, testLogger)

		expectedResult := &eph.EphemerisResult{
			Constellations: []eph.Constellation{
				{Name: "Leo", Abbrev: "Leo", Stars: []eph.CelestialBody{}},
				{Name: "Ursa Major", Abbrev: "UMa", Stars: []eph.CelestialBody{}},
			},
		}

		// Should expand Zodiac + include Ursa Major
		mockService.On("GetFixedStars", mock.Anything, mock.Anything, mock.MatchedBy(func(consts []string) bool {
			zodiacMembers := []string{"Ari", "Tau", "Gem", "Cnc", "Leo", "Vir", "Lib", "Sco", "Sgr", "Cap", "Aqr", "Psc"}
			hasLeo := containsString(consts, "Leo")
			hasUMa := containsString(consts, "UMa")
			hasAllZodiac := true
			for _, member := range zodiacMembers {
				if !containsString(consts, member) {
					hasAllZodiac = false
					break
				}
			}
			return hasLeo && hasUMa && hasAllZodiac
		})).Return(expectedResult.Constellations, nil)

		req := createAuthenticatedRequest("GET", "/api/v1/fixed-stars?year=2024&month=1&day=1&constellations=Zodiac,UMa", "test-key")
		req = createAuthenticatedContext(req, &models.APIKey{Name: "Test Key", Scopes: []string{"read:ephemeris"}})

		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetFixedStars(w, r)
		})).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("empty_constellations_defaults_to_all", func(t *testing.T) {
		mockService := &mockEphemerisService{}
		testLogger := createTestLogger(t)
		handler := NewEphemerisHandler(mockService, testLogger)

		// When no constellations specified, should return all available
		mockService.On("GetFixedStars", mock.Anything, mock.Anything, []string{}).
			Return([]eph.Constellation{}, nil)

		req := createAuthenticatedRequest("GET", "/api/v1/fixed-stars?year=2024&month=1&day=1", "test-key")
		req = createAuthenticatedContext(req, &models.APIKey{Name: "Test Key", Scopes: []string{"read:ephemeris"}})

		w := httptest.NewRecorder()

		// Apply request ID middleware to set up logger context
		requestIDMiddleware := middleware.NewRequestIDMiddleware(testLogger)
		requestIDMiddleware.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Now execute the actual handler with the context set up
			handler.GetFixedStars(w, r)
		})).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})
}

// Helper function for string slice contains check
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// createTestLogger creates a development logger for testing
func createTestLogger(t *testing.T) *logger.Logger {
	t.Helper()

	testLogger, err := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test-ephemeris", Environment: "test",
	})
	require.NoError(t, err)
	require.NotNil(t, testLogger)
	return testLogger
}

// Helper functions for tests
func stringPtr(s string) *string {
	return &s
}

func floatPtr(f float64) *float64 {
	return &f
}

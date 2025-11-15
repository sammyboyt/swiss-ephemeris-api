//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"astral-backend/eph"
	"astral-backend/pkg/auth"
	"astral-backend/pkg/handlers"
	"astral-backend/pkg/logger"
	"astral-backend/pkg/middleware"
	"astral-backend/pkg/models"
	"astral-backend/pkg/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestAPIKeyLifecycle_E2E is a comprehensive end-to-end test for the complete API key lifecycle
func TestAPIKeyLifecycle_E2E(t *testing.T) {
	// Skip if not running e2e tests or MongoDB not available
	if os.Getenv("RUN_E2E") != "true" {
		t.Skip("Skipping E2E tests - set RUN_E2E=true to run")
	}

	// Setup test database
	db, cleanup := setupE2EDatabase(t)
	defer cleanup()

	// Setup components
	logger, err := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "e2e-test", Environment: "test",
	})
	require.NoError(t, err)

	repo := repository.NewAPIKeyRepository(db, logger)

	// Create concrete implementations for E2E testing
	hasher := &auth.SecurePasswordHasher{}
	keyGen := &auth.SecureKeyGenerator{}
	idGen := &auth.UUIDGenerator{}

	service := auth.NewAPIKeyService(repo, hasher, idGen, keyGen, logger) // We'll create these later
	// Create cached ephemeris service for E2E tests
	cachedService := eph.NewCachedEphemerisService(nil, logger.Logger) // nil cache = direct calculations
	handler := handlers.NewEphemerisHandler(cachedService, logger)

	// Create test HTTP server with middleware
	mux := http.NewServeMux()
	authMiddleware := middleware.NewAuthMiddleware(service, logger, "")

	// Setup routes with authentication
	mux.Handle("/planets", authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets)))
	mux.Handle("/houses", authMiddleware.RequireScope("read:ephemeris").Authenticate(http.HandlerFunc(handler.GetHouses)))
	mux.Handle("/chart", authMiddleware.RequireScope("read:ephemeris").Authenticate(http.HandlerFunc(handler.GetChart)))

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test Phase 1: API Key Creation
	t.Run("APIKeyCreation", func(t *testing.T) {
		ctx := context.Background()

		// Create an API key
		req := models.CreateAPIKeyRequest{
			Name:        "E2E Test Key",
			Description: "End-to-end testing key",
			Scopes:      []string{"read:ephemeris", "admin"},
		}

		response, err := service.CreateAPIKey(ctx, req, "127.0.0.1")
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, "E2E Test Key", response.Name)
		assert.Contains(t, response.Scopes, "read:ephemeris")
		assert.NotEmpty(t, response.Key) // Plain key should be returned

		// Store the API key for later tests
		apiKey := response.Key
		keyID := response.ID

		// Test Phase 2: API Key Validation
		t.Run("APIKeyValidation", func(t *testing.T) {
			// Add a small delay to ensure database consistency
			time.Sleep(100 * time.Millisecond)

			validatedKey, err := service.ValidateAPIKey(ctx, apiKey)
			require.NoError(t, err)
			assert.Equal(t, "E2E Test Key", validatedKey.Name)
			assert.Contains(t, validatedKey.Scopes, "read:ephemeris")
			assert.True(t, validatedKey.IsActive)
		})

		// Test Phase 3: Authenticated API Calls
		t.Run("AuthenticatedAPICalls", func(t *testing.T) {
			testCases := []struct {
				name           string
				url            string
				expectedStatus int
				checkResponse  func(t *testing.T, body []byte)
			}{
				{
					name:           "Get Planets - Success",
					url:            "/planets?year=2023&month=12&day=25&ut=12.0",
					expectedStatus: http.StatusOK,
					checkResponse: func(t *testing.T, body []byte) {
						var response handlers.PlanetResponse
						err := json.Unmarshal(body, &response)
						require.NoError(t, err)
						assert.NotEmpty(t, response.Planets)
						assert.Len(t, response.Planets, 12) // Should have all planets
						for _, planet := range response.Planets {
							assert.NotEmpty(t, planet.Name)
							assert.True(t, planet.Longitude >= 0 && planet.Longitude <= 360)
						}
					},
				},
				{
					name:           "Get Houses - Success",
					url:            "/houses?year=2023&month=12&day=25&ut=12.0&lat=40.7128&lng=-74.0060",
					expectedStatus: http.StatusOK,
					checkResponse: func(t *testing.T, body []byte) {
						var houses []eph.House
						err := json.Unmarshal(body, &houses)
						require.NoError(t, err)
						assert.NotEmpty(t, houses)
						// Should have houses from multiple systems (P, K, E, W)
						assert.True(t, len(houses) >= 12)
					},
				},
				{
					name:           "Get Chart - Success",
					url:            "/chart?year=2023&month=12&day=25&ut=12.0&lat=40.7128&lng=-74.0060",
					expectedStatus: http.StatusOK,
					checkResponse: func(t *testing.T, body []byte) {
						var response handlers.ChartResponse
						err := json.Unmarshal(body, &response)
						require.NoError(t, err)
						assert.NotEmpty(t, response.Planets)
						assert.NotEmpty(t, response.Houses)
						assert.Len(t, response.Planets, 12)
					},
				},
				{
					name:           "Invalid Parameters",
					url:            "/planets?year=invalid&month=12&day=25&ut=12.0",
					expectedStatus: http.StatusBadRequest,
					checkResponse: func(t *testing.T, body []byte) {
						var errorResp map[string]interface{}
						err := json.Unmarshal(body, &errorResp)
						require.NoError(t, err)
						assert.Contains(t, errorResp, "error")
					},
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					// Create request with API key
					req, err := http.NewRequest("GET", server.URL+tc.url, nil)
					require.NoError(t, err)
					req.Header.Set("Authorization", "Bearer "+apiKey)

					client := &http.Client{Timeout: 10 * time.Second}
					resp, err := client.Do(req)
					require.NoError(t, err)
					defer resp.Body.Close()

					assert.Equal(t, tc.expectedStatus, resp.StatusCode)

					body := make([]byte, 0)
					if resp.Body != nil {
						buf := bytes.NewBuffer(body)
						_, err := buf.ReadFrom(resp.Body)
						require.NoError(t, err)
						body = buf.Bytes()
					}

					if tc.checkResponse != nil {
						tc.checkResponse(t, body)
					}
				})
			}
		})

		// Declare variables for sharing between tests
		var initialUsageCount int64

		// Test Phase 4: API Key Management & Revocation During Use
		t.Run("APIKeyManagement", func(t *testing.T) {
			// List API keys
			keys, err := service.ListAPIKeys(ctx)
			require.NoError(t, err)
			assert.NotEmpty(t, keys)
			found := false
			for _, key := range keys {
				if key.ID == keyID {
					found = true
					assert.Equal(t, "E2E Test Key", key.Name)
					assert.Contains(t, key.Scopes, "read:ephemeris")
					break
				}
			}
			assert.True(t, found, "Created API key should be in the list")

			// Get specific API key
			keyDetails, err := service.GetAPIKey(ctx, keyID)
			require.NoError(t, err)
			assert.Equal(t, "E2E Test Key", keyDetails.Name)
			initialUsageCount = keyDetails.UsageCount
		})

		// Test Phase 5: Revocation During Active Use
		t.Run("RevocationDuringUse", func(t *testing.T) {
			// Make several successful requests
			client := &http.Client{Timeout: 10 * time.Second}

			// First request - should succeed
			req1, _ := http.NewRequest("GET", server.URL+"/planets?year=2023&month=12&day=25&ut=12.0", nil)
			req1.Header.Set("Authorization", "Bearer "+apiKey)
			resp1, err := client.Do(req1)
			require.NoError(t, err)
			defer resp1.Body.Close()
			assert.Equal(t, http.StatusOK, resp1.StatusCode)

			// Second request - should succeed
			req2, _ := http.NewRequest("GET", server.URL+"/houses?year=2023&month=12&day=25&ut=12.0&lat=40.7128&lng=-74.0060", nil)
			req2.Header.Set("Authorization", "Bearer "+apiKey)
			resp2, err := client.Do(req2)
			require.NoError(t, err)
			defer resp2.Body.Close()
			assert.Equal(t, http.StatusOK, resp2.StatusCode)

			// Verify usage count increased
			keyDetails, err := service.GetAPIKey(ctx, keyID)
			require.NoError(t, err)
			assert.True(t, keyDetails.UsageCount >= initialUsageCount+2, "Usage count should have increased by at least 2")

			// Now revoke the key while it's still being used
			err = service.RevokeAPIKey(ctx, keyID)
			require.NoError(t, err)

			// Verify key is revoked in database
			revokedKey, err := service.GetAPIKey(ctx, keyID)
			require.NoError(t, err)
			assert.False(t, revokedKey.IsActive)

			// Third request - should fail gracefully with 401
			req3, _ := http.NewRequest("GET", server.URL+"/chart?year=2023&month=12&day=25&ut=12.0&lat=40.7128&lng=-74.0060", nil)
			req3.Header.Set("Authorization", "Bearer "+apiKey)
			resp3, err := client.Do(req3)
			require.NoError(t, err)
			defer resp3.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp3.StatusCode)

			// Verify the response contains proper error information
			var errorResp map[string]interface{}
			err = json.NewDecoder(resp3.Body).Decode(&errorResp)
			assert.NoError(t, err)
			assert.Contains(t, errorResp, "error")
			assert.Equal(t, "AUTHENTICATION_ERROR", errorResp["error"].(map[string]interface{})["code"])

			// Additional requests should also fail
			req4, _ := http.NewRequest("GET", server.URL+"/planets?year=2023&month=12&day=26&ut=12.0", nil)
			req4.Header.Set("Authorization", "Bearer "+apiKey)
			resp4, err := client.Do(req4)
			require.NoError(t, err)
			defer resp4.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp4.StatusCode, "Revoked key should continue to fail")
		})

		// Test Phase 5: Usage Tracking
		t.Run("UsageTracking", func(t *testing.T) {
			// First, reactivate the key for usage tracking test
			err = service.RevokeAPIKey(ctx, keyID) // This actually toggles, so calling again reactivates
			require.NoError(t, err)

			initialUsage := 0

			// Make several API calls to track usage
			for i := 0; i < 3; i++ {
				req, err := http.NewRequest("GET", server.URL+"/planets?year=2023&month=12&day=25&ut=12.0", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+apiKey)

				client := &http.Client{Timeout: 10 * time.Second}
				resp, err := client.Do(req)
				require.NoError(t, err)
				resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode)
			}

			// Check usage count increased
			keyDetails, err := service.GetAPIKey(ctx, keyID)
			require.NoError(t, err)
			assert.True(t, keyDetails.UsageCount >= int64(initialUsage+3), "Usage count should have increased")
			assert.NotNil(t, keyDetails.LastUsedAt, "Last used timestamp should be set")
		})
	})
}

// TestConcurrentAPIKeyUsage tests concurrent API key usage
func TestConcurrentAPIKeyUsage_E2E(t *testing.T) {
	if os.Getenv("RUN_E2E") != "true" {
		t.Skip("Skipping E2E tests - set RUN_E2E=true to run")
	}

	// Setup test database
	db, cleanup := setupE2EDatabase(t)
	defer cleanup()

	// Setup components
	logger, err := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "e2e-concurrent", Environment: "test",
	})
	require.NoError(t, err)

	repo := repository.NewAPIKeyRepository(db, logger)
	service := auth.NewAPIKeyService(repo, nil, nil, nil, logger)
	// Create cached ephemeris service for E2E tests
	cachedService := eph.NewCachedEphemerisService(nil, logger.Logger) // nil cache = direct calculations
	handler := handlers.NewEphemerisHandler(cachedService, logger)

	// Create test HTTP server
	mux := http.NewServeMux()
	authMiddleware := middleware.NewAuthMiddleware(service, logger, "")
	mux.Handle("/planets", authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets)))

	server := httptest.NewServer(mux)
	defer server.Close()

	// Create API key
	ctx := context.Background()
	req := models.CreateAPIKeyRequest{
		Name:   "Concurrent Test Key",
		Scopes: []string{"read:ephemeris"},
	}

	response, err := service.CreateAPIKey(ctx, req, "127.0.0.1")
	require.NoError(t, err)
	apiKey := response.Key

	// Test concurrent requests
	const numGoroutines = 10
	const requestsPerGoroutine = 5

	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < requestsPerGoroutine; j++ {
				req, err := http.NewRequest("GET", server.URL+"/planets?year=2023&month=12&day=25&ut=12.0", nil)
				if err != nil {
					errors <- fmt.Errorf("failed to create request: %w", err)
					continue
				}
				req.Header.Set("Authorization", "Bearer "+apiKey)

				client := &http.Client{Timeout: 10 * time.Second}
				resp, err := client.Do(req)
				if err != nil {
					errors <- fmt.Errorf("request failed: %w", err)
					continue
				}
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					errors <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
					continue
				}
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Goroutine completed
		case err := <-errors:
			t.Errorf("Concurrent request error: %v", err)
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}

	// Check final usage count
	close(errors)
	if len(errors) > 0 {
		t.Fatalf("Had %d errors in concurrent requests", len(errors))
	}

	// Verify usage count
	keyDetails, err := service.GetAPIKey(ctx, response.ID)
	require.NoError(t, err)
	expectedUsage := int64(numGoroutines * requestsPerGoroutine)
	assert.Equal(t, expectedUsage, keyDetails.UsageCount, "Usage count should match concurrent requests")
}

// TestAPIKeyExpiration_E2E tests API key expiration functionality
func TestAPIKeyExpiration_E2E(t *testing.T) {
	if os.Getenv("RUN_E2E") != "true" {
		t.Skip("Skipping E2E tests - set RUN_E2E=true to run")
	}

	// Setup test database
	db, cleanup := setupE2EDatabase(t)
	defer cleanup()

	// Setup components
	logger, err := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "e2e-expiration", Environment: "test",
	})
	require.NoError(t, err)

	repo := repository.NewAPIKeyRepository(db, logger)
	service := auth.NewAPIKeyService(repo, nil, nil, nil, logger)

	// Create API key that expires in 1 second
	ctx := context.Background()
	expiresAt := time.Now().Add(1 * time.Second)
	req := models.CreateAPIKeyRequest{
		Name:      "Expiring Test Key",
		Scopes:    []string{"read:ephemeris"},
		ExpiresAt: &expiresAt,
	}

	response, err := service.CreateAPIKey(ctx, req, "127.0.0.1")
	require.NoError(t, err)
	apiKey := response.Key

	// Key should work initially
	validatedKey, err := service.ValidateAPIKey(ctx, apiKey)
	require.NoError(t, err)
	assert.True(t, validatedKey.IsActive)

	// Wait for expiration
	time.Sleep(1100 * time.Millisecond)

	// Key should now be expired
	_, err = service.ValidateAPIKey(ctx, apiKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

// TestRateLimiting_E2E tests rate limiting functionality (placeholder for future implementation)
func TestRateLimiting_E2E(t *testing.T) {
	if os.Getenv("RUN_E2E") != "true" {
		t.Skip("Skipping E2E tests - set RUN_E2E=true to run")
	}

	t.Skip("Rate limiting not yet implemented - placeholder for future enhancement")
}

// TestErrorScenarios_E2E tests various error scenarios end-to-end
func TestErrorScenarios_E2E(t *testing.T) {
	if os.Getenv("RUN_E2E") != "true" {
		t.Skip("Skipping E2E tests - set RUN_E2E=true to run")
	}

	// Setup test database
	db, cleanup := setupE2EDatabase(t)
	defer cleanup()

	// Setup components
	logger, err := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "e2e-errors", Environment: "test",
	})
	require.NoError(t, err)

	repo := repository.NewAPIKeyRepository(db, logger)
	service := auth.NewAPIKeyService(repo, nil, nil, nil, logger)
	// Create cached ephemeris service for E2E tests
	cachedService := eph.NewCachedEphemerisService(nil, logger.Logger) // nil cache = direct calculations
	handler := handlers.NewEphemerisHandler(cachedService, logger)

	// Create test HTTP server
	mux := http.NewServeMux()
	authMiddleware := middleware.NewAuthMiddleware(service, logger, "")
	mux.Handle("/planets", authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets)))

	server := httptest.NewServer(mux)
	defer server.Close()

	testCases := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name: "No API Key",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest("GET", server.URL+"/planets?year=2023&month=12&day=25&ut=12.0", nil)
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, body []byte) {
				assert.Contains(t, string(body), "AUTHENTICATION_ERROR")
			},
		},
		{
			name: "Invalid API Key",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest("GET", server.URL+"/planets?year=2023&month=12&day=25&ut=12.0", nil)
				req.Header.Set("Authorization", "Bearer invalid-key")
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, body []byte) {
				assert.Contains(t, string(body), "AUTHENTICATION_ERROR")
			},
		},
		{
			name: "Invalid Parameters",
			setupRequest: func() *http.Request {
				// Create a valid key first
				ctx := context.Background()
				req := models.CreateAPIKeyRequest{
					Name:   "Error Test Key",
					Scopes: []string{"read:ephemeris"},
				}
				response, err := service.CreateAPIKey(ctx, req, "127.0.0.1")
				require.NoError(t, err)

				req2, _ := http.NewRequest("GET", server.URL+"/planets?year=invalid&month=12&day=25&ut=12.0", nil)
				req2.Header.Set("Authorization", "Bearer "+response.Key)
				return req2
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				assert.Contains(t, string(body), "VALIDATION_ERROR")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.setupRequest()

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			body := make([]byte, 0)
			if resp.Body != nil {
				buf := bytes.NewBuffer(body)
				buf.ReadFrom(resp.Body)
				body = buf.Bytes()
			}

			if tc.checkResponse != nil {
				tc.checkResponse(t, body)
			}
		})
	}
}

// setupE2EDatabase sets up a test MongoDB database for E2E tests
func setupE2EDatabase(t *testing.T) (*mongo.Database, func()) {
	t.Helper()

	// Get MongoDB URI from environment
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	// Create test database name
	dbName := "astral_e2e_" + fmt.Sprintf("%d", time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	require.NoError(t, err, "Failed to connect to MongoDB")

	// Ping to ensure connection
	err = client.Ping(ctx, nil)
	require.NoError(t, err, "Failed to ping MongoDB")

	db := client.Database(dbName)

	// Cleanup function
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := db.Drop(ctx)
		if err != nil {
			t.Logf("Failed to drop E2E database: %v", err)
		}
		err = client.Disconnect(ctx)
		if err != nil {
			t.Logf("Failed to disconnect: %v", err)
		}
	}

	return db, cleanup
}

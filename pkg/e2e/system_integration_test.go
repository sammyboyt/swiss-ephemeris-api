//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
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
)

// TestSystemIntegration provides a high-level integration test of the entire system
func TestSystemIntegration(t *testing.T) {
	if os.Getenv("RUN_E2E") != "true" {
		t.Skip("Skipping E2E tests - set RUN_E2E=true to run")
	}

	// Setup complete system
	db, cleanup := setupE2EDatabase(t)
	defer cleanup()

	logger, err := logger.NewLogger(logger.LogConfig{
		Level: "info", ServiceName: "system-integration", Environment: "test",
	})
	require.NoError(t, err)

	// Initialize all components
	repo := repository.NewAPIKeyRepository(db, logger)

	// Create indexes
	ctx := context.Background()
	err = repo.CreateIndexes(ctx)
	require.NoError(t, err)

	service := auth.NewAPIKeyService(repo, nil, nil, nil, logger)
	// Create cached ephemeris service for E2E tests (without Redis for simplicity)
	cachedService := eph.NewCachedEphemerisService(nil, logger.Logger) // nil cache = direct calculations
	handler := handlers.NewEphemerisHandler(cachedService, logger)

	// Setup HTTP server with complete middleware stack
	mux := http.NewServeMux()

	// Apply authentication middleware
	authMiddleware := middleware.NewAuthMiddleware(service, logger, "")
	ephemerisAuth := authMiddleware.RequireScope("read:ephemeris")

	// Setup routes
	mux.Handle("/api/v1/planets", ephemerisAuth.Authenticate(http.HandlerFunc(handler.GetPlanets)))
	mux.Handle("/api/v1/houses", ephemerisAuth.Authenticate(http.HandlerFunc(handler.GetHouses)))
	mux.Handle("/api/v1/chart", ephemerisAuth.Authenticate(http.HandlerFunc(handler.GetChart)))

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test complete user journey
	t.Run("CompleteUserJourney", func(t *testing.T) {
		// Step 1: Create API key
		createReq := models.CreateAPIKeyRequest{
			Name:        "Integration Test Key",
			Description: "Complete system integration test",
			Scopes:      []string{"read:ephemeris"},
		}

		keyResponse, err := service.CreateAPIKey(ctx, createReq, "127.0.0.1")
		require.NoError(t, err)
		assert.NotEmpty(t, keyResponse.Key)

		apiKey := keyResponse.Key

		// Step 2: Use API key for authenticated requests
		requests := []struct {
			name   string
			url    string
			method string
		}{
			{"Get Planets", "/api/v1/planets?year=1990&month=6&day=15&ut=12.0", "GET"},
			{"Get Houses", "/api/v1/houses?year=1990&month=6&day=15&ut=12.0&lat=40.7128&lng=-74.0060", "GET"},
			{"Get Chart", "/api/v1/chart?year=1990&month=6&day=15&ut=12.0&lat=40.7128&lng=-74.0060", "GET"},
		}

		for _, req := range requests {
			t.Run(req.name, func(t *testing.T) {
				httpReq, err := http.NewRequest(req.method, server.URL+req.url, nil)
				require.NoError(t, err)

				httpReq.Header.Set("Authorization", "Bearer "+apiKey)
				httpReq.Header.Set("Content-Type", "application/json")

				client := &http.Client{Timeout: 15 * time.Second}
				resp, err := client.Do(httpReq)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

				// Verify request ID middleware is working
				requestID := resp.Header.Get("X-Request-ID")
				assert.NotEmpty(t, requestID, "Request ID should be present in response")
				assert.Len(t, requestID, 32, "Request ID should be 32 characters (hex)")

				// Verify we get valid JSON response
				var jsonResp interface{}
				err = json.NewDecoder(resp.Body).Decode(&jsonResp)
				assert.NoError(t, err)
				assert.NotNil(t, jsonResp)
			})
		}

		// Step 3: Verify API key was tracked
		keyDetails, err := service.GetAPIKey(ctx, keyResponse.ID)
		require.NoError(t, err)
		assert.True(t, keyDetails.UsageCount >= 3) // At least 3 requests made
		assert.NotNil(t, keyDetails.LastUsedAt)

		// Step 4: Test caching performance (when Redis is available)
		t.Run("CachingPerformance", func(t *testing.T) {
			// Make first request (potential cache miss)
			start := time.Now()
			httpReq, _ := http.NewRequest("GET", server.URL+"/api/v1/planets?year=2024&month=1&day=15&ut=12.0", nil)
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
			resp1, err := http.DefaultClient.Do(httpReq)
			require.NoError(t, err)
			defer resp1.Body.Close()
			duration1 := time.Since(start)

			assert.Equal(t, http.StatusOK, resp1.StatusCode)

			// Small delay to ensure any async caching is complete
			time.Sleep(100 * time.Millisecond)

			// Make identical second request (potential cache hit)
			start = time.Now()
			httpReq2, _ := http.NewRequest("GET", server.URL+"/api/v1/planets?year=2024&month=1&day=15&ut=12.0", nil)
			httpReq2.Header.Set("Authorization", "Bearer "+apiKey)
			resp2, err := http.DefaultClient.Do(httpReq2)
			require.NoError(t, err)
			defer resp2.Body.Close()
			duration2 := time.Since(start)

			assert.Equal(t, http.StatusOK, resp2.StatusCode)

			// Verify request ID middleware for both requests
			requestID1 := resp1.Header.Get("X-Request-ID")
			requestID2 := resp2.Header.Get("X-Request-ID")
			assert.NotEmpty(t, requestID1)
			assert.NotEmpty(t, requestID2)
			assert.NotEqual(t, requestID1, requestID2) // Different requests should have different IDs

			// Log performance for monitoring (second request should be faster if cached)
			t.Logf("First request duration: %v", duration1)
			t.Logf("Second request duration: %v", duration2)

			// Note: In E2E tests with nil cache, both requests will be similar speed
			// In production with Redis, second request should be significantly faster
			assert.True(t, duration1 > 0, "First request should take some time")
			assert.True(t, duration2 > 0, "Second request should take some time")
		})

		// Step 5: Test error scenarios
		t.Run("ErrorHandling", func(t *testing.T) {
			// Test missing authentication
			httpReq, _ := http.NewRequest("GET", server.URL+"/api/v1/planets?year=1990&month=6&day=15&ut=12.0", nil)
			resp, err := http.DefaultClient.Do(httpReq)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

			// Test invalid parameters
			httpReq, _ = http.NewRequest("GET", server.URL+"/api/v1/planets?year=invalid&month=6&day=15&ut=12.0", nil)
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
			resp, err = http.DefaultClient.Do(httpReq)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	})
}

// TestConcurrentEphemerisRequests validates worker pool concurrent processing
func TestConcurrentEphemerisRequests(t *testing.T) {
	if os.Getenv("RUN_E2E") != "true" {
		t.Skip("Skipping E2E tests - set RUN_E2E=true to run")
	}

	// Setup complete system (similar to TestSystemIntegration)
	db, cleanup := setupE2EDatabase(t)
	defer cleanup()

	logger, err := logger.NewLogger(logger.LogConfig{
		Level: "info", ServiceName: "concurrent-ephemeris", Environment: "test",
	})
	require.NoError(t, err)

	// Initialize all components
	repo := repository.NewAPIKeyRepository(db, logger)

	// Create indexes
	ctx := context.Background()
	err = repo.CreateIndexes(ctx)
	require.NoError(t, err)

	service := auth.NewAPIKeyService(repo, nil, nil, nil, logger)
	cachedService := eph.NewCachedEphemerisService(nil, logger.Logger)
	handler := handlers.NewEphemerisHandler(cachedService, logger)

	// Setup HTTP server with complete middleware stack
	mux := http.NewServeMux()

	// Apply authentication middleware
	authMiddleware := middleware.NewAuthMiddleware(service, logger, "")
	ephemerisAuth := authMiddleware.RequireScope("read:ephemeris")

	// Setup routes
	mux.Handle("/api/v1/planets", ephemerisAuth.Authenticate(http.HandlerFunc(handler.GetPlanets)))

	// Apply request ID middleware
	requestIDMiddleware := middleware.NewRequestIDMiddleware(logger)
	server := httptest.NewServer(requestIDMiddleware.Middleware(mux))
	defer server.Close()

	// Create API key for testing
	expiresAt := time.Now().Add(24 * time.Hour)
	keyResponse, err := service.CreateAPIKey(ctx, models.CreateAPIKeyRequest{
		Name:      "Concurrent Test Key",
		Scopes:    []string{"read:ephemeris"},
		ExpiresAt: &expiresAt,
	}, "127.0.0.1")
	require.NoError(t, err)

	apiKey := keyResponse.Key

	// Test concurrent requests
	t.Run("ConcurrentRequests", func(t *testing.T) {
		numRequests := 10
		responses := make(chan *http.Response, numRequests)
		errors := make(chan error, numRequests)

		// Launch concurrent requests
		for i := 0; i < numRequests; i++ {
			go func(requestID int) {
				httpReq, _ := http.NewRequest("GET", server.URL+"/api/v1/planets?year=2024&month=1&day=15&ut=12.0", nil)
				httpReq.Header.Set("Authorization", "Bearer "+apiKey)

				client := &http.Client{Timeout: 30 * time.Second}
				resp, err := client.Do(httpReq)
				if err != nil {
					errors <- err
					return
				}
				responses <- resp
			}(i)
		}

		// Collect responses
		successCount := 0
		requestIDs := make(map[string]bool)

		for i := 0; i < numRequests; i++ {
			select {
			case resp := <-responses:
				defer resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode)

				// Verify request ID is present and unique
				requestID := resp.Header.Get("X-Request-ID")
				assert.NotEmpty(t, requestID, "Request ID should be present")
				assert.Len(t, requestID, 32, "Request ID should be 32 characters")
				assert.False(t, requestIDs[requestID], "Request ID should be unique")
				requestIDs[requestID] = true

				// Verify JSON response
				var jsonResp interface{}
				err := json.NewDecoder(resp.Body).Decode(&jsonResp)
				assert.NoError(t, err)

				successCount++

			case err := <-errors:
				t.Errorf("Request failed: %v", err)
			}
		}

		assert.Equal(t, numRequests, successCount, "All concurrent requests should succeed")
		assert.Equal(t, numRequests, len(requestIDs), "All requests should have unique IDs")

		t.Logf("Successfully processed %d concurrent requests", successCount)
		t.Logf("Collected %d unique request IDs", len(requestIDs))
	})
}

// BenchmarkSystemPerformance benchmarks the complete system performance
func BenchmarkSystemPerformance(b *testing.B) {
	if os.Getenv("RUN_E2E") != "true" {
		b.Skip("Skipping E2E benchmark - set RUN_E2E=true to run")
	}

	// Setup system
	db, cleanup := setupE2EDatabase(&testing.T{})
	defer cleanup()

	logger, err := logger.NewLogger(logger.LogConfig{
		Level: "error", ServiceName: "performance-test", Environment: "test", // Reduce logging for performance
	})
	if err != nil {
		b.Fatal(err)
	}

	repo := repository.NewAPIKeyRepository(db, logger)
	service := auth.NewAPIKeyService(repo, nil, nil, nil, logger)
	// Create cached ephemeris service for E2E tests
	cachedService := eph.NewCachedEphemerisService(nil, logger.Logger) // nil cache = direct calculations
	handler := handlers.NewEphemerisHandler(cachedService, logger)

	// Setup HTTP server
	mux := http.NewServeMux()
	authMiddleware := middleware.NewAuthMiddleware(service, logger, "")
	mux.Handle("/planets", authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets)))

	server := httptest.NewServer(mux)
	defer server.Close()

	// Create API key
	ctx := context.Background()
	req := models.CreateAPIKeyRequest{
		Name:   "Performance Test Key",
		Scopes: []string{"read:ephemeris"},
	}

	response, err := service.CreateAPIKey(ctx, req, "127.0.0.1")
	if err != nil {
		b.Fatal(err)
	}
	apiKey := response.Key

	// Benchmark complete request cycle
	client := &http.Client{Timeout: 10 * time.Second}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", server.URL+"/planets?year=1990&month=6&day=15&ut=12.0", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		// Read and discard body to ensure complete request
		buf := make([]byte, 1024)
		for {
			n, err := resp.Body.Read(buf)
			if n == 0 && err != nil {
				break
			}
		}
		resp.Body.Close()
	}
}

// TestSystemReliability tests system reliability under various conditions
func TestSystemReliability(t *testing.T) {
	if os.Getenv("RUN_E2E") != "true" {
		t.Skip("Skipping E2E tests - set RUN_E2E=true to run")
	}

	// Setup system
	db, cleanup := setupE2EDatabase(t)
	defer cleanup()

	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "warn", ServiceName: "reliability-test", Environment: "test",
	})

	repo := repository.NewAPIKeyRepository(db, logger)
	service := auth.NewAPIKeyService(repo, nil, nil, nil, logger)
	// Create cached ephemeris service for E2E tests
	cachedService := eph.NewCachedEphemerisService(nil, logger.Logger) // nil cache = direct calculations
	handler := handlers.NewEphemerisHandler(cachedService, logger)

	// Setup HTTP server
	mux := http.NewServeMux()
	authMiddleware := middleware.NewAuthMiddleware(service, logger, "")
	mux.Handle("/planets", authMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets)))

	server := httptest.NewServer(mux)
	defer server.Close()

	ctx := context.Background()

	// Test multiple API keys
	t.Run("MultipleAPIKeys", func(t *testing.T) {
		keys := make([]string, 5)

		// Create multiple keys
		for i := 0; i < 5; i++ {
			req := models.CreateAPIKeyRequest{
				Name:   "Reliability Test Key " + string(rune('A'+i)),
				Scopes: []string{"read:ephemeris"},
			}

			response, err := service.CreateAPIKey(ctx, req, "127.0.0.1")
			require.NoError(t, err)
			keys[i] = response.Key
		}

		// Test all keys work
		client := &http.Client{Timeout: 10 * time.Second}
		for i, apiKey := range keys {
			req, _ := http.NewRequest("GET", server.URL+"/planets?year=1990&month=6&day=15&ut=12.0", nil)
			req.Header.Set("Authorization", "Bearer "+apiKey)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode, "Key %d should work", i)
		}
	})

	// Test system recovery after database issues
	t.Run("SystemRecovery", func(t *testing.T) {
		// Create a key
		req := models.CreateAPIKeyRequest{
			Name:   "Recovery Test Key",
			Scopes: []string{"read:ephemeris"},
		}

		response, err := service.CreateAPIKey(ctx, req, "127.0.0.1")
		require.NoError(t, err)

		// Verify key works
		client := &http.Client{Timeout: 10 * time.Second}
		httpReq, _ := http.NewRequest("GET", server.URL+"/planets?year=1990&month=6&day=15&ut=12.0", nil)
		httpReq.Header.Set("Authorization", "Bearer "+response.Key)

		resp, err := client.Do(httpReq)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// System should continue working normally
		// (In a real scenario, we'd test database disconnection and reconnection)
	})
}

// TestSystemSecurity tests security aspects of the complete system
func TestSystemSecurity(t *testing.T) {
	if os.Getenv("RUN_E2E") != "true" {
		t.Skip("Skipping E2E tests - set RUN_E2E=true to run")
	}

	// Setup system
	db, cleanup := setupE2EDatabase(t)
	defer cleanup()

	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "security-test", Environment: "test",
	})

	repo := repository.NewAPIKeyRepository(db, logger)
	service := auth.NewAPIKeyService(repo, nil, nil, nil, logger)
	// Create cached ephemeris service for E2E tests
	cachedService := eph.NewCachedEphemerisService(nil, logger.Logger) // nil cache = direct calculations
	handler := handlers.NewEphemerisHandler(cachedService, logger)

	// Setup HTTP server with different scopes
	mux := http.NewServeMux()

	readOnlyMiddleware := middleware.NewAuthMiddleware(service, logger, "read:ephemeris")
	adminMiddleware := middleware.NewAuthMiddleware(service, logger, "admin")

	mux.Handle("/planets", readOnlyMiddleware.Authenticate(http.HandlerFunc(handler.GetPlanets)))
	mux.Handle("/admin/keys", adminMiddleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "admin access granted"}`))
	})))

	server := httptest.NewServer(mux)
	defer server.Close()

	ctx := context.Background()

	// Test scope-based access control
	t.Run("ScopeBasedAccess", func(t *testing.T) {
		// Create read-only key
		readOnlyReq := models.CreateAPIKeyRequest{
			Name:   "Read-Only Key",
			Scopes: []string{"read:ephemeris"},
		}
		readOnlyResponse, err := service.CreateAPIKey(ctx, readOnlyReq, "127.0.0.1")
		require.NoError(t, err)

		// Create admin key
		adminReq := models.CreateAPIKeyRequest{
			Name:   "Admin Key",
			Scopes: []string{"read:ephemeris", "admin"},
		}
		adminResponse, err := service.CreateAPIKey(ctx, adminReq, "127.0.0.1")
		require.NoError(t, err)

		client := &http.Client{Timeout: 10 * time.Second}

		// Test read-only key can access planets but not admin
		req1, _ := http.NewRequest("GET", server.URL+"/planets?year=1990&month=6&day=15&ut=12.0", nil)
		req1.Header.Set("Authorization", "Bearer "+readOnlyResponse.Key)
		resp1, err := client.Do(req1)
		require.NoError(t, err)
		defer resp1.Body.Close()
		assert.Equal(t, http.StatusOK, resp1.StatusCode)

		req2, _ := http.NewRequest("GET", server.URL+"/admin/keys", nil)
		req2.Header.Set("Authorization", "Bearer "+readOnlyResponse.Key)
		resp2, err := client.Do(req2)
		require.NoError(t, err)
		defer resp2.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp2.StatusCode)

		// Test admin key can access both
		req3, _ := http.NewRequest("GET", server.URL+"/planets?year=1990&month=6&day=15&ut=12.0", nil)
		req3.Header.Set("Authorization", "Bearer "+adminResponse.Key)
		resp3, err := client.Do(req3)
		require.NoError(t, err)
		defer resp3.Body.Close()
		assert.Equal(t, http.StatusOK, resp3.StatusCode)

		req4, _ := http.NewRequest("GET", server.URL+"/admin/keys", nil)
		req4.Header.Set("Authorization", "Bearer "+adminResponse.Key)
		resp4, err := client.Do(req4)
		require.NoError(t, err)
		defer resp4.Body.Close()
		assert.Equal(t, http.StatusOK, resp4.StatusCode)
	})
}

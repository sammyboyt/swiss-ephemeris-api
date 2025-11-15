package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"astral-backend/eph"
	"astral-backend/pkg/auth"
	"astral-backend/pkg/cache"
	"astral-backend/pkg/handlers"
	"astral-backend/pkg/logger"
	"astral-backend/pkg/middleware"
	"astral-backend/pkg/models"
	"astral-backend/pkg/repository"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	appLogger, err := logger.NewLogger(logger.LogConfig{
		Level:       getEnvOrDefault("LOG_LEVEL", "info"),
		ServiceName: "astral-backend",
		Environment: getEnvOrDefault("ENVIRONMENT", "production"),
	})
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}

	appLogger.Info("Starting Astral Backend API server")

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoURI := getEnvOrDefault("MONGODB_URI", "mongodb://localhost:27017")
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		appLogger.Error("Failed to connect to MongoDB", zap.Error(err))
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// Ping MongoDB
	if err := mongoClient.Ping(ctx, nil); err != nil {
		appLogger.Error("Failed to ping MongoDB", zap.Error(err))
		log.Fatal("Failed to ping MongoDB:", err)
	}

	appLogger.Info("Connected to MongoDB successfully")

	// Initialize repository
	apiKeyRepo := repository.NewAPIKeyRepository(mongoClient.Database("astral"), appLogger)

	// Create indexes
	if err := apiKeyRepo.CreateIndexes(ctx); err != nil {
		appLogger.Error("Failed to create database indexes", zap.Error(err))
		log.Fatal("Failed to create database indexes:", err)
	}

	// Initialize cache
	cacheConfig := cache.CacheConfig{
		RedisURL:     getEnvOrDefault("REDIS_URL", "redis://redis:6379"),
		DefaultTTL:   1 * time.Hour,
		EphemerisTTL: 24 * time.Hour,
	}

	redisCache, err := cache.NewRedisCache(cacheConfig, appLogger.Logger)
	if err != nil {
		appLogger.Warn("Failed to initialize Redis cache, falling back to no cache", zap.Error(err))
		// Could implement in-memory cache as fallback
	}

	// Initialize services
	hasher := &auth.SecurePasswordHasher{}
	keyGen := &auth.SecureKeyGenerator{}
	idGen := &auth.UUIDGenerator{}

	apiKeyService := auth.NewAPIKeyService(apiKeyRepo, hasher, idGen, keyGen, appLogger)

	// Initialize worker pool for concurrent calculations
	workerPool := eph.NewWorkerPool(4, appLogger.Logger) // 4 workers for concurrent processing
	workerPool.Start()

	// Ensure worker pool is stopped on shutdown
	defer func() {
		appLogger.Info("Stopping worker pool")
		workerPool.Stop()
	}()

	// Initialize cached ephemeris service
	var ephemerisService interface {
		GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]eph.Planet, error)
		GetHousesCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]eph.House, error)
		GetChartCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]eph.Planet, []eph.House, error)
	}

	if redisCache != nil {
		ephemerisService = eph.NewCachedEphemerisService(redisCache, appLogger.Logger)
		appLogger.Info("Initialized cached ephemeris service with Redis")
	} else {
		// Fallback to direct calculations
		ephemerisService = &eph.DirectEphemerisService{Logger: appLogger.Logger}
		appLogger.Info("Initialized direct ephemeris service (no caching)")
	}

	// Log worker pool stats
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stats := workerPool.GetStats()
				appLogger.Info("Worker pool stats",
					zap.Any("stats", stats))
			case <-ctx.Done():
				return
			}
		}
	}()

	// Initialize handlers
	ephemerisHandler := handlers.NewEphemerisHandler(ephemerisService, appLogger)

	// Setup router
	router := setupRouter(apiKeyService, ephemerisHandler, appLogger)

	// Configure server
	port := getEnvOrDefault("PORT", "8080")
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		appLogger.Info("Server starting", zap.String("port", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("Server failed to start", zap.Error(err))
			log.Fatal("Server failed to start:", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Error("Server forced to shutdown", zap.Error(err))
	}

	// Close MongoDB connection
	if err := mongoClient.Disconnect(ctx); err != nil {
		appLogger.Error("Failed to disconnect from MongoDB", zap.Error(err))
	}

	appLogger.Info("Server stopped")
}

func setupRouter(apiKeyService auth.APIKeyServiceInterface, ephemerisHandler *handlers.EphemerisHandler, logger *logger.Logger) *mux.Router {
	router := mux.NewRouter()

	// Request ID middleware - applied first
	requestIDMiddleware := middleware.NewRequestIDMiddleware(logger)
	router.Use(func(next http.Handler) http.Handler {
		return requestIDMiddleware.Middleware(next)
	})

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "astral-backend"}`))
	}).Methods("GET")

	// TEMPORARY: API key creation endpoint for integration testing (remove in production)
	router.HandleFunc("/api/v1/keys", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		expiresAt := time.Now().Add(24 * time.Hour)
		keyResponse, err := apiKeyService.CreateAPIKey(context.Background(), models.CreateAPIKeyRequest{
			Name:      "Integration Test Key",
			Scopes:    []string{"read:ephemeris"},
			ExpiresAt: &expiresAt,
		}, "127.0.0.1")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keyResponse)
	}).Methods("POST")

	// API v1 routes
	v1 := router.PathPrefix("/api/v1").Subrouter()

	// Authentication middleware
	authMiddleware := middleware.NewAuthMiddleware(apiKeyService, logger, "")
	ephemerisAuth := authMiddleware.RequireScope("read:ephemeris")

	// Ephemeris endpoints
	v1.Handle("/planets", ephemerisAuth.Authenticate(http.HandlerFunc(ephemerisHandler.GetPlanets))).Methods("GET")
	v1.Handle("/houses", ephemerisAuth.Authenticate(http.HandlerFunc(ephemerisHandler.GetHouses))).Methods("GET")
	v1.Handle("/chart", ephemerisAuth.Authenticate(http.HandlerFunc(ephemerisHandler.GetChart))).Methods("GET")

	// Add CORS middleware if needed
	router.Use(mux.CORSMethodMiddleware(router))

	return router
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

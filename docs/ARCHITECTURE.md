# Astral Backend - System Architecture

## Overview

The Astral Backend is a high-performance REST API service for astrological calculations built with Go, featuring enterprise-grade reliability, observability, and scalability. This document explains the system architecture for mid-level developers.

## 🏗️ System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Client Applications                      │
│  (Web Apps, Mobile Apps, Third-party Integrations)         │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ▼ HTTP/HTTPS
┌─────────────────────────────────────────────────────────────┐
│                 API Gateway / Load Balancer                │
│  (Nginx, AWS ALB, Cloud Load Balancer - Future)            │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ▼ REST API
┌─────────────────────────────────────────────────────────────┐
│                Astral Backend Service                      │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Request Processing Layer              │   │
│  │  ┌─────────────────────────────────────────────────┐ │   │
│  │  │   Authentication & Authorization Middleware    │ │   │
│  │  │   - API Key Validation                         │ │   │
│  │  │   - Scope-based Access Control                 │ │   │
│  │  └─────────────────────────────────────────────────┘ │   │
│  │  ┌─────────────────────────────────────────────────┐ │   │
│  │  │   Request ID Middleware                        │ │   │
│  │  │   - UUID Generation                            │ │   │
│  │  │   - Context Propagation                        │ │   │
│  │  └─────────────────────────────────────────────────┘ │   │
│  │  ┌─────────────────────────────────────────────────┐ │   │
│  │  │   Route Handlers                               │ │   │
│  │  │   - Input Validation                           │ │   │
│  │  │   - Business Logic Orchestration               │ │   │
│  │  └─────────────────────────────────────────────────┘ │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │             Business Logic Layer                   │   │
│  │  ┌─────────────────────────────────────────────────┐ │   │
│  │  │   Ephemeris Service                            │ │   │
│  │  │   - Calculation Orchestration                  │ │   │
│  │  │   - Cache Management                           │ │   │
│  │  │   - Worker Pool Coordination                   │ │   │
│  │  └─────────────────────────────────────────────────┘ │   │
│  │  ┌─────────────────────────────────────────────────┐ │   │
│  │  │   Circuit Breaker                              │ │   │
│  │  │   - Fault Tolerance                            │ │   │
│  │  │   - Automatic Recovery                         │ │   │
│  │  └─────────────────────────────────────────────────┘ │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │             Data Access Layer                      │   │
│  │  ┌─────────────────────────────────────────────────┐ │   │
│  │  │   Redis Cache                                  │ │   │
│  │  │   - High-speed Data Retrieval                  │ │   │
│  │  │   - TTL-based Expiration                       │ │   │
│  │  └─────────────────────────────────────────────────┘ │   │
│  │  ┌─────────────────────────────────────────────────┐ │   │
│  │  │   MongoDB Repository                           │ │   │
│  │  │   - API Key Storage                            │ │   │
│  │  │   - Audit Logging                              │ │   │
│  │  └─────────────────────────────────────────────────┘ │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │             External Dependencies                  │   │
│  │  ┌─────────────────────────────────────────────────┐ │   │
│  │  │   Swiss Ephemeris C Library                    │ │   │
│  │  │   - Astronomical Calculations                  │ │   │
│  │  │   - Planet Positions                           │ │   │
│  │  │   - House Systems                              │ │   │
│  │  └─────────────────────────────────────────────────┘ │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## 📦 Component Breakdown

### 1. Request Processing Layer

#### HTTP Server (Gorilla Mux)

```go
// cmd/server/main.go
router := mux.NewRouter()

// Middleware stack
router.Use(requestIDMiddleware)
router.Use(authMiddleware)

// Routes
v1 := router.PathPrefix("/api/v1").Subrouter()
v1.Handle("/planets", authMiddleware.Authenticate(planetsHandler)).Methods("GET")
```

**Responsibilities:**

- HTTP request/response handling
- Route matching and dispatching
- Middleware coordination

#### Authentication Middleware

```go
// pkg/middleware/auth_middleware.go
type AuthMiddleware struct {
    service auth.APIKeyServiceInterface
    logger  *logger.Logger
}

func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := extractAPIKey(r)
        if !m.service.ValidateAPIKey(ctx, apiKey, scope) {
            respondWithError(w, errors.NewAuthenticationError("Invalid API key"))
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**Responsibilities:**

- Extract API keys from headers/query params
- Validate API keys against database
- Check scope-based permissions
- Set authenticated context

#### Request ID Middleware

```go
// pkg/middleware/request_id.go
type RequestIDMiddleware struct {
    logger *logger.Logger
}

func (m *RequestIDMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = generateUUID()
        }

        // Add to response header
        w.Header().Set("X-Request-ID", requestID)

        // Add to context for logging
        ctx := context.WithValue(r.Context(), RequestIDKey{}, requestID)
        logger := m.logger.WithFields(zap.String("request_id", requestID))

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Responsibilities:**

- Generate unique request IDs
- Propagate IDs through request context
- Add IDs to response headers
- Enable distributed tracing

### 2. Business Logic Layer

#### Ephemeris Service Interface

```go
// pkg/handlers/ephemeris.go
type EphemerisService interface {
    GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]eph.Planet, error)
    GetHousesCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]eph.House, error)
    GetChartCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]eph.Planet, []eph.House, error)
}
```

#### Cached Ephemeris Service

```go
// eph/cache.go
type CachedEphemerisService struct {
    cache   cache.Cache
    logger  *zap.Logger
    breaker *CircuitBreaker
}

func (s *CachedEphemerisService) GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]eph.Planet, error) {
    key := cache.GenerateEphemerisKey("planets", yr, mon, day, ut)

    // Check cache first
    if cached, err := s.cache.Get(ctx, key); err == nil {
        return unmarshalPlanets(cached), nil
    }

    // Calculate with circuit breaker
    result, err := s.breaker.Call(func() (interface{}, error) {
        return s.calculatePlanets(yr, mon, day, ut)
    })

    if err != nil {
        return nil, err
    }

    // Cache result
    planets := result.([]eph.Planet)
    s.cache.Set(ctx, key, marshalPlanets(planets), 24*time.Hour)

    return planets, nil
}
```

**Responsibilities:**

- Coordinate cache/redis operations
- Orchestrate worker pool usage
- Implement circuit breaker pattern
- Handle cache key generation and TTL

#### Worker Pool

```go
// eph/worker_pool.go
type WorkerPool struct {
    workers   int
    jobQueue  chan Job
    results   chan Result
    semaphore chan struct{}
}

func (wp *WorkerPool) Submit(job Job) (interface{}, error) {
    // Acquire semaphore
    wp.semaphore <- struct{}{}
    defer func() { <-wp.semaphore }()

    // Submit job
    resultChan := make(chan Result, 1)
    wp.jobQueue <- Job{
        Task:       job.Task,
        ResultChan: resultChan,
    }

    // Wait for result with timeout
    select {
    case result := <-resultChan:
        return result.Value, result.Error
    case <-time.After(30 * time.Second):
        return nil, errors.New("calculation timeout")
    }
}
```

**Responsibilities:**

- Manage concurrent ephemeris calculations
- Prevent CGO library conflicts
- Handle job timeouts
- Provide semaphore-based limiting

### 3. Data Access Layer

#### Redis Cache Implementation

```go
// pkg/cache/redis_cache.go
type RedisCache struct {
    client *redis.Client
    config CacheConfig
    logger *zap.Logger
}

func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
    return c.client.Get(ctx, c.prefixKey(key)).Bytes()
}

func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
    return c.client.Set(ctx, c.prefixKey(key), value, ttl).Err()
}

func (c *RedisCache) prefixKey(key string) string {
    return fmt.Sprintf("%s:%s", c.config.Namespace, key)
}
```

**Responsibilities:**

- Redis connection management
- Key prefixing and TTL handling
- JSON serialization/deserialization
- Connection pooling

#### MongoDB Repository

```go
// pkg/repository/api_key_repository.go
type APIKeyRepository struct {
    collection *mongo.Collection
    logger     *logger.Logger
}

func (r *APIKeyRepository) GetByKey(ctx context.Context, hashedKey string) (*models.APIKey, error) {
    filter := bson.M{"key_hash": hashedKey, "is_active": true}

    var apiKey models.APIKey
    err := r.collection.FindOne(ctx, filter).Decode(&apiKey)
    if err != nil {
        if err == mongo.ErrNoDocuments {
            return nil, errors.NewNotFoundError("API key not found")
        }
        return nil, err
    }

    return &apiKey, nil
}
```

**Responsibilities:**

- API key CRUD operations
- Database connection management
- Index management
- Error handling and logging

### 4. External Dependencies

#### Swiss Ephemeris CGO Wrapper

```go
// eph/eph.go
/*
#include <swephexp.h>
#include <sweph.h>
*/
import "C"

// GetPlanets calculates planetary positions
func GetPlanets(yr, mon, day int, ut float64) ([]Planet, error) {
    // Convert Go types to C types
    tjd := C.double(swe_julday(yr, mon, day, ut, C.SE_GREG_CAL))

    var planets []Planet
    for i := 0; i < 12; i++ { // 12 planets
        var xx [6]C.double
        var serr [256]C.char

        // Call Swiss Ephemeris
        ret := C.swe_calc_ut(tjd, C.int(i), C.int(iflag), &xx[0], &serr[0])

        if ret < 0 {
            return nil, fmt.Errorf("calculation error for planet %d: %s", i, C.GoString(&serr[0]))
        }

        planets = append(planets, Planet{
            ID:        i,
            Name:      planetNames[i],
            Longitude: float64(xx[0]),
            Retrograde: xx[3] < 0, // speed < 0 means retrograde
        })
    }

    return planets, nil
}
```

**Responsibilities:**

- CGO memory management
- Type conversion between Go and C
- Error handling from C library
- Resource cleanup

## 🔄 Data Flow Diagrams

### Normal Request Flow

```
Client Request
       ↓
  Authentication
       ↓
   Request ID Injection
       ↓
   Input Validation
       ↓
   Cache Check (Redis)
       ↓
   Worker Pool Submission
       ↓
   Swiss Ephemeris Calculation
       ↓
   Result Caching
       ↓
   JSON Response
       ↓
   Structured Logging
```

### Cache Hit Flow

```
Client Request
       ↓
  Authentication
       ↓
   Request ID Injection
       ↓
   Input Validation
       ↓
   Cache Check (Redis)
   ├─ Cache Hit → Return Cached Data
   └─ Cache Miss → Continue to Calculation
```

### Error Handling Flow

```
Client Request
       ↓
  Authentication Failure
       ↓
   Error Response (401)
       ↓
   Structured Error Logging
```

## 🏭 Design Patterns Used

### 1. Dependency Injection

```go
// Constructor injection for testability
func NewEphemerisHandler(service EphemerisService, logger *logger.Logger) *EphemerisHandler {
    return &EphemerisHandler{
        service: service,
        logger:  logger,
    }
}
```

### 2. Repository Pattern

```go
// Abstract data access
type APIKeyRepository interface {
    Create(ctx context.Context, key *models.APIKey) error
    GetByKey(ctx context.Context, hashedKey string) (*models.APIKey, error)
    UpdateUsage(ctx context.Context, id string, increment int) error
}
```

### 3. Middleware Chain

```go
// Composable request processing
router.Use(requestIDMiddleware)
router.Use(corsMiddleware)
router.Use(authMiddleware)
router.Use(loggingMiddleware)
```

### 4. Circuit Breaker

```go
// Fault tolerance pattern
func (cb *CircuitBreaker) Call(fn func() (interface{}, error)) (interface{}, error) {
    if cb.state == StateOpen {
        return nil, errors.New("circuit breaker is open")
    }

    result, err := fn()
    cb.recordResult(err == nil)

    return result, err
}
```

### 5. Worker Pool

```go
// Concurrency management
type WorkerPool struct {
    workers  int
    jobQueue chan Job
}

func (wp *WorkerPool) startWorkers() {
    for i := 0; i < wp.workers; i++ {
        go func() {
            for job := range wp.jobQueue {
                result := job.Task()
                job.ResultChan <- Result{Value: result}
            }
        }()
    }
}
```

## 🔒 Security Architecture

### API Key Authentication

- **Hashing**: Argon2 for secure key storage
- **Validation**: Constant-time comparison
- **Scopes**: Granular permission control
- **Expiration**: Time-based key lifecycle

### Request Validation

- **Input Sanitization**: Prevent injection attacks
- **Type Validation**: Strong typing with struct tags
- **Business Rules**: Domain-specific validation
- **Error Masking**: Prevent information leakage

### Audit Logging

```go
// Security event logging
logger.SecurityLog(r.Context(), "api_key_used", map[string]interface{}{
    "key_id":      apiKey.ID,
    "endpoint":    r.URL.Path,
    "user_agent":  r.Header.Get("User-Agent"),
    "remote_addr": getRealIP(r),
})
```

## 📊 Performance Characteristics

### Response Times

- **Cache Hit**: 50-200ms
- **Cache Miss**: 500-2000ms
- **Concurrent Load**: 4x baseline with worker pool

### Scalability Limits

- **Worker Pool**: 4 concurrent calculations (CGO limitation)
- **Cache Size**: Redis memory constraints
- **Database**: MongoDB connection pool limits

### Monitoring Points

- **Request Latency**: Histogram by endpoint
- **Cache Hit Rate**: Percentage of cache hits
- **Worker Pool Utilization**: Active workers gauge
- **Error Rate**: Error counter by type

## 🚀 Deployment Architecture

### Docker Compose (Development)

```yaml
# docker/astral-backend/docker-compose.yml
version: "3.8"
services:
  app:
    build: .
    depends_on:
      mongodb:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      - MONGODB_URI=mongodb://AS:dev-mongo-password@mongodb:27017/astral?authSource=admin
      - REDIS_URL=redis://redis:6379
```

### Production Considerations

- **Horizontal Scaling**: Multiple app instances behind load balancer
- **Database Sharding**: MongoDB sharding for high volume
- **Redis Cluster**: Redis clustering for cache scalability
- **CDN**: Static asset caching
- **Rate Limiting**: Distributed rate limiting with Redis

## 🧪 Testing Strategy

### Unit Tests

```go
// pkg/handlers/ephemeris_test.go
func TestEphemerisHandler_GetPlanets_Success(t *testing.T) {
    // Mock service and logger
    mockService := &mockEphemerisService{}
    handler := NewEphemerisHandler(mockService, logger)

    // Test request
    req := httptest.NewRequest("GET", "/planets?year=2024&month=1&day=15&ut=12.0", nil)
    w := httptest.NewRecorder()

    handler.GetPlanets(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
}
```

### Integration Tests

```go
// pkg/e2e/system_integration_test.go
func TestSystemIntegration(t *testing.T) {
    // Setup full system with real dependencies
    db, cleanup := setupE2EDatabase(t)
    defer cleanup()

    // Test complete user journey
    // Create API key → Make requests → Verify results
}
```

### Performance Benchmarks

```go
// pkg/handlers/ephemeris_benchmark_test.go
func BenchmarkEphemerisHandler_GetPlanets(b *testing.B) {
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Measure request handling performance
    }
}
```

## 🔧 Development Workflow

### Local Development

1. **Code Changes**: Modify source files
2. **Unit Tests**: `make test-unit`
3. **Integration Tests**: `make test-integration`
4. **Manual Testing**: `./health-check.sh`
5. **Commit**: Follow conventional commit format

### Code Review Checklist

- [ ] Unit tests added/modified
- [ ] Integration tests pass
- [ ] Documentation updated
- [ ] Performance impact assessed
- [ ] Security implications reviewed

### CI/CD Pipeline

```
Code Push → Lint → Unit Tests → Integration Tests → Build → Deploy to Staging → E2E Tests → Production Deploy
```

## 📝 Key Design Decisions

### 1. CGO for Swiss Ephemeris

**Decision**: Use CGO instead of reimplementing complex astronomical calculations
**Rationale**: Swiss Ephemeris is the industry standard with 30+ years of validation
**Trade-offs**: Memory management complexity, build complexity, platform dependencies

### 2. Redis for Caching

**Decision**: Redis over in-memory cache for production scalability
**Rationale**: Distributed cache, persistence, TTL support, monitoring
**Trade-offs**: Additional infrastructure dependency, network latency

### 3. Worker Pool Pattern

**Decision**: Fixed-size worker pool instead of unlimited goroutines
**Rationale**: Prevent CGO library conflicts, resource exhaustion
**Trade-offs**: Potential queuing delays, fixed concurrency limits

### 4. Interface-Based Design

**Decision**: Dependency injection with interfaces throughout
**Rationale**: Testability, modularity, dependency management
**Trade-offs**: Additional boilerplate code, interface maintenance

### 5. Structured Logging

**Decision**: Zap logger with structured JSON output
**Rationale**: Observability, log aggregation, performance
**Trade-offs**: Less readable local development logs

This architecture provides a robust, scalable, and maintainable foundation for astrological calculations with enterprise-grade reliability and observability.

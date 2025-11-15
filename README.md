# Astral Backend - Enterprise Astrological API Service

[![CI/CD](https://github.com/username/astral-backend/actions/workflows/ci.yml/badge.svg)](https://github.com/username/astral-backend/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org)
[![Docker](https://img.shields.io/badge/docker-ready-blue.svg)](https://docker.com)

A production-ready, high-performance REST API service for astrological calculations using the Swiss Ephemeris library. Built with Go, featuring Redis caching, request tracing, and Docker orchestration.

## 🚀 Quick Start

### Prerequisites

-   Go 1.21+
-   Docker & Docker Compose
-   MongoDB (for API keys)
-   Redis (for caching)

### Run the Full Stack

```bash
# Clone and navigate to project
cd astral-backend

# Start all services (MongoDB, Redis, API)
cd docker/astral-backend
docker-compose up --build -d

# Run health check
cd ../..
./health-check.sh

# Test the API
curl -H "Authorization: Bearer YOUR_API_KEY" \
  "http://localhost:8080/api/v1/planets?year=2024&month=1&day=15&ut=12.0"
```

## 🛠️ Development Quick Start

```bash
# Setup development environment (includes Swiss Ephemeris build)
make setup-dev

# Run all tests
make test-all

# Build application
make build

# Test full Docker stack
make docker-test

# Clean build artifacts
make clean
```

## 📋 Table of Contents

-   [Architecture Overview](#architecture-overview)
-   [API Documentation](#api-documentation)
-   [Development Setup](#development-setup)
-   [Testing](#testing)
-   [Deployment](#deployment)
-   [Monitoring & Observability](#monitoring--observability)
-   [Troubleshooting](#troubleshooting)

## 🏗️ Architecture Overview

### Core Components

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   REST API      │────│  Business Logic │────│  Swiss Eph. C   │
│   (Gin/Mux)     │    │  (Services)     │    │  Library        │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Auth          │    │   Redis Cache   │    │  Worker Pool    │
│   Middleware    │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   MongoDB       │    │   Circuit       │    │   Request ID    │
│   (API Keys)    │    │   Breaker       │    │   Tracing       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Key Features

-   **🔐 Authentication**: API key-based auth with scopes
-   **⚡ Performance**: Redis caching (20-40x speedup)
-   **🔄 Scalability**: Worker pool for concurrent calculations
-   **🛡️ Reliability**: Circuit breaker pattern
-   **📊 Observability**: Request tracing and structured logging
-   **🐳 Containerized**: Docker-based deployment

### Data Flow

1. **Request** → Authentication middleware validates API key
2. **Cache Check** → Redis checked for existing calculation
3. **Calculation** → Worker pool processes ephemeris calculations
4. **Response** → JSON response with request ID tracing
5. **Logging** → Structured logs with request correlation

## 📚 API Documentation

### Authentication

All API endpoints require authentication via Bearer token:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  "http://localhost:8080/api/v1/planets?year=2024&month=1&day=15&ut=12.0"
```

### Endpoints

#### Health Check

```http
GET /health
```

**Response:**

```json
{
    "status": "healthy",
    "service": "astral-backend"
}
```

#### Get Planetary Positions

```http
GET /api/v1/planets?year={year}&month={month}&day={day}&ut={ut}
```

**Parameters:**

-   `year`: Integer (e.g., 2024)
-   `month`: Integer 1-12
-   `day`: Integer 1-31
-   `ut`: Float (Universal Time in decimal hours)

**Response:**

```json
{
    "planets": [
        {
            "id": 0,
            "name": "Sun",
            "longitude": 294.81869079051665,
            "retrograde": false
        }
    ]
}
```

#### Get House Cusps

```http
GET /api/v1/houses?year={year}&month={month}&day={day}&ut={ut}&lat={lat}&lng={lng}
```

**Additional Parameters:**

-   `lat`: Float (latitude in decimal degrees)
-   `lng`: Float (longitude in decimal degrees)

**Response:**

```json
{
    "houses": [
        {
            "id": 1,
            "longitude": 123.456,
            "hsys": "P"
        }
    ]
}
```

#### Get Complete Chart

```http
GET /api/v1/chart?year={year}&month={month}&day={day}&ut={ut}&lat={lat}&lng={lng}
```

**Response:**

```json
{
  "planets": [...],
  "houses": [...]
}
```

### Error Responses

```json
{
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "Request validation failed",
        "details": {
            "field": "year",
            "reason": "must be a valid year"
        }
    }
}
```

## 💻 Development Setup

### Local Development

```bash
# Install dependencies
go mod download

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run quality checks
make quality

# Build the application
make build

# Run tests
make test-unit

# Start services for integration testing
make mongo-up
make test-integration
```

### Code Structure

```
astral-backend/
├── cmd/server/           # Application entry point
├── pkg/
│   ├── auth/            # Authentication & API keys
│   ├── cache/           # Redis caching layer
│   ├── handlers/        # HTTP request handlers
│   ├── logger/          # Structured logging
│   ├── middleware/      # HTTP middleware
│   ├── models/          # Data models
│   └── repository/      # Database access layer
├── eph/                 # Ephemeris calculations (CGO)
├── docker/              # Docker configurations
└── scripts/             # Utility scripts
```

### Key Files to Understand

1. **`cmd/server/main.go`** - Application bootstrap and routing
2. **`eph/eph.go`** - CGO wrapper for Swiss Ephemeris
3. **`pkg/handlers/ephemeris.go`** - Main API handlers
4. **`pkg/auth/api_key_service.go`** - Authentication logic
5. **`pkg/cache/redis_cache.go`** - Caching implementation

## 🧪 Testing

### Test Types

```bash
# Unit tests (fast, isolated)
make test-unit

# Integration tests (requires MongoDB)
make test-integration

# End-to-end tests (requires full stack)
make test-e2e

# All tests
make test-all

# Performance benchmarks
make bench
```

### Test Coverage

Current coverage: ~64% across all packages

```bash
# Generate coverage report
make coverage-html
# Opens coverage.html in browser
```

### Manual Testing

```bash
# Run comprehensive health check
./health-check.sh

# Test specific endpoints
curl -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/planets?year=2024&month=1&day=15&ut=12.0"
```

## 🚀 Deployment

### Docker Compose (Development)

```bash
cd docker/astral-backend
docker-compose up --build -d
```

### Production Deployment

```bash
# Build production image
make build-prod

# Deploy with docker-compose
docker-compose -f docker-compose.prod.yml up -d

# Or use Kubernetes
kubectl apply -f k8s/
```

### Environment Variables

```bash
# Database
MONGODB_URI=mongodb://user:pass@host:27017/db

# Caching
REDIS_URL=redis://host:6379

# Application
LOG_LEVEL=info
PORT=8080
ENVIRONMENT=production
```

## 📊 Monitoring & Observability

### Request Tracing

Every request gets a unique 32-character hex ID in the `X-Request-ID` header:

```bash
curl -v http://localhost:8080/api/v1/planets?...
< X-Request-Id: 74dae0de1234567890abcdef12345678
```

### Structured Logging

All logs include request IDs for correlation:

```json
{
    "level": "info",
    "timestamp": "2024-01-15T10:30:00Z",
    "service": "astral-backend",
    "request_id": "74dae0de1234567890abcdef12345678",
    "message": "Planets calculated successfully",
    "planet_count": 12
}
```

### Health Checks

-   **Application**: `GET /health`
-   **MongoDB**: Built-in Docker health checks
-   **Redis**: Built-in Docker health checks

### Performance Metrics

-   Response times: ~50-200ms (cached), ~500-2000ms (uncached)
-   Cache hit rate: Monitor Redis INFO stats
-   Concurrent requests: Worker pool metrics in logs

## 🔧 Troubleshooting

### Common Issues

#### Application Won't Start

```bash
# Check logs
docker-compose logs app

# Verify environment variables
docker-compose exec app env

# Test database connectivity
docker-compose exec app mongo --eval "db.stats()"
```

#### Slow Performance

```bash
# Check Redis cache status
docker-compose exec redis redis-cli info stats

# Verify worker pool logs
docker-compose logs app | grep "worker"

# Check system resources
docker stats
```

#### Authentication Issues

```bash
# Verify API key exists
docker-compose exec mongodb mongo astral --eval "db.api_keys.find()"

# Check API key format
echo "YOUR_API_KEY" | wc -c  # Should be ~50 chars
```

### Debug Mode

```bash
# Enable debug logging
export LOG_LEVEL=debug

# Run with verbose output
docker-compose up --build

# Check application metrics
curl http://localhost:8080/debug/vars
```

## 📈 Performance Optimization

### Caching Strategy

-   **TTL**: 24 hours for ephemeris data
-   **Key Format**: `eph:{type}:{hash}` (e.g., `eph:planets:a1b2c3d4...`)
-   **Invalidation**: Automatic expiration

### Worker Pool Configuration

-   **Workers**: 4 concurrent calculation threads
-   **Queue**: Unlimited buffered channel
-   **Timeout**: 30 seconds per calculation

### Memory Management

-   **CGO**: Careful memory management for C library calls
-   **Connection Pooling**: MongoDB and Redis connection reuse
-   **Goroutine Limits**: Worker pool prevents resource exhaustion

## 🤝 Contributing

### Code Standards

```bash
# Run all quality checks
make quality

# Format code
go fmt ./...

# Run linter
golangci-lint run
```

### Testing Requirements

-   All new code must have unit tests
-   Integration tests for database operations
-   Maintain >60% test coverage
-   All tests must pass CI

### Commit Standards

```
feat: add new API endpoint
fix: resolve authentication bug
docs: update API documentation
test: add integration tests
```

## 📝 API Key Management

### Creating API Keys (Development Only)

```bash
# POST to create key (temporary endpoint)
curl -X POST http://localhost:8080/api/v1/keys
```

### Key Format

-   **Length**: ~50 characters
-   **Format**: Base64-encoded secure random bytes
-   **Scopes**: `read:ephemeris` (current), extensible for future features

## 🔒 Security Considerations

-   API keys use Argon2 hashing for storage
-   Request validation prevents injection attacks
-   HTTPS recommended for production
-   Rate limiting implemented at API key level
-   Audit logging for security events

## 🚨 Production Checklist

-   [ ] Environment variables configured
-   [ ] HTTPS enabled
-   [ ] Database backups configured
-   [ ] Monitoring alerts set up
-   [ ] Log aggregation configured
-   [ ] Performance baselines established
-   [ ] Security scanning completed
-   [ ] Load testing completed

---

## 📞 Support

For issues or questions:

1. Check the troubleshooting section
2. Review application logs
3. Run the health check script
4. Check Docker container status

**🎯 This service provides high-performance, reliable astrological calculations with enterprise-grade observability and scalability.**

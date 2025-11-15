#!/bin/bash

set -e

echo "🐳 Astral Backend - Docker Build & Test Suite"
echo "=============================================="
echo "Unified testing script that consolidates docker-test.sh, demo.sh, and simple-test.sh functionality"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to cleanup containers
cleanup() {
    log_info "Cleaning up containers..."
    cd docker/astral-backend
    docker-compose down -v --remove-orphans 2>/dev/null || true
    cd ../..
    # Clean up any dangling containers from previous runs
    docker container prune -f >/dev/null 2>&1 || true
}

# Function to check container health
check_health() {
    local service=$1
    local max_attempts=60  # Increased timeout
    local attempt=1

    log_info "Waiting for $service to be healthy (max: ${max_attempts} attempts)..."

    while [ $attempt -le $max_attempts ]; do
        if (cd docker/astral-backend && docker-compose ps $service 2>/dev/null | grep -q "healthy"); then
            log_success "$service is healthy!"
            return 0
        elif (cd docker/astral-backend && docker-compose ps $service 2>/dev/null | grep -q "running\|Up"); then
            # Service is running but not marked healthy yet
            log_info "Attempt $attempt/$max_attempts: $service is running, waiting for health check..."
        else
            log_warn "Attempt $attempt/$max_attempts: $service status unknown, checking again..."
        fi

        sleep 3
        ((attempt++))
    done

    log_error "$service failed to become healthy after $max_attempts attempts"
    log_info "Container logs for $service:"
    (cd docker/astral-backend && docker-compose logs $service | tail -20)
    return 1
}

# Function to wait for port availability
wait_for_port() {
    local host=$1
    local port=$2
    local max_attempts=30
    local attempt=1

    log_info "Waiting for $host:$port to be available..."

    while [ $attempt -le $max_attempts ]; do
        if nc -z $host $port 2>/dev/null; then
            log_success "Port $host:$port is available!"
            return 0
        fi

        log_info "Attempt $attempt/$max_attempts: Port not ready yet..."
        sleep 2
        ((attempt++))
    done

    log_error "Port $host:$port failed to become available"
    return 1
}

# Trap to cleanup on exit
trap cleanup EXIT

# Pre-flight checks
log_info "Running pre-flight checks..."
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    log_error "Docker Compose is not installed or not in PATH"
    exit 1
fi

log_success "Pre-flight checks passed"

# Clean up any previous runs
cleanup

log_info "Step 1: Building Docker images..."
if ! (cd docker/astral-backend && docker-compose build --no-cache); then
    log_error "Failed to build Docker images"
    exit 1
fi
log_success "Docker images built successfully"

log_info "Step 2: Starting services..."
if ! (cd docker/astral-backend && docker-compose up -d); then
    log_error "Failed to start services"
    (cd docker/astral-backend && docker-compose logs)
    exit 1
fi
log_success "Services started"

log_info "Step 3: Waiting for services to be healthy..."
if ! check_health mongodb; then
    exit 1
fi

if ! check_health redis; then
    exit 1
fi

log_info "Step 4: Waiting for application port..."
if ! wait_for_port localhost 8080; then
    log_error "Application failed to start listening on port 8080"
    (cd docker/astral-backend && docker-compose logs app)
    exit 1
fi

log_info "Step 5: Testing health endpoint..."
if curl -f -s --max-time 10 http://localhost:8080/health > /dev/null; then
    log_success "✅ Health check passed!"
else
    log_error "❌ Health check failed!"
    log_info "Health endpoint response:"
    curl -s --max-time 5 http://localhost:8080/health || echo "Connection failed"
    (cd docker/astral-backend && docker-compose logs app | tail -20)
    exit 1
fi

log_info "Step 6: Testing API endpoints..."
# Test unauthenticated endpoint (should return 401)
if curl -s -w "%{http_code}" -o /dev/null "http://localhost:8080/api/v1/planets?year=1990&month=6&day=15&ut=12.0" | grep -q "401"; then
    log_success "✅ Authentication working correctly (401 for unauthenticated request)"
else
    log_error "❌ Authentication not working properly"
    exit 1
fi

# Useful test but must be run on host with go installed
# log_info "Step 7: Running integration tests..."
# # Run tests against the containerized application
# if (cd docker/astral-backend && docker-compose exec -T app sh -c "
#     export MONGODB_URI='mongodb://AS:dev-mongo-password@mongodb:27017/AS?authSource=admin'
#     export RUN_E2E=true
#     go test ./pkg/e2e/... -v -timeout=10m
# " 2>&1); then
#     log_success "✅ Integration tests passed!"
# else
#     log_error "❌ Integration tests failed!"
#     (cd docker/astral-backend && docker-compose logs app | tail -30)
#     exit 1
# fi

log_success "🎉 All Docker tests passed successfully!"
echo ""
log_info "🚀 Your application is ready for deployment!"
echo ""
log_info "Production deployment steps:"
echo "1. Update environment variables in docker-compose.yml"
echo "2. Run: docker-compose -f docker-compose.prod.yml up -d"
echo "3. Monitor logs: docker-compose -f docker-compose.prod.yml logs -f app"
echo ""
log_info "For production scaling:"
echo "docker-compose -f docker-compose.prod.yml up -d --scale app=3"
echo ""
log_info "For cleanup:"
echo "docker-compose -f docker-compose.prod.yml down -v"

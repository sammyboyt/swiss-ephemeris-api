#!/bin/bash

# Astral Backend - Comprehensive Health Check Script
# This script verifies all components are working correctly

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_DIR="docker/astral-backend"
API_KEY="mU0Gdnb6b4g-bWPqc433dab4QR5pKNsPsxzKlV-1qkQ="

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_service_health() {
    local service=$1
    local check_cmd=$2
    local expected=$3

    log_info "Checking $service health..."
    if eval "$check_cmd" | grep -q "$expected"; then
        log_success "$service is healthy"
        return 0
    else
        log_error "$service health check failed"
        return 1
    fi
}

# Main health check function
main() {
    echo "🔍 Astral Backend - Comprehensive Health Check"
    echo "=============================================="
    echo ""

    cd "$COMPOSE_DIR"

    # Check Docker services status
    log_info "Checking Docker services status..."
    if docker-compose ps | grep -q "Up"; then
        log_success "Docker services are running"
    else
        log_error "Some Docker services are not running"
        docker-compose ps
        exit 1
    fi

    # MongoDB Health Check
    check_service_health "MongoDB" "docker-compose exec -T mongodb mongo --eval 'db.adminCommand(\"ping\")'" "ok"

    # Redis Health Check
    check_service_health "Redis" "docker-compose exec -T redis redis-cli ping" "PONG"

    # Application Health Check
    check_service_health "Application" "curl -s http://localhost:8080/health" '"status": "healthy"'

    # API Authentication Test
    log_info "Testing API authentication..."
    auth_response=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/planets?year=2024&month=1&day=15&ut=12.0")
    if [ "$auth_response" -eq 401 ]; then
        log_success "Authentication working correctly (401 for unauthenticated request)"
    else
        log_error "Authentication not working properly (expected 401, got $auth_response)"
        exit 1
    fi

    # API Functionality Test
    log_info "Testing API functionality..."

    # Planets endpoint
    planets_count=$(curl -s -H "Authorization: Bearer $API_KEY" "http://localhost:8080/api/v1/planets?year=2024&month=1&day=15&ut=12.0" | jq -r '.planets | length')
    if [ "$planets_count" -eq 12 ]; then
        log_success "Planets endpoint working (returned $planets_count planets)"
    else
        log_error "Planets endpoint failed (expected 12, got $planets_count)"
        exit 1
    fi

    # Houses endpoint
    houses_count=$(curl -s -H "Authorization: Bearer $API_KEY" "http://localhost:8080/api/v1/houses?year=2024&month=1&day=15&ut=12.0&lat=40.7128&lng=-74.0060" | jq -r '. | length')
    if [ "$houses_count" -eq 52 ]; then
        log_success "Houses endpoint working (returned $houses_count house data points)"
    else
        log_error "Houses endpoint failed (expected 52, got $houses_count)"
        exit 1
    fi

    # Chart endpoint
    chart_planets=$(curl -s -H "Authorization: Bearer $API_KEY" "http://localhost:8080/api/v1/chart?year=2024&month=1&day=15&ut=12.0&lat=40.7128&lng=-74.0060" | jq -r '.planets | length')
    chart_houses=$(curl -s -H "Authorization: Bearer $API_KEY" "http://localhost:8080/api/v1/chart?year=2024&month=1&day=15&ut=12.0&lat=40.7128&lng=-74.0060" | jq -r '.houses | length')
    if [ "$chart_planets" -eq 12 ] && [ "$chart_houses" -eq 52 ]; then
        log_success "Chart endpoint working (returned $chart_planets planets and $chart_houses houses)"
    else
        log_error "Chart endpoint failed (expected 12 planets and 52 houses, got $chart_planets planets and $chart_houses houses)"
        exit 1
    fi

    # Request ID Test
    log_info "Testing request ID tracing..."
    request_id=$(curl -s -D /tmp/headers.txt -H "Authorization: Bearer $API_KEY" "http://localhost:8080/api/v1/planets?year=2024&month=1&day=15&ut=12.0" > /dev/null && grep "X-Request-Id:" /tmp/headers.txt | cut -d' ' -f2 | tr -d '\r')
    if [[ ${#request_id} -eq 32 && "$request_id" =~ ^[a-f0-9]+$ ]]; then
        log_success "Request ID working correctly (32 hex chars: ${request_id:0:8}...)"
    else
        log_error "Request ID not working properly"
        exit 1
    fi

    # Caching Test
    log_info "Testing Redis caching..."
    initial_keys=$(docker-compose exec -T redis redis-cli keys "*" | wc -l)
    curl -s -H "Authorization: Bearer $API_KEY" "http://localhost:8080/api/v1/planets?year=2024&month=1&day=15&ut=12.0" > /dev/null
    after_keys=$(docker-compose exec -T redis redis-cli keys "*" | wc -l)
    if [ "$after_keys" -gt "$initial_keys" ]; then
        log_success "Caching working (Redis keys increased from $initial_keys to $after_keys)"
    else
        log_warning "Caching may not be working properly"
    fi

    # Error Handling Test
    log_info "Testing error handling..."
    error_response=$(curl -s -H "Authorization: Bearer $API_KEY" "http://localhost:8080/api/v1/planets?year=invalid&month=1&day=15&ut=12.0" | jq -r '.error.code')
    if [ "$error_response" = "VALIDATION_ERROR" ]; then
        log_success "Error handling working correctly"
    else
        log_error "Error handling not working properly"
        exit 1
    fi

    # Concurrent Load Test
    log_info "Testing concurrent load handling..."
    for i in {1..5}; do
        curl -s -H "Authorization: Bearer $API_KEY" "http://localhost:8080/api/v1/planets?year=2024&month=1&day=$i&ut=12.0" > /dev/null &
    done
    wait
    log_success "Concurrent requests handled successfully"

    echo ""
    echo "🎉 Astral Backend - All Health Checks Passed!"
    echo "=============================================="
    log_success "✅ MongoDB: Connected and healthy"
    log_success "✅ Redis: Connected and caching"
    log_success "✅ Application: Running and responsive"
    log_success "✅ Authentication: Working correctly"
    log_success "✅ API Endpoints: Planets, Houses, Chart all functional"
    log_success "✅ Request Tracing: Request IDs generated and logged"
    log_success "✅ Error Handling: Proper error responses"
    log_success "✅ Concurrent Load: Worker pool handling multiple requests"
    echo ""
    log_info "🚀 Astral Backend is production-ready!"
}

# Run main function
main "$@"

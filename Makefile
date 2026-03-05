# Astral Backend - Enterprise API Key Management System
# Comprehensive Test Suite and Build Pipeline

.PHONY: help test test-unit test-integration test-e2e test-all coverage coverage-html lint build clean docker-test ci

# Default target
help: ## Show this help message
	@echo "Astral Backend - Enterprise API Key Management System"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

# Test Commands
test-unit: ## Run unit tests only (pkg only)
	@echo "Running unit tests (pkg only)..."
	go test ./pkg/... -v -short

test-unit-all: build-sweph ## Run all unit tests including eph (requires Swiss Ephemeris)
	@echo "Running all unit tests (including eph with CGO)..."
	go test ./pkg/... ./eph/... -v -short -timeout=30s

test-eph: build-sweph ## Run ephemeris tests only (requires Swiss Ephemeris)
	@echo "Running ephemeris tests with CGO..."
	go test ./eph/... -v -timeout=30s

test-integration: ## Run integration tests (requires MongoDB)
	@echo "Running integration tests..."
	go test -tags=integration ./pkg/... -v

test-e2e: ## Run end-to-end tests (requires MongoDB)
	@echo "Running end-to-end tests..."
	RUN_E2E=true go test -tags=e2e ./pkg/e2e/... -v -timeout=5m

test-all: build-sweph ## Run all tests (unit, integration, e2e, eph)
	@echo "Running all tests..."
	@echo "1. Unit tests (pkg + eph)..."
	go test ./pkg/... ./eph/... -v -short -timeout=30s
	@echo "2. Integration tests..."
	go test -tags=integration ./pkg/... -v
	@echo "3. End-to-end tests..."
	RUN_E2E=true go test -tags=e2e ./pkg/e2e/... -v -timeout=5m

# Coverage Commands
coverage: build-sweph ## Generate coverage report (includes eph)
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out ./pkg/... ./eph/... -short -timeout=30s
	go tool cover -func=coverage.out

coverage-html: build-sweph ## Generate HTML coverage report (includes eph)
	@echo "Generating HTML coverage report..."
	go test -coverprofile=coverage.out ./pkg/... ./eph/... -short -timeout=30s
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

coverage-integration: ## Generate coverage for integration tests
	@echo "Generating integration test coverage..."
	go test -tags=integration -coverprofile=coverage-integration.out ./pkg/...
	go tool cover -func=coverage-integration.out

coverage-eph: build-sweph ## Generate coverage for ephemeris tests only
	@echo "Generating ephemeris test coverage..."
	go test -coverprofile=coverage-eph.out ./eph/... -timeout=30s
	go tool cover -func=coverage-eph.out

# Quality Checks
lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

quality: lint fmt vet ## Run all quality checks

# Build Commands
build: ## Build the application to dist/
	@echo "Building application..."
	go build -o dist/bin/astral-backend ./cmd/server

build-test: ## Build test binaries
	@echo "Building test binaries..."
	go test -c -o bin/unit-tests ./pkg/...
	go test -c -tags=integration -o bin/integration-tests ./pkg/...
	go test -c -tags=e2e -o bin/e2e-tests ./pkg/e2e/...

# Docker Commands
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t astral-backend:latest .

docker-test: ## Run full integration tests in Docker containers
	@echo "Running full integration tests in Docker containers..."
	./scripts/test-integration.sh

# Database Setup
mongo-up: ## Start MongoDB container for testing
	@echo "Starting MongoDB..."
	docker run -d --name astral-mongo -p 27017:27017 -e MONGO_INITDB_ROOT_USERNAME=admin -e MONGO_INITDB_ROOT_PASSWORD=password mongo:latest

mongo-down: ## Stop MongoDB container
	@echo "Stopping MongoDB..."
	docker stop astral-mongo
	docker rm astral-mongo

# CI Pipeline
ci: quality test-unit coverage ## Run CI pipeline (quality checks, unit tests, coverage)
	@echo "CI pipeline completed successfully!"

ci-full: quality test-all coverage-html ## Run full CI pipeline (all tests, coverage)
	@echo "Full CI pipeline completed successfully!"

# Development Setup
setup-dev: ## Setup development environment with Swiss Ephemeris
	@echo "Setting up development environment..."
	@echo "1. Installing dependencies..."
	go mod download
	@echo "2. Installing tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "3. Building Swiss Ephemeris library..."
	./scripts/build-sweph.sh
	@echo "4. Setting up pre-commit hooks..."
	mkdir -p .git/hooks
	@echo '#!/bin/bash\n\necho "🔍 Running pre-commit checks..."\n\n# Run quality checks\necho "  📏 Running quality checks..."\nif ! make quality > /dev/null 2>&1; then\n    echo "❌ Quality checks failed. Run '\''make quality'\'' to fix."\n    exit 1\nfi\n\n# Check for build artifacts\necho "  🧹 Checking for build artifacts..."\nif [ -f "astral-backend" ] || [ -f "bin/astral-backend" ] || [ -f "dist/bin/astral-backend" ] || [ -f "eph/sweph/src/libswe.a" ] || ls eph/sweph/src/*.o 1> /dev/null 2>&1; then\n    echo "❌ Build artifacts found! Run '\''make clean'\'' before committing."\n    exit 1\nfi\n\n# Run unit tests\necho "  🧪 Running unit tests..."\nif [ -f "eph/sweph/src/libswe.a" ]; then\n    if ! go test ./pkg/... ./eph/... -short -timeout=30s > /dev/null 2>&1; then\n        echo "❌ Unit tests failed. Fix tests before committing."\n        exit 1\n    fi\nelse\n    if ! go test ./pkg/... -short > /dev/null 2>&1; then\n        echo "❌ Unit tests failed. Fix tests before committing."\n        exit 1\n    fi\n    echo "  ⚠️  Warning: Swiss Ephemeris library not built. Run '\''make build-sweph'\'' for full test coverage."\nfi\n\necho "✅ All pre-commit checks passed!"' > .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
	@echo "Development environment setup complete!"

build-sweph: ## Build Swiss Ephemeris library
	@echo "Building Swiss Ephemeris library..."
	./scripts/build-sweph.sh

# Cleanup
clean: ## Clean build artifacts and test files
	@echo "Cleaning up..."
	rm -rf dist/
	rm -f bin/*
	rm -f astral-backend astral-backend-new
	rm -f coverage.out coverage.html coverage-*.out
	rm -f *-test
	rm -f eph/sweph/src/*.o eph/sweph/src/libswe.a eph/sweph/src/swetest
	go clean ./...

# Database Commands
db-migrate: ## Run database migrations
	@echo "Running database migrations..."
	# Add migration commands here

db-seed: ## Seed database with test data
	@echo "Seeding database..."
	# Add seeding commands here

# Documentation
docs: ## Generate documentation
	@echo "Generating documentation..."
	go doc -all ./pkg/... > docs/api.md

# Benchmarking
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./pkg/...

bench-profile: ## Run benchmarks with profiling
	@echo "Running benchmarks with profiling..."
	go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./pkg/...

# Security
security-scan: ## Run security scan
	@echo "Running security scan..."
	gosec ./...

# Performance
perf-test: ## Run performance tests
	@echo "Running performance tests..."
	# Add performance test commands here

# Monitoring
health-check: ## Run health checks
	@echo "Running health checks..."
	@echo "✓ Code quality checks"
	-make quality
	@echo "✓ Unit tests"
	-make test-unit
	@echo "✓ Build check"
	-make build

# Utility
count-lines: ## Count lines of code
	@echo "Counting lines of code..."
	find . -name "*.go" -not -path "./vendor/*" | xargs wc -l

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

deps-audit: ## Audit dependencies for vulnerabilities
	@echo "Auditing dependencies..."
	go mod verify
	# Add additional security checks here

# Environment
env-check: ## Check environment setup
	@echo "Checking environment..."
	@echo "Go version: $$(go version)"
	@echo "MongoDB connection: $$(timeout 5 bash -c "</dev/tcp/localhost/27017" && echo "OK" || echo "NOT AVAILABLE")"
	@echo "Docker: $$(docker --version 2>/dev/null || echo "NOT INSTALLED")"

# Quick development loop
dev: fmt lint test-unit build ## Quick development check (format, lint, test, build)

# Production build
build-prod: ## Build for production to dist/
	@echo "Building for production..."
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dist/bin/astral-backend-prod ./cmd/server

# Docker Compose
compose-up: ## Start all services with Docker Compose
	@echo "Starting services..."
	docker-compose up -d

compose-down: ## Stop all services
	@echo "Stopping services..."
	docker-compose down

compose-logs: ## Show service logs
	docker-compose logs -f

# Release
release: build-prod ## Create release artifacts
	@echo "Creating release..."
	# Add release commands here

# Troubleshooting
debug-test: ## Debug test failures
	@echo "Debugging test failures..."
	go test -v -race ./pkg/...

debug-build: ## Debug build issues
	@echo "Debugging build issues..."
	go build -x -v ./cmd/server

# Information
info: ## Show project information
	@echo "Astral Backend - Enterprise API Key Management System"
	@echo "=================================================="
	@echo "Version: $$(git describe --tags --abbrev=0 2>/dev/null || echo 'dev')"
	@echo "Commit: $$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
	@echo "Go Version: $$(go version | cut -d' ' -f3)"
	@echo "Build Time: $$(date)"
	@echo ""
	@echo "Components:"
	@echo "  ✓ Error handling system"
	@echo "  ✓ Structured logging"
	@echo "  ✓ API key models and validation"
	@echo "  ✓ Authentication service"
	@echo "  ✓ MongoDB repository"
	@echo "  ✓ Authentication middleware"
	@echo "  ✓ Ephemeris handlers"
	@echo "  ✓ End-to-end tests"
	@echo ""
	@echo "Test Coverage: Run 'make coverage' to check"

# =============================================================================
# Lambda Deployment Commands
# =============================================================================

LAMBDA_IMAGE_NAME ?= astral-backend
LAMBDA_IMAGE_TAG ?= dev
AWS_REGION ?= eu-west-2
AWS_ACCOUNT_ID ?= $(shell aws sts get-caller-identity --query Account --output text 2>/dev/null || echo "")
ECR_REPO_URL ?= $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(LAMBDA_IMAGE_NAME)

lambda-help: ## Show Lambda deployment help
	@echo "Lambda Deployment Commands"
	@echo "=========================="
	@echo ""
	@echo "Prerequisites:"
	@echo "  1. AWS CLI configured with credentials"
	@echo "  2. Docker running"
	@echo "  3. Terraform installed (for infrastructure)"
	@echo ""
	@echo "Quick Start:"
	@echo "  make lambda-build    - Build Lambda Docker image"
	@echo "  make lambda-push     - Push to ECR (requires login)"
	@echo "  make lambda-deploy   - Full deploy (build + push + terraform)"
	@echo "  make lambda-test     - Test deployed endpoint"
	@echo "  make lambda-destroy  - Destroy all AWS resources"
	@echo ""
	@echo "Current config:"
	@echo "  Region: $(AWS_REGION)"
	@echo "  Image:  $(LAMBDA_IMAGE_NAME):$(LAMBDA_IMAGE_TAG)"
	@echo "  ECR:    $(ECR_REPO_URL)"

lambda-build: ## Build Lambda Docker image with CGO support
	@echo "Building Lambda Docker image..."
	DOCKER_BUILDKIT=1 docker build \
		-t $(LAMBDA_IMAGE_NAME):$(LAMBDA_IMAGE_TAG) \
		--platform linux/amd64 \
		.
	@echo "✅ Built: $(LAMBDA_IMAGE_NAME):$(LAMBDA_IMAGE_TAG)"

lambda-ecr-login: ## Login to Amazon ECR
	@echo "Logging into ECR..."
	aws ecr get-login-password --region $(AWS_REGION) | \
		docker login --username AWS --password-stdin $(ECR_REPO_URL)

lambda-tag: ## Tag image for ECR
	docker tag $(LAMBDA_IMAGE_NAME):$(LAMBDA_IMAGE_TAG) $(ECR_REPO_URL):$(LAMBDA_IMAGE_TAG)

lambda-push: lambda-tag ## Push image to ECR
	@echo "Pushing to ECR: $(ECR_REPO_URL):$(LAMBDA_IMAGE_TAG)"
	docker push $(ECR_REPO_URL):$(LAMBDA_IMAGE_TAG)
	@echo "✅ Pushed successfully"

lambda-test-local: ## Test Lambda locally with RIE
	@echo "Starting Lambda Runtime Interface Emulator..."
	@echo "Test with: curl -XPOST http://localhost:9000/2015-03-31/functions/function/invocations -d '{\"httpMethod\": \"GET\", \"path\": \"/health\"}'"
	docker run -p 9000:8080 $(LAMBDA_IMAGE_NAME):$(LAMBDA_IMAGE_TAG)

lambda-init: ## Initialize Terraform
	cd terraform && terraform init

lambda-plan: ## Run Terraform plan
	cd terraform && terraform plan

lambda-apply: ## Run Terraform apply
	cd terraform && terraform apply

lambda-destroy: ## Destroy Terraform infrastructure
	cd terraform && terraform destroy

lambda-deploy: lambda-build lambda-push lambda-apply ## Full deployment (build + push + terraform)
	@echo "🚀 Deployment complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Get API endpoint: cd terraform && terraform output api_gateway_endpoint"
	@echo "  2. Get API key: cd terraform && terraform output -raw api_key_value"
	@echo "  3. Test: curl -H 'x-api-key: <key>' <endpoint>/health"

lambda-test: ## Test deployed Lambda endpoint
	@ENDPOINT=$$(cd terraform && terraform output -raw api_gateway_endpoint 2>/dev/null) || echo ""; \
	KEY=$$(cd terraform && terraform output -raw api_key_value 2>/dev/null) || echo ""; \
	if [ -n "$$ENDPOINT" ] && [ -n "$$KEY" ]; then \
		echo "Testing endpoint: $$ENDPOINT/health"; \
		curl -s -H "x-api-key: $$KEY" "$$ENDPOINT/health" | jq . || echo "Install jq for pretty output"; \
	else \
		echo "❌ Endpoint or API key not found. Run 'terraform apply' first."; \
	fi

lambda-outputs: ## Show Terraform outputs
	@cd terraform && terraform output

lambda-clean: ## Clean Lambda build artifacts
	docker rmi $(LAMBDA_IMAGE_NAME):$(LAMBDA_IMAGE_TAG) 2>/dev/null || true
	docker rmi $(ECR_REPO_URL):$(LAMBDA_IMAGE_TAG) 2>/dev/null || true
	docker system prune -f
	@echo "✅ Cleaned Docker images"

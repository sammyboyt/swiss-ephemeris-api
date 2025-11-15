# Contributing to Astral Backend

## Development Setup

1. **Clone and setup:**
   ```bash
   git clone <repository-url>
   cd astral-backend
   make setup-dev
   ```

2. **Run tests:**
   ```bash
   make test-all
   ```

3. **Build:**
   ```bash
   make build
   ```

## Code Quality

- Run `make quality` before committing
- Ensure all tests pass with `make test-all`
- Follow Go best practices and project structure

## Commit Guidelines

- Use conventional commit format
- Run `make clean` before committing
- Pre-commit hooks will prevent committing build artifacts

## Development Workflow

1. **Create a feature branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make changes and test:**
   ```bash
   make test-unit  # Run unit tests
   make quality    # Run linting and formatting
   ```

3. **Build and test Docker integration:**
   ```bash
   make docker-test  # Test full containerized stack
   ```

4. **Commit your changes:**
   ```bash
   git add .
   git commit -m "feat: add your feature description"
   ```

5. **Push and create PR:**
   ```bash
   git push origin feature/your-feature-name
   ```

## Testing Strategy

### Unit Tests (`make test-unit`)
- Test individual components and functions
- Mock external dependencies
- Run quickly and frequently

### Integration Tests (`make test-integration`)
- Test component interactions
- May require database setup
- Validate data flow between components

### Docker Integration Tests (`make docker-test`)
- Test complete containerized application
- Validate infrastructure and security
- Ensure deployment readiness

### End-to-End Tests (`make test-e2e`)
- Test complete user journeys
- Require Swiss Ephemeris CGO library
- Validate full astronomical calculations

## Architecture Overview

- **API Layer**: RESTful endpoints with authentication
- **Service Layer**: Business logic with API key management
- **Repository Layer**: MongoDB data persistence
- **Ephemeris Layer**: Swiss Ephemeris astronomical calculations
- **Infrastructure**: Docker, Redis caching, structured logging

## Code Organization

```
├── cmd/server/          # Application entry point
├── pkg/
│   ├── auth/           # Authentication and authorization
│   ├── cache/          # Redis caching layer
│   ├── handlers/       # HTTP request handlers
│   ├── middleware/     # HTTP middleware
│   ├── models/         # Data models and validation
│   ├── repository/     # Data access layer
│   └── e2e/            # End-to-end tests
├── eph/                # Swiss Ephemeris integration
├── scripts/            # Build and utility scripts
└── docker/             # Containerization configuration
```

## Security Considerations

- API keys are hashed using Argon2id
- Authentication is required for all ephemeris endpoints
- Input validation prevents injection attacks
- Structured logging for audit trails

## Performance Guidelines

- Use Redis caching for expensive calculations
- Implement worker pools for concurrent processing
- Monitor memory usage with CGO libraries
- Profile performance with `make bench`

## Questions?

- Check existing documentation in `docs/` directory
- Review test files for usage examples
- Open an issue for clarification

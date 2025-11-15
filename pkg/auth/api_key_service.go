package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"astral-backend/pkg/errors"
	"astral-backend/pkg/logger"
	"astral-backend/pkg/models"

	"go.uber.org/zap"
	"golang.org/x/crypto/argon2"
)

// APIKeyServiceInterface defines the interface for API key business logic operations
type APIKeyServiceInterface interface {
	CreateAPIKey(ctx context.Context, req models.CreateAPIKeyRequest, clientIP string) (*models.CreateAPIKeyResponse, error)
	ValidateAPIKey(ctx context.Context, apiKey string) (*models.APIKey, error)
	RevokeAPIKey(ctx context.Context, keyID string) error
	GetAPIKey(ctx context.Context, keyID string) (models.APIKeyResponse, error)
	ListAPIKeys(ctx context.Context) ([]models.APIKeyResponse, error)
}

// APIKeyRepository defines the interface for API key storage operations
type APIKeyRepository interface {
	Create(ctx context.Context, key *models.APIKey) error
	GetByID(ctx context.Context, id string) (*models.APIKey, error)
	GetByKeyHash(ctx context.Context, hash string) (*models.APIKey, error)
	Update(ctx context.Context, key *models.APIKey) error
	UpdateStatus(ctx context.Context, id string, isActive bool) error
	UpdateUsage(ctx context.Context, id string) error
	List(ctx context.Context) ([]models.APIKey, error)
	Delete(ctx context.Context, id string) error
}

// PasswordHasher defines the interface for password/key hashing
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) (bool, error)
}

// IDGenerator defines the interface for generating unique IDs
type IDGenerator interface {
	Generate() string
}

// KeyGenerator defines the interface for generating secure API keys
type KeyGenerator interface {
	Generate() string
}

// APIKeyService provides business logic for API key management
type APIKeyService struct {
	repo         APIKeyRepository
	hasher       PasswordHasher
	idGenerator  IDGenerator
	keyGenerator KeyGenerator
	logger       *logger.Logger
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(repo APIKeyRepository, hasher PasswordHasher, idGen IDGenerator, keyGen KeyGenerator, logger *logger.Logger) *APIKeyService {
	return &APIKeyService{
		repo:         repo,
		hasher:       hasher,
		idGenerator:  idGen,
		keyGenerator: keyGen,
		logger:       logger,
	}
}

// CreateAPIKey creates a new API key with the specified configuration
func (s *APIKeyService) CreateAPIKey(ctx context.Context, req models.CreateAPIKeyRequest, clientIP string) (*models.CreateAPIKeyResponse, error) {
	s.logger.Info("Creating API key",
		zap.String("name", req.Name),
		zap.Strings("scopes", req.Scopes),
		zap.String("client_ip", clientIP),
	)

	// Validate request
	if err := s.validateCreateRequest(req); err != nil {
		s.logger.Error("Invalid API key creation request", zap.Error(err))
		return nil, err
	}

	// Validate scopes
	if err := models.ValidateScopes(req.Scopes); err != nil {
		s.logger.Error("Invalid scopes in API key creation request", zap.Error(err))
		return nil, err
	}

	// Generate secure API key
	plainKey := s.keyGenerator.Generate()

	// Hash the key for storage
	hashedKey, err := s.hasher.Hash(plainKey)
	if err != nil {
		s.logger.Error("Failed to hash API key", zap.Error(err))
		return nil, fmt.Errorf("hashing API key: %w", err)
	}

	// Convert scopes to permissions
	permissions := models.ScopesToPermissions(req.Scopes)

	// Create API key record
	apiKey := &models.APIKey{
		KeyHash:     hashedKey,
		Name:        req.Name,
		Description: req.Description,
		Scopes:      req.Scopes,
		Permissions: permissions,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		ExpiresAt:   req.ExpiresAt,
		CreatedByIP: clientIP,
		UsageCount:  0,
		RateLimit:   req.RateLimit,
	}

	// Store in repository
	if err := s.repo.Create(ctx, apiKey); err != nil {
		s.logger.Error("Failed to store API key",
			zap.Error(err),
			zap.String("key_name", apiKey.Name),
		)
		return nil, fmt.Errorf("creating API key: %w", err)
	}

	s.logger.Info("API key created successfully",
		zap.String("key_name", apiKey.Name),
		zap.Strings("scopes", apiKey.Scopes),
	)

	// Return response with the plain key (only shown once)
	response := &models.CreateAPIKeyResponse{
		APIKeyResponse: apiKey.ToResponse(),
		Key:            plainKey,
	}

	return response, nil
}

// ValidateAPIKey validates an API key and returns the key details
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, providedKey string) (*models.APIKey, error) {
	// Get all API keys and verify each one (since Argon2 uses salt, we can't hash and lookup directly)
	apiKeys, err := s.repo.List(ctx)
	if err != nil {
		s.logger.Error("Failed to list API keys for validation", zap.Error(err))
		return nil, errors.NewAuthenticationError("Authentication failed")
	}

	// Find the key by verifying the hash for each active key
	for _, apiKey := range apiKeys {
		if !apiKey.IsActive {
			continue // Skip inactive keys
		}

		if apiKey.IsExpired() {
			continue // Skip expired keys
		}

		// Verify the provided key against this stored key's hash
		valid, err := s.hasher.Verify(providedKey, apiKey.KeyHash)
		if err != nil {
			s.logger.Warn("Error verifying API key hash",
				zap.String("key_id", apiKey.ID.Hex()),
				zap.Error(err),
			)
			continue
		}

		if valid {
			// Key is valid, update usage
			if err := s.repo.UpdateUsage(ctx, apiKey.ID.Hex()); err != nil {
				s.logger.Warn("Failed to update API key usage",
					zap.String("key_id", apiKey.ID.Hex()),
					zap.Error(err),
				)
				// Don't fail the authentication for usage tracking errors
			}

			s.logger.Info("API key validated successfully",
				zap.String("key_name", apiKey.Name),
				zap.String("key_id", apiKey.ID.Hex()),
			)
			return &apiKey, nil
		}
	}

	s.logger.Warn("No valid API key found for provided key")
	return nil, errors.NewAuthenticationError("Invalid API key")
}

// RevokeAPIKey revokes an API key by setting it inactive
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, keyID string) error {
	s.logger.Info("Revoking API key", zap.String("key_id", keyID))

	if err := s.repo.UpdateStatus(ctx, keyID, false); err != nil {
		s.logger.Error("Failed to revoke API key",
			zap.Error(err),
			zap.String("key_id", keyID),
		)
		return fmt.Errorf("revoking API key: %w", err)
	}

	s.logger.Info("API key revoked successfully", zap.String("key_id", keyID))
	return nil
}

// GetAPIKey retrieves an API key by ID
func (s *APIKeyService) GetAPIKey(ctx context.Context, keyID string) (models.APIKeyResponse, error) {
	apiKey, err := s.repo.GetByID(ctx, keyID)
	if err != nil {
		s.logger.Error("Failed to get API key",
			zap.Error(err),
			zap.String("key_id", keyID),
		)
		return models.APIKeyResponse{}, fmt.Errorf("getting API key: %w", err)
	}

	return apiKey.ToResponse(), nil
}

// ListAPIKeys returns all API keys (without sensitive data)
func (s *APIKeyService) ListAPIKeys(ctx context.Context) ([]models.APIKeyResponse, error) {
	apiKeys, err := s.repo.List(ctx)
	if err != nil {
		s.logger.Error("Failed to list API keys", zap.Error(err))
		return nil, fmt.Errorf("listing API keys: %w", err)
	}

	responses := make([]models.APIKeyResponse, len(apiKeys))
	for i, key := range apiKeys {
		responses[i] = key.ToResponse()
	}

	s.logger.Info("API keys listed",
		zap.Int("count", len(responses)),
	)

	return responses, nil
}

// validateCreateRequest validates the API key creation request
func (s *APIKeyService) validateCreateRequest(req models.CreateAPIKeyRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.NewValidationError("name", "Name is required")
	}

	if len(req.Name) > 100 {
		return errors.NewValidationError("name", "Name must be 100 characters or less")
	}

	if len(req.Scopes) == 0 {
		return errors.NewValidationError("scopes", "At least one scope is required")
	}

	if req.ExpiresAt != nil && req.ExpiresAt.Before(time.Now()) {
		return errors.NewValidationError("expires_at", "Expiration date must be in the future")
	}

	if req.RateLimit != nil {
		if req.RateLimit.Requests <= 0 {
			return errors.NewValidationError("rate_limit.requests", "Requests must be greater than 0")
		}
		if req.RateLimit.Window < time.Second {
			return errors.NewValidationError("rate_limit.window", "Window must be at least 1 second")
		}
	}

	return nil
}

// Secure implementations

// SecurePasswordHasher implements secure password hashing using Argon2
type SecurePasswordHasher struct{}

func (h *SecurePasswordHasher) Hash(password string) (string, error) {
	// Generate salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	// Hash with Argon2id
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, uint8(2), 32)

	// Format: $argon2id$v=19$m=65536,t=1,p=2$salt$hash
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		64*1024, // 64 MiB
		1,       // iterations
		2,       // parallelism
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func (h *SecurePasswordHasher) Verify(password, hashedPassword string) (bool, error) {
	// Parse the hash format and verify
	parts := strings.Split(hashedPassword, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("invalid hash format")
	}

	// Extract parameters
	var version int
	var memory, iterations, parallelism uint32
	fmt.Sscanf(parts[2], "v=%d", &version)
	fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decoding salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decoding hash: %w", err)
	}

	// Compute hash with provided parameters
	computedHash := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(parallelism), uint32(len(expectedHash)))

	// Constant-time comparison
	return subtleConstantTimeCompare(computedHash, expectedHash), nil
}

// SecureKeyGenerator generates cryptographically secure API keys
type SecureKeyGenerator struct{}

func (g *SecureKeyGenerator) Generate() string {
	// Generate 32 bytes of cryptographically secure random data
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("failed to generate random key: %v", err))
	}

	// Encode as URL-safe base64
	return base64.URLEncoding.EncodeToString(bytes)
}

// UUIDGenerator generates UUIDs for API key IDs
type UUIDGenerator struct{}

func (g *UUIDGenerator) Generate() string {
	// Generate a simple UUID-like string
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("failed to generate UUID: %v", err))
	}

	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// subtleConstantTimeCompare performs constant-time comparison
func subtleConstantTimeCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

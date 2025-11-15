package auth

import (
	"context"
	"testing"
	"time"

	"astral-backend/pkg/logger"
	"astral-backend/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Mock implementations
type mockAPIKeyRepo struct {
	mock.Mock
	keys map[string]*models.APIKey // Keyed by string ID for testing
}

func (m *mockAPIKeyRepo) Create(ctx context.Context, key *models.APIKey) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *mockAPIKeyRepo) GetByID(ctx context.Context, id string) (*models.APIKey, error) {
	args := m.Called(ctx, id)
	// For testing, assume the ID is a string that maps to our test keys
	if key, exists := m.keys[id]; exists {
		return key, args.Error(1)
	}
	return nil, mongo.ErrNoDocuments
}

func (m *mockAPIKeyRepo) GetByKeyHash(ctx context.Context, hash string) (*models.APIKey, error) {
	args := m.Called(ctx, hash)
	for _, key := range m.keys {
		if key.KeyHash == hash {
			return key, args.Error(1)
		}
	}
	return nil, mongo.ErrNoDocuments
}

func (m *mockAPIKeyRepo) Update(ctx context.Context, key *models.APIKey) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *mockAPIKeyRepo) UpdateStatus(ctx context.Context, id string, isActive bool) error {
	args := m.Called(ctx, id, isActive)
	if key, exists := m.keys[id]; exists {
		key.IsActive = isActive
		key.UpdatedAt = time.Now()
	}
	return args.Error(0)
}

func (m *mockAPIKeyRepo) UpdateUsage(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	if key, exists := m.keys[id]; exists {
		key.UpdateUsage()
	}
	return args.Error(0)
}

func (m *mockAPIKeyRepo) List(ctx context.Context) ([]models.APIKey, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.APIKey), args.Error(1)
}

func (m *mockAPIKeyRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	delete(m.keys, id)
	return args.Error(0)
}

type mockPasswordHasher struct {
	mock.Mock
}

func (m *mockPasswordHasher) Hash(password string) (string, error) {
	args := m.Called(password)
	return args.String(0), args.Error(1)
}

func (m *mockPasswordHasher) Verify(password, hash string) (bool, error) {
	args := m.Called(password, hash)
	return args.Bool(0), args.Error(1)
}

type mockIDGenerator struct {
	mock.Mock
}

func (m *mockIDGenerator) Generate() string {
	args := m.Called()
	return args.String(0)
}

type mockKeyGenerator struct {
	mock.Mock
}

func (m *mockKeyGenerator) Generate() string {
	args := m.Called()
	return args.String(0)
}

// Test setup helper
func setupTestService(t *testing.T) (*APIKeyService, *mockAPIKeyRepo, *mockPasswordHasher, *mockIDGenerator, *mockKeyGenerator) {
	repo := &mockAPIKeyRepo{keys: make(map[string]*models.APIKey)}
	hasher := &mockPasswordHasher{}
	idGen := &mockIDGenerator{}
	keyGen := &mockKeyGenerator{}

	logger, _ := logger.NewLogger(logger.LogConfig{
		Level: "debug", ServiceName: "test", Environment: "test",
	})

	service := NewAPIKeyService(repo, hasher, idGen, keyGen, logger)

	return service, repo, hasher, idGen, keyGen
}

func TestAPIKeyService_CreateAPIKey(t *testing.T) {
	service, repo, hasher, idGen, keyGen := setupTestService(t)

	// Setup mocks
	keyGen.On("Generate").Return("sk-test123456789")
	hasher.On("Hash", "sk-test123456789").Return("hashed-key", nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.APIKey")).Return(nil)

	req := models.CreateAPIKeyRequest{
		Name:   "Test Key",
		Scopes: []string{"read:ephemeris"},
	}

	response, err := service.CreateAPIKey(context.Background(), req, "127.0.0.1")

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "Test Key", response.Name)
	assert.Equal(t, "sk-test123456789", response.Key) // Plain key returned
	assert.True(t, response.IsActive)
	assert.Contains(t, response.Scopes, "read:ephemeris")

	// Verify repository was called
	repo.AssertExpectations(t)
	hasher.AssertExpectations(t)
	idGen.AssertExpectations(t)
	keyGen.AssertExpectations(t)
}

func TestAPIKeyService_CreateAPIKey_InvalidRequest(t *testing.T) {
	service, _, _, _, _ := setupTestService(t)

	tests := []struct {
		name        string
		req         models.CreateAPIKeyRequest
		expectError bool
		errorCheck  func(t *testing.T, err error)
	}{
		{
			name: "empty name",
			req: models.CreateAPIKeyRequest{
				Name:   "",
				Scopes: []string{"read:ephemeris"},
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "VALIDATION_ERROR")
			},
		},
		{
			name: "empty scopes",
			req: models.CreateAPIKeyRequest{
				Name: "Test Key",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "VALIDATION_ERROR")
			},
		},
		{
			name: "invalid scope",
			req: models.CreateAPIKeyRequest{
				Name:   "Test Key",
				Scopes: []string{"invalid:scope"},
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "Invalid scope")
			},
		},
		{
			name: "past expiration",
			req: models.CreateAPIKeyRequest{
				Name:      "Test Key",
				Scopes:    []string{"read:ephemeris"},
				ExpiresAt: &time.Time{}, // Zero time is in the past
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "VALIDATION_ERROR")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateAPIKey(context.Background(), tt.req, "127.0.0.1")
			if tt.expectError {
				assert.Error(t, err)
				tt.errorCheck(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPIKeyService_ValidateAPIKey(t *testing.T) {
	service, repo, hasher, _, _ := setupTestService(t)

	// Create a test key
	testKey := &models.APIKey{
		KeyHash:  "hashed-valid-key",
		Name:     "Test Key",
		Scopes:   []string{"read:ephemeris"},
		IsActive: true,
	}

	// Manually set up the key in the mock
	repo.keys["key-123"] = testKey

	// Setup mocks
	repo.On("List", mock.Anything).Return([]models.APIKey{*testKey}, nil)
	hasher.On("Verify", "valid-key", testKey.KeyHash).Return(true, nil)
	repo.On("UpdateUsage", mock.Anything, testKey.ID.Hex()).Return(nil).Maybe() // UpdateUsage is called asynchronously

	// Test valid key
	key, err := service.ValidateAPIKey(context.Background(), "valid-key")

	assert.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, "Test Key", key.Name)

	hasher.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestAPIKeyService_ValidateAPIKey_Invalid(t *testing.T) {
	service, repo, hasher, _, _ := setupTestService(t)

	// Setup mocks for invalid key - return empty list
	repo.On("List", mock.Anything).Return([]models.APIKey{}, nil)

	_, err := service.ValidateAPIKey(context.Background(), "invalid-key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AUTHENTICATION_ERROR")

	hasher.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestAPIKeyService_ValidateAPIKey_Inactive(t *testing.T) {
	service, repo, hasher, _, _ := setupTestService(t)

	// Create inactive key
	testKey := models.APIKey{
		KeyHash:  "hashed-key",
		IsActive: false, // Inactive
	}

	repo.On("List", mock.Anything).Return([]models.APIKey{testKey}, nil)
	// No Verify call expected for inactive key

	_, err := service.ValidateAPIKey(context.Background(), "inactive-key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AUTHENTICATION_ERROR")

	hasher.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestAPIKeyService_ValidateAPIKey_Expired(t *testing.T) {
	service, repo, hasher, _, _ := setupTestService(t)

	// Create expired key
	pastTime := time.Now().Add(-1 * time.Hour)
	testKey := models.APIKey{
		KeyHash:   "hashed-key",
		IsActive:  true,
		ExpiresAt: &pastTime, // Expired
	}

	repo.On("List", mock.Anything).Return([]models.APIKey{testKey}, nil)
	// No Verify call expected for expired key

	_, err := service.ValidateAPIKey(context.Background(), "expired-key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AUTHENTICATION_ERROR")

	hasher.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestAPIKeyService_RevokeAPIKey(t *testing.T) {
	service, repo, _, _, _ := setupTestService(t)

	// Create test key
	testKey := &models.APIKey{
		IsActive: true,
	}

	repo.keys["key-123"] = testKey
	repo.On("UpdateStatus", mock.Anything, "key-123", false).Return(nil)

	err := service.RevokeAPIKey(context.Background(), "key-123")

	assert.NoError(t, err)
	assert.False(t, testKey.IsActive)

	repo.AssertExpectations(t)
}

func TestAPIKeyService_GetAPIKey(t *testing.T) {
	service, repo, _, _, _ := setupTestService(t)

	// Create test key
	testKey := &models.APIKey{
		Name: "Test Key",
	}

	repo.keys["key-123"] = testKey
	repo.On("GetByID", mock.Anything, "key-123").Return(nil, nil)

	response, err := service.GetAPIKey(context.Background(), "key-123")

	assert.NoError(t, err)
	assert.Equal(t, "Test Key", response.Name)

	repo.AssertExpectations(t)
}

func TestAPIKeyService_ListAPIKeys(t *testing.T) {
	service, repo, _, _, _ := setupTestService(t)

	// Create test keys
	key1 := &models.APIKey{Name: "Key 1"}
	key2 := &models.APIKey{Name: "Key 2"}
	repo.keys["key-1"] = key1
	repo.keys["key-2"] = key2

	repo.On("List", mock.Anything).Return([]models.APIKey{
		*key1,
		*key2,
	}, nil)

	responses, err := service.ListAPIKeys(context.Background())

	assert.NoError(t, err)
	assert.Len(t, responses, 2)
	assert.Equal(t, "Key 1", responses[0].Name)
	assert.Equal(t, "Key 2", responses[1].Name)

	repo.AssertExpectations(t)
}

// Test secure implementations
func TestSecurePasswordHasher_Hash(t *testing.T) {
	hasher := &SecurePasswordHasher{}

	hash, err := hasher.Hash("test-password")

	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Contains(t, hash, "$argon2id$")
}

func TestSecurePasswordHasher_Verify(t *testing.T) {
	hasher := &SecurePasswordHasher{}

	hash, err := hasher.Hash("test-password")
	assert.NoError(t, err)

	// Valid password
	valid, err := hasher.Verify("test-password", hash)
	assert.NoError(t, err)
	assert.True(t, valid)

	// Invalid password
	valid, err = hasher.Verify("wrong-password", hash)
	assert.NoError(t, err)
	assert.False(t, valid)
}

func TestSecureKeyGenerator_Generate(t *testing.T) {
	generator := &SecureKeyGenerator{}

	key1 := generator.Generate()
	key2 := generator.Generate()

	assert.NotEmpty(t, key1)
	assert.NotEmpty(t, key2)
	assert.NotEqual(t, key1, key2)                     // Should be unique
	assert.True(t, len(key1) >= 43 && len(key1) <= 44) // base64url encoding of 32 bytes can be 43 or 44 chars
}

func TestUUIDGenerator_Generate(t *testing.T) {
	generator := &UUIDGenerator{}

	id1 := generator.Generate()
	id2 := generator.Generate()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2) // Should be unique
	assert.Contains(t, id1, "-") // Should contain dashes like UUID
}

// Benchmark tests
func BenchmarkAPIKeyService_ValidateAPIKey(b *testing.B) {
	service, repo, hasher, _, _ := setupTestService(&testing.T{})

	// Create test key
	testKey := models.APIKey{
		ID:       primitive.NewObjectID(),
		KeyHash:  "hashed-key",
		IsActive: true,
	}

	// Setup mocks
	hasher.ExpectedCalls = nil // Reset expectations for benchmark
	repo.On("List", mock.Anything).Return([]models.APIKey{testKey}, nil).Maybe()
	hasher.On("Verify", "test-key", "hashed-key").Return(true, nil).Maybe()
	repo.On("UpdateUsage", mock.Anything, testKey.ID.Hex()).Return(nil).Maybe()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.ValidateAPIKey(context.Background(), "test-key")
	}
}

func BenchmarkSecurePasswordHasher_Hash(b *testing.B) {
	hasher := &SecurePasswordHasher{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hasher.Hash("benchmark-password")
	}
}

func BenchmarkSecureKeyGenerator_Generate(b *testing.B) {
	generator := &SecureKeyGenerator{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		generator.Generate()
	}
}

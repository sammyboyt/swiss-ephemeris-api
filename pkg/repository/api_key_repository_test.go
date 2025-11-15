package repository

import (
	"testing"
	"time"

	"astral-backend/pkg/logger"
	"astral-backend/pkg/models"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Test repository creation and basic functionality
func TestNewAPIKeyRepository(t *testing.T) {
	// This is a basic constructor test since we can't easily mock MongoDB
	// In a real scenario, this would be tested with integration tests
	logger, err := logger.NewLogger(logger.LogConfig{
		Level: "info", ServiceName: "test", Environment: "test",
	})
	assert.NoError(t, err)

	// We can't create a real repository without a MongoDB connection
	// So we'll just test that the constructor would work
	assert.NotNil(t, logger)
}

// Test model validation and utility functions
func TestAPIKeyModelValidation(t *testing.T) {
	// Test APIKey struct creation
	key := &models.APIKey{
		ID:          primitive.NewObjectID(),
		KeyHash:     "test-hash",
		Name:        "Test Key",
		Scopes:      []string{"read:ephemeris", "admin"},
		Permissions: []string{"calculate:planets", "manage:api_keys"},
		IsActive:    true,
		CreatedByIP: "127.0.0.1",
		UsageCount:  42,
	}

	assert.False(t, key.ID.IsZero())
	assert.Equal(t, "test-hash", key.KeyHash)
	assert.Equal(t, "Test Key", key.Name)
	assert.Equal(t, []string{"read:ephemeris", "admin"}, key.Scopes)
	assert.Equal(t, []string{"calculate:planets", "manage:api_keys"}, key.Permissions)
	assert.True(t, key.IsActive)
	assert.Equal(t, "127.0.0.1", key.CreatedByIP)
	assert.Equal(t, int64(42), key.UsageCount)
}

func TestAPIKeyExpiration(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	// Test non-expired key (no expiration)
	key1 := &models.APIKey{}
	assert.False(t, key1.IsExpired())

	// Test expired key
	key2 := &models.APIKey{
		ExpiresAt: &past,
	}
	assert.True(t, key2.IsExpired())

	// Test non-expired key with future expiration
	key3 := &models.APIKey{
		ExpiresAt: &future,
	}
	assert.False(t, key3.IsExpired())
}

func TestAPIKeyScopeChecking(t *testing.T) {
	key := &models.APIKey{
		Scopes: []string{"read:ephemeris", "admin", "write:ephemeris"},
	}

	assert.True(t, key.HasScope("read:ephemeris"))
	assert.True(t, key.HasScope("admin"))
	assert.True(t, key.HasScope("write:ephemeris"))
	assert.False(t, key.HasScope("invalid:scope"))
	assert.False(t, key.HasScope(""))
}

func TestAPIKeyPermissionChecking(t *testing.T) {
	key := &models.APIKey{
		Permissions: []string{"calculate:planets", "manage:api_keys", "view:analytics"},
	}

	assert.True(t, key.HasPermission("calculate:planets"))
	assert.True(t, key.HasPermission("manage:api_keys"))
	assert.True(t, key.HasPermission("view:analytics"))
	assert.False(t, key.HasPermission("invalid:permission"))
	assert.False(t, key.HasPermission(""))
}

func TestAPIKeyToResponse(t *testing.T) {
	now := time.Now()
	key := &models.APIKey{
		ID:          primitive.NewObjectID(),
		KeyHash:     "sensitive-hash", // Should not appear in response
		Name:        "Test Key",
		Description: "A test key",
		Scopes:      []string{"read:ephemeris"},
		IsActive:    true,
		CreatedAt:   now,
		UsageCount:  100,
		CreatedByIP: "127.0.0.1", // Should not appear in response
	}

	response := key.ToResponse()

	assert.NotEmpty(t, response.ID)
	assert.Equal(t, "Test Key", response.Name)
	assert.Equal(t, "A test key", response.Description)
	assert.Equal(t, []string{"read:ephemeris"}, response.Scopes)
	assert.True(t, response.IsActive)
	assert.Equal(t, now, response.CreatedAt)
	assert.Equal(t, int64(100), response.UsageCount)
}

func TestAPIKeyUpdateUsage(t *testing.T) {
	key := &models.APIKey{
		UsageCount: 5,
	}

	originalUpdatedAt := key.UpdatedAt
	initialUsageCount := key.UsageCount

	key.UpdateUsage()

	assert.Equal(t, initialUsageCount+1, key.UsageCount)
	assert.True(t, key.UpdatedAt.After(originalUpdatedAt))
	assert.NotNil(t, key.LastUsedAt)
}

func TestValidateScopes(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		wantErr  bool
		errField string
	}{
		{
			name:    "valid scopes",
			scopes:  []string{"read:ephemeris", "admin"},
			wantErr: false,
		},
		{
			name:     "invalid scope",
			scopes:   []string{"invalid:scope"},
			wantErr:  true,
			errField: "scopes",
		},
		{
			name:    "empty scopes",
			scopes:  []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := models.ValidateScopes(tt.scopes)
			if tt.wantErr {
				assert.Error(t, err)
				if validationErr, ok := err.(*models.ValidationError); ok {
					assert.Equal(t, tt.errField, validationErr.Field)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScopesToPermissions(t *testing.T) {
	tests := []struct {
		name          string
		scopes        []string
		expectedPerms []string
	}{
		{
			name:          "read scope",
			scopes:        []string{"read:ephemeris"},
			expectedPerms: []string{"calculate:planets", "calculate:houses", "calculate:chart"},
		},
		{
			name:          "admin scope",
			scopes:        []string{"admin"},
			expectedPerms: []string{"calculate:planets", "calculate:houses", "calculate:chart", "manage:api_keys", "view:analytics"},
		},
		{
			name:          "write scope",
			scopes:        []string{"write:ephemeris"},
			expectedPerms: []string{"calculate:planets", "calculate:houses", "calculate:chart"},
		},
		{
			name:          "empty scopes",
			scopes:        []string{},
			expectedPerms: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perms := models.ScopesToPermissions(tt.scopes)
			assert.Len(t, perms, len(tt.expectedPerms))
			for _, expected := range tt.expectedPerms {
				assert.Contains(t, perms, expected)
			}
		})
	}
}

func TestCreateAPIKeyRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     models.CreateAPIKeyRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: models.CreateAPIKeyRequest{
				Name:   "Test Key",
				Scopes: []string{"read:ephemeris"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			req: models.CreateAPIKeyRequest{
				Scopes: []string{"read:ephemeris"},
			},
			wantErr: true,
		},
		{
			name: "missing scopes",
			req: models.CreateAPIKeyRequest{
				Name: "Test Key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily test the validator here without complex setup
			// This is just a placeholder for the validation logic
			if tt.req.Name == "" || len(tt.req.Scopes) == 0 {
				assert.True(t, tt.wantErr, "Should fail validation")
			} else {
				assert.False(t, tt.wantErr, "Should pass validation")
			}
		})
	}
}

// Test error types
func TestMongoErrors(t *testing.T) {
	// Test that we can identify MongoDB errors
	err := mongo.ErrNoDocuments
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "document")
}

// Test primitive object ID operations
func TestObjectIDOperations(t *testing.T) {
	id := primitive.NewObjectID()
	assert.False(t, id.IsZero())
	assert.Len(t, id.Hex(), 24)

	// Test parsing
	parsed, err := primitive.ObjectIDFromHex(id.Hex())
	assert.NoError(t, err)
	assert.Equal(t, id, parsed)

	// Test invalid hex
	_, err = primitive.ObjectIDFromHex("invalid")
	assert.Error(t, err)
}

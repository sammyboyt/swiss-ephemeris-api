package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestAPIKeyValidation(t *testing.T) {
	tests := []struct {
		name    string
		key     APIKey
		wantErr bool
	}{
		{
			name: "valid key",
			key: APIKey{
				Name:   "Test Key",
				Scopes: []string{"read:ephemeris"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			key: APIKey{
				Scopes: []string{"read:ephemeris"},
			},
			wantErr: true,
		},
		{
			name: "empty name",
			key: APIKey{
				Name:   "",
				Scopes: []string{"read:ephemeris"},
			},
			wantErr: true,
		},
		{
			name: "name too long",
			key: APIKey{
				Name:   string(make([]byte, 101)), // 101 characters
				Scopes: []string{"read:ephemeris"},
			},
			wantErr: true,
		},
		{
			name: "empty scopes",
			key: APIKey{
				Name: "Test Key",
			},
			wantErr: true,
		},
		{
			name: "nil scopes",
			key: APIKey{
				Name: "Test Key",
			},
			wantErr: true,
		},
		{
			name: "valid with description",
			key: APIKey{
				Name:        "Test Key",
				Description: "A test API key",
				Scopes:      []string{"read:ephemeris"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPIKeyExpiration(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		expected  bool
	}{
		{"no expiration", nil, false},
		{"not expired", &future, false},
		{"expired", &past, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := APIKey{ExpiresAt: tt.expiresAt}
			assert.Equal(t, tt.expected, key.IsExpired())
		})
	}
}

func TestAPIKeyHasScope(t *testing.T) {
	key := APIKey{
		Scopes: []string{"read:ephemeris", "admin"},
	}

	tests := []struct {
		scope    string
		expected bool
	}{
		{"read:ephemeris", true},
		{"admin", true},
		{"write:ephemeris", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run("scope_"+tt.scope, func(t *testing.T) {
			assert.Equal(t, tt.expected, key.HasScope(tt.scope))
		})
	}
}

func TestAPIKeyHasPermission(t *testing.T) {
	key := APIKey{
		Permissions: []string{"calculate:planets", "manage:api_keys"},
	}

	tests := []struct {
		permission string
		expected   bool
	}{
		{"calculate:planets", true},
		{"manage:api_keys", true},
		{"calculate:houses", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run("permission_"+tt.permission, func(t *testing.T) {
			assert.Equal(t, tt.expected, key.HasPermission(tt.permission))
		})
	}
}

func TestAPIKeyToResponse(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-1 * time.Hour)

	objectID := primitive.NewObjectID()

	key := APIKey{
		ID:          objectID,
		KeyHash:     "hashed-key", // Should not appear in response
		Name:        "Test Key",
		Description: "A test API key",
		Scopes:      []string{"read:ephemeris", "admin"},
		Permissions: []string{"calculate:planets"},
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   &now,
		LastUsedAt:  &lastUsed,
		CreatedByIP: "127.0.0.1", // Should not appear in response
		UsageCount:  42,
		RateLimit: &RateLimit{
			Requests: 100,
			Window:   time.Hour,
		},
	}

	response := key.ToResponse()

	// Check that public data is included
	assert.Equal(t, objectID.Hex(), response.ID)
	assert.Equal(t, "Test Key", response.Name)
	assert.Equal(t, "A test API key", response.Description)
	assert.Equal(t, []string{"read:ephemeris", "admin"}, response.Scopes)
	assert.True(t, response.IsActive)
	assert.Equal(t, now, response.CreatedAt)
	assert.Equal(t, &now, response.ExpiresAt)
	assert.Equal(t, &lastUsed, response.LastUsedAt)
	assert.Equal(t, int64(42), response.UsageCount)
}

func TestAPIKeyUpdateUsage(t *testing.T) {
	key := APIKey{
		UsageCount: 0,
		UpdatedAt:  time.Now().Add(-1 * time.Hour), // Old timestamp
	}

	initialUpdatedAt := key.UpdatedAt
	initialUsageCount := key.UsageCount

	// Small delay to ensure timestamp difference
	time.Sleep(1 * time.Millisecond)

	key.UpdateUsage()

	// Check that usage count increased
	assert.Equal(t, initialUsageCount+1, key.UsageCount)

	// Check that updated_at changed
	assert.True(t, key.UpdatedAt.After(initialUpdatedAt))

	// Check that last_used_at is set
	assert.NotNil(t, key.LastUsedAt)
	assert.True(t, key.LastUsedAt.After(initialUpdatedAt))
}

func TestValidateScopes(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		wantErr  bool
		errField string
	}{
		{
			name:    "valid single scope",
			scopes:  []string{"read:ephemeris"},
			wantErr: false,
		},
		{
			name:    "valid multiple scopes",
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
			name:     "mixed valid and invalid",
			scopes:   []string{"read:ephemeris", "invalid:scope"},
			wantErr:  true,
			errField: "scopes",
		},
		{
			name:    "empty scopes",
			scopes:  []string{},
			wantErr: false, // Empty scopes are allowed here, validation happens elsewhere
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScopes(tt.scopes)
			if tt.wantErr {
				assert.Error(t, err)
				if validationErr, ok := err.(*ValidationError); ok {
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
			name:          "multiple scopes",
			scopes:        []string{"read:ephemeris", "admin"},
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
		{
			name:          "invalid scope",
			scopes:        []string{"invalid:scope"},
			expectedPerms: []string{}, // Invalid scopes are ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perms := ScopesToPermissions(tt.scopes)

			// Check that all expected permissions are present
			for _, expected := range tt.expectedPerms {
				assert.Contains(t, perms, expected, "Expected permission %s not found", expected)
			}

			// Check that no unexpected permissions are present
			assert.Len(t, perms, len(tt.expectedPerms), "Unexpected number of permissions")
		})
	}
}

func TestRateLimitValidation(t *testing.T) {
	tests := []struct {
		name    string
		limit   RateLimit
		wantErr bool
	}{
		{
			name: "valid rate limit",
			limit: RateLimit{
				Requests: 100,
				Window:   time.Hour,
			},
			wantErr: false,
		},
		{
			name: "zero requests",
			limit: RateLimit{
				Requests: 0,
				Window:   time.Hour,
			},
			wantErr: true,
		},
		{
			name: "negative requests",
			limit: RateLimit{
				Requests: -1,
				Window:   time.Hour,
			},
			wantErr: true,
		},
		{
			name: "zero window",
			limit: RateLimit{
				Requests: 100,
				Window:   0,
			},
			wantErr: true,
		},
		{
			name: "negative window",
			limit: RateLimit{
				Requests: 100,
				Window:   -1 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.limit)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateAPIKeyRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateAPIKeyRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: CreateAPIKeyRequest{
				Name:   "Test Key",
				Scopes: []string{"read:ephemeris"},
			},
			wantErr: false,
		},
		{
			name:    "missing name",
			req:     CreateAPIKeyRequest{Scopes: []string{"read:ephemeris"}},
			wantErr: true,
		},
		{
			name: "missing scopes",
			req: CreateAPIKeyRequest{
				Name: "Test Key",
			},
			wantErr: true,
		},
		{
			name: "with description",
			req: CreateAPIKeyRequest{
				Name:        "Test Key",
				Description: "A test key",
				Scopes:      []string{"read:ephemeris"},
			},
			wantErr: false,
		},
		{
			name: "with expiration",
			req: CreateAPIKeyRequest{
				Name:      "Test Key",
				Scopes:    []string{"read:ephemeris"},
				ExpiresAt: &time.Time{},
			},
			wantErr: false,
		},
		{
			name: "with rate limit",
			req: CreateAPIKeyRequest{
				Name:   "Test Key",
				Scopes: []string{"read:ephemeris"},
				RateLimit: &RateLimit{
					Requests: 100,
					Window:   time.Hour,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkAPIKeyToResponse(b *testing.B) {
	key := APIKey{
		ID:          primitive.NewObjectID(),
		Name:        "Benchmark Key",
		Description: "A benchmark API key",
		Scopes:      []string{"read:ephemeris", "admin"},
		IsActive:    true,
		CreatedAt:   time.Now(),
		UsageCount:  1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = key.ToResponse()
	}
}

func BenchmarkScopesToPermissions(b *testing.B) {
	scopes := []string{"read:ephemeris", "admin"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ScopesToPermissions(scopes)
	}
}

func BenchmarkValidateScopes(b *testing.B) {
	scopes := []string{"read:ephemeris", "admin"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateScopes(scopes)
	}
}

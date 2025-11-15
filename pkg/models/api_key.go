package models

import (
	"time"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// APIKey represents an API key entity stored in the database
type APIKey struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	KeyHash     string             `json:"-" bson:"key_hash"` // Never exposed in JSON
	Name        string             `json:"name" bson:"name" validate:"required,min=1,max=100"`
	Description string             `json:"description,omitempty" bson:"description"`
	Scopes      []string           `json:"scopes" bson:"scopes" validate:"required,min=1"`
	Permissions []string           `json:"permissions" bson:"permissions"`
	IsActive    bool               `json:"is_active" bson:"is_active"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" bson:"updated_at"`
	ExpiresAt   *time.Time         `json:"expires_at,omitempty" bson:"expires_at,omitempty"`
	LastUsedAt  *time.Time         `json:"last_used_at,omitempty" bson:"last_used_at,omitempty"`
	CreatedByIP string             `json:"-" bson:"created_by_ip"`
	UsageCount  int64              `json:"usage_count" bson:"usage_count"`
	RateLimit   *RateLimit         `json:"rate_limit,omitempty" bson:"rate_limit,omitempty"`
}

// RateLimit defines rate limiting configuration for an API key
type RateLimit struct {
	Requests int           `json:"requests" bson:"requests" validate:"min=1"`
	Window   time.Duration `json:"window" bson:"window" validate:"min=1s"`
}

// CreateAPIKeyRequest represents the request to create a new API key
type CreateAPIKeyRequest struct {
	Name        string     `json:"name" validate:"required,min=1,max=100"`
	Description string     `json:"description,omitempty"`
	Scopes      []string   `json:"scopes" validate:"required,min=1"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RateLimit   *RateLimit `json:"rate_limit,omitempty"`
}

// APIKeyResponse represents the API key data returned to clients (without sensitive info)
type APIKeyResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Scopes      []string   `json:"scopes"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	UsageCount  int64      `json:"usage_count"`
}

// CreateAPIKeyResponse represents the response when creating a new API key (includes the key)
type CreateAPIKeyResponse struct {
	APIKeyResponse
	Key string `json:"key"` // Only shown once during creation
}

// IsExpired checks if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false // No expiration
	}
	return time.Now().After(*k.ExpiresAt)
}

// HasScope checks if the API key has a specific scope
func (k *APIKey) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasPermission checks if the API key has a specific permission
func (k *APIKey) HasPermission(permission string) bool {
	for _, p := range k.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// ToResponse converts APIKey to APIKeyResponse (safe for client consumption)
func (k *APIKey) ToResponse() APIKeyResponse {
	id := ""
	if !k.ID.IsZero() {
		id = k.ID.Hex()
	}

	return APIKeyResponse{
		ID:          id,
		Name:        k.Name,
		Description: k.Description,
		Scopes:      k.Scopes,
		IsActive:    k.IsActive,
		CreatedAt:   k.CreatedAt,
		ExpiresAt:   k.ExpiresAt,
		LastUsedAt:  k.LastUsedAt,
		UsageCount:  k.UsageCount,
	}
}

// UpdateUsage increments the usage count and updates last used timestamp
func (k *APIKey) UpdateUsage() {
	now := time.Now()
	k.LastUsedAt = &now
	k.UsageCount++
	k.UpdatedAt = now
}

// ValidateScopes validates that all provided scopes are valid
func ValidateScopes(scopes []string) error {
	validScopes := map[string]bool{
		"read:ephemeris":  true,
		"write:ephemeris": true,
		"admin":           true,
	}

	for _, scope := range scopes {
		if !validScopes[scope] {
			return &ValidationError{
				Field:   "scopes",
				Message: "Invalid scope: " + scope,
			}
		}
	}

	return nil
}

// ScopesToPermissions converts scopes to a list of permissions
func ScopesToPermissions(scopes []string) []string {
	permissions := make(map[string]bool)

	scopePermissions := map[string][]string{
		"read:ephemeris": {
			"calculate:planets",
			"calculate:houses",
			"calculate:chart",
		},
		"write:ephemeris": {
			"calculate:planets",
			"calculate:houses",
			"calculate:chart",
		},
		"admin": {
			"calculate:planets",
			"calculate:houses",
			"calculate:chart",
			"manage:api_keys",
			"view:analytics",
		},
	}

	for _, scope := range scopes {
		if perms, exists := scopePermissions[scope]; exists {
			for _, perm := range perms {
				permissions[perm] = true
			}
		}
	}

	result := make([]string, 0, len(permissions))
	for perm := range permissions {
		result = append(result, perm)
	}

	return result
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

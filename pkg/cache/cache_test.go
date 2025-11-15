package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockCache is a test implementation of the Cache interface
type MockCache struct {
	mock.Mock
}

// NewMockCache creates a new mock cache
func NewMockCache() *MockCache {
	return &MockCache{}
}

func (m *MockCache) Get(ctx context.Context, key string) (interface{}, bool) {
	args := m.Called(ctx, key)
	return args.Get(0), args.Bool(1)
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCache) Clear(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestCacheInterface(t *testing.T) {
	cache := &MockCache{}

	// RED: Should store and retrieve values
	key := "test-key"
	value := map[string]interface{}{"data": "test"}

	cache.On("Set", mock.Anything, key, value, time.Hour).Return(nil)
	cache.On("Get", mock.Anything, key).Return(value, true)

	err := cache.Set(context.Background(), key, value, time.Hour)
	assert.NoError(t, err)

	retrieved, found := cache.Get(context.Background(), key)
	assert.True(t, found)
	assert.Equal(t, value, retrieved)

	cache.AssertExpectations(t)
}

func TestCacheExpiration(t *testing.T) {
	cache := &MockCache{}

	// RED: Should expire values after TTL
	key := "expiring-key"
	value := "test-value"

	cache.On("Set", mock.Anything, key, value, 100*time.Millisecond).Return(nil)
	cache.On("Get", mock.Anything, key).Return(value, true).Once()
	cache.On("Get", mock.Anything, key).Return(nil, false).Once()

	err := cache.Set(context.Background(), key, value, 100*time.Millisecond)
	assert.NoError(t, err)

	// Should exist immediately
	retrieved, found := cache.Get(context.Background(), key)
	assert.True(t, found)
	assert.Equal(t, value, retrieved)

	// Should expire after TTL
	time.Sleep(150 * time.Millisecond)
	retrieved, found = cache.Get(context.Background(), key)
	assert.False(t, found)
	assert.Nil(t, retrieved)

	cache.AssertExpectations(t)
}

func TestCacheDelete(t *testing.T) {
	cache := &MockCache{}

	// RED: Should delete values
	key := "delete-key"
	value := "test-value"

	cache.On("Set", mock.Anything, key, value, time.Hour).Return(nil)
	cache.On("Delete", mock.Anything, key).Return(nil)
	cache.On("Get", mock.Anything, key).Return(nil, false)

	err := cache.Set(context.Background(), key, value, time.Hour)
	assert.NoError(t, err)

	err = cache.Delete(context.Background(), key)
	assert.NoError(t, err)

	retrieved, found := cache.Get(context.Background(), key)
	assert.False(t, found)
	assert.Nil(t, retrieved)

	cache.AssertExpectations(t)
}

func TestCacheClear(t *testing.T) {
	cache := &MockCache{}

	// RED: Should clear all values
	cache.On("Clear", mock.Anything).Return(nil)

	err := cache.Clear(context.Background())
	assert.NoError(t, err)

	cache.AssertExpectations(t)
}

// Redis-specific tests (skip if Redis not available)
func TestRedisCache_Connection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test")
	}

	config := CacheConfig{RedisURL: "redis://localhost:6379"}
	logger, _ := zap.NewDevelopment()
	cache, err := NewRedisCache(config, logger)

	// RED: Should connect to Redis successfully
	assert.NoError(t, err)
	assert.NotNil(t, cache)
}

func TestRedisCache_BasicOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test")
	}

	config := CacheConfig{RedisURL: "redis://localhost:6379"}
	logger, _ := zap.NewDevelopment()
	cache, err := NewRedisCache(config, logger)
	require.NoError(t, err)

	// RED: Should perform basic cache operations
	key := "redis-test-key"
	value := []string{"item1", "item2", "item3"}

	err = cache.Set(context.Background(), key, value, time.Minute)
	assert.NoError(t, err)

	retrieved, found := cache.Get(context.Background(), key)
	assert.True(t, found)

	retrievedSlice, ok := retrieved.([]interface{})
	assert.True(t, ok)
	assert.Len(t, retrievedSlice, 3)

	// Clean up
	err = cache.Delete(context.Background(), key)
	assert.NoError(t, err)
}

func TestRedisCache_JSONSerialization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test")
	}

	config := CacheConfig{RedisURL: "redis://localhost:6379"}
	logger, _ := zap.NewDevelopment()
	cache, err := NewRedisCache(config, logger)
	require.NoError(t, err)

	// RED: Should handle complex data structures
	key := "complex-key"
	value := map[string]interface{}{
		"planets": []map[string]interface{}{
			{
				"id":         0,
				"name":       "Sun",
				"longitude":  123.45,
				"retrograde": false,
			},
		},
		"timestamp": time.Now().Unix(),
	}

	err = cache.Set(context.Background(), key, value, time.Minute)
	assert.NoError(t, err)

	retrieved, found := cache.Get(context.Background(), key)
	assert.True(t, found)

	retrievedMap, ok := retrieved.(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, retrievedMap, "planets")

	// Clean up
	cache.Delete(context.Background(), key)
}

func TestRedisCache_Expiration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test")
	}

	config := CacheConfig{RedisURL: "redis://localhost:6379"}
	logger, _ := zap.NewDevelopment()
	cache, err := NewRedisCache(config, logger)
	require.NoError(t, err)

	// RED: Should respect TTL
	key := "ttl-test-key"
	value := "expires-soon"

	err = cache.Set(context.Background(), key, value, 100*time.Millisecond)
	assert.NoError(t, err)

	// Should exist immediately
	retrieved, found := cache.Get(context.Background(), key)
	assert.True(t, found)
	assert.Equal(t, value, retrieved)

	// Should expire after TTL
	time.Sleep(150 * time.Millisecond)
	retrieved, found = cache.Get(context.Background(), key)
	assert.False(t, found)
}

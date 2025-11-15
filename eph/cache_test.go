package eph

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// mockCache is a test implementation of the Cache interface
type mockCache struct {
	mock.Mock
}

func (m *mockCache) Get(ctx context.Context, key string) (interface{}, bool) {
	args := m.Called(ctx, key)
	return args.Get(0), args.Bool(1)
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *mockCache) Clear(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestCachedEphemerisService_Planets(t *testing.T) {
	mockCache := &mockCache{}
	logger, _ := zap.NewDevelopment()
	service := NewCachedEphemerisService(mockCache, logger)

	// Setup mock expectations
	expectedPlanets := []Planet{
		{ID: 0, Name: "Sun", Longitude: 123.45, Retrograde: false},
		{ID: 1, Name: "Moon", Longitude: 234.56, Retrograde: false},
	}

	// First call - cache miss
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]eph.Planet"), mock.AnythingOfType("time.Duration")).Return(nil).Once()

	// RED: Should cache planet calculations
	planets1, err := service.GetPlanetsCached(context.Background(), 2024, 1, 15, 12.0)
	assert.NoError(t, err)
	assert.NotEmpty(t, planets1)

	// Second call should use cache
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(expectedPlanets, true).Once()

	planets2, err := service.GetPlanetsCached(context.Background(), 2024, 1, 15, 12.0)
	assert.NoError(t, err)
	assert.Equal(t, expectedPlanets, planets2)

	mockCache.AssertExpectations(t)
}

func TestCachedEphemerisService_Houses(t *testing.T) {
	mockCache := &mockCache{}
	logger, _ := zap.NewDevelopment()
	service := NewCachedEphemerisService(mockCache, logger)

	expectedHouses := []House{
		{ID: 1, Longitude: 45.67, Hsys: "P"},
		{ID: 2, Longitude: 75.89, Hsys: "P"},
	}

	// First call - cache miss
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]eph.House"), mock.AnythingOfType("time.Duration")).Return(nil).Once()

	// RED: Should cache house calculations
	houses1, err := service.GetHousesCached(context.Background(), 2024, 1, 15, 12.0, 40.7128, -74.0060)
	assert.NoError(t, err)
	assert.NotEmpty(t, houses1)

	// Second call should use cache
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(expectedHouses, true).Once()

	houses2, err := service.GetHousesCached(context.Background(), 2024, 1, 15, 12.0, 40.7128, -74.0060)
	assert.NoError(t, err)
	assert.Equal(t, expectedHouses, houses2)

	mockCache.AssertExpectations(t)
}

func TestCachedEphemerisService_Chart(t *testing.T) {
	mockCache := &mockCache{}
	logger, _ := zap.NewDevelopment()
	service := NewCachedEphemerisService(mockCache, logger)

	expectedPlanets := []Planet{{ID: 0, Name: "Sun"}}
	expectedHouses := []House{{ID: 1, Longitude: 45.67, Hsys: "P"}}

	chartData := map[string]interface{}{
		"planets": expectedPlanets,
		"houses":  expectedHouses,
	}

	// First call - cache miss
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}"), mock.AnythingOfType("time.Duration")).Return(nil).Once()

	// RED: Should cache chart calculations
	planets1, houses1, err := service.GetChartCached(context.Background(), 2024, 1, 15, 12.0, 40.7128, -74.0060)
	assert.NoError(t, err)
	assert.NotEmpty(t, planets1)
	assert.NotEmpty(t, houses1)

	// Second call should use cache
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(chartData, true).Once()

	planets2, houses2, err := service.GetChartCached(context.Background(), 2024, 1, 15, 12.0, 40.7128, -74.0060)
	assert.NoError(t, err)
	assert.Equal(t, expectedPlanets, planets2)
	assert.Equal(t, expectedHouses, houses2)

	mockCache.AssertExpectations(t)
}

func TestCachedEphemerisService_CacheCorruption(t *testing.T) {
	mockCache := &mockCache{}
	logger, _ := zap.NewDevelopment()
	service := NewCachedEphemerisService(mockCache, logger)

	// RED: Should handle cache corruption gracefully
	// Return invalid data type from cache
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return("invalid data", true).Once()
	mockCache.On("Delete", mock.Anything, mock.AnythingOfType("string")).Return(nil).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]eph.Planet"), mock.AnythingOfType("time.Duration")).Return(nil).Once()

	// Should calculate fresh data despite cache hit with wrong type
	planets, err := service.GetPlanetsCached(context.Background(), 2024, 1, 15, 12.0)
	assert.NoError(t, err)
	assert.NotEmpty(t, planets)

	mockCache.AssertExpectations(t)
}

func TestCachedEphemerisService_CacheFailure(t *testing.T) {
	mockCache := &mockCache{}
	logger, _ := zap.NewDevelopment()
	service := NewCachedEphemerisService(mockCache, logger)

	// RED: Should work even if caching fails
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]eph.Planet"), mock.AnythingOfType("time.Duration")).Return(assert.AnError).Once()

	// Should still return valid data despite cache failure
	planets, err := service.GetPlanetsCached(context.Background(), 2024, 1, 15, 12.0)
	assert.NoError(t, err)
	assert.NotEmpty(t, planets)

	mockCache.AssertExpectations(t)
}

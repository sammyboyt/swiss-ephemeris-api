package eph

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestCachedEphemerisService(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		params         []interface{}
		cacheSetup     func(*mockCache)
		validateResult func(t *testing.T, result interface{}, err error)
		expectCacheHit bool
	}{
		{
			name:   "planets_cache_miss_and_hit",
			method: "GetPlanetsCached",
			params: []interface{}{2024, 1, 15, 12.0},
			cacheSetup: func(mc *mockCache) {
				expectedBodies := []CelestialBody{
					{ID: 0, Name: "Sun", Longitude: 123.45, Retrograde: false, Type: TypePlanet},
					{ID: 1, Name: "Moon", Longitude: 234.56, Retrograde: false, Type: TypePlanet},
				}
				// First call - cache miss
				mc.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
				mc.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]eph.CelestialBody"), mock.AnythingOfType("time.Duration")).Return(nil).Once()
				// Second call - cache hit
				mc.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(expectedBodies, true).Once()
			},
			validateResult: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				bodies, ok := result.([]CelestialBody)
				assert.True(t, ok)
				assert.NotEmpty(t, bodies)
				for _, body := range bodies {
					assertValidCelestialBody(t, body)
				}
			},
			expectCacheHit: true,
		},
		{
			name:   "houses_cache_miss_and_hit",
			method: "GetHousesCached",
			params: []interface{}{2024, 1, 15, 12.0, 40.7128, -74.0060},
			cacheSetup: func(mc *mockCache) {
				expectedHouses := []House{
					{ID: 1, Longitude: 45.67, Hsys: "P"},
					{ID: 2, Longitude: 75.89, Hsys: "P"},
				}
				// First call - cache miss
				mc.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
				mc.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]eph.House"), mock.AnythingOfType("time.Duration")).Return(nil).Once()
				// Second call - cache hit
				mc.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(expectedHouses, true).Once()
			},
			validateResult: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				houses, ok := result.([]House)
				assert.True(t, ok)
				assert.NotEmpty(t, houses)
				for _, house := range houses {
					assertValidHouse(t, house)
				}
			},
			expectCacheHit: true,
		},
		{
			name:   "chart_cache_miss_and_hit",
			method: "GetChartCached",
			params: []interface{}{2024, 1, 15, 12.0, 40.7128, -74.0060},
			cacheSetup: func(mc *mockCache) {
				expectedBodies := []CelestialBody{{ID: 0, Name: "Sun", Type: TypePlanet}}
				expectedHouses := []House{{ID: 1, Longitude: 45.67, Hsys: "P"}}
				chartData := map[string]interface{}{
					"bodies": expectedBodies,
					"houses": expectedHouses,
				}
				// First call - cache miss
				mc.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
				mc.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]interface {}"), mock.AnythingOfType("time.Duration")).Return(nil).Once()
				// Second call - cache hit
				mc.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(chartData, true).Once()
			},
			validateResult: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				results := result.([]interface{})
				bodies, houses := results[0].([]CelestialBody), results[1].([]House)
				assert.NotEmpty(t, bodies)
				assert.NotEmpty(t, houses)
				for _, body := range bodies {
					assertValidCelestialBody(t, body)
				}
				for _, house := range houses {
					assertValidHouse(t, house)
				}
			},
			expectCacheHit: true,
		},
		{
			name:   "cache_corruption_handling",
			method: "GetPlanetsCached",
			params: []interface{}{2024, 1, 15, 12.0},
			cacheSetup: func(mc *mockCache) {
				// Return invalid data type from cache
				mc.On("Get", mock.Anything, mock.AnythingOfType("string")).Return("invalid data", true).Once()
				mc.On("Delete", mock.Anything, mock.AnythingOfType("string")).Return(nil).Once()
				mc.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]eph.CelestialBody"), mock.AnythingOfType("time.Duration")).Return(nil).Once()
			},
			validateResult: func(t *testing.T, result interface{}, err error) {
				// Should calculate fresh data despite cache corruption
				assert.NoError(t, err)
				bodies, ok := result.([]CelestialBody)
				assert.True(t, ok)
				assert.NotEmpty(t, bodies)
				for _, body := range bodies {
					assertValidCelestialBody(t, body)
				}
			},
			expectCacheHit: false,
		},
		{
			name:   "cache_failure_graceful_degradation",
			method: "GetPlanetsCached",
			params: []interface{}{2024, 1, 15, 12.0},
			cacheSetup: func(mc *mockCache) {
				// Cache get fails
				mc.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
				// Cache set fails
				mc.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]eph.CelestialBody"), mock.AnythingOfType("time.Duration")).Return(assert.AnError).Once()
			},
			validateResult: func(t *testing.T, result interface{}, err error) {
				// Should still return calculation results despite cache failure
				assert.NoError(t, err)
				bodies, ok := result.([]CelestialBody)
				assert.True(t, ok)
				assert.NotEmpty(t, bodies)
				for _, body := range bodies {
					assertValidCelestialBody(t, body)
				}
			},
			expectCacheHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := &mockCache{}
			logger := createTestLogger(t)
			service := NewCachedEphemerisService(mockCache, logger)

			// Setup cache expectations
			tt.cacheSetup(mockCache)

			// First call (cache miss)
			var result1 interface{}
			var err1 error

			switch tt.method {
			case "GetPlanetsCached":
				result1, err1 = service.GetPlanetsCached(context.Background(), tt.params[0].(int), tt.params[1].(int), tt.params[2].(int), tt.params[3].(float64))
			case "GetHousesCached":
				result1, err1 = service.GetHousesCached(context.Background(), tt.params[0].(int), tt.params[1].(int), tt.params[2].(int), tt.params[3].(float64), tt.params[4].(float64), tt.params[5].(float64))
			case "GetChartCached":
				bodies, houses, err := service.GetChartCached(context.Background(), tt.params[0].(int), tt.params[1].(int), tt.params[2].(int), tt.params[3].(float64), tt.params[4].(float64), tt.params[5].(float64))
				result1 = []interface{}{bodies, houses}
				err1 = err
			}

			tt.validateResult(t, result1, err1)

			// Second call if cache hit expected
			if tt.expectCacheHit {
				var result2 interface{}
				var err2 error

				switch tt.method {
				case "GetPlanetsCached":
					result2, err2 = service.GetPlanetsCached(context.Background(), tt.params[0].(int), tt.params[1].(int), tt.params[2].(int), tt.params[3].(float64))
				case "GetHousesCached":
					result2, err2 = service.GetHousesCached(context.Background(), tt.params[0].(int), tt.params[1].(int), tt.params[2].(int), tt.params[3].(float64), tt.params[4].(float64), tt.params[5].(float64))
				case "GetChartCached":
					bodies, houses, err := service.GetChartCached(context.Background(), tt.params[0].(int), tt.params[1].(int), tt.params[2].(int), tt.params[3].(float64), tt.params[4].(float64), tt.params[5].(float64))
					result2 = []interface{}{bodies, houses}
					err2 = err
				}

				tt.validateResult(t, result2, err2)
			}

			mockCache.AssertExpectations(t)
		})
	}
}

func TestCachedEphemerisService_CalculateBodies(t *testing.T) {
	mockCache := &mockCache{}
	logger := createTestLogger(t)
	service := NewCachedEphemerisService(mockCache, logger)

	expectedResult := mockCalculationResult([]CelestialBody{
		{ID: 0, Name: "Sun", Longitude: 280.45, Type: TypePlanet},
		{ID: 1, Name: "Moon", Longitude: 123.67, Type: TypePlanet},
	})

	timeReq := AstroTimeRequest{
		Year:      2024,
		Month:     1,
		Day:       1,
		UT:        12.0,
		Gregorian: true,
	}

	config := GetTraditionalBodiesConfig()

	// Cache miss
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*eph.EphemerisResult"), mock.AnythingOfType("time.Duration")).Return(nil).Once()

	// Test CalculateBodies
	result, err := service.CalculateBodies(createTestContext(), timeReq, config)
	assert.NoError(t, err)
	validateEphemerisResult(t, result)

	// Cache hit
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(expectedResult, true).Once()

	result2, err := service.CalculateBodies(createTestContext(), timeReq, config)
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result2)
	assert.True(t, result2.Metadata.Cached)

	mockCache.AssertExpectations(t)
}

func TestCachedEphemerisService_GetTraditionalBodies(t *testing.T) {
	mockCache := &mockCache{}
	logger := createTestLogger(t)
	service := NewCachedEphemerisService(mockCache, logger)

	timeReq := AstroTimeRequest{
		Year:      2024,
		Month:     1,
		Day:       1,
		UT:        12.0,
		Gregorian: true,
	}

	// Cache miss
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*eph.EphemerisResult"), mock.AnythingOfType("time.Duration")).Return(nil).Once()

	bodies, err := service.GetTraditionalBodies(createTestContext(), timeReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, bodies)
	for _, body := range bodies {
		assertValidCelestialBody(t, body)
	}

	mockCache.AssertExpectations(t)
}

func TestCachedEphemerisService_GetExtendedBodies(t *testing.T) {
	mockCache := &mockCache{}
	logger := createTestLogger(t)
	service := NewCachedEphemerisService(mockCache, logger)

	timeReq := AstroTimeRequest{
		Year:      2024,
		Month:     1,
		Day:       1,
		UT:        12.0,
		Gregorian: true,
	}

	// Cache miss
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*eph.EphemerisResult"), mock.AnythingOfType("time.Duration")).Return(nil).Once()

	bodies, err := service.GetExtendedBodies(createTestContext(), timeReq, []CelestialBodyType{TypeNode, TypeCentaur})
	assert.NoError(t, err)
	assert.NotEmpty(t, bodies)
	for _, body := range bodies {
		assertValidCelestialBody(t, body)
	}

	mockCache.AssertExpectations(t)
}

func TestCachedEphemerisService_GetFixedStars(t *testing.T) {
	mockCache := &mockCache{}
	logger := createTestLogger(t)
	service := NewCachedEphemerisService(mockCache, logger)

	timeReq := AstroTimeRequest{
		Year:      2024,
		Month:     1,
		Day:       1,
		UT:        12.0,
		Gregorian: true,
	}

	constellations := []string{"Leo", "Virgo"}

	// Cache miss
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]eph.Constellation"), mock.AnythingOfType("time.Duration")).Return(nil).Once()

	stars, err := service.GetFixedStars(createTestContext(), timeReq, constellations)
	assert.NoError(t, err)
	// Note: Fixed stars might return empty if Swiss Ephemeris data is not available
	// This is acceptable - the test verifies the method doesn't crash
	for _, constellation := range stars {
		assert.NotEmpty(t, constellation.Name)
		for _, star := range constellation.Stars {
			assertValidCelestialBody(t, star)
		}
	}

	mockCache.AssertExpectations(t)
}

func TestCachedEphemerisService_GetFullChart(t *testing.T) {
	mockCache := &mockCache{}
	logger := createTestLogger(t)
	service := NewCachedEphemerisService(mockCache, logger)

	timeReq := AstroTimeRequest{
		Year:      2024,
		Month:     1,
		Day:       1,
		UT:        12.0,
		Gregorian: true,
	}

	// Cache miss - this will call both bodies and houses calculations
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*eph.EphemerisResult"), mock.AnythingOfType("time.Duration")).Return(nil).Once()
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]eph.House"), mock.AnythingOfType("time.Duration")).Return(nil).Once()

	result, err := service.GetFullChart(createTestContext(), timeReq, 40.7128, -74.0060)
	assert.NoError(t, err)
	validateEphemerisResult(t, result)

	mockCache.AssertExpectations(t)
}

// TestCachedEphemerisService_GetFullChart_SeparateHouses tests that GetFullChart returns houses in separate field
func TestCachedEphemerisService_GetFullChart_SeparateHouses(t *testing.T) {
	mockCache := &mockCache{}
	logger := createTestLogger(t)
	service := NewCachedEphemerisService(mockCache, logger)

	timeReq := AstroTimeRequest{
		Year:      2024,
		Month:     1,
		Day:       1,
		UT:        12.0,
		Gregorian: true,
	}

	// Cache miss - this will call both bodies and houses calculations
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*eph.EphemerisResult"), mock.AnythingOfType("time.Duration")).Return(nil).Once()
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, false).Once()
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("[]eph.House"), mock.AnythingOfType("time.Duration")).Return(nil).Once()

	result, err := service.GetFullChart(createTestContext(), timeReq, 40.7128, -74.0060)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify houses are in separate field, not mixed with bodies
	assert.NotEmpty(t, result.Houses, "Houses should be in separate field")
	assert.Len(t, result.Houses, 12, "Should have exactly 12 houses")

	// Verify no house objects are in the bodies array
	hasHouseInBodies := false
	for _, body := range result.Bodies {
		if body.Type == "house" {
			hasHouseInBodies = true
			break
		}
	}
	assert.False(t, hasHouseInBodies, "Houses should not be mixed with celestial bodies")

	// Verify house properties
	for _, house := range result.Houses {
		assert.True(t, house.ID >= 1 && house.ID <= 12, "House ID should be between 1 and 12")
		assert.True(t, house.Longitude >= 0 && house.Longitude < 360, "House longitude should be valid")
		assert.Equal(t, "P", house.Hsys, "House system should be Placidus")
	}

	// Verify all house IDs 1-12 are present
	houseIDs := make(map[int]bool)
	for _, house := range result.Houses {
		houseIDs[house.ID] = true
	}
	for i := 1; i <= 12; i++ {
		assert.True(t, houseIDs[i], "House ID %d should be present", i)
	}

	mockCache.AssertExpectations(t)
}

// TestCachedEphemerisService_ZodiacCaching tests Zodiac-specific caching behavior
func TestCachedEphemerisService_ZodiacCaching(t *testing.T) {
	t.Run("zodiac_constellation_expansion", func(t *testing.T) {
		// Test that Zodiac expands to all 12 constellations
		expanded := ExpandZodiacConstellations([]string{ZodiacAbbrev})
		assert.Len(t, expanded, 12)

		expectedZodiac := []string{"Ari", "Tau", "Gem", "Cnc", "Leo", "Vir", "Lib", "Sco", "Sgr", "Cap", "Aqr", "Psc"}
		assert.Equal(t, expectedZodiac, expanded)
	})

	t.Run("zodiac_with_additional_constellations", func(t *testing.T) {
		// Test Zodiac + additional constellations
		expanded := ExpandZodiacConstellations([]string{ZodiacAbbrev, "UMa", "Ori"})
		assert.Len(t, expanded, 14) // 12 zodiac + 2 additional

		// Should contain all zodiac constellations plus the additional ones
		assert.Contains(t, expanded, "Leo")
		assert.Contains(t, expanded, "Vir")
		assert.Contains(t, expanded, "UMa")
		assert.Contains(t, expanded, "Ori")
	})

	t.Run("zodiac_deduplication", func(t *testing.T) {
		// Test that duplicates are removed
		expanded := ExpandZodiacConstellations([]string{ZodiacAbbrev, "Leo"})
		assert.Len(t, expanded, 12) // Should still be 12, no duplication

		// Count occurrences of Leo
		leoCount := 0
		for _, constell := range expanded {
			if constell == "Leo" {
				leoCount++
			}
		}
		assert.Equal(t, 1, leoCount, "Leo should appear only once")
	})

	t.Run("zodiac_cache_key_consistency", func(t *testing.T) {
		// Test that the same constellation set produces consistent results
		// Direct zodiac constellations
		directZodiac := []string{"Ari", "Tau", "Gem", "Cnc", "Leo", "Vir", "Lib", "Sco", "Sgr", "Cap", "Aqr", "Psc"}

		// Expanded from "Zodiac"
		expandedZodiac := ExpandZodiacConstellations([]string{ZodiacAbbrev})

		assert.Equal(t, directZodiac, expandedZodiac,
			"Direct zodiac list should match expanded Zodiac list")

		// In a real scenario, both would generate the same cache key
		// We can't test the actual caching here due to CGO limitations,
		// but we validate the expansion logic
	})

	t.Run("zodiac_result_structure", func(t *testing.T) {
		// Test that zodiac-related functions return expected data structures
		zodiacMembers := getZodiacMemberConstellations()
		assert.Len(t, zodiacMembers, 12)

		// Verify all members are valid constellations
		for _, member := range zodiacMembers {
			_, exists := GetConstellationByAbbrev(member)
			assert.True(t, exists, "Zodiac member %s should be a valid constellation", member)
		}

		// Verify Zodiac constellation exists
		zodiacConst, exists := GetConstellationByAbbrev(ZodiacAbbrev)
		assert.True(t, exists, "Zodiac meta-constellation should exist")
		assert.Equal(t, ZodiacAbbrev, zodiacConst.Abbrev)
		assert.Equal(t, 0.0, zodiacConst.RAStart)
		assert.Equal(t, 360.0, zodiacConst.RAEnd)
	})
}

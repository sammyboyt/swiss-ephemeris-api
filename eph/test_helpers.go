// Package eph provides test helper functions for the ephemeris package.
// These functions are designed to reduce code duplication and improve test maintainability.
package eph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"astral-backend/pkg/cache"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// createTestEphemerisService creates a CachedEphemerisService with a mock cache for testing.
//
//nolint:unused
func createTestEphemerisService(t *testing.T, cache cache.Cache) *CachedEphemerisService {
	t.Helper()
	t.Helper()

	logger, err := zap.NewDevelopment()
	assert.NoError(t, err, "Failed to create test logger")

	return NewCachedEphemerisService(cache, logger)
}

// createTestLogger creates a development logger for testing.
func createTestLogger(t *testing.T) *zap.Logger {
	t.Helper()

	logger, err := zap.NewDevelopment()
	assert.NoError(t, err, "Failed to create test logger")
	return logger
}

// assertValidCelestialBody validates that a CelestialBody has all required fields set correctly.
func assertValidCelestialBody(t *testing.T, body CelestialBody) {
	t.Helper()

	assert.NotEmpty(t, body.Name, "Body name should not be empty")
	assert.True(t, body.ID >= 0, "Body ID should be non-negative")
	assert.True(t, body.Longitude >= 0 && body.Longitude < 360, "Body longitude should be 0-360 degrees")
	assert.True(t, body.Latitude >= -90 && body.Latitude <= 90, "Body latitude should be -90 to +90 degrees")
	assert.NotEmpty(t, body.Type, "Body type should not be empty")

	// Speed validation (if speed calculation is enabled)
	if body.SpeedLongitude != 0 {
		// Retrograde check - for outer planets, retrograde motion is common
		if body.Retrograde && body.SpeedLongitude > 0 {
			t.Errorf("Body %s is marked retrograde but has positive speed: %.6f", body.Name, body.SpeedLongitude)
		}
	}
}

// assertValidHouse validates that a House has all required fields set correctly.
func assertValidHouse(t *testing.T, house House) {
	t.Helper()

	assert.True(t, house.ID >= 0 && house.ID <= 12, "House ID should be 0-12")
	assert.True(t, house.Longitude >= 0 && house.Longitude < 360, "House longitude should be 0-360 degrees")
	assert.NotEmpty(t, house.Hsys, "House system should not be empty")
}

// assertValidConstellation validates that a Constellation has all required fields.
//
//nolint:unused
func assertValidConstellation(t *testing.T, constell ConstellationDefinition) {
	t.Helper()
	t.Helper()

	assert.NotEmpty(t, constell.Name, "Constellation name should not be empty")
	assert.NotEmpty(t, constell.Abbrev, "Constellation abbreviation should not be empty")
	assert.NotEmpty(t, constell.LatinName, "Constellation Latin name should not be empty")
}

// validateEphemerisResult performs comprehensive validation of an EphemerisResult.
func validateEphemerisResult(t *testing.T, result *EphemerisResult) {
	t.Helper()

	assert.NotNil(t, result, "EphemerisResult should not be nil")
	assert.NotEmpty(t, result.Bodies, "Result should contain celestial bodies")
	assert.True(t, result.Metadata.CalculationTimeMs >= 0, "Calculation time should be non-negative")
	assert.True(t, result.Metadata.BodiesCalculated > 0, "Should have calculated at least one body")

	// Validate each body in the result
	for _, body := range result.Bodies {
		assertValidCelestialBody(t, body)
	}

	// Optionally validate houses if present (for backward compatibility)
	if len(result.Houses) > 0 {
		validateHouses(t, result.Houses)
	}
}

// validateHouses performs validation of house data
func validateHouses(t *testing.T, houses []House) {
	t.Helper()

	// For complete house systems, require at least 12 houses
	if len(houses) > 0 && len(houses) < 12 {
		panic(fmt.Sprintf("Should have at least 12 houses for a complete system, got %d", len(houses)))
	}

	// Validate each house individually
	for _, house := range houses {
		validateHouse(t, house)
	}
}

// validateHouse performs validation of a single house
func validateHouse(t *testing.T, house House) {
	t.Helper()

	if house.ID < 1 || house.ID > 12 {
		panic(fmt.Sprintf("House ID should be between 1 and 12, got %d", house.ID))
	}
	if house.Longitude < 0 || house.Longitude >= 360 {
		panic(fmt.Sprintf("House longitude should be between 0 and 360 degrees, got %f", house.Longitude))
	}
	if house.Hsys == "" {
		panic("House system should not be empty")
	}
}

// createTestContext creates a background context for testing.
func createTestContext() context.Context {
	return context.Background()
}

// mockCalculationResult creates a mock calculation result for testing cache behavior.
func mockCalculationResult(bodies []CelestialBody) *EphemerisResult {
	return &EphemerisResult{
		Bodies: bodies,
		Metadata: CalculationMetadata{
			CalculationTimeMs: 10,
			BodiesCalculated:  len(bodies),
			Cached:            false,
		},
		Timestamp: time.Now(),
	}
}

// assertErrorContains checks that an error message contains expected text.
//
//nolint:unused
func assertErrorContains(t *testing.T, err error, expectedSubstring string) {
	t.Helper()
	t.Helper()

	assert.Error(t, err, "Expected an error")
	assert.Contains(t, err.Error(), expectedSubstring, "Error message should contain expected text")
}

// assertNoErrorWithContext provides detailed error reporting for test failures.
//
//nolint:unused
func assertNoErrorWithContext(t *testing.T, err error, context string) {
	t.Helper()
	t.Helper()

	if err != nil {
		t.Errorf("%s failed with error: %v", context, err)
	}
}

// validateCacheKeyFormat validates that cache keys follow expected patterns.
//
//nolint:unused
func validateCacheKeyFormat(t *testing.T, key string) {
	t.Helper()
	t.Helper()

	assert.NotEmpty(t, key, "Cache key should not be empty")
	assert.True(t, len(key) > 10, "Cache key should be reasonably long")
	// Could add more specific validation based on key generation patterns
}

// benchmarkHelper provides a standardized way to run benchmark calculations.
//
//nolint:unused
func benchmarkHelper(b *testing.B, name string, calculationFunc func() error) {
	b.Helper()
	b.Helper()

	b.Run(name, func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := calculationFunc(); err != nil {
				b.Fatalf("Benchmark calculation failed: %v", err)
			}
		}
	})
}

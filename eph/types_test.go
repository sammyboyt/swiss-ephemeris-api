package eph

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Pure Go tests for data structures (no CGO dependencies)

// TestCelestialBodyDataStructure tests the CelestialBody struct and its properties
func TestCelestialBodyDataStructure(t *testing.T) {
	t.Run("celestial_body_creation", func(t *testing.T) {
		body := CelestialBody{
			ID:             0,
			Name:           "Test Body",
			Type:           TypePlanet,
			Longitude:      123.45,
			Latitude:       5.67,
			Distance:       1.5,
			SpeedLongitude: 0.98,
			SpeedLatitude:  0.01,
			SpeedDistance:  0.002,
			Retrograde:     false,
			Magnitude:      nil,
			Category:       "test",
			Sequence:       1,
		}

		if body.ID != 0 {
			t.Errorf("Expected ID 0, got %d", body.ID)
		}
		if body.Name != "Test Body" {
			t.Errorf("Expected name 'Test Body', got %s", body.Name)
		}
		if body.Type != TypePlanet {
			t.Errorf("Expected type TypePlanet, got %s", body.Type)
		}
		if body.Longitude != 123.45 {
			t.Errorf("Expected longitude 123.45, got %f", body.Longitude)
		}
		if body.Retrograde != false {
			t.Error("Expected retrograde false")
		}
	})

	t.Run("celestial_body_with_magnitude", func(t *testing.T) {
		mag := 2.5
		body := CelestialBody{
			ID:        -1,
			Name:      "Test Star",
			Type:      TypeFixedStar,
			Magnitude: &mag,
			Category:  "fixed_star",
		}

		if body.ID != -1 {
			t.Errorf("Expected ID -1 for fixed star, got %d", body.ID)
		}
		if body.Type != TypeFixedStar {
			t.Errorf("Expected type TypeFixedStar, got %s", body.Type)
		}
		if body.Magnitude == nil || *body.Magnitude != 2.5 {
			t.Errorf("Expected magnitude 2.5, got %v", body.Magnitude)
		}
	})
}

// TestConstellationDataStructure tests the Constellation struct
func TestConstellationDataStructure(t *testing.T) {
	t.Run("constellation_creation", func(t *testing.T) {
		stars := []CelestialBody{
			{ID: -1, Name: "Star1", Type: TypeFixedStar},
			{ID: -2, Name: "Star2", Type: TypeFixedStar},
		}

		constell := Constellation{
			Name:      "Ursa Major",
			Abbrev:    "UMa",
			LatinName: "Ursa Major",
			Stars:     stars,
			StarCount: len(stars),
		}

		if constell.Name != "Ursa Major" {
			t.Errorf("Expected name 'Ursa Major', got %s", constell.Name)
		}
		if constell.Abbrev != "UMa" {
			t.Errorf("Expected abbrev 'UMa', got %s", constell.Abbrev)
		}
		if constell.StarCount != 2 {
			t.Errorf("Expected star count 2, got %d", constell.StarCount)
		}
		if len(constell.Stars) != 2 {
			t.Errorf("Expected 2 stars, got %d", len(constell.Stars))
		}
	})
}

// TestEphemerisConfig tests configuration structures
func TestEphemerisConfig(t *testing.T) {
	t.Run("config_creation", func(t *testing.T) {
		config := EphemerisConfig{
			IncludeTraditional: true,
			IncludeNodes:       true,
			IncludeAsteroids:   false,
			IncludeCentaurs:    true,
			UseSpeed:           true,
			MaxStarMagnitude:   3.0,
			Constellations:     []string{"Leo", "Virgo"},
		}

		if !config.IncludeTraditional {
			t.Error("Expected IncludeTraditional to be true")
		}
		if !config.IncludeNodes {
			t.Error("Expected IncludeNodes to be true")
		}
		if config.IncludeAsteroids {
			t.Error("Expected IncludeAsteroids to be false")
		}
		if !config.IncludeCentaurs {
			t.Error("Expected IncludeCentaurs to be true")
		}
		if !config.UseSpeed {
			t.Error("Expected UseSpeed to be true")
		}
		if config.MaxStarMagnitude != 3.0 {
			t.Errorf("Expected MaxStarMagnitude 3.0, got %f", config.MaxStarMagnitude)
		}
		if len(config.Constellations) != 2 {
			t.Errorf("Expected 2 constellations, got %d", len(config.Constellations))
		}
	})

	t.Run("preset_configs", func(t *testing.T) {
		traditional := GetTraditionalBodiesConfig()
		if !traditional.IncludeTraditional {
			t.Error("Traditional config should include traditional bodies")
		}
		if traditional.IncludeAsteroids {
			t.Error("Traditional config should not include asteroids")
		}

		extended := GetExtendedBodiesConfig()
		if extended.IncludeTraditional {
			t.Error("Extended config should not include traditional bodies by default")
		}
		if !extended.IncludeNodes || !extended.IncludeCentaurs || !extended.IncludeAsteroids {
			t.Error("Extended config should include all extended body types")
		}

		all := GetAllBodiesConfig()
		if !all.IncludeTraditional || !all.IncludeNodes || !all.IncludeCentaurs || !all.IncludeAsteroids {
			t.Error("All config should include all body types")
		}
	})
}

// TestBodyRegistry tests the body definition registry
func TestBodyRegistry(t *testing.T) {
	t.Run("get_available_bodies", func(t *testing.T) {
		bodies := GetAvailableBodies()

		if len(bodies) < 20 {
			t.Errorf("Expected at least 20 bodies, got %d", len(bodies))
		}

		// Check for specific required bodies
		found := make(map[int]bool)
		for _, body := range bodies {
			found[body.ID] = true
		}

		requiredIDs := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 15, 16, 17, 18, 19, 20}
		for _, id := range requiredIDs {
			if !found[id] {
				t.Errorf("Required body ID %d not found", id)
			}
		}
	})

	t.Run("get_body_by_id", func(t *testing.T) {
		body, found := GetBodyByID(0)
		if !found {
			t.Fatal("Body 0 (Sun) should be found")
		}
		if body.Name != "Sun" {
			t.Errorf("Expected Sun, got %s", body.Name)
		}
		if body.Type != TypePlanet {
			t.Errorf("Expected TypePlanet, got %s", body.Type)
		}

		_, found = GetBodyByID(999)
		if found {
			t.Error("Body 999 should not be found")
		}
	})

	t.Run("body_types", func(t *testing.T) {
		bodies := GetAvailableBodies()

		planetCount := 0
		nodeCount := 0
		asteroidCount := 0
		centaurCount := 0

		for _, body := range bodies {
			switch body.Type {
			case TypePlanet:
				planetCount++
			case TypeNode:
				nodeCount++
			case TypeAsteroid:
				asteroidCount++
			case TypeCentaur:
				centaurCount++
			}
		}

		if planetCount < 10 {
			t.Errorf("Expected at least 10 planets, got %d", planetCount)
		}
		if nodeCount < 4 {
			t.Errorf("Expected at least 4 nodes, got %d", nodeCount)
		}
		if asteroidCount < 4 {
			t.Errorf("Expected at least 4 asteroids, got %d", asteroidCount)
		}
		if centaurCount < 2 {
			t.Errorf("Expected at least 2 centaurs, got %d", centaurCount)
		}
	})
}

// TestConstellationRegistry tests constellation definitions
func TestConstellationRegistry(t *testing.T) {
	t.Run("get_available_constellations", func(t *testing.T) {
		constellations := GetAvailableConstellations()

		if len(constellations) < 20 {
			t.Errorf("Expected at least 20 constellations, got %d", len(constellations))
		}

		// Check for major constellations
		found := make(map[string]bool)
		for _, constell := range constellations {
			found[constell.Abbrev] = true
		}

		required := []string{"Leo", "Vir", "Sco", "Sgr", "UMa", "Ori"}
		for _, abbrev := range required {
			if !found[abbrev] {
				t.Errorf("Required constellation %s not found", abbrev)
			}
		}
	})

	t.Run("get_constellation_by_abbrev", func(t *testing.T) {
		constell, found := GetConstellationByAbbrev("Leo")
		if !found {
			t.Fatal("Leo constellation should be found")
		}
		if constell.Name != "Leo" {
			t.Errorf("Expected name 'Leo', got %s", constell.Name)
		}
		if constell.LatinName != "Leo" {
			t.Errorf("Expected Latin name 'Leo', got %s", constell.LatinName)
		}

		_, found = GetConstellationByAbbrev("XYZ")
		if found {
			t.Error("Non-existent constellation XYZ should not be found")
		}
	})
}

// TestUtilityFunctions tests utility functions
func TestUtilityFunctions(t *testing.T) {
	t.Run("unique_function", func(t *testing.T) {
		input := []int{1, 2, 2, 3, 1, 4, 3}
		result := unique(input)
		expected := []int{1, 2, 3, 4}

		if len(result) != len(expected) {
			t.Errorf("Expected length %d, got %d", len(expected), len(result))
		}

		for i, v := range expected {
			if result[i] != v {
				t.Errorf("Expected %d at position %d, got %d", v, i, result[i])
			}
		}
	})

	t.Run("constellation_abbrev_to_name", func(t *testing.T) {
		name := constellationAbbrevToName("Leo")
		if name != "Leo" {
			t.Errorf("Expected 'Leo', got %s", name)
		}

		name = constellationAbbrevToName("XYZ")
		if name != "Unknown" {
			t.Errorf("Expected 'Unknown' for invalid abbrev, got %s", name)
		}
	})
}

// TestConfigurationValidation tests configuration validation
func TestConfigurationValidation(t *testing.T) {
	t.Run("empty_config_defaults", func(t *testing.T) {
		config := EphemerisConfig{}
		// Empty config should not crash and should have zero values
		assert.False(t, config.UseSpeed, "UseSpeed should default to false")
		assert.Equal(t, int32(0), config.CalculationFlags, "CalculationFlags should default to 0")
	})

	t.Run("config_with_custom_bodies", func(t *testing.T) {
		config := EphemerisConfig{
			CustomBodies: []int{0, 5, 10, 17}, // Sun, Jupiter, Mean Node, Ceres
			UseSpeed:     true,
		}

		if len(config.CustomBodies) != 4 {
			t.Errorf("Expected 4 custom bodies, got %d", len(config.CustomBodies))
		}
		if !config.UseSpeed {
			t.Error("Expected UseSpeed to be true")
		}
	})
}

// Test context functions
func TestContextFunctions(t *testing.T) {
	t.Run("new_context_with_bodies", func(t *testing.T) {
		// Test NewContext with CelestialBody slice
		ctx := context.Background()
		testBodies := []CelestialBody{
			{ID: 0, Name: "Sun", Type: TypePlanet},
		}

		newCtx := NewContext(ctx, testBodies)
		assert.NotNil(t, newCtx)
		assert.NotEqual(t, ctx, newCtx) // Should be different context

		// Test FromContext
		retrieved, exists := FromContext(newCtx)
		assert.True(t, exists)
		assert.Equal(t, testBodies, retrieved)
	})

	t.Run("new_context_with_houses", func(t *testing.T) {
		// Test NewContext with House slice
		ctx := context.Background()
		testHouses := []House{
			{ID: 1, Longitude: 45.0, Hsys: "P"},
		}

		newCtx := NewContext(ctx, testHouses)
		assert.NotNil(t, newCtx)

		// Test FromContext
		retrieved, exists := FromContext(newCtx)
		assert.True(t, exists)
		assert.Equal(t, testHouses, retrieved)
	})

	t.Run("new_context_with_invalid_type", func(t *testing.T) {
		// Test NewContext with invalid type (should return original context)
		ctx := context.Background()
		testValue := "invalid type"

		newCtx := NewContext(ctx, testValue)
		assert.Equal(t, ctx, newCtx) // Should return original context

		// Test FromContext
		retrieved, exists := FromContext(newCtx)
		assert.False(t, exists)
		assert.Nil(t, retrieved)
	})

	t.Run("from_context_with_empty_context", func(t *testing.T) {
		// Test FromContext with context that doesn't have the value
		ctx := context.Background()
		retrieved, exists := FromContext(ctx)
		assert.False(t, exists)
		assert.Nil(t, retrieved) // Should return nil for missing value
	})
}

// Test GetBodyBySEConstant function
func TestGetBodyBySEConstant(t *testing.T) {
	t.Run("valid_se_constants", func(t *testing.T) {
		// Test some known SE constants
		testCases := []struct {
			seConstant int32
			expectName string
		}{
			{0, "Sun"},        // SE_SUN
			{1, "Moon"},       // SE_MOON
			{2, "Mercury"},    // SE_MERCURY
			{3, "Venus"},      // SE_VENUS
			{4, "Mars"},       // SE_MARS
			{10, "Mean Node"}, // SE_MEAN_NODE
		}

		for _, tc := range testCases {
			bodyDef, found := GetBodyBySEConstant(tc.seConstant)
			assert.True(t, found, "SE constant %d should be found", tc.seConstant)
			assert.Equal(t, tc.expectName, bodyDef.Name)

			// Convert BodyDefinition to CelestialBody for validation
			celestialBody := CelestialBody{
				ID:   bodyDef.ID,
				Name: bodyDef.Name,
				Type: bodyDef.Type,
			}
			assertValidCelestialBody(t, celestialBody)
		}
	})

	t.Run("invalid_se_constant", func(t *testing.T) {
		// Test invalid SE constant
		body, found := GetBodyBySEConstant(99999)
		assert.False(t, found, "Invalid SE constant should not be found")
		assert.Equal(t, BodyDefinition{}, body)
	})
}

// TestAstroTimeRequest tests time request structure
func TestAstroTimeRequest(t *testing.T) {
	t.Run("time_request_creation", func(t *testing.T) {
		timeReq := AstroTimeRequest{
			Year:      2024,
			Month:     1,
			Day:       15,
			UT:        14.5,
			Gregorian: true,
		}

		if timeReq.Year != 2024 {
			t.Errorf("Expected year 2024, got %d", timeReq.Year)
		}
		if timeReq.Month != 1 {
			t.Errorf("Expected month 1, got %d", timeReq.Month)
		}
		if timeReq.Day != 15 {
			t.Errorf("Expected day 15, got %d", timeReq.Day)
		}
		if timeReq.UT != 14.5 {
			t.Errorf("Expected UT 14.5, got %f", timeReq.UT)
		}
		if !timeReq.Gregorian {
			t.Error("Expected Gregorian to be true")
		}
	})
}

// TestEphemerisResult tests the result structure
func TestEphemerisResult(t *testing.T) {
	t.Run("result_creation", func(t *testing.T) {
		bodies := []CelestialBody{
			{ID: 0, Name: "Sun", Type: TypePlanet},
			{ID: 1, Name: "Moon", Type: TypePlanet},
		}

		result := EphemerisResult{
			Bodies: bodies,
			Metadata: CalculationMetadata{
				CalculationTimeMs: 150,
				BodiesCalculated:  2,
				Cached:            false,
			},
			Timestamp: time.Now(),
		}

		if len(result.Bodies) != 2 {
			t.Errorf("Expected 2 bodies, got %d", len(result.Bodies))
		}
		if result.Metadata.BodiesCalculated != 2 {
			t.Errorf("Expected 2 bodies calculated, got %d", result.Metadata.BodiesCalculated)
		}
		if result.Metadata.Cached {
			t.Error("Expected cached to be false")
		}
		if result.Metadata.CalculationTimeMs != 150 {
			t.Errorf("Expected calculation time 150ms, got %d", result.Metadata.CalculationTimeMs)
		}
	})
}

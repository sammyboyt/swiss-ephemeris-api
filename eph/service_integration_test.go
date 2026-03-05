package eph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockEphemerisService implements EphemerisService for testing
type MockEphemerisService struct {
	mock.Mock
}

func (m *MockEphemerisService) GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]CelestialBody, error) {
	args := m.Called(ctx, yr, mon, day, ut)
	return args.Get(0).([]CelestialBody), args.Error(1)
}

func (m *MockEphemerisService) GetHousesCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]House, error) {
	args := m.Called(ctx, yr, mon, day, ut, lat, lng)
	return args.Get(0).([]House), args.Error(1)
}

func (m *MockEphemerisService) GetChartCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]CelestialBody, []House, error) {
	args := m.Called(ctx, yr, mon, day, ut, lat, lng)
	return args.Get(0).([]CelestialBody), args.Get(1).([]House), args.Error(2)
}

func (m *MockEphemerisService) CalculateBodies(ctx context.Context, time AstroTimeRequest, config EphemerisConfig) (*EphemerisResult, error) {
	args := m.Called(ctx, time, config)
	return args.Get(0).(*EphemerisResult), args.Error(1)
}

func (m *MockEphemerisService) GetTraditionalBodies(ctx context.Context, time AstroTimeRequest) ([]CelestialBody, error) {
	args := m.Called(ctx, time)
	return args.Get(0).([]CelestialBody), args.Error(1)
}

func (m *MockEphemerisService) GetExtendedBodies(ctx context.Context, time AstroTimeRequest, types []CelestialBodyType) ([]CelestialBody, error) {
	args := m.Called(ctx, time, types)
	return args.Get(0).([]CelestialBody), args.Error(1)
}

func (m *MockEphemerisService) GetFixedStars(ctx context.Context, time AstroTimeRequest, constellations []string) ([]Constellation, error) {
	args := m.Called(ctx, time, constellations)
	return args.Get(0).([]Constellation), args.Error(1)
}

func (m *MockEphemerisService) GetFullChart(ctx context.Context, time AstroTimeRequest, lat, lng float64) (*EphemerisResult, error) {
	args := m.Called(ctx, time, lat, lng)
	return args.Get(0).(*EphemerisResult), args.Error(1)
}

// Service layer integration tests
// These tests verify the service interfaces work correctly with the new CelestialBody system

func TestEphemerisService_Interface(t *testing.T) {
	t.Run("interface_compliance", func(t *testing.T) {
		// RED: Services should implement EphemerisService interface
		mockCache := &mockCache{}
		logger, _ := zap.NewDevelopment()

		var cachedService EphemerisService = NewCachedEphemerisService(mockCache, logger)
		var directService EphemerisService = &DirectEphemerisService{Logger: logger}

		// GREEN: Both services should implement the interface
		assert.NotNil(t, cachedService)
		assert.NotNil(t, directService)

		// Interface compliance is verified by the successful assignment above
		// The actual method calls would require CGO and Swiss Ephemeris library
	})

	t.Run("service_initialization", func(t *testing.T) {
		// RED: Should create services without errors
		mockCache := &mockCache{}
		logger, _ := zap.NewDevelopment()

		cachedService := NewCachedEphemerisService(mockCache, logger)
		directService := &DirectEphemerisService{Logger: logger}

		// GREEN: Services should be created successfully
		assert.NotNil(t, cachedService)
		assert.NotNil(t, directService)
	})

	t.Run("configuration_presets", func(t *testing.T) {
		// RED: Configuration presets should work
		traditional := GetTraditionalBodiesConfig()
		extended := GetExtendedBodiesConfig()
		all := GetAllBodiesConfig()

		// GREEN: Presets should be valid
		assert.NotNil(t, traditional)
		assert.NotNil(t, extended)
		assert.NotNil(t, all)

		// Traditional should only include traditional bodies
		assert.True(t, traditional.IncludeTraditional)
		assert.False(t, traditional.IncludeAsteroids)
		assert.False(t, traditional.IncludeCentaurs)

		// Extended should include extended bodies
		assert.True(t, extended.IncludeNodes)
		assert.True(t, extended.IncludeAsteroids)
		assert.True(t, extended.IncludeCentaurs)

		// All should include everything
		assert.True(t, all.IncludeTraditional)
		assert.True(t, all.IncludeNodes)
		assert.True(t, all.IncludeAsteroids)
		assert.True(t, all.IncludeCentaurs)
	})

	t.Run("body_registry_integrity", func(t *testing.T) {
		// RED: Body registry should be complete
		bodies := GetAvailableBodies()

		// GREEN: Should have reasonable number of bodies
		assert.Greater(t, len(bodies), 15, "Should have at least 16 bodies")

		// Check for required traditional bodies
		foundSun := false
		foundMoon := false
		foundMars := false
		foundJupiter := false

		for _, body := range bodies {
			switch body.Name {
			case "Sun":
				foundSun = true
				assert.Equal(t, TypePlanet, body.Type)
			case "Moon":
				foundMoon = true
				assert.Equal(t, TypePlanet, body.Type)
			case "Mars":
				foundMars = true
				assert.Equal(t, TypePlanet, body.Type)
			case "Jupiter":
				foundJupiter = true
				assert.Equal(t, TypePlanet, body.Type)
			}
		}

		assert.True(t, foundSun, "Sun should be in registry")
		assert.True(t, foundMoon, "Moon should be in registry")
		assert.True(t, foundMars, "Mars should be in registry")
		assert.True(t, foundJupiter, "Jupiter should be in registry")
	})

	t.Run("constellation_registry_integrity", func(t *testing.T) {
		// RED: Constellation registry should be complete
		constellations := GetAvailableConstellations()

		// GREEN: Should have major constellations
		assert.Greater(t, len(constellations), 20, "Should have at least 21 constellations")

		// Check for major constellations
		foundLeo := false
		foundUrsaMajor := false
		foundOrion := false

		for _, constell := range constellations {
			switch constell.Abbrev {
			case "Leo":
				foundLeo = true
			case "UMa":
				foundUrsaMajor = true
			case "Ori":
				foundOrion = true
			}
		}

		assert.True(t, foundLeo, "Leo should be in registry")
		assert.True(t, foundUrsaMajor, "Ursa Major should be in registry")
		assert.True(t, foundOrion, "Orion should be in registry")
	})

	// Test DirectEphemerisService methods that are currently untested
	t.Run("direct_service_methods", func(t *testing.T) {
		logger := createTestLogger(t)
		directSvc := &DirectEphemerisService{Logger: logger}

		timeReq := AstroTimeRequest{
			Year:      2024,
			Month:     1,
			Day:       1,
			UT:        12.0,
			Gregorian: true,
		}

		// Test CalculateBodies
		config := GetTraditionalBodiesConfig()
		result, err := directSvc.CalculateBodies(createTestContext(), timeReq, config)
		assert.NoError(t, err)
		validateEphemerisResult(t, result)

		// Test CalculateBodiesCached (should delegate to CalculateBodies)
		result2, err := directSvc.CalculateBodiesCached(createTestContext(), timeReq, config)
		assert.NoError(t, err)
		validateEphemerisResult(t, result2)

		// Test GetTraditionalBodies
		bodies, err := directSvc.GetTraditionalBodies(createTestContext(), timeReq)
		assert.NoError(t, err)
		assert.NotEmpty(t, bodies)
		for _, body := range bodies {
			assertValidCelestialBody(t, body)
		}

		// Test GetExtendedBodies
		extendedBodies, err := directSvc.GetExtendedBodies(createTestContext(), timeReq, []CelestialBodyType{TypeNode})
		assert.NoError(t, err)
		for _, body := range extendedBodies {
			assertValidCelestialBody(t, body)
		}

		// Test GetFixedStars
		stars, err := directSvc.GetFixedStars(createTestContext(), timeReq, []string{"Leo"})
		assert.NoError(t, err)
		// Fixed stars might be empty if Swiss Ephemeris data unavailable
		for _, constellation := range stars {
			assert.NotEmpty(t, constellation.Name)
			for _, star := range constellation.Stars {
				assertValidCelestialBody(t, star)
			}
		}

		// Test GetFullChart
		fullResult, err := directSvc.GetFullChart(createTestContext(), timeReq, 40.7128, -74.0060)
		assert.NoError(t, err)
		validateEphemerisResult(t, fullResult)
	})
}

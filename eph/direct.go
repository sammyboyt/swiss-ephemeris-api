package eph

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// DirectEphemerisService provides direct ephemeris calculations without caching
// Used as a fallback when Redis is not available
type DirectEphemerisService struct {
	Logger *zap.Logger
}

// GetPlanetsCached implements the cached interface but without actual caching
func (s *DirectEphemerisService) GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]CelestialBody, error) {
	if s.Logger != nil {
		s.Logger.Debug("Direct ephemeris calculation (no cache)", zap.Int("year", yr), zap.Int("month", mon), zap.Int("day", day))
	}
	return GetPlanets(yr, mon, day, ut)
}

// GetHousesCached implements the cached interface but without actual caching
func (s *DirectEphemerisService) GetHousesCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]House, error) {
	if s.Logger != nil {
		s.Logger.Debug("Direct house calculation (no cache)", zap.Int("year", yr), zap.Int("month", mon), zap.Int("day", day))
	}
	return GetHouses(yr, mon, day, ut, lat, lng), nil
}

// GetChartCached implements the cached interface but without actual caching
func (s *DirectEphemerisService) GetChartCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]CelestialBody, []House, error) {
	if s.Logger != nil {
		s.Logger.Debug("Direct chart calculation (no cache)", zap.Int("year", yr), zap.Int("month", mon), zap.Int("day", day))
	}

	bodies, err := GetPlanets(yr, mon, day, ut)
	if err != nil {
		return nil, nil, err
	}

	houses := GetHouses(yr, mon, day, ut, lat, lng)
	return bodies, houses, nil
}

// CalculateBodies performs direct calculation without caching
func (s *DirectEphemerisService) CalculateBodies(ctx context.Context, astroTime AstroTimeRequest, config EphemerisConfig) (*EphemerisResult, error) {
	if s.Logger != nil {
		s.Logger.Debug("Direct bodies calculation (no cache)", zap.Any("time", astroTime))
	}

	bodies, err := CalculateCelestialBodies(astroTime.Year, astroTime.Month, astroTime.Day, astroTime.UT, config)
	if err != nil {
		return nil, err
	}

	return &EphemerisResult{
		Bodies: bodies,
		Metadata: CalculationMetadata{
			CalculationTimeMs: 0, // Not measured for direct service
			BodiesCalculated:  len(bodies),
			Cached:            false,
		},
		Timestamp: time.Now(),
	}, nil
}

// CalculateBodiesCached is an alias for CalculateBodies (no actual caching)
func (s *DirectEphemerisService) CalculateBodiesCached(ctx context.Context, astroTime AstroTimeRequest, config EphemerisConfig) (*EphemerisResult, error) {
	return s.CalculateBodies(ctx, astroTime, config)
}

// GetTraditionalBodies returns traditional celestial bodies
func (s *DirectEphemerisService) GetTraditionalBodies(ctx context.Context, astroTime AstroTimeRequest) ([]CelestialBody, error) {
	if s.Logger != nil {
		s.Logger.Debug("Direct traditional bodies calculation (no cache)", zap.Any("time", astroTime))
	}
	return CalculateCelestialBodies(astroTime.Year, astroTime.Month, astroTime.Day, astroTime.UT, GetTraditionalBodiesConfig())
}

// GetExtendedBodies returns extended celestial bodies
func (s *DirectEphemerisService) GetExtendedBodies(ctx context.Context, astroTime AstroTimeRequest, types []CelestialBodyType) ([]CelestialBody, error) {
	if s.Logger != nil {
		s.Logger.Debug("Direct extended bodies calculation (no cache)", zap.Any("time", astroTime), zap.Any("types", types))
	}

	config := GetExtendedBodiesConfig()
	bodies, err := CalculateCelestialBodies(astroTime.Year, astroTime.Month, astroTime.Day, astroTime.UT, config)
	if err != nil {
		return nil, err
	}

	// Filter by requested types
	var filtered []CelestialBody
	for _, body := range bodies {
		for _, reqType := range types {
			if body.Type == reqType {
				filtered = append(filtered, body)
				break
			}
		}
	}
	return filtered, nil
}

// GetFixedStars returns fixed stars grouped by constellations
func (s *DirectEphemerisService) GetFixedStars(ctx context.Context, astroTime AstroTimeRequest, constellations []string) ([]Constellation, error) {
	if s.Logger != nil {
		s.Logger.Debug("Direct fixed stars calculation (no cache)", zap.Any("time", astroTime), zap.Any("constellations", constellations))
	}
	return CalculateFixedStars(astroTime.Year, astroTime.Month, astroTime.Day, astroTime.UT, constellations, 3.0)
}

// GetFullChart returns complete chart data (bodies + houses)
func (s *DirectEphemerisService) GetFullChart(ctx context.Context, astroTime AstroTimeRequest, lat, lng float64) (*EphemerisResult, error) {
	if s.Logger != nil {
		s.Logger.Debug("Direct full chart calculation (no cache)", zap.Any("time", astroTime), zap.Float64("lat", lat), zap.Float64("lng", lng))
	}

	// Get all bodies
	bodies, err := CalculateCelestialBodies(astroTime.Year, astroTime.Month, astroTime.Day, astroTime.UT, GetAllBodiesConfig())
	if err != nil {
		return nil, err
	}

	// Add houses
	houses := GetHouses(astroTime.Year, astroTime.Month, astroTime.Day, astroTime.UT, lat, lng)

	// Convert houses to CelestialBody format
	var houseBodies []CelestialBody
	for _, house := range houses {
		houseBodies = append(houseBodies, CelestialBody{
			ID:        house.ID,
			Name:      fmt.Sprintf("House %d", house.ID),
			Type:      "house",
			Longitude: house.Longitude,
			Category:  "house",
		})
	}

	result := &EphemerisResult{
		Bodies: append(bodies, houseBodies...),
		Metadata: CalculationMetadata{
			CalculationTimeMs: 0,
			BodiesCalculated:  len(bodies) + len(houseBodies),
			Cached:            false,
		},
		Timestamp: time.Now(),
	}

	return result, nil
}

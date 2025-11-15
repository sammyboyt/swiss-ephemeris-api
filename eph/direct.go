package eph

import (
	"context"

	"go.uber.org/zap"
)

// DirectEphemerisService provides direct ephemeris calculations without caching
// Used as a fallback when Redis is not available
type DirectEphemerisService struct {
	Logger *zap.Logger
}

// GetPlanetsCached implements the cached interface but without actual caching
func (s *DirectEphemerisService) GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]Planet, error) {
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
func (s *DirectEphemerisService) GetChartCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]Planet, []House, error) {
	if s.Logger != nil {
		s.Logger.Debug("Direct chart calculation (no cache)", zap.Int("year", yr), zap.Int("month", mon), zap.Int("day", day))
	}

	planets, err := GetPlanets(yr, mon, day, ut)
	if err != nil {
		return nil, nil, err
	}

	houses := GetHouses(yr, mon, day, ut, lat, lng)
	return planets, houses, nil
}

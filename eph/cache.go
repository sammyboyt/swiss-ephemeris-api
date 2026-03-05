package eph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"astral-backend/pkg/cache"

	"go.uber.org/zap"
)

type CachedEphemerisService struct {
	cache  cache.Cache
	logger *zap.Logger
	ttl    time.Duration
}

func NewCachedEphemerisService(cache cache.Cache, logger *zap.Logger) *CachedEphemerisService {
	return &CachedEphemerisService{
		cache:  cache,
		logger: logger,
		ttl:    24 * time.Hour, // Ephemeris data is stable
	}
}

func (s *CachedEphemerisService) GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]CelestialBody, error) {
	params := map[string]interface{}{
		"year": yr, "month": mon, "day": day, "ut": ut,
	}
	cacheKey := cache.GenerateEphemerisKey("planets", params)

	// Try cache first
	if cached, found := s.cache.Get(ctx, cacheKey); found {
		s.logger.Debug("Cache hit for planets", zap.String("key", cacheKey))
		if bodies, ok := cached.([]CelestialBody); ok {
			return bodies, nil
		}
		// Cache corruption - remove invalid entry
		_ = s.cache.Delete(ctx, cacheKey)
	}

	// Cache miss - calculate
	s.logger.Debug("Cache miss for planets, calculating", zap.String("key", cacheKey))
	bodies, err := GetPlanets(yr, mon, day, ut)
	if err != nil {
		return nil, err
	}

	// Cache result
	if err := s.cache.Set(ctx, cacheKey, bodies, s.ttl); err != nil {
		s.logger.Warn("Failed to cache planets result", zap.Error(err))
	}

	return bodies, nil
}

func (s *CachedEphemerisService) GetHousesCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]House, error) {
	params := map[string]interface{}{
		"year": yr, "month": mon, "day": day, "ut": ut, "lat": lat, "lng": lng,
	}
	cacheKey := cache.GenerateEphemerisKey("houses", params)

	// Try cache first
	if cached, found := s.cache.Get(ctx, cacheKey); found {
		s.logger.Debug("Cache hit for houses", zap.String("key", cacheKey))
		if houses, ok := cached.([]House); ok {
			return houses, nil
		}
		// Cache corruption - remove invalid entry
		_ = s.cache.Delete(ctx, cacheKey)
	}

	// Cache miss - calculate
	s.logger.Debug("Cache miss for houses, calculating", zap.String("key", cacheKey))
	houses := GetHouses(yr, mon, day, ut, lat, lng)

	// Cache result
	if err := s.cache.Set(ctx, cacheKey, houses, s.ttl); err != nil {
		s.logger.Warn("Failed to cache houses result", zap.Error(err))
	}

	return houses, nil
}

func (s *CachedEphemerisService) GetChartCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]CelestialBody, []House, error) {
	params := map[string]interface{}{
		"year": yr, "month": mon, "day": day, "ut": ut, "lat": lat, "lng": lng,
	}
	cacheKey := cache.GenerateEphemerisKey("chart", params)

	// Try cache first
	if cached, found := s.cache.Get(ctx, cacheKey); found {
		s.logger.Debug("Cache hit for chart", zap.String("key", cacheKey))
		if chart, ok := cached.(map[string]interface{}); ok {
			if bodies, ok := chart["bodies"].([]CelestialBody); ok {
				if houses, ok := chart["houses"].([]House); ok {
					return bodies, houses, nil
				}
			}
		}
		// Cache corruption - remove invalid entry
		_ = s.cache.Delete(ctx, cacheKey)
	}

	// Cache miss - calculate
	s.logger.Debug("Cache miss for chart, calculating", zap.String("key", cacheKey))
	bodies, err := GetPlanets(yr, mon, day, ut)
	if err != nil {
		return nil, nil, err
	}

	houses := GetHouses(yr, mon, day, ut, lat, lng)

	chart := map[string]interface{}{
		"bodies": bodies,
		"houses": houses,
	}

	// Cache result
	if err := s.cache.Set(ctx, cacheKey, chart, s.ttl); err != nil {
		s.logger.Warn("Failed to cache chart result", zap.Error(err))
	}

	return bodies, houses, nil
}

func (s *CachedEphemerisService) InvalidateCache(ctx context.Context) error {
	s.logger.Info("Invalidating ephemeris cache")
	return s.cache.Clear(ctx)
}

// CalculateBodies calculates celestial bodies with caching
func (s *CachedEphemerisService) CalculateBodies(ctx context.Context, astroTime AstroTimeRequest, config EphemerisConfig) (*EphemerisResult, error) {
	start := time.Now()

	// Generate cache key based on all parameters
	params := map[string]interface{}{
		"year":        astroTime.Year,
		"month":       astroTime.Month,
		"day":         astroTime.Day,
		"ut":          astroTime.UT,
		"gregorian":   astroTime.Gregorian,
		"traditional": config.IncludeTraditional,
		"nodes":       config.IncludeNodes,
		"asteroids":   config.IncludeAsteroids,
		"centaurs":    config.IncludeCentaurs,
		"speed":       config.UseSpeed,
		"flags":       config.CalculationFlags,
	}
	cacheKey := cache.GenerateEphemerisKey("bodies", params)

	// Try cache first
	if cached, found := s.cache.Get(ctx, cacheKey); found {
		s.logger.Debug("Cache hit for bodies", zap.String("key", cacheKey))
		if result, ok := cached.(*EphemerisResult); ok {
			result.Metadata.Cached = true
			result.Metadata.CacheKey = cacheKey
			return result, nil
		}
		s.cache.Delete(ctx, cacheKey) // Remove corrupted cache
	}

	// Cache miss - calculate
	s.logger.Debug("Cache miss for bodies, calculating", zap.String("key", cacheKey))
	bodies, err := CalculateCelestialBodies(astroTime.Year, astroTime.Month, astroTime.Day, astroTime.UT, config)
	if err != nil {
		return nil, err
	}

	result := &EphemerisResult{
		Bodies: bodies,
		Metadata: CalculationMetadata{
			CalculationTimeMs: int(time.Since(start).Milliseconds()),
			BodiesCalculated:  len(bodies),
			Cached:            false,
			CacheKey:          cacheKey,
		},
		Timestamp: time.Now(),
	}

	// Cache result
	if err := s.cache.Set(ctx, cacheKey, result, s.ttl); err != nil {
		s.logger.Warn("Failed to cache bodies result", zap.Error(err))
	}

	return result, nil
}

// CalculateBodiesCached is an alias for CalculateBodies (both are cached)
func (s *CachedEphemerisService) CalculateBodiesCached(ctx context.Context, astroTime AstroTimeRequest, config EphemerisConfig) (*EphemerisResult, error) {
	return s.CalculateBodies(ctx, astroTime, config)
}

// GetTraditionalBodies returns traditional celestial bodies (planets)
func (s *CachedEphemerisService) GetTraditionalBodies(ctx context.Context, astroTime AstroTimeRequest) ([]CelestialBody, error) {
	result, err := s.CalculateBodies(ctx, astroTime, GetTraditionalBodiesConfig())
	if err != nil {
		return nil, err
	}
	return result.Bodies, nil
}

// GetExtendedBodies returns extended bodies (nodes, asteroids, centaurs)
func (s *CachedEphemerisService) GetExtendedBodies(ctx context.Context, astroTime AstroTimeRequest, types []CelestialBodyType) ([]CelestialBody, error) {
	config := GetExtendedBodiesConfig()
	// Filter by requested types
	result, err := s.CalculateBodies(ctx, astroTime, config)
	if err != nil {
		return nil, err
	}

	// Filter bodies by requested types
	var filtered []CelestialBody
	for _, body := range result.Bodies {
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
func (s *CachedEphemerisService) GetFixedStars(ctx context.Context, astroTime AstroTimeRequest, constellations []string) ([]Constellation, error) {
	start := time.Now()

	// Generate cache key
	params := map[string]interface{}{
		"year":           astroTime.Year,
		"month":          astroTime.Month,
		"day":            astroTime.Day,
		"ut":             astroTime.UT,
		"constellations": strings.Join(constellations, ","),
	}
	cacheKey := cache.GenerateEphemerisKey("fixed_stars", params)

	// Try cache first
	if cached, found := s.cache.Get(ctx, cacheKey); found {
		s.logger.Debug("Cache hit for fixed stars", zap.String("key", cacheKey))
		if constellations, ok := cached.([]Constellation); ok {
			return constellations, nil
		}
		s.cache.Delete(ctx, cacheKey)
	}

	// Cache miss - calculate
	s.logger.Debug("Cache miss for fixed stars, calculating", zap.String("key", cacheKey))
	constells, err := CalculateFixedStars(astroTime.Year, astroTime.Month, astroTime.Day, astroTime.UT, constellations, 3.0) // Magnitude limit
	if err != nil {
		return nil, err
	}

	// Cache result
	if err := s.cache.Set(ctx, cacheKey, constells, s.ttl); err != nil {
		s.logger.Warn("Failed to cache fixed stars result", zap.Error(err))
	}

	s.logger.Info("Fixed stars calculated",
		zap.Int("constellations", len(constells)),
		zap.Duration("duration", time.Since(start)))

	return constells, nil
}

// GetFullChart returns complete chart data (bodies + houses)
func (s *CachedEphemerisService) GetFullChart(ctx context.Context, astroTime AstroTimeRequest, lat, lng float64) (*EphemerisResult, error) {
	// Get all bodies
	result, err := s.CalculateBodies(ctx, astroTime, GetAllBodiesConfig())
	if err != nil {
		return nil, err
	}

	// Add houses separately (not cached for now - houses depend on location)
	houses, err := s.GetHousesCached(ctx, astroTime.Year, astroTime.Month, astroTime.Day, astroTime.UT, lat, lng)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate houses: %w", err)
	}

	// Set houses in separate field instead of mixing with bodies
	result.Houses = houses
	result.Metadata.BodiesCalculated = len(result.Bodies)

	return result, nil
}

func (s *CachedEphemerisService) GetCacheStats(ctx context.Context) (map[string]interface{}, error) {
	// This would require Redis-specific methods to get stats
	// For now, return basic info
	return map[string]interface{}{
		"cache_type": "redis",
		"ttl_hours":  s.ttl.Hours(),
	}, nil
}

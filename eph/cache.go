package eph

import (
	"context"
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

func (s *CachedEphemerisService) GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]Planet, error) {
	params := map[string]interface{}{
		"year": yr, "month": mon, "day": day, "ut": ut,
	}
	cacheKey := cache.GenerateEphemerisKey("planets", params)

	// Try cache first
	if cached, found := s.cache.Get(ctx, cacheKey); found {
		s.logger.Debug("Cache hit for planets", zap.String("key", cacheKey))
		if planets, ok := cached.([]Planet); ok {
			return planets, nil
		}
		// Cache corruption - remove invalid entry
		s.cache.Delete(ctx, cacheKey)
	}

	// Cache miss - calculate
	s.logger.Debug("Cache miss for planets, calculating", zap.String("key", cacheKey))
	planets, err := GetPlanets(yr, mon, day, ut)
	if err != nil {
		return nil, err
	}

	// Cache result
	if err := s.cache.Set(ctx, cacheKey, planets, s.ttl); err != nil {
		s.logger.Warn("Failed to cache planets result", zap.Error(err))
	}

	return planets, nil
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
		s.cache.Delete(ctx, cacheKey)
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

func (s *CachedEphemerisService) GetChartCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]Planet, []House, error) {
	params := map[string]interface{}{
		"year": yr, "month": mon, "day": day, "ut": ut, "lat": lat, "lng": lng,
	}
	cacheKey := cache.GenerateEphemerisKey("chart", params)

	// Try cache first
	if cached, found := s.cache.Get(ctx, cacheKey); found {
		s.logger.Debug("Cache hit for chart", zap.String("key", cacheKey))
		if chart, ok := cached.(map[string]interface{}); ok {
			if planets, ok := chart["planets"].([]Planet); ok {
				if houses, ok := chart["houses"].([]House); ok {
					return planets, houses, nil
				}
			}
		}
		// Cache corruption - remove invalid entry
		s.cache.Delete(ctx, cacheKey)
	}

	// Cache miss - calculate
	s.logger.Debug("Cache miss for chart, calculating", zap.String("key", cacheKey))
	planets, err := GetPlanets(yr, mon, day, ut)
	if err != nil {
		return nil, nil, err
	}

	houses := GetHouses(yr, mon, day, ut, lat, lng)

	chart := map[string]interface{}{
		"planets": planets,
		"houses":  houses,
	}

	// Cache result
	if err := s.cache.Set(ctx, cacheKey, chart, s.ttl); err != nil {
		s.logger.Warn("Failed to cache chart result", zap.Error(err))
	}

	return planets, houses, nil
}

func (s *CachedEphemerisService) InvalidateCache(ctx context.Context) error {
	s.logger.Info("Invalidating ephemeris cache")
	return s.cache.Clear(ctx)
}

func (s *CachedEphemerisService) GetCacheStats(ctx context.Context) (map[string]interface{}, error) {
	// This would require Redis-specific methods to get stats
	// For now, return basic info
	return map[string]interface{}{
		"cache_type": "redis",
		"ttl_hours":  s.ttl.Hours(),
	}, nil
}

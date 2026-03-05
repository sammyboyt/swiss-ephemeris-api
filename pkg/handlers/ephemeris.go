package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"astral-backend/eph"
	"astral-backend/pkg/errors"
	"astral-backend/pkg/logger"
	"astral-backend/pkg/middleware"

	"go.uber.org/zap"
)

// Use the EphemerisService interface from the eph package

// EphemerisHandler handles ephemeris-related HTTP requests
type EphemerisHandler struct {
	service eph.EphemerisService
	logger  *logger.Logger
}

// NewEphemerisHandler creates a new ephemeris handler
func NewEphemerisHandler(service eph.EphemerisService, logger *logger.Logger) *EphemerisHandler {
	return &EphemerisHandler{
		service: service,
		logger:  logger,
	}
}

// PlanetRequest represents the request parameters for planet calculations
type PlanetRequest struct {
	Year  int     `json:"year" validate:"required,min=1900,max=2100"`
	Month int     `json:"month" validate:"required,min=1,max=12"`
	Day   int     `json:"day" validate:"required,min=1,max=31"`
	UT    float64 `json:"ut" validate:"required,min=0,max=24"`
}

// ChartRequest represents the request parameters for complete chart calculations
type ChartRequest struct {
	PlanetRequest
	Lat float64 `json:"lat" validate:"required,min=-90,max=90"`
	Lng float64 `json:"lng" validate:"required,min=-180,max=180"`
}

// CelestialBodyResponse represents the response for celestial body data
type CelestialBodyResponse struct {
	Bodies []eph.CelestialBody `json:"bodies"`
}

// ChartResponse represents the response for complete chart data
type ChartResponse struct {
	Bodies []eph.CelestialBody `json:"bodies"`
	Houses []eph.House         `json:"houses"`
}

// GetPlanets handles requests for planetary positions
func (h *EphemerisHandler) GetPlanets(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	ctxLogger := middleware.GetLoggerFromContext(r.Context())
	apiKey, _ := middleware.GetAPIKeyFromContext(r.Context())

	if ctxLogger != nil {
		h.logger = ctxLogger
	}

	h.logger.Info("Processing get planets request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("key_name", apiKey.Name),
	)

	// Parse query parameters
	req, err := h.parsePlanetRequest(r)
	if err != nil {
		h.logger.Warn("Failed to parse planet request",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewValidationError("query", err.Error()))
		return
	}

	// Calculate planets
	planets, err := h.service.GetPlanetsCached(r.Context(), req.Year, req.Month, req.Day, req.UT)
	if err != nil {
		h.logger.Error("Failed to calculate planets",
			zap.String("request_id", requestID),
			zap.Error(err),
			zap.Int("year", req.Year),
			zap.Int("month", req.Month),
			zap.Int("day", req.Day),
			zap.Float64("ut", req.UT),
		)
		h.respondWithError(w, errors.NewEphemerisError("Failed to calculate planetary positions"))
		return
	}

	response := CelestialBodyResponse{Bodies: planets}

	h.logger.Info("Bodies calculated successfully",
		zap.String("request_id", requestID),
		zap.Int("body_count", len(planets)),
		zap.String("key_name", apiKey.Name),
	)

	h.respondWithJSON(w, http.StatusOK, response)
}

// GetHouses handles requests for house positions
func (h *EphemerisHandler) GetHouses(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	ctxLogger := middleware.GetLoggerFromContext(r.Context())
	apiKey, _ := middleware.GetAPIKeyFromContext(r.Context())

	if ctxLogger != nil {
		h.logger = ctxLogger
	}

	h.logger.Info("Processing get houses request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("key_name", apiKey.Name),
	)

	// Parse query parameters
	req, err := h.parseChartRequest(r)
	if err != nil {
		h.logger.Warn("Failed to parse chart request",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewValidationError("query", err.Error()))
		return
	}

	// Calculate houses
	houses, err := h.service.GetHousesCached(r.Context(), req.Year, req.Month, req.Day, req.UT, req.Lat, req.Lng)
	if err != nil {
		h.logger.Error("Failed to calculate houses",
			zap.String("request_id", requestID),
			zap.Error(err),
			zap.Int("year", req.Year),
			zap.Int("month", req.Month),
			zap.Int("day", req.Day),
			zap.Float64("ut", req.UT),
			zap.Float64("lat", req.Lat),
			zap.Float64("lng", req.Lng),
		)
		h.respondWithError(w, errors.NewEphemerisError("Failed to calculate house positions"))
		return
	}

	h.logger.Info("Houses calculated successfully",
		zap.String("request_id", requestID),
		zap.Int("house_count", len(houses)),
		zap.Float64("lat", req.Lat),
		zap.Float64("lng", req.Lng),
		zap.String("key_name", apiKey.Name),
	)

	h.respondWithJSON(w, http.StatusOK, houses)
}

// GetChart handles requests for complete astrological charts
func (h *EphemerisHandler) GetChart(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	ctxLogger := middleware.GetLoggerFromContext(r.Context())
	apiKey, _ := middleware.GetAPIKeyFromContext(r.Context())

	if ctxLogger != nil {
		h.logger = ctxLogger
	}

	h.logger.Info("Processing get chart request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("key_name", apiKey.Name),
	)

	// Parse query parameters
	req, err := h.parseChartRequest(r)
	if err != nil {
		h.logger.Warn("Failed to parse chart request",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewValidationError("query", err.Error()))
		return
	}

	// Calculate planets and houses
	planets, houses, err := h.service.GetChartCached(r.Context(), req.Year, req.Month, req.Day, req.UT, req.Lat, req.Lng)
	if err != nil {
		h.logger.Error("Failed to calculate chart",
			zap.String("request_id", requestID),
			zap.Error(err),
			zap.Int("year", req.Year),
			zap.Int("month", req.Month),
			zap.Int("day", req.Day),
			zap.Float64("ut", req.UT),
			zap.Float64("lat", req.Lat),
			zap.Float64("lng", req.Lng),
		)
		h.respondWithError(w, errors.NewEphemerisError("Failed to calculate chart"))
		return
	}

	response := ChartResponse{
		Bodies: planets,
		Houses: houses,
	}

	h.logger.Info("Chart calculated successfully",
		zap.String("request_id", requestID),
		zap.Int("body_count", len(planets)),
		zap.Int("house_count", len(houses)),
		zap.Float64("lat", req.Lat),
		zap.Float64("lng", req.Lng),
		zap.String("key_name", apiKey.Name),
	)

	h.respondWithJSON(w, http.StatusOK, response)
}

// parsePlanetRequest parses query parameters for planet calculations
func (h *EphemerisHandler) parsePlanetRequest(r *http.Request) (*PlanetRequest, error) {
	q := r.URL.Query()

	year, err := h.parseIntParam(q.Get("year"), "year", 1900, 2100)
	if err != nil {
		return nil, err
	}

	month, err := h.parseIntParam(q.Get("month"), "month", 1, 12)
	if err != nil {
		return nil, err
	}

	day, err := h.parseIntParam(q.Get("day"), "day", 1, 31)
	if err != nil {
		return nil, err
	}

	ut, err := h.parseFloatParam(q.Get("ut"), "ut", 0, 24)
	if err != nil {
		return nil, err
	}

	if math.IsNaN(ut) {
		return nil, errors.NewValidationError("ut", "Invalid UT time: NaN")
	}

	return &PlanetRequest{
		Year:  year,
		Month: month,
		Day:   day,
		UT:    ut,
	}, nil
}

// parseChartRequest parses query parameters for chart calculations
func (h *EphemerisHandler) parseChartRequest(r *http.Request) (*ChartRequest, error) {
	planetReq, err := h.parsePlanetRequest(r)
	if err != nil {
		return nil, err
	}

	q := r.URL.Query()

	lat, err := h.parseFloatParam(q.Get("lat"), "lat", -90, 90)
	if err != nil {
		return nil, err
	}

	lng, err := h.parseFloatParam(q.Get("lng"), "lng", -180, 180)
	if err != nil {
		return nil, err
	}

	if math.IsNaN(lat) || math.IsNaN(lng) {
		return nil, errors.NewValidationError("coordinates", "Invalid coordinates: NaN")
	}

	return &ChartRequest{
		PlanetRequest: *planetReq,
		Lat:           lat,
		Lng:           lng,
	}, nil
}

// parseIntParam parses and validates an integer parameter
func (h *EphemerisHandler) parseIntParam(value, name string, min, max int) (int, error) {
	if value == "" {
		return 0, errors.NewValidationError(name, name+" parameter is required")
	}

	val, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.NewValidationError(name, name+" must be a valid integer")
	}

	if val < min || val > max {
		return 0, errors.NewValidationError(name, name+" must be between "+strconv.Itoa(min)+" and "+strconv.Itoa(max))
	}

	return val, nil
}

// parseFloatParam parses and validates a float parameter
func (h *EphemerisHandler) parseFloatParam(value, name string, min, max float64) (float64, error) {
	if value == "" {
		return 0, errors.NewValidationError(name, name+" parameter is required")
	}

	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, errors.NewValidationError(name, name+" must be a valid number")
	}

	if val < min || val > max {
		return 0, errors.NewValidationError(name, name+" must be between "+strconv.FormatFloat(min, 'f', -1, 64)+" and "+strconv.FormatFloat(max, 'f', -1, 64))
	}

	return val, nil
}

// respondWithJSON sends a JSON response
func (h *EphemerisHandler) respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

// respondWithError sends an error response
func (h *EphemerisHandler) respondWithError(w http.ResponseWriter, err error) {
	var apiErr *errors.APIError
	if errors.IsAPIError(err) {
		apiErr, _ = errors.GetAPIError(err)
	} else {
		apiErr = errors.NewInternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.StatusCode())

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    apiErr.Code(),
			"message": apiErr.Message(),
		},
	}

	if details := apiErr.Details(); details != nil {
		response["error"].(map[string]interface{})["details"] = details
	}

	json.NewEncoder(w).Encode(response)
}

// GetBodies handles requests for celestial bodies with flexible configuration
func (h *EphemerisHandler) GetBodies(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	logger := middleware.GetLoggerFromContext(r.Context())

	// Parse time parameters
	timeReq, err := h.parseTimeRequest(r)
	if err != nil {
		logger.Warn("Failed to parse time request",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewValidationError("query", err.Error()))
		return
	}

	// Parse configuration from query parameters
	config := h.parseBodiesConfig(r)

	// Calculate bodies
	result, err := h.service.CalculateBodies(r.Context(), timeReq, config)
	if err != nil {
		logger.Error("Failed to calculate bodies",
			zap.String("request_id", requestID),
			zap.Error(err),
			zap.Any("time", timeReq),
		)
		h.respondWithError(w, errors.NewEphemerisError("Failed to calculate celestial bodies"))
		return
	}

	logger.Info("Bodies calculated successfully",
		zap.String("request_id", requestID),
		zap.Int("bodies", len(result.Bodies)),
		zap.Bool("cached", result.Metadata.Cached),
	)

	h.respondWithJSON(w, http.StatusOK, result)
}

// GetTraditionalBodies handles requests for traditional celestial bodies (planets)
func (h *EphemerisHandler) GetTraditionalBodies(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	logger := middleware.GetLoggerFromContext(r.Context())

	timeReq, err := h.parseTimeRequest(r)
	if err != nil {
		logger.Warn("Failed to parse time request",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewValidationError("query", err.Error()))
		return
	}

	bodies, err := h.service.GetTraditionalBodies(r.Context(), timeReq)
	if err != nil {
		logger.Error("Failed to get traditional bodies",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewEphemerisError("Failed to calculate traditional bodies"))
		return
	}

	result := &eph.EphemerisResult{
		Bodies:    bodies,
		Metadata:  eph.CalculationMetadata{BodiesCalculated: len(bodies)},
		Timestamp: time.Now(),
	}

	logger.Info("Traditional bodies calculated successfully",
		zap.String("request_id", requestID),
		zap.Int("bodies", len(bodies)),
	)

	h.respondWithJSON(w, http.StatusOK, result)
}

// GetExtendedBodies handles requests for extended celestial bodies
func (h *EphemerisHandler) GetExtendedBodies(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	logger := middleware.GetLoggerFromContext(r.Context())

	timeReq, err := h.parseTimeRequest(r)
	if err != nil {
		logger.Warn("Failed to parse time request",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewValidationError("query", err.Error()))
		return
	}

	// Parse types parameter
	typesParam := r.URL.Query().Get("types")
	var types []eph.CelestialBodyType
	if typesParam != "" {
		typeStrings := strings.Split(typesParam, ",")
		for _, ts := range typeStrings {
			types = append(types, eph.CelestialBodyType(strings.TrimSpace(ts)))
		}
	} else {
		// Default to all extended types
		types = []eph.CelestialBodyType{eph.TypeNode, eph.TypeCentaur, eph.TypeAsteroid}
	}

	bodies, err := h.service.GetExtendedBodies(r.Context(), timeReq, types)
	if err != nil {
		logger.Error("Failed to get extended bodies",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewEphemerisError("Failed to calculate extended bodies"))
		return
	}

	result := &eph.EphemerisResult{
		Bodies:    bodies,
		Metadata:  eph.CalculationMetadata{BodiesCalculated: len(bodies)},
		Timestamp: time.Now(),
	}

	logger.Info("Extended bodies calculated successfully",
		zap.String("request_id", requestID),
		zap.Int("bodies", len(bodies)),
		zap.Any("types", types),
	)

	h.respondWithJSON(w, http.StatusOK, result)
}

// GetFixedStars handles requests for fixed stars grouped by constellations
func (h *EphemerisHandler) GetFixedStars(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	logger := middleware.GetLoggerFromContext(r.Context())

	timeReq, err := h.parseTimeRequest(r)
	if err != nil {
		logger.Warn("Failed to parse time request",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewValidationError("query", err.Error()))
		return
	}

	// Parse constellations parameter
	constellationsParam := r.URL.Query().Get("constellations")
	constellations := []string{} // Start with empty slice instead of nil
	if constellationsParam != "" {
		constellations = strings.Split(constellationsParam, ",")
		for i, c := range constellations {
			constellations[i] = strings.TrimSpace(c)
		}

		// Expand Zodiac constellation if requested
		constellations = eph.ExpandZodiacConstellations(constellations)
	}

	constells, err := h.service.GetFixedStars(r.Context(), timeReq, constellations)
	if err != nil {
		logger.Error("Failed to get fixed stars",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewEphemerisError("Failed to calculate fixed stars"))
		return
	}

	result := &eph.EphemerisResult{
		Constellations: constells,
		Metadata:       eph.CalculationMetadata{BodiesCalculated: len(constells)},
		Timestamp:      time.Now(),
	}

	logger.Info("Fixed stars calculated successfully",
		zap.String("request_id", requestID),
		zap.Int("constellations", len(constells)),
	)

	h.respondWithJSON(w, http.StatusOK, result)
}

// GetFullChart handles requests for complete chart data (bodies + houses)
func (h *EphemerisHandler) GetFullChart(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())
	logger := middleware.GetLoggerFromContext(r.Context())

	timeReq, err := h.parseTimeRequest(r)
	if err != nil {
		logger.Warn("Failed to parse time request",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewValidationError("query", err.Error()))
		return
	}

	// Parse location parameters
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		h.respondWithError(w, errors.NewValidationError("lat", "Invalid latitude"))
		return
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		h.respondWithError(w, errors.NewValidationError("lng", "Invalid longitude"))
		return
	}

	result, err := h.service.GetFullChart(r.Context(), timeReq, lat, lng)
	if err != nil {
		logger.Error("Failed to get full chart",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		h.respondWithError(w, errors.NewEphemerisError("Failed to calculate full chart"))
		return
	}

	logger.Info("Full chart calculated successfully",
		zap.String("request_id", requestID),
		zap.Int("bodies", len(result.Bodies)),
		zap.Float64("lat", lat),
		zap.Float64("lng", lng),
	)

	h.respondWithJSON(w, http.StatusOK, result)
}

// parseTimeRequest parses time parameters from the request
func (h *EphemerisHandler) parseTimeRequest(r *http.Request) (eph.AstroTimeRequest, error) {
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")
	dayStr := r.URL.Query().Get("day")
	utStr := r.URL.Query().Get("ut")

	if yearStr == "" || monthStr == "" || dayStr == "" {
		return eph.AstroTimeRequest{}, fmt.Errorf("year, month, and day are required")
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return eph.AstroTimeRequest{}, fmt.Errorf("invalid year: %w", err)
	}

	month, err := strconv.Atoi(monthStr)
	if err != nil {
		return eph.AstroTimeRequest{}, fmt.Errorf("invalid month: %w", err)
	}

	day, err := strconv.Atoi(dayStr)
	if err != nil {
		return eph.AstroTimeRequest{}, fmt.Errorf("invalid day: %w", err)
	}

	ut := 12.0 // Default to noon
	if utStr != "" {
		ut, err = strconv.ParseFloat(utStr, 64)
		if err != nil {
			return eph.AstroTimeRequest{}, fmt.Errorf("invalid UT: %w", err)
		}
	}

	return eph.AstroTimeRequest{
		Year:      year,
		Month:     month,
		Day:       day,
		UT:        ut,
		Gregorian: true, // Default to Gregorian
	}, nil
}

// parseBodiesConfig parses body configuration from query parameters
func (h *EphemerisHandler) parseBodiesConfig(r *http.Request) eph.EphemerisConfig {
	config := eph.EphemerisConfig{
		UseSpeed:         true,
		CalculationFlags: eph.SEFLG_SWIEPH | eph.SEFLG_SPEED,
	}

	// Parse boolean flags
	if r.URL.Query().Get("traditional") == "true" {
		config.IncludeTraditional = true
	}
	if r.URL.Query().Get("nodes") == "true" {
		config.IncludeNodes = true
	}
	if r.URL.Query().Get("asteroids") == "true" {
		config.IncludeAsteroids = true
	}
	if r.URL.Query().Get("centaurs") == "true" {
		config.IncludeCentaurs = true
	}

	// Parse max magnitude for stars
	if magStr := r.URL.Query().Get("max_magnitude"); magStr != "" {
		if mag, err := strconv.ParseFloat(magStr, 64); err == nil {
			config.MaxStarMagnitude = mag
		}
	}

	// Parse constellations
	if constStr := r.URL.Query().Get("constellations"); constStr != "" {
		config.Constellations = strings.Split(constStr, ",")
		for i, c := range config.Constellations {
			config.Constellations[i] = strings.TrimSpace(c)
		}
	}

	// If no specific config set, default to traditional bodies
	if !config.IncludeTraditional && !config.IncludeNodes && !config.IncludeAsteroids && !config.IncludeCentaurs {
		config.IncludeTraditional = true
	}

	return config
}

package handlers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"astral-backend/eph"
	"astral-backend/pkg/errors"
	"astral-backend/pkg/logger"
	"astral-backend/pkg/middleware"

	"go.uber.org/zap"
)

// EphemerisService defines the interface for ephemeris calculations
type EphemerisService interface {
	GetPlanetsCached(ctx context.Context, yr, mon, day int, ut float64) ([]eph.Planet, error)
	GetHousesCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]eph.House, error)
	GetChartCached(ctx context.Context, yr, mon, day int, ut, lat, lng float64) ([]eph.Planet, []eph.House, error)
}

// EphemerisHandler handles ephemeris-related HTTP requests
type EphemerisHandler struct {
	service EphemerisService
	logger  *logger.Logger
}

// NewEphemerisHandler creates a new ephemeris handler
func NewEphemerisHandler(service EphemerisService, logger *logger.Logger) *EphemerisHandler {
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

// PlanetResponse represents the response for planet data
type PlanetResponse struct {
	Planets []eph.Planet `json:"planets"`
}

// ChartResponse represents the response for complete chart data
type ChartResponse struct {
	Planets []eph.Planet `json:"planets"`
	Houses  []eph.House  `json:"houses"`
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

	response := PlanetResponse{Planets: planets}

	h.logger.Info("Planets calculated successfully",
		zap.String("request_id", requestID),
		zap.Int("planet_count", len(planets)),
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
		Planets: planets,
		Houses:  houses,
	}

	h.logger.Info("Chart calculated successfully",
		zap.String("request_id", requestID),
		zap.Int("planet_count", len(planets)),
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

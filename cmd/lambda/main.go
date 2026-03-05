package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"astral-backend/eph"
	"astral-backend/pkg/logger"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"go.uber.org/zap"
)

var (
	appLogger        *logger.Logger
	ephemerisService eph.EphemerisService
)

// init runs once per Lambda execution environment (not per invocation)
func init() {
	var err error

	// Initialize logger
	appLogger, err = logger.NewLogger(logger.LogConfig{
		Level:       getEnvOrDefault("LOG_LEVEL", "info"),
		ServiceName: "astral-lambda",
		Environment: getEnvOrDefault("ENVIRONMENT", "production"),
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	appLogger.Info("Initializing Lambda execution environment")

	// Initialize ephemeris service (stateless - no cache, no Redis)
	// Using DirectEphemerisService because Lambda is stateless
	ephemerisService = &eph.DirectEphemerisService{Logger: appLogger.Logger}

	appLogger.Info("Lambda environment initialized successfully")
}

// handler is the main Lambda entry point
func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Add request context to logger
	requestID := request.RequestContext.RequestID
	ctxLogger := appLogger.With(zap.String("request_id", requestID))

	ctxLogger.Info("Incoming request",
		zap.String("http_method", request.HTTPMethod),
		zap.String("path", request.Path),
		zap.String("resource", request.Resource),
	)

	// Route based on path and method
	switch {
	case request.Path == "/health" && request.HTTPMethod == http.MethodGet:
		return handleHealth(ctxLogger), nil

	case request.Path == "/api/v1/planets" && request.HTTPMethod == http.MethodGet:
		return handleGetPlanets(ctx, request, ctxLogger), nil

	case request.Path == "/api/v1/houses" && request.HTTPMethod == http.MethodGet:
		return handleGetHouses(ctx, request, ctxLogger), nil

	case request.Path == "/api/v1/chart" && request.HTTPMethod == http.MethodGet:
		return handleGetChart(ctx, request, ctxLogger), nil

	case request.Path == "/api/v1/bodies" && request.HTTPMethod == http.MethodGet:
		return handleGetBodies(ctx, request, ctxLogger), nil

	case request.Path == "/api/v1/traditional" && request.HTTPMethod == http.MethodGet:
		return handleGetTraditionalBodies(ctx, request, ctxLogger), nil

	case request.Path == "/api/v1/extended" && request.HTTPMethod == http.MethodGet:
		return handleGetExtendedBodies(ctx, request, ctxLogger), nil

	case request.Path == "/api/v1/fixed-stars" && request.HTTPMethod == http.MethodGet:
		return handleGetFixedStars(ctx, request, ctxLogger), nil

	case request.Path == "/api/v1/full-chart" && request.HTTPMethod == http.MethodGet:
		return handleGetFullChart(ctx, request, ctxLogger), nil

	default:
		ctxLogger.Warn("Route not found",
			zap.String("path", request.Path),
			zap.String("method", request.HTTPMethod),
		)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusNotFound,
			Body:       `{"error": "Not Found", "message": "The requested resource does not exist"}`,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}, nil
	}
}

// handleHealth returns a simple health check response
func handleHealth(logger *zap.Logger) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       `{"status": "healthy", "service": "astral-lambda"}`,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
	}
}

// PlanetRequest represents the request parameters for calculations
type PlanetRequest struct {
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Day   int     `json:"day"`
	UT    float64 `json:"ut"`
}

// parsePlanetRequest extracts query parameters from the request
func parsePlanetRequest(request events.APIGatewayProxyRequest) (PlanetRequest, error) {
	req := PlanetRequest{}

	// Parse required parameters with defaults
	year, err := strconv.Atoi(getQueryParam(request, "year", fmt.Sprintf("%d", time.Now().Year())))
	if err != nil {
		return req, fmt.Errorf("invalid year parameter")
	}
	req.Year = year

	month, err := strconv.Atoi(getQueryParam(request, "month", "1"))
	if err != nil || month < 1 || month > 12 {
		return req, fmt.Errorf("invalid month parameter")
	}
	req.Month = month

	day, err := strconv.Atoi(getQueryParam(request, "day", "1"))
	if err != nil || day < 1 || day > 31 {
		return req, fmt.Errorf("invalid day parameter")
	}
	req.Day = day

	ut, err := strconv.ParseFloat(getQueryParam(request, "ut", "12.0"), 64)
	if err != nil || ut < 0 || ut > 24 {
		return req, fmt.Errorf("invalid ut parameter")
	}
	req.UT = ut

	return req, nil
}

// ChartRequest extends PlanetRequest with location data
type ChartRequest struct {
	PlanetRequest
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// parseChartRequest extracts chart parameters including lat/lng
func parseChartRequest(request events.APIGatewayProxyRequest) (ChartRequest, error) {
	planetReq, err := parsePlanetRequest(request)
	if err != nil {
		return ChartRequest{}, err
	}

	req := ChartRequest{PlanetRequest: planetReq}

	// Parse location (required for chart calculations)
	lat, err := strconv.ParseFloat(getQueryParam(request, "lat", ""), 64)
	if err != nil || lat < -90 || lat > 90 {
		return req, fmt.Errorf("invalid or missing lat parameter (must be -90 to 90)")
	}
	req.Lat = lat

	lng, err := strconv.ParseFloat(getQueryParam(request, "lng", ""), 64)
	if err != nil || lng < -180 || lng > 180 {
		return req, fmt.Errorf("invalid or missing lng parameter (must be -180 to 180)")
	}
	req.Lng = lng

	return req, nil
}

// handleGetPlanets handles GET /api/v1/planets
func handleGetPlanets(ctx context.Context, request events.APIGatewayProxyRequest, logger *zap.Logger) events.APIGatewayProxyResponse {
	req, err := parsePlanetRequest(request)
	if err != nil {
		logger.Warn("Failed to parse request", zap.Error(err))
		return errorResponse(http.StatusBadRequest, "Invalid request: "+err.Error())
	}

	// Calculate planetary positions using direct service (no cache in Lambda)
	planets, err := ephemerisService.GetPlanetsCached(ctx, req.Year, req.Month, req.Day, req.UT)
	if err != nil {
		logger.Error("Failed to calculate planets",
			zap.Error(err),
			zap.Int("year", req.Year),
			zap.Int("month", req.Month),
			zap.Int("day", req.Day),
		)
		return errorResponse(http.StatusInternalServerError, "Failed to calculate planetary positions")
	}

	// Build response
	response := map[string]interface{}{
		"bodies": planets,
		"metadata": map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"count":     len(planets),
		},
	}

	body, _ := json.Marshal(response)
	return jsonResponse(http.StatusOK, string(body))
}

// handleGetHouses handles GET /api/v1/houses
func handleGetHouses(ctx context.Context, request events.APIGatewayProxyRequest, logger *zap.Logger) events.APIGatewayProxyResponse {
	req, err := parseChartRequest(request)
	if err != nil {
		logger.Warn("Failed to parse chart request", zap.Error(err))
		return errorResponse(http.StatusBadRequest, "Invalid request: "+err.Error())
	}

	// Calculate houses
	houses := eph.GetHouses(req.Year, req.Month, req.Day, req.UT, req.Lat, req.Lng)

	response := map[string]interface{}{
		"houses": houses,
		"location": map[string]float64{
			"latitude":  req.Lat,
			"longitude": req.Lng,
		},
		"metadata": map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"count":     len(houses),
		},
	}

	body, _ := json.Marshal(response)
	return jsonResponse(http.StatusOK, string(body))
}

// handleGetChart handles GET /api/v1/chart (legacy combined endpoint)
func handleGetChart(ctx context.Context, request events.APIGatewayProxyRequest, logger *zap.Logger) events.APIGatewayProxyResponse {
	req, err := parseChartRequest(request)
	if err != nil {
		logger.Warn("Failed to parse chart request", zap.Error(err))
		return errorResponse(http.StatusBadRequest, "Invalid request: "+err.Error())
	}

	// Calculate planets and houses synchronously
	planets, err := ephemerisService.GetPlanetsCached(ctx, req.Year, req.Month, req.Day, req.UT)
	if err != nil {
		logger.Error("Failed to calculate planets", zap.Error(err))
		return errorResponse(http.StatusInternalServerError, "Failed to calculate planetary positions")
	}

	houses := eph.GetHouses(req.Year, req.Month, req.Day, req.UT, req.Lat, req.Lng)

	response := map[string]interface{}{
		"bodies":   planets,
		"houses":   houses,
		"location": map[string]float64{"latitude": req.Lat, "longitude": req.Lng},
		"metadata": map[string]interface{}{
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"body_count":  len(planets),
			"house_count": len(houses),
		},
	}

	body, _ := json.Marshal(response)
	return jsonResponse(http.StatusOK, string(body))
}

// handleGetBodies handles GET /api/v1/bodies (modern endpoint)
func handleGetBodies(ctx context.Context, request events.APIGatewayProxyRequest, logger *zap.Logger) events.APIGatewayProxyResponse {
	req, err := parsePlanetRequest(request)
	if err != nil {
		return errorResponse(http.StatusBadRequest, "Invalid request: "+err.Error())
	}

	// Use extended config by default for full body list
	config := eph.GetExtendedBodiesConfig()

	// Allow override via query param
	if bodyType := request.QueryStringParameters["type"]; bodyType == "traditional" {
		config = eph.GetTraditionalBodiesConfig()
	}

	bodies, err := ephemerisService.CalculateBodiesCached(ctx, eph.AstroTimeRequest{
		Year: req.Year, Month: req.Month, Day: req.Day, UT: req.UT,
	}, config)

	if err != nil {
		logger.Error("Failed to calculate bodies", zap.Error(err))
		return errorResponse(http.StatusInternalServerError, "Failed to calculate celestial bodies")
	}

	response := map[string]interface{}{
		"bodies": bodies.Bodies,
		"metadata": map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"count":     len(bodies.Bodies),
			"cached":    bodies.Metadata.Cached,
		},
	}

	body, _ := json.Marshal(response)
	return jsonResponse(http.StatusOK, string(body))
}

// handleGetTraditionalBodies handles GET /api/v1/traditional
func handleGetTraditionalBodies(ctx context.Context, request events.APIGatewayProxyRequest, logger *zap.Logger) events.APIGatewayProxyResponse {
	req, err := parsePlanetRequest(request)
	if err != nil {
		return errorResponse(http.StatusBadRequest, "Invalid request: "+err.Error())
	}

	config := eph.GetTraditionalBodiesConfig()
	bodies, err := ephemerisService.CalculateBodiesCached(ctx, eph.AstroTimeRequest{
		Year: req.Year, Month: req.Month, Day: req.Day, UT: req.UT,
	}, config)

	if err != nil {
		logger.Error("Failed to calculate traditional bodies", zap.Error(err))
		return errorResponse(http.StatusInternalServerError, "Failed to calculate traditional bodies")
	}

	response := map[string]interface{}{
		"bodies": bodies.Bodies,
		"metadata": map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"count":     len(bodies.Bodies),
			"type":      "traditional",
		},
	}

	body, _ := json.Marshal(response)
	return jsonResponse(http.StatusOK, string(body))
}

// handleGetExtendedBodies handles GET /api/v1/extended
func handleGetExtendedBodies(ctx context.Context, request events.APIGatewayProxyRequest, logger *zap.Logger) events.APIGatewayProxyResponse {
	req, err := parsePlanetRequest(request)
	if err != nil {
		return errorResponse(http.StatusBadRequest, "Invalid request: "+err.Error())
	}

	config := eph.GetExtendedBodiesConfig()
	bodies, err := ephemerisService.CalculateBodiesCached(ctx, eph.AstroTimeRequest{
		Year: req.Year, Month: req.Month, Day: req.Day, UT: req.UT,
	}, config)

	if err != nil {
		logger.Error("Failed to calculate extended bodies", zap.Error(err))
		return errorResponse(http.StatusInternalServerError, "Failed to calculate extended bodies")
	}

	response := map[string]interface{}{
		"bodies": bodies.Bodies,
		"metadata": map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"count":     len(bodies.Bodies),
			"type":      "extended",
		},
	}

	body, _ := json.Marshal(response)
	return jsonResponse(http.StatusOK, string(body))
}

// handleGetFixedStars handles GET /api/v1/fixed-stars
func handleGetFixedStars(ctx context.Context, request events.APIGatewayProxyRequest, logger *zap.Logger) events.APIGatewayProxyResponse {
	req, err := parsePlanetRequest(request)
	if err != nil {
		return errorResponse(http.StatusBadRequest, "Invalid request: "+err.Error())
	}

	// Parse optional constellations parameter
	constellations := []string{}
	if constellParam := request.QueryStringParameters["constellations"]; constellParam != "" {
		constellations = parseStringSlice(constellParam)
	}

	stars, err := ephemerisService.GetFixedStars(ctx, eph.AstroTimeRequest{
		Year: req.Year, Month: req.Month, Day: req.Day, UT: req.UT,
	}, constellations)

	if err != nil {
		logger.Error("Failed to calculate fixed stars", zap.Error(err))
		return errorResponse(http.StatusInternalServerError, "Failed to calculate fixed stars")
	}

	response := map[string]interface{}{
		"constellations": stars,
		"metadata": map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"count":     len(stars),
		},
	}

	body, _ := json.Marshal(response)
	return jsonResponse(http.StatusOK, string(body))
}

// handleGetFullChart handles GET /api/v1/full-chart
func handleGetFullChart(ctx context.Context, request events.APIGatewayProxyRequest, logger *zap.Logger) events.APIGatewayProxyResponse {
	req, err := parseChartRequest(request)
	if err != nil {
		return errorResponse(http.StatusBadRequest, "Invalid request: "+err.Error())
	}

	result, err := ephemerisService.GetFullChart(ctx, eph.AstroTimeRequest{
		Year: req.Year, Month: req.Month, Day: req.Day, UT: req.UT,
	}, req.Lat, req.Lng)

	if err != nil {
		logger.Error("Failed to generate full chart", zap.Error(err))
		return errorResponse(http.StatusInternalServerError, "Failed to generate full chart")
	}

	response := map[string]interface{}{
		"bodies":    result.Bodies,
		"houses":    result.Houses,
		"location":  map[string]float64{"latitude": req.Lat, "longitude": req.Lng},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"metadata": map[string]interface{}{
			"calculation_time_ms": result.Metadata.CalculationTimeMs,
			"bodies_calculated":   result.Metadata.BodiesCalculated,
		},
	}

	body, _ := json.Marshal(response)
	return jsonResponse(http.StatusOK, string(body))
}

// Helper functions

func getQueryParam(request events.APIGatewayProxyRequest, key, defaultValue string) string {
	if val, ok := request.QueryStringParameters[key]; ok {
		return val
	}
	if val, ok := request.MultiValueQueryStringParameters[key]; ok && len(val) > 0 {
		return val[0]
	}
	return defaultValue
}

func parseStringSlice(s string) []string {
	if s == "" {
		return nil
	}
	result := []string{}
	for _, item := range splitAndTrim(s, ",") {
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func splitAndTrim(s, sep string) []string {
	parts := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if i < len(s)-len(sep)+1 && s[i:i+len(sep)] == sep {
			parts = append(parts, s[start:i])
			start = i + len(sep)
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func jsonResponse(statusCode int, body string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       body,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type, x-api-key",
		},
	}
}

func errorResponse(statusCode int, message string) events.APIGatewayProxyResponse {
	body, _ := json.Marshal(map[string]string{
		"error":   http.StatusText(statusCode),
		"message": message,
	})
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type, x-api-key",
		},
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	lambda.Start(handler)
}

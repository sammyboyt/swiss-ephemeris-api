# Astral Backend API Documentation

## Overview

The Astral Backend provides comprehensive astronomical calculations including planets, asteroids, centaurs, lunar nodes, and fixed stars grouped by constellations.

## Authentication

All endpoints require API key authentication via `X-API-Key` header.

## Time Parameters

All endpoints accept these query parameters:

-   `year` (required): Year (e.g., 2024)
-   `month` (required): Month 1-12
-   `day` (required): Day 1-31
-   `ut` (optional): Universal Time in hours (default: 12.0)

## Endpoints

### GET /api/v1/bodies

Calculate celestial bodies with flexible configuration.

**Query Parameters:**

-   `traditional=true`: Include traditional planets (Sun, Moon, Mercury-Jupiter-Saturn, Uranus-Neptune-Pluto)
-   `nodes=true`: Include lunar nodes and apogees (Mean/True Node, Mean/True Lilith)
-   `asteroids=true`: Include main asteroids (Ceres, Pallas, Juno, Vesta)
-   `centaurs=true`: Include centaurs (Chiron, Pholus)
-   `constellations=Leo,Virgo`: Include fixed stars from specified constellations
-   `constellations=Zodiac`: **NEW** Include fixed stars from all 12 traditional zodiac constellations
-   `max_magnitude=3.0`: Maximum star magnitude (default: 3.0)

**Sample Request:**

```
GET /api/v1/bodies?year=2024&month=1&day=1&traditional=true&asteroids=true
```

**Sample Response:**

```json
{
    "bodies": [
        {
            "id": 0,
            "name": "Sun",
            "type": "planet",
            "longitude": 280.45,
            "latitude": 0.0,
            "distance_au": 0.983,
            "speed_longitude": 0.986,
            "retrograde": false,
            "category": "traditional",
            "sequence": 1
        },
        {
            "id": 17,
            "name": "Ceres",
            "type": "asteroid",
            "longitude": 123.67,
            "latitude": 8.23,
            "distance_au": 2.987,
            "speed_longitude": 0.456,
            "retrograde": false,
            "category": "asteroid",
            "sequence": 17
        }
    ],
    "metadata": {
        "calculation_time_ms": 45,
        "bodies_calculated": 15,
        "cached": false
    },
    "timestamp": "2024-01-01T12:00:00Z"
}
```

### GET /api/v1/traditional

Get traditional celestial bodies only.

**Sample Request:**

```
GET /api/v1/traditional?year=2024&month=1&day=1
```

**Sample Response:**

```json
{
    "bodies": [
        {
            "id": 0,
            "name": "Sun",
            "type": "planet",
            "longitude": 280.45,
            "retrograde": false
        },
        {
            "id": 1,
            "name": "Moon",
            "type": "planet",
            "longitude": 123.67,
            "retrograde": false
        },
        {
            "id": 2,
            "name": "Mercury",
            "type": "planet",
            "longitude": 287.12,
            "retrograde": false
        }
        // ... 7 more planets
    ],
    "metadata": { "bodies_calculated": 10 },
    "timestamp": "2024-01-01T12:00:00Z"
}
```

### GET /api/v1/extended

Get extended celestial bodies (nodes, asteroids, centaurs).

**Query Parameters:**

-   `types=node,centaur,asteroid`: Filter by body types (comma-separated)

**Sample Request:**

```
GET /api/v1/extended?year=2024&month=1&day=1&types=centaur,asteroid
```

**Sample Response:**

```json
{
    "bodies": [
        {
            "id": 15,
            "name": "Chiron",
            "type": "centaur",
            "longitude": 12.34,
            "latitude": 5.67,
            "distance_au": 18.5,
            "category": "centaur"
        },
        {
            "id": 17,
            "name": "Ceres",
            "type": "asteroid",
            "longitude": 123.67,
            "latitude": 8.23,
            "distance_au": 2.987,
            "category": "asteroid"
        }
    ],
    "metadata": { "bodies_calculated": 6 },
    "timestamp": "2024-01-01T12:00:00Z"
}
```

### GET /api/v1/fixed-stars

Get fixed stars grouped by constellations.

**Query Parameters:**

-   `constellations=Leo,Virgo,UrsaMajor`: Specific constellations (comma-separated)

**Sample Request:**

```
GET /api/v1/fixed-stars?year=2024&month=1&day=1&constellations=Leo
```

**Sample Response:**

```json
{
    "constellations": [
        {
            "name": "Leo",
            "abbrev": "Leo",
            "latin_name": "Leo",
            "stars": [
                {
                    "id": -1,
                    "name": "Regulus",
                    "type": "fixed_star",
                    "constellation": "Leo",
                    "longitude": 150.23,
                    "latitude": 0.12,
                    "magnitude": 1.35,
                    "speed_longitude": -0.001,
                    "retrograde": true,
                    "category": "fixed_star"
                }
            ],
            "star_count": 15
        }
    ],
    "metadata": { "bodies_calculated": 15 },
    "timestamp": "2024-01-01T12:00:00Z"
}
```

### GET /api/v1/full-chart

Get complete chart data (bodies + houses).

**Query Parameters:**

-   `lat`: Latitude (required)
-   `lng`: Longitude (required)

**Sample Request:**

```
GET /api/v1/full-chart?year=2024&month=1&day=1&lat=40.7128&lng=-74.0060
```

**Sample Response:**

```json
{
    "bodies": [
        // All celestial bodies + houses
        { "id": 0, "name": "Sun", "type": "planet", "longitude": 280.45 },
        { "id": 1, "name": "Moon", "type": "planet", "longitude": 123.67 },
        { "id": 13, "name": "House 1", "type": "house", "longitude": 123.45 },
        { "id": 14, "name": "House 2", "type": "house", "longitude": 153.67 }
        // ... all bodies and 12 houses
    ],
    "metadata": {
        "bodies_calculated": 25,
        "cached": true,
        "cache_key": "bodies:2024:1:1:12.000000:..."
    },
    "timestamp": "2024-01-01T12:00:00Z"
}
```

## Data Structures

### CelestialBody

```json
{
    "id": 0,
    "name": "Sun",
    "type": "planet",
    "constellation": null,
    "longitude": 280.45,
    "latitude": 0.0,
    "distance_au": 0.983,
    "speed_longitude": 0.986,
    "speed_latitude": 0.0,
    "speed_distance": 0.0,
    "retrograde": false,
    "magnitude": null,
    "category": "traditional",
    "sequence": 1
}
```

### Constellation

```json
{
  "name": "Leo",
  "abbrev": "Leo",
  "latin_name": "Leo",
  "stars": [...],
  "star_count": 15
}
```

### CalculationMetadata

```json
{
    "calculation_time_ms": 45,
    "bodies_calculated": 15,
    "cached": false,
    "cache_key": "bodies:2024:1:1:12.000000:..."
}
```

## Error Responses

```json
{
    "error": {
        "type": "validation_error",
        "message": "Invalid year: must be between -2000 and 3000",
        "details": { "field": "year", "value": "invalid" }
    }
}
```

## Available Body Types

-   `planet`: Sun, Moon, traditional planets
-   `node`: Lunar nodes and apogees
-   `asteroid`: Main asteroids (Ceres, Pallas, Juno, Vesta)
-   `centaur`: Centaurs (Chiron, Pholus)
-   `fixed_star`: Fixed stars by constellation

## Zodiac Constellation Support

### Special Zodiac Constellation

Use `constellations=Zodiac` to retrieve fixed stars from all 12 traditional zodiac constellations simultaneously.

**Zodiac Constellations Included:**

-   Aries (Ari), Taurus (Tau), Gemini (Gem), Cancer (Cnc)
-   Leo (Leo), Virgo (Vir), Libra (Lib), Scorpio (Sco)
-   Sagittarius (Sgr), Capricornus (Cap), Aquarius (Aqr), Pisces (Psc)

**Sample Request:**

```
GET /api/v1/bodies?year=2024&month=1&day=1&constellations=Zodiac&max_magnitude=3.0
```

**Combined Usage:**

```
GET /api/v1/bodies?year=2024&month=1&day=1&constellations=Zodiac,UMa,Ori&max_magnitude=4.0
```

## Available Constellations

Leo, Virgo, Scorpio, Sagittarius, Capricornus, Aquarius, Pisces, Aries, Taurus, Gemini, Cancer, Libra, Lyra, Cygnus, Pegasus, Andromeda, Cassiopeia, Perseus, Auriga, Canis Major, Canis Minor, Draco, Hercules, Bootes, Corona Borealis, Ursa Major, Ursa Minor, Orion

## Rate Limits

-   100 requests per minute per API key
-   Burst limit: 10 requests per second

## Caching

-   Results cached for 24 hours
-   Cache keys based on all parameters
-   Cache hit/miss indicated in metadata

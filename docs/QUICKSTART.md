# Astral Backend - Quick Start Guide

Get the Astral Backend API service up and running in 5 minutes.

## 🚀 Prerequisites

- **Docker & Docker Compose** (latest versions)
- **Git** for cloning the repository
- **curl** for testing the API

## 📦 Installation

```bash
# Clone the repository
git clone <repository-url>
cd astral-backend

# Verify Docker is running
docker --version
docker-compose --version
```

## 🔨 Build & Run

### One-Command Setup

```bash
# Build and start all services
cd docker/astral-backend
docker-compose up --build -d

# Wait for services to be healthy (30-60 seconds)
sleep 30
```

### Verify Everything Works

```bash
# Run the comprehensive health check
cd ../..
./health-check.sh
```

You should see:

```
🎉 Astral Backend - All Health Checks Passed!
==============================================
✅ MongoDB: Connected and healthy
✅ Redis: Connected and caching
✅ Application: Running and responsive
✅ Authentication: Working correctly
✅ API Endpoints: Planets, Houses, Chart all functional
✅ Request Tracing: Request IDs generated and logged
✅ Error Handling: Proper error responses
✅ Concurrent Load: Worker pool handling multiple requests

🚀 Astral Backend is production-ready!
```

## 🧪 Test the API

### Get Your First API Key

```bash
# Create a test API key (temporary endpoint for development)
API_KEY=$(curl -s -X POST http://localhost:8080/api/v1/keys | grep -o '"key":"[^"]*' | cut -d'"' -f4)

echo "Your API Key: $API_KEY"
```

### Test Planetary Positions

```bash
# Get planetary positions for January 15, 2024 at noon UTC
curl -s -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/planets?year=2024&month=1&day=15&ut=12.0" | jq '.'
```

**Expected Response:**

```json
{
  "planets": [
    {
      "id": 0,
      "name": "Sun",
      "longitude": 294.81869079051665,
      "retrograde": false
    },
    {
      "id": 1,
      "name": "Moon",
      "longitude": 349.8920994848993,
      "retrograde": false
    }
    // ... 10 more planets
  ]
}
```

### Test House Cusps

```bash
# Get house cusps for New York City coordinates
curl -s -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/houses?year=2024&month=1&day=15&ut=12.0&lat=40.7128&lng=-74.0060" | jq '.'
```

### Test Complete Chart

```bash
# Get both planets and houses in one request
curl -s -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/chart?year=2024&month=1&day=15&ut=12.0&lat=40.7128&lng=-74.0060" | jq '.'
```

### Test Error Handling

```bash
# Try with invalid date (should return 400)
curl -s -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/planets?year=invalid&month=1&day=15&ut=12.0" | jq '.'

# Try without authentication (should return 401)
curl -s "http://localhost:8080/api/v1/planets?year=2024&month=1&day=15&ut=12.0" | jq '.'
```

## 🔍 Understanding the Results

### Planetary Data

- **id**: Planet index (0=Sun, 1=Moon, 2=Mercury, etc.)
- **name**: Planet name
- **longitude**: Position in degrees (0-360°)
- **retrograde**: Boolean indicating retrograde motion

### House Data

- **id**: House number (1-12)
- **longitude**: Cusp position in degrees
- **hsys**: House system used ("P" = Placidus)

### Coordinate Systems

- **Longitude**: East/West position (-180° to +180°)
- **Latitude**: North/South position (-90° to +90°)
- **Universal Time (UT)**: Time in decimal hours (0.0 to 23.999...)

## 📊 Performance Testing

### Caching Demonstration

```bash
# First request (cache miss - slower)
time curl -s -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/planets?year=2024&month=1&day=15&ut=12.0" > /dev/null

# Second request (cache hit - faster)
time curl -s -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/planets?year=2024&month=1&day=15&ut=12.0" > /dev/null
```

### Concurrent Load Testing

```bash
# Test concurrent requests
for i in {1..5}; do
  curl -s -H "Authorization: Bearer $API_KEY" \
    "http://localhost:8080/api/v1/planets?year=2024&month=1&day=$i&ut=12.0" > /dev/null &
done
wait
echo "All concurrent requests completed successfully"
```

## 🛑 Troubleshooting

### Services Not Starting

```bash
# Check service status
cd docker/astral-backend
docker-compose ps

# Check logs
docker-compose logs app
docker-compose logs mongodb
docker-compose logs redis
```

### API Returns Errors

```bash
# Test basic connectivity
curl -s http://localhost:8080/health

# Check if services are healthy
docker-compose exec mongodb mongo --eval "db.stats()"
docker-compose exec redis redis-cli ping
```

### Port Conflicts

```bash
# Check what's using port 8080
lsof -i :8080

# Change ports in docker-compose.yml if needed
# app.ports: - "8081:8080"
```

## 🧹 Cleanup

```bash
# Stop all services
cd docker/astral-backend
docker-compose down

# Remove all data (including databases)
docker-compose down -v

# Remove images
docker-compose down --rmi all
```

## 🎯 Next Steps

### Development

```bash
# Run unit tests
make test-unit

# Run integration tests
make test-integration

# View test coverage
make coverage-html

# Check code quality
make quality
```

### Production Deployment

```bash
# Build production image
make build-prod

# Deploy with production config
docker-compose -f docker-compose.prod.yml up -d
```

### API Integration

```javascript
// JavaScript example
const API_KEY = "your-api-key-here";

async function getPlanets(year, month, day, ut) {
  const response = await fetch(
    `http://localhost:8080/api/v1/planets?year=${year}&month=${month}&day=${day}&ut=${ut}`,
    {
      headers: {
        Authorization: `Bearer ${API_KEY}`,
        "Content-Type": "application/json",
      },
    }
  );

  const data = await response.json();
  return data.planets;
}
```

```python
# Python example
import requests

API_KEY = 'your-api-key-here'

def get_planets(year, month, day, ut):
    response = requests.get(
        f'http://localhost:8080/api/v1/planets',
        params={'year': year, 'month': month, 'day': day, 'ut': ut},
        headers={'Authorization': f'Bearer {API_KEY}'}
    )
    return response.json()['planets']
```

## 📚 Documentation

- **[README.md](../README.md)** - Complete documentation
- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System design details
- **[API.md](API.md)** - Detailed API reference
- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Production deployment guide

## 🆘 Getting Help

1. **Check the health**: `./health-check.sh`
2. **Review logs**: `docker-compose logs`
3. **Test manually**: Use curl commands above
4. **Check documentation**: See README.md and docs/

**🎉 You're now ready to build astrological applications with the Astral Backend API!**

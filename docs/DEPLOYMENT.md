# Astral Backend - Production Deployment Guide

Complete production deployment guide for the Astral Backend API service.

## 📋 Pre-Deployment Checklist

- [ ] **Infrastructure Ready**
  - [ ] Docker runtime available
  - [ ] MongoDB instance provisioned
  - [ ] Redis instance provisioned
  - [ ] Load balancer configured (optional)

- [ ] **Environment Configuration**
  - [ ] Environment variables set
  - [ ] Database credentials secured
  - [ ] API keys generated (if needed)
  - [ ] SSL certificates configured

- [ ] **Monitoring Setup**
  - [ ] Log aggregation configured
  - [ ] Health check endpoints monitored
  - [ ] Performance metrics collected
  - [ ] Alert thresholds defined

- [ ] **Security Configuration**
  - [ ] HTTPS enabled
  - [ ] API key rotation policy defined
  - [ ] Rate limiting configured
  - [ ] Security headers set

## 🚀 Deployment Options

### Option 1: Docker Compose (Single Server)

```bash
# Production docker-compose.yml
version: '3.8'

services:
  astral-backend:
    image: astral-backend:latest
    environment:
      - MONGODB_URI=mongodb://user:pass@host:27017/astral?authSource=admin&ssl=true
      - REDIS_URL=redis://host:6379?password=redispass
      - LOG_LEVEL=info
      - ENVIRONMENT=production
      - PORT=8080
    ports:
      - "8080:8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    depends_on:
      - mongodb
      - redis

  mongodb:
    image: mongo:4.4
    environment:
      - MONGO_INITDB_ROOT_USERNAME=astral
      - MONGO_INITDB_ROOT_PASSWORD=secure-password
    volumes:
      - mongodb_data:/data/db
      - ./mongo-init:/docker-entrypoint-initdb.d
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass secure-redis-password
    volumes:
      - redis_data:/data
    restart: unless-stopped

volumes:
  mongodb_data:
  redis_data:
```

### Option 2: Kubernetes Deployment

```yaml
# k8s/deployment.yml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: astral-backend
spec:
  replicas: 3
  selector:
    matchLabels:
      app: astral-backend
  template:
    metadata:
      labels:
        app: astral-backend
    spec:
      containers:
      - name: astral-backend
        image: astral-backend:v1.0.0
        ports:
        - containerPort: 8080
        env:
        - name: MONGODB_URI
          valueFrom:
            secretKeyRef:
              name: astral-secrets
              key: mongodb-uri
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: astral-secrets
              key: redis-url
        - name: LOG_LEVEL
          value: "info"
        - name: ENVIRONMENT
          value: "production"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"

---
apiVersion: v1
kind: Service
metadata:
  name: astral-backend-service
spec:
  selector:
    app: astral-backend
  ports:
    - port: 80
      targetPort: 8080
  type: LoadBalancer
```

### Option 3: AWS ECS/Fargate

```hcl
# Terraform example (simplified)
resource "aws_ecs_task_definition" "astral_backend" {
  family                   = "astral-backend"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"

  container_definitions = jsonencode([{
    name  = "astral-backend"
    image = "astral-backend:latest"

    environment = [
      {
        name  = "MONGODB_URI"
        value = "mongodb://..."
      },
      {
        name  = "REDIS_URL"
        value = "redis://..."
      }
    ]

    portMappings = [{
      containerPort = 8080
      hostPort      = 8080
    }]

    healthCheck = {
      command = ["CMD-SHELL", "wget --no-verbose --tries=1 --spider http://localhost:8080/health"]
      interval = 30
      timeout  = 5
      retries  = 3
    }
  }])
}
```

## ⚙️ Environment Configuration

### Required Environment Variables

```bash
# Database Configuration
MONGODB_URI=mongodb://username:password@host:27017/database?authSource=admin&ssl=true&replicaSet=rs0
REDIS_URL=redis://username:password@host:6379/0

# Application Configuration
LOG_LEVEL=info                    # debug, info, warn, error
ENVIRONMENT=production           # development, staging, production
PORT=8080                        # Server port

# Security (Optional)
CORS_ORIGINS=https://yourdomain.com,https://app.yourdomain.com
RATE_LIMIT_REQUESTS=1000         # Requests per window
RATE_LIMIT_WINDOW=3600           # Window in seconds

# Monitoring (Optional)
SENTRY_DSN=https://...           # Error tracking
DATADOG_API_KEY=...              # Metrics
```

### Database Setup

#### MongoDB Initialization

```javascript
// mongo-init/init.js
db = db.getSiblingDB('astral');

// Create application user
db.createUser({
  user: 'astral_app',
  pwd: 'secure-app-password',
  roles: [
    {
      role: 'readWrite',
      db: 'astral'
    }
  ]
});

// Create indexes
db.api_keys.createIndex({ "key_hash": 1 }, { unique: true });
db.api_keys.createIndex({ "is_active": 1, "expires_at": 1 });
db.api_keys.createIndex({ "created_at": 1 });
```

#### Redis Configuration

```redis.conf
# redis.conf
requirepass your-secure-redis-password
maxmemory 256mb
maxmemory-policy allkeys-lru

# Enable persistence
save 900 1
save 300 10
save 60 10000
```

## 🔒 Security Configuration

### SSL/TLS Setup

```nginx
# nginx.conf (example reverse proxy)
server {
    listen 443 ssl http2;
    server_name api.astral-backend.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains";
}
```

### API Key Management

```bash
# Generate production API keys
curl -X POST https://your-api.com/api/v1/keys \
  -H "Authorization: Bearer admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production App Key",
    "scopes": ["read:ephemeris"],
    "expires_at": "2025-01-01T00:00:00Z"
  }'
```

### Rate Limiting

```go
// Custom rate limiting middleware
func RateLimitMiddleware(requests int, window time.Duration) mux.MiddlewareFunc {
    limiter := tollbooth.NewLimiter(requests/window.Seconds(), nil)
    limiter.SetIPLookups([]string{"X-Real-IP", "X-Forwarded-For"})
    return tollbooth.LimitHandler(limiter, nil)
}
```

## 📊 Monitoring & Observability

### Health Checks

```bash
# Application health
curl https://your-api.com/health

# Database connectivity
curl https://your-api.com/debug/database

# Cache status
curl https://your-api.com/debug/cache
```

### Log Aggregation

```yaml
# fluent-bit config
[INPUT]
    Name tail
    Path /var/log/astral-backend/*.log
    Tag astral-backend

[OUTPUT]
    Name elasticsearch
    Host elasticsearch-host
    Port 9200
    Index astral-backend
```

### Metrics Collection

```go
// Prometheus metrics
var (
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "endpoint", "status"},
    )

    cacheHits = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_hits_total",
            Help: "Total number of cache hits",
        },
        []string{"cache_type"},
    )
)
```

### Alerting Rules

```yaml
# alert_rules.yml
groups:
  - name: astral-backend
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"

      - alert: DatabaseDown
        expr: up{job="astral-backend-db"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Database is down"
```

## 🔄 Backup & Recovery

### Database Backups

```bash
# MongoDB backup
mongodump --host mongodb-host --port 27017 \
          --username astral_backup --password secure-password \
          --db astral --out /backup/$(date +%Y%m%d_%H%M%S)

# Automated backup script
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
mongodump --uri="$MONGODB_URI" --out="/backups/astral_$DATE"
find /backups -name "astral_*" -mtime +7 -delete
```

### Redis Backups

```bash
# Redis backup
redis-cli -h redis-host -a secure-password --rdb /backup/redis_backup.rdb

# Automated Redis backup
#!/bin/bash
redis-cli -a $REDIS_PASSWORD BGSAVE
cp /var/lib/redis/dump.rdb /backup/redis_$(date +%Y%m%d_%H%M%S).rdb
```

### Disaster Recovery

```bash
# Restore MongoDB
mongorestore --host mongodb-host --port 27017 \
             --username astral_admin --password secure-password \
             --db astral /backup/astral_backup

# Restore Redis
redis-cli -h redis-host -a secure-password FLUSHALL
redis-cli -h redis-host -a secure-password < /backup/redis_backup.rdb
```

## 🚀 Scaling Strategies

### Horizontal Scaling

```yaml
# Kubernetes HPA
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: astral-backend-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: astral-backend
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### Database Scaling

```javascript
// MongoDB sharding setup
sh.enableSharding("astral")
sh.shardCollection("astral.api_keys", { "_id": 1 })
sh.shardCollection("astral.usage_logs", { "timestamp": 1 })
```

### Cache Scaling

```yaml
# Redis Cluster
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis-cluster
spec:
  serviceName: redis-cluster
  replicas: 6
  template:
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        command: ["redis-server", "/etc/redis/redis.conf"]
        volumeMounts:
        - name: config
          mountPath: /etc/redis
```

## 🧪 Post-Deployment Testing

### Load Testing

```bash
# Install hey (HTTP load testing)
go install github.com/rakyll/hey@latest

# Load test
hey -n 1000 -c 10 -m GET \
    -H "Authorization: Bearer $API_KEY" \
    "https://your-api.com/api/v1/planets?year=2024&month=1&day=15&ut=12.0"
```

### Integration Testing

```bash
# Run E2E tests against production
RUN_E2E=true MONGODB_URI=$PROD_MONGODB_URI go test ./pkg/e2e/...
```

### Performance Monitoring

```bash
# Monitor response times
curl -w "@curl-format.txt" -s -H "Authorization: Bearer $API_KEY" \
     "https://your-api.com/api/v1/planets?year=2024&month=1&day=15&ut=12.0"
```

## 📝 Maintenance Procedures

### Rolling Updates

```bash
# Zero-downtime deployment
kubectl set image deployment/astral-backend astral-backend=astral-backend:v1.1.0
kubectl rollout status deployment/astral-backend
```

### Log Rotation

```bash
# Logrotate configuration
/var/log/astral-backend/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 644 astral-backend astral-backend
    postrotate
        docker-compose exec astral-backend kill -USR1 1
    endscript
}
```

### Certificate Renewal

```bash
# Let's Encrypt renewal
certbot certonly --webroot -w /var/www/html -d your-api.com
docker-compose exec nginx nginx -s reload
```

## 🚨 Incident Response

### Service Degradation

1. **Check health endpoints**
   ```bash
   curl -f https://your-api.com/health || echo "Service unhealthy"
   ```

2. **Check logs**
   ```bash
   kubectl logs -l app=astral-backend --tail=100
   ```

3. **Check resource usage**
   ```bash
   kubectl top pods
   ```

4. **Scale up if needed**
   ```bash
   kubectl scale deployment astral-backend --replicas=5
   ```

### Data Recovery

1. **Stop the service**
   ```bash
   kubectl scale deployment astral-backend --replicas=0
   ```

2. **Restore from backup**
   ```bash
   mongorestore --uri="$MONGODB_URI" /backups/latest/
   ```

3. **Restart service**
   ```bash
   kubectl scale deployment astral-backend --replicas=3
   ```

## 📊 Success Metrics

### Performance Targets
- **Response Time**: P95 < 500ms
- **Error Rate**: < 0.1%
- **Uptime**: > 99.9%
- **Cache Hit Rate**: > 90%

### Business Metrics
- **API Calls**: Track daily/weekly usage
- **Unique Users**: Monitor API key usage
- **Popular Endpoints**: Identify optimization opportunities

---

## 🎯 Production Readiness Checklist

### Pre-Launch
- [ ] **Security**
  - [ ] HTTPS configured
  - [ ] API keys rotated
  - [ ] Security headers set
  - [ ] Rate limiting enabled

- [ ] **Reliability**
  - [ ] Health checks configured
  - [ ] Monitoring alerts set
  - [ ] Backup procedures tested
  - [ ] Rollback plan documented

- [ ] **Performance**
  - [ ] Load testing completed
  - [ ] Performance baselines established
  - [ ] Auto-scaling configured
  - [ ] CDN setup (if applicable)

- [ ] **Operations**
  - [ ] Runbooks documented
  - [ ] On-call rotation established
  - [ ] Incident response tested
  - [ ] Post-mortem process defined

### Go-Live
- [ ] **Gradual Rollout**
  - [ ] Blue-green deployment
  - [ ] Feature flags configured
  - [ ] Rollback procedures ready
  - [ ] Monitoring dashboards active

- [ ] **Validation**
  - [ ] Smoke tests pass
  - [ ] Integration tests pass
  - [ ] Performance tests pass
  - [ ] Security scans clean

**🚀 Your Astral Backend is now production-ready with enterprise-grade reliability, security, and scalability!**

# Astral Backend - AWS Lambda Deployment Guide

This Terraform configuration deploys the Astral Backend as a stateless AWS Lambda function with API Gateway (REST API v1) for API key management.

## Architecture Overview

```
┌─────────────────┐
│   Client        │
│   (Browser/CLI) │
└────────┬────────┘
         │ HTTPS + x-api-key header
         ▼
┌─────────────────────────┐
│   API Gateway (REST)    │
│   - API Key validation  │
│   - Usage plan          │
│   - CORS handling       │
└────────┬────────────────┘
         │ Invoke
         ▼
┌─────────────────────────┐
│   Lambda Function       │
│   - Container Image     │
│   - CGO/Swiss Ephemeris │
│   - Stateless           │
│   - No DB/Cache         │
└────────┬────────────────┘
         │ Read
         ▼
┌─────────────────────────┐
│   /var/task/ephemeris/  │
│   - sepl_18.se1 (476KB) │
└─────────────────────────┘
```

## Prerequisites

1. **AWS CLI** configured with credentials
2. **Terraform** >= 1.5.0
3. **Docker** (for building container image)
4. **Go** 1.24+ (for local testing)

## Deployment Steps

### Step 1: Build and Push Docker Image

```bash
# Login to ECR (run this first)
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin $(aws sts get-caller-identity --query Account --output text).dkr.ecr.us-east-1.amazonaws.com

# Build the image
DOCKER_BUILDKIT=1 docker build -t astral-backend:latest .

# Tag for ECR (replace ACCOUNT_ID with your AWS account ID)
docker tag astral-backend:latest $(aws sts get-caller-identity --query Account --output text).dkr.ecr.us-east-1.amazonaws.com/astral-backend:dev

# Push to ECR
docker push $(aws sts get-caller-identity --query Account --output text).dkr.ecr.us-east-1.amazonaws.com/astral-backend:dev
```

### Step 2: Deploy Infrastructure

```bash
cd terraform

# Initialize Terraform
terraform init

# Create tfvars file
cp terraform.tfvars.example terraform.tfvars

# Review changes
terraform plan

# Deploy
terraform apply
```

### Step 3: Test the API

```bash
# Get the API endpoint and key from Terraform outputs
export API_ENDPOINT=$(terraform output -raw api_gateway_endpoint)
export API_KEY=$(terraform output -raw api_key_value)

# Test health endpoint
curl -H "x-api-key: $API_KEY" \
  "$API_ENDPOINT/health"

# Test planets calculation
curl -H "x-api-key: $API_KEY" \
  "$API_ENDPOINT/api/v1/planets?year=2024&month=3&day=5&ut=12.0"

# Test full chart (requires lat/lng)
curl -H "x-api-key: $API_KEY" \
  "$API_ENDPOINT/api/v1/full-chart?year=2024&month=3&day=5&ut=12.0&lat=40.7128&lng=-74.0060"
```

## API Endpoints

| Endpoint | Method | Description | Query Parameters |
|----------|--------|-------------|------------------|
| `/health` | GET | Health check | None |
| `/api/v1/planets` | GET | Planetary positions | `year`, `month`, `day`, `ut` |
| `/api/v1/houses` | GET | House cusps | `year`, `month`, `day`, `ut`, `lat`, `lng` |
| `/api/v1/chart` | GET | Combined chart | `year`, `month`, `day`, `ut`, `lat`, `lng` |
| `/api/v1/bodies` | GET | Celestial bodies | `year`, `month`, `day`, `ut`, `type` |
| `/api/v1/traditional` | GET | Traditional planets | `year`, `month`, `day`, `ut` |
| `/api/v1/extended` | GET | Extended bodies | `year`, `month`, `day`, `ut` |
| `/api/v1/fixed-stars` | GET | Fixed stars | `year`, `month`, `day`, `ut`, `constellations` |
| `/api/v1/full-chart` | GET | Complete chart | `year`, `month`, `day`, `ut`, `lat`, `lng` |

## Authentication

All endpoints (except CORS preflight) require the `x-api-key` header:

```http
GET /api/v1/planets?year=2024&month=3&day=5 HTTP/1.1
Host: xxxxxx.execute-api.us-east-1.amazonaws.com
x-api-key: your-api-key-here
```

## Cost Optimization

### $0 Idle Cost Achieved

| Component | Idle Cost | Pay-per-Use |
|-----------|-----------|-------------|
| Lambda | $0.00 | $0.20/1M requests |
| API Gateway | $0.00 | $3.50/1M requests |
| ECR | ~$0.10/month (storage) | Push/pull per use |
| **Total Idle** | **$0.10/month** | Usage-based |

### Why REST API (v1) over HTTP API (v2)?

**HTTP API v2** is cheaper ($1.00/1M vs $3.50/1M) BUT lacks:
- Native API key usage plans
- Built-in rate limiting per key
- API key management console

For this use case, the **REST API v1**'s built-in API key management justifies the cost difference.

## Troubleshooting

### Lambda Cold Start Issues

If experiencing slow cold starts:
1. Check memory allocation (512MB recommended)
2. Verify ephemeris files are in `/var/task/ephemeris/`
3. Enable X-Ray tracing to identify bottlenecks

### CGO/Swiss Ephemeris Errors

```bash
# Test locally with Lambda Runtime Interface Emulator
docker run -p 9000:8080 astral-backend:latest

# Then test
curl -XPOST "http://localhost:9000/2015-03-31/functions/function/invocations" \
  -d '{"httpMethod": "GET", "path": "/health"}'
```

### API Key Not Working

1. Verify key is enabled: AWS Console → API Gateway → API Keys
2. Check key is associated with usage plan: Usage Plans → [plan] → API Keys
3. Ensure usage plan is linked to API stage: Usage Plans → [plan] → API Stages

## Cleanup

```bash
# Destroy all resources
terraform destroy

# Remove ECR images (optional)
aws ecr batch-delete-image \
  --repository-name astral-backend \
  --image-ids imageTag=dev
```

## Security Considerations

1. **API Keys** are stored in Terraform state - use S3 backend with encryption
2. **No database** means no data persistence risk
3. **API Gateway** provides DDoS protection (AWS Shield)
4. **Lambda** runs in isolated execution environment
5. **Least privilege** IAM role (only CloudWatch logs)

## Monitoring

### CloudWatch Metrics

```bash
# View Lambda invocations
aws cloudwatch get-metric-statistics \
  --namespace AWS/Lambda \
  --metric-name Invocations \
  --dimensions Name=FunctionName,Value=astral-backend-dev \
  --start-time 2024-03-01T00:00:00Z \
  --end-time 2024-03-05T00:00:00Z \
  --period 3600 \
  --statistics Sum
```

### X-Ray Tracing

Enable in Lambda console to trace:
- Cold start duration
- Swiss Ephemeris calculation time
- API Gateway latency

## Next Steps

1. **Custom Domain**: Add Route53 + ACM certificate
2. **Monitoring**: CloudWatch alarms for error rates
3. **CI/CD**: GitHub Actions for automated deployment
4. **Additional API Keys**: Create keys per client/user
5. **WAF**: Add Web Application Firewall for extra protection

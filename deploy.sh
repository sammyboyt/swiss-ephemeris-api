#!/bin/bash
# =============================================================================
# Astral Backend - Full Deployment Script
# Handles the chicken-and-egg problem with ECR + Terraform
# =============================================================================

set -e  # Exit on any error

# Configuration
AWS_REGION="${AWS_REGION:-eu-west-2}"
ENVIRONMENT="${ENVIRONMENT:-dev}"
IMAGE_NAME="astral-backend"
IMAGE_TAG="${ENVIRONMENT}"
MEMORY_SIZE="${MEMORY_SIZE:-1024}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    command -v aws >/dev/null 2>&1 || { log_error "AWS CLI is required but not installed. Aborting."; exit 1; }
    command -v docker >/dev/null 2>&1 || { log_error "Docker is required but not installed. Aborting."; exit 1; }
    command -v terraform >/dev/null 2>&1 || { log_error "Terraform is required but not installed. Aborting."; exit 1; }

    # Check AWS credentials
    if ! aws sts get-caller-identity >/dev/null 2>&1; then
        log_error "AWS credentials not configured. Run 'aws configure' first."
        exit 1
    fi

    ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
    log_success "AWS Account: $ACCOUNT_ID"
    log_success "AWS Region: $AWS_REGION"
}

# Step 1: Create ECR Repository (Terraform)
step1_create_ecr() {
    log_info "Step 1: Creating ECR Repository..."
    cd terraform

    # Initialize Terraform if needed
    if [ ! -d ".terraform" ]; then
        terraform init
    fi

    # Create only the ECR repository
    terraform apply -target=aws_ecr_repository.astral_backend -auto-approve

    ECR_URL="${ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${IMAGE_NAME}"
    log_success "ECR Repository created: $ECR_URL"

    cd ..
}

# Step 2: Build and Push Docker Image
step2_build_and_push() {
    log_info "Step 2: Building and Pushing Docker Image..."

    ECR_URL="${ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${IMAGE_NAME}"

    # Login to ECR
    log_info "Logging into ECR..."
    aws ecr get-login-password --region "$AWS_REGION" | \
        docker login --username AWS --password-stdin "$ECR_URL"

    # Build the image
    log_info "Building Docker image (this may take 2-3 minutes)..."
    DOCKER_BUILDKIT=1 docker build \
        -t "${IMAGE_NAME}:${IMAGE_TAG}" \
        --platform linux/amd64 \
        --build-arg BUILDKIT_INLINE_CACHE=1 \
        .

    # Tag for ECR
    docker tag "${IMAGE_NAME}:${IMAGE_TAG}" "${ECR_URL}:${IMAGE_TAG}"

    # Push to ECR
    log_info "Pushing to ECR..."
    docker push "${ECR_URL}:${IMAGE_TAG}"

    # Get the image digest for verification
    IMAGE_DIGEST=$(aws ecr describe-images \
        --repository-name "$IMAGE_NAME" \
        --image-ids imageTag="$IMAGE_TAG" \
        --query 'imageDetails[0].imageDigest' \
        --output text)

    log_success "Image pushed successfully!"
    log_info "Image Digest: $IMAGE_DIGEST"
}

# Step 3: Deploy Full Infrastructure (Terraform)
step3_deploy_infrastructure() {
    log_info "Step 3: Deploying Infrastructure..."
    cd terraform

    # Create tfvars if it doesn't exist
    if [ ! -f "terraform.tfvars" ]; then
        log_info "Creating terraform.tfvars from example..."
        cat > terraform.tfvars <<EOF
aws_region           = "${AWS_REGION}"
environment          = "${ENVIRONMENT}"
lambda_memory_size   = ${MEMORY_SIZE}
lambda_timeout       = 15
lambda_storage       = 512
api_key_name         = "astral-${ENVIRONMENT}-key"
usage_plan_limit     = 1000
usage_plan_rate_limit = 10
log_retention_days   = 7
EOF
    fi

    # Full Terraform apply
    log_info "Running Terraform apply (this creates Lambda, API Gateway, etc.)..."
    terraform apply -auto-approve

    # Get outputs
    API_ENDPOINT=$(terraform output -raw api_gateway_endpoint)
    API_KEY=$(terraform output -raw api_key_value)
    LAMBDA_VERSION=$(terraform output -raw lambda_version)

    log_success "Infrastructure deployed!"
    log_info "Lambda Version: $LAMBDA_VERSION"
    log_info "API Endpoint: $API_ENDPOINT"

    cd ..
}

# Step 4: Wait for propagation and test
step4_test_deployment() {
    log_info "Step 4: Testing Deployment..."

    cd terraform
    API_ENDPOINT=$(terraform output -raw api_gateway_endpoint)
    API_KEY=$(terraform output -raw api_key_value)
    cd ..

    # Wait for API Gateway propagation
    log_info "Waiting 30 seconds for API Gateway propagation..."
    sleep 30

    log_info "Testing health endpoint..."

    # Test with curl
    HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "x-api-key: $API_KEY" \
        "${API_ENDPOINT}/health" 2>/dev/null || echo "000")

    if [ "$HTTP_STATUS" == "200" ]; then
        log_success "Health check PASSED (HTTP 200)"

        # Get the response
        HEALTH_RESPONSE=$(curl -s \
            -H "x-api-key: $API_KEY" \
            "${API_ENDPOINT}/health")
        log_info "Response: $HEALTH_RESPONSE"

        return 0
    else
        log_error "Health check FAILED (HTTP $HTTP_STATUS)"

        # Try to get more info
        log_warning "Attempting to get error response..."
        curl -v -H "x-api-key: $API_KEY" "${API_ENDPOINT}/health" 2>&1 || true

        return 1
    fi
}

# Step 5: Test actual chart calculation
step5_test_calculation() {
    log_info "Step 5: Testing Chart Calculation..."

    cd terraform
    API_ENDPOINT=$(terraform output -raw api_gateway_endpoint)
    API_KEY=$(terraform output -raw api_key_value)
    cd ..

    # Test planetary positions
    log_info "Testing /api/v1/planets..."
    PLANETS_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}\n" \
        -H "x-api-key: $API_KEY" \
        "${API_ENDPOINT}/api/v1/planets?year=2024&month=3&day=5&ut=12.0" \
        2>/dev/null || echo "HTTP_CODE:000")

    HTTP_CODE=$(echo "$PLANETS_RESPONSE" | grep "HTTP_CODE:" | cut -d: -f2)
    BODY=$(echo "$PLANETS_RESPONSE" | grep -v "HTTP_CODE:")

    if [ "$HTTP_CODE" == "200" ]; then
        log_success "Planets calculation PASSED (HTTP 200)"

        # Count bodies
        BODY_COUNT=$(echo "$BODY" | grep -o '"count":[0-9]*' | cut -d: -f2)
        log_info "Calculated $BODY_COUNT celestial bodies"

        # Test full chart
        log_info "Testing /api/v1/full-chart..."
        CHART_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}\n" \
            -H "x-api-key: $API_KEY" \
            "${API_ENDPOINT}/api/v1/full-chart?year=2024&month=3&day=5&ut=12.0&lat=40.7128&lng=-74.0060" \
            2>/dev/null || echo "HTTP_CODE:000")

        CHART_HTTP=$(echo "$CHART_RESPONSE" | grep "HTTP_CODE:" | cut -d: -f2)

        if [ "$CHART_HTTP" == "200" ]; then
            log_success "Full chart calculation PASSED (HTTP 200)"
            log_success "🎉 DEPLOYMENT SUCCESSFUL! 🎉"
            return 0
        else
            log_error "Full chart calculation FAILED (HTTP $CHART_HTTP)"
            return 1
        fi
    else
        log_error "Planets calculation FAILED (HTTP $HTTP_CODE)"
        log_error "Response: $BODY"
        return 1
    fi
}

# Show final summary
show_summary() {
    cd terraform
    API_ENDPOINT=$(terraform output -raw api_gateway_endpoint)
    API_KEY=$(terraform output -raw api_key_value)
    LAMBDA_VERSION=$(terraform output -raw lambda_version)
    ECR_URL=$(terraform output -raw ecr_repository_url)
    cd ..

    echo ""
    echo "============================================================================="
    echo "                       DEPLOYMENT SUMMARY"
    echo "============================================================================="
    echo ""
    echo -e "${GREEN}✓${NC} Lambda Function: astral-backend-${ENVIRONMENT} (v${LAMBDA_VERSION})"
    echo -e "${GREEN}✓${NC} API Gateway: ${API_ENDPOINT}"
    echo -e "${GREEN}✓${NC} ECR Repository: ${ECR_URL}"
    echo -e "${GREEN}✓${NC} Memory: ${MEMORY_SIZE}MB (optimized for CGO cold starts)"
    echo ""
    echo "Test Commands:"
    echo "--------------"
    echo "Health Check:"
    echo "  curl -H 'x-api-key: ${API_KEY}' '${API_ENDPOINT}/health'"
    echo ""
    echo "Planets:"
    echo "  curl -H 'x-api-key: ${API_KEY}' '${API_ENDPOINT}/api/v1/planets?year=2024&month=3&day=5&ut=12.0'"
    echo ""
    echo "Full Chart:"
    echo "  curl -H 'x-api-key: ${API_KEY}' '${API_ENDPOINT}/api/v1/full-chart?year=2024&month=3&day=5&ut=12.0&lat=40.7128&lng=-74.0060'"
    echo ""
    echo "============================================================================="
}

# Check CloudWatch logs for cold start info
check_cold_start() {
    log_info "Checking CloudWatch logs for cold start metrics..."

    FUNCTION_NAME="astral-backend-${ENVIRONMENT}"

    # Get recent log streams
    LOG_STREAM=$(aws logs describe-log-streams \
        --log-group-name "/aws/lambda/${FUNCTION_NAME}" \
        --order-by LastEventTime \
        --descending \
        --limit 1 \
        --query 'logStreams[0].logStreamName' \
        --output text 2>/dev/null || echo "")

    if [ -n "$LOG_STREAM" ] && [ "$LOG_STREAM" != "None" ]; then
        log_info "Recent log stream: $LOG_STREAM"

        # Get logs and look for Init Duration
        INIT_DURATION=$(aws logs filter-log-events \
            --log-group-name "/aws/lambda/${FUNCTION_NAME}" \
            --log-stream-names "$LOG_STREAM" \
            --filter-pattern "Init Duration" \
            --limit 1 \
            --query 'events[0].message' \
            --output text 2>/dev/null || echo "")

        if [ -n "$INIT_DURATION" ] && [ "$INIT_DURATION" != "None" ]; then
            log_info "Cold Start Metrics: $INIT_DURATION"

            # Extract duration
            DURATION_MS=$(echo "$INIT_DURATION" | grep -o 'Init Duration: [0-9.]*' | cut -d' ' -f3)
            if [ -n "$DURATION_MS" ]; then
                DURATION_SEC=$(echo "scale=2; $DURATION_MS / 1000" | bc 2>/dev/null || echo "N/A")
                log_info "Init Duration: ${DURATION_SEC}s (Swiss Ephemeris C-library loading)"

                if (( $(echo "$DURATION_MS > 3000" | bc -l 2>/dev/null || echo "0") )); then
                    log_warning "Cold start is >3s. Consider increasing memory allocation."
                fi
            fi
        fi
    else
        log_warning "No CloudWatch logs found yet. They may take a minute to appear."
    fi
}

# Main deployment flow
main() {
    echo "============================================================================="
    echo "              Astral Backend - Lambda Deployment"
    echo "============================================================================="
    echo ""

    check_prerequisites

    echo ""
    echo "Deployment Plan:"
    echo "  1. Create ECR Repository (Terraform)"
    echo "  2. Build and Push Docker Image"
    echo "  3. Deploy Lambda + API Gateway (Terraform)"
    echo "  4. Test Health Endpoint"
    echo "  5. Test Chart Calculation"
    echo ""
    read -p "Continue? (y/n): " confirm
    if [[ ! $confirm =~ ^[Yy]$ ]]; then
        log_info "Deployment cancelled."
        exit 0
    fi

    step1_create_ecr
    step2_build_and_push
    step3_deploy_infrastructure

    if step4_test_deployment; then
        step5_test_calculation
        show_summary
        check_cold_start
    else
        log_error "Deployment tests failed. Check the logs above."
        echo ""
        echo "Troubleshooting:"
        echo "  1. Check CloudWatch: /aws/lambda/astral-backend-${ENVIRONMENT}"
        echo "  2. Verify API Key in AWS Console → API Gateway → API Keys"
        echo "  3. Check ECR image exists: aws ecr describe-images --repository-name astral-backend"
        echo "  4. Re-run: ./deploy.sh"
        exit 1
    fi
}

# Allow sourcing or running
if [ "${BASH_SOURCE[0]}" == "${0}" ]; then
    main "$@"
fi

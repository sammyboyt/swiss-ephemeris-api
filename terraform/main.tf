# =============================================================================
# Astral Backend - AWS Serverless Infrastructure
# Terraform Configuration for Lambda + API Gateway (REST API v1)
# PRODUCTION-READY with Image Digest Tracking
# =============================================================================

terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # S3 backend for state management (recommended for production)
  # backend "s3" {
  #   bucket = "astral-terraform-state"
  #   key    = "astral-backend/terraform.tfstate"
  #   region = "us-east-1"
  #   encrypt = true
  # }
}

# Default AWS provider
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "astral-backend"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

# =============================================================================
# Variables
# =============================================================================

variable "aws_region" {
  description = "AWS region for deployment"
  type        = string
  default     = "eu-west-2"
}

variable "environment" {
  description = "Deployment environment (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "lambda_memory_size" {
  description = "Lambda memory allocation in MB (128-10240). Higher = more CPU"
  type        = number
  default     = 1024  # Increased from 512 for better CGO cold start performance
}

variable "lambda_timeout" {
  description = "Lambda timeout in seconds (max 29 for API Gateway)"
  type        = number
  default     = 15
}

variable "lambda_storage" {
  description = "Lambda ephemeral storage in MB (512-10240) for /tmp"
  type        = number
  default     = 512
}

variable "api_key_name" {
  description = "Name of the API key for testing"
  type        = string
  default     = "astral-default-key"
}

variable "usage_plan_limit" {
  description = "Monthly request limit for usage plan"
  type        = number
  default     = 1000
}

variable "usage_plan_rate_limit" {
  description = "Requests per second limit"
  type        = number
  default     = 10
}

variable "usage_plan_burst_limit" {
  description = "Burst limit for rate throttling"
  type        = number
  default     = 20
}

variable "log_retention_days" {
  description = "CloudWatch log retention in days (cost optimization)"
  type        = number
  default     = 7
}

# =============================================================================
# Data Sources
# =============================================================================

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# Get the most recent image from ECR (for initial deployment if no local build)
data "aws_ecr_image" "lambda_image" {
  repository_name = aws_ecr_repository.astral_backend.name
  most_recent     = true
}

# =============================================================================
# ECR Repository (Container Registry)
# =============================================================================

resource "aws_ecr_repository" "astral_backend" {
  name                 = "astral-backend"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  force_delete = true

  tags = {
    Name = "astral-backend"
  }
}

resource "aws_ecr_lifecycle_policy" "astral_backend" {
  repository = aws_ecr_repository.astral_backend.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep last 30 images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = 30
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}

# =============================================================================
# IAM Role for Lambda Execution
# =============================================================================

resource "aws_iam_role" "lambda_execution" {
  name = "astral-lambda-execution-role-${var.environment}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_basic_execution" {
  role       = aws_iam_role.lambda_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# CloudWatch Logs policy
resource "aws_iam_role_policy" "lambda_cloudwatch" {
  name = "cloudwatch-logs-policy"
  role = aws_iam_role.lambda_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DescribeLogGroups",
          "logs:DescribeLogStreams"
        ]
        Resource = "arn:aws:logs:*:*:*"
      }
    ]
  })
}

# X-Ray tracing policy
resource "aws_iam_role_policy_attachment" "lambda_xray" {
  role       = aws_iam_role.lambda_execution.name
  policy_arn = "arn:aws:iam::aws:policy/AWSXRayDaemonWriteAccess"
}

# =============================================================================
# Lambda Function (Container Image with Digest)
# =============================================================================

resource "aws_lambda_function" "astral_backend" {
  function_name = "astral-backend-${var.environment}"
  role          = aws_iam_role.lambda_execution.arn
  package_type  = "Image"
  
  # Use image digest for proper change detection (not just tag)
  image_uri     = "${aws_ecr_repository.astral_backend.repository_url}@${data.aws_ecr_image.lambda_image.image_digest}"

  # Resource allocation - increased for CGO cold start performance
  memory_size     = var.lambda_memory_size
  timeout         = var.lambda_timeout
  ephemeral_storage {
    size = var.lambda_storage  # /tmp space for Swiss Ephemeris temp files
  }

  # Environment variables
  environment {
    variables = {
      LOG_LEVEL   = var.environment == "prod" ? "info" : "debug"
      ENVIRONMENT = var.environment
    }
  }

  # Tracing for debugging cold starts
  tracing_config {
    mode = "Active"
  }

  # Publish versions for rollbacks
  publish = true

  tags = {
    Name = "astral-backend-lambda"
  }

  depends_on = [aws_ecr_repository.astral_backend]
}

# Lambda function URL (alternative to API Gateway for testing)
resource "aws_lambda_function_url" "astral_backend" {
  function_name      = aws_lambda_function.astral_backend.function_name
  authorization_type = "NONE"  # For testing only - API Gateway handles auth in production

  cors {
    allow_credentials = true
    allow_origins     = ["*"]
    allow_methods     = ["*"]
    allow_headers     = ["date", "keep-alive"]
    expose_headers    = ["keep-alive", "date"]
    max_age          = 86400
  }
}

# CloudWatch Log Group for Lambda with retention
resource "aws_cloudwatch_log_group" "lambda_logs" {
  name              = "/aws/lambda/${aws_lambda_function.astral_backend.function_name}"
  retention_in_days = var.log_retention_days

  tags = {
    Name = "astral-lambda-logs"
  }
}

# =============================================================================
# API Gateway (REST API v1) - NOT HTTP API v2
# This is CRITICAL for native API Key support
# =============================================================================

resource "aws_api_gateway_rest_api" "astral_api" {
  name        = "astral-backend-api-${var.environment}"
  description = "Astral Backend Ephemeris API - ${var.environment}"

  endpoint_configuration {
    types = ["EDGE"]
  }

  binary_media_types = ["*/*"]
}

# -----------------------------------------------------------------------------
# API Gateway Resources (Paths)
# -----------------------------------------------------------------------------

# Proxy resource for all paths
resource "aws_api_gateway_resource" "proxy" {
  rest_api_id = aws_api_gateway_rest_api.astral_api.id
  parent_id   = aws_api_gateway_rest_api.astral_api.root_resource_id
  path_part   = "{proxy+}"
}

# -----------------------------------------------------------------------------
# API Gateway Methods
# -----------------------------------------------------------------------------

# ANY method for /{proxy+} with API key required
resource "aws_api_gateway_method" "proxy" {
  rest_api_id      = aws_api_gateway_rest_api.astral_api.id
  resource_id      = aws_api_gateway_resource.proxy.id
  http_method      = "ANY"
  authorization    = "NONE"
  api_key_required = true

  request_parameters = {
    "method.request.path.proxy" = true
  }
}

# Root path method
resource "aws_api_gateway_method" "root" {
  rest_api_id      = aws_api_gateway_rest_api.astral_api.id
  resource_id      = aws_api_gateway_rest_api.astral_api.root_resource_id
  http_method      = "ANY"
  authorization    = "NONE"
  api_key_required = true
}

# OPTIONS method for CORS (no API key required)
resource "aws_api_gateway_method" "options_proxy" {
  rest_api_id      = aws_api_gateway_rest_api.astral_api.id
  resource_id      = aws_api_gateway_resource.proxy.id
  http_method      = "OPTIONS"
  authorization    = "NONE"
  api_key_required = false
}

resource "aws_api_gateway_method" "options_root" {
  rest_api_id      = aws_api_gateway_rest_api.astral_api.id
  resource_id      = aws_api_gateway_rest_api.astral_api.root_resource_id
  http_method      = "OPTIONS"
  authorization    = "NONE"
  api_key_required = false
}

# -----------------------------------------------------------------------------
# Lambda Integration
# -----------------------------------------------------------------------------

resource "aws_api_gateway_integration" "lambda_proxy" {
  rest_api_id = aws_api_gateway_rest_api.astral_api.id
  resource_id = aws_api_gateway_resource.proxy.id
  http_method = aws_api_gateway_method.proxy.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.astral_backend.invoke_arn
  timeout_milliseconds    = 29000
}

resource "aws_api_gateway_integration" "lambda_root" {
  rest_api_id = aws_api_gateway_rest_api.astral_api.id
  resource_id = aws_api_gateway_rest_api.astral_api.root_resource_id
  http_method = aws_api_gateway_method.root.http_method

  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.astral_backend.invoke_arn
}

# Mock integration for OPTIONS (CORS)
resource "aws_api_gateway_integration" "options_proxy" {
  rest_api_id = aws_api_gateway_rest_api.astral_api.id
  resource_id = aws_api_gateway_resource.proxy.id
  http_method = aws_api_gateway_method.options_proxy.http_method

  type = "MOCK"
  request_templates = {
    "application/json" = "{\"statusCode\": 200}"
  }
}

resource "aws_api_gateway_integration" "options_root" {
  rest_api_id = aws_api_gateway_rest_api.astral_api.id
  resource_id = aws_api_gateway_rest_api.astral_api.root_resource_id
  http_method = aws_api_gateway_method.options_root.http_method

  type = "MOCK"
  request_templates = {
    "application/json" = "{\"statusCode\": 200}"
  }
}

# -----------------------------------------------------------------------------
# Lambda Permission for API Gateway
# -----------------------------------------------------------------------------

resource "aws_lambda_permission" "api_gateway" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.astral_backend.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.astral_api.execution_arn}/*/*"
}

# -----------------------------------------------------------------------------
# CORS Configuration
# -----------------------------------------------------------------------------

resource "aws_api_gateway_method_response" "options_200" {
  rest_api_id = aws_api_gateway_rest_api.astral_api.id
  resource_id = aws_api_gateway_resource.proxy.id
  http_method = aws_api_gateway_method.options_proxy.http_method
  status_code = "200"

  response_parameters = {
    "method.response.header.Access-Control-Allow-Headers" = true
    "method.response.header.Access-Control-Allow-Methods" = true
    "method.response.header.Access-Control-Allow-Origin"  = true
  }
}

resource "aws_api_gateway_integration_response" "options_proxy" {
  rest_api_id = aws_api_gateway_rest_api.astral_api.id
  resource_id = aws_api_gateway_resource.proxy.id
  http_method = aws_api_gateway_method.options_proxy.http_method
  status_code = aws_api_gateway_method_response.options_200.status_code

  response_parameters = {
    "method.response.header.Access-Control-Allow-Headers" = "'Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token,x-api-key'"
    "method.response.header.Access-Control-Allow-Methods" = "'GET,POST,OPTIONS'"
    "method.response.header.Access-Control-Allow-Origin"  = "'*'"
  }
}

# -----------------------------------------------------------------------------
# API Deployment & Stage
# -----------------------------------------------------------------------------

resource "aws_api_gateway_deployment" "astral_api" {
  rest_api_id = aws_api_gateway_rest_api.astral_api.id

  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_resource.proxy.id,
      aws_api_gateway_method.proxy.id,
      aws_api_gateway_integration.lambda_proxy.id,
      aws_api_gateway_method.root.id,
      aws_api_gateway_integration.lambda_root.id,
      aws_lambda_function.astral_backend.version,  # Trigger on Lambda version change
    ]))
  }

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    aws_api_gateway_integration.lambda_proxy,
    aws_api_gateway_integration.lambda_root,
  ]
}

resource "aws_api_gateway_stage" "astral_api" {
  deployment_id = aws_api_gateway_deployment.astral_api.id
  rest_api_id   = aws_api_gateway_rest_api.astral_api.id
  stage_name    = var.environment

  # Note: To enable access logging, you need to set the CloudWatch Logs role ARN
  # in API Gateway Settings in the AWS Console first
  # access_log_settings {
  #   destination_arn = aws_cloudwatch_log_group.api_gateway.arn
  #   format = jsonencode({...})
  # }

  xray_tracing_enabled = true

  tags = {
    Name = "astral-api-${var.environment}"
  }
}

# CloudWatch Log Group for API Gateway
resource "aws_cloudwatch_log_group" "api_gateway" {
  name              = "/aws/apigateway/${aws_api_gateway_rest_api.astral_api.name}"
  retention_in_days = var.log_retention_days
}

# =============================================================================
# API Keys & Usage Plans
# =============================================================================

resource "aws_api_gateway_api_key" "default" {
  name        = var.api_key_name
  description = "Default API key for ${var.environment} environment"
  enabled     = true

  tags = {
    Name = "astral-api-key-${var.environment}"
  }
}

resource "aws_api_gateway_usage_plan" "astral_usage_plan" {
  name        = "astral-usage-plan-${var.environment}"
  description = "Usage plan for Astral Backend API"

  api_stages {
    api_id = aws_api_gateway_rest_api.astral_api.id
    stage  = aws_api_gateway_stage.astral_api.stage_name
  }

  quota_settings {
    limit  = var.usage_plan_limit
    offset = 0
    period = "MONTH"
  }

  throttle_settings {
    burst_limit = var.usage_plan_burst_limit
    rate_limit  = var.usage_plan_rate_limit
  }

  tags = {
    Name = "astral-usage-plan-${var.environment}"
  }
}

resource "aws_api_gateway_usage_plan_key" "default" {
  key_id        = aws_api_gateway_api_key.default.id
  key_type      = "API_KEY"
  usage_plan_id = aws_api_gateway_usage_plan.astral_usage_plan.id
}

# =============================================================================
# Outputs
# =============================================================================

output "ecr_repository_url" {
  description = "URL of the ECR repository"
  value       = aws_ecr_repository.astral_backend.repository_url
}

output "lambda_function_name" {
  description = "Name of the Lambda function"
  value       = aws_lambda_function.astral_backend.function_name
}

output "lambda_version" {
  description = "Published version of the Lambda function"
  value       = aws_lambda_function.astral_backend.version
}

output "lambda_invoke_arn" {
  description = "ARN for Lambda invocation"
  value       = aws_lambda_function.astral_backend.invoke_arn
}

output "lambda_function_url" {
  description = "Direct Lambda Function URL (for testing, no API key required)"
  value       = aws_lambda_function_url.astral_backend.function_url
}

output "api_gateway_id" {
  description = "ID of the REST API"
  value       = aws_api_gateway_rest_api.astral_api.id
}

output "api_gateway_endpoint" {
  description = "Base URL for API Gateway (requires x-api-key header)"
  value       = aws_api_gateway_stage.astral_api.invoke_url
}

output "api_key_id" {
  description = "ID of the default API key"
  value       = aws_api_gateway_api_key.default.id
}

output "api_key_value" {
  description = "Value of the default API key (sensitive)"
  value       = aws_api_gateway_api_key.default.value
  sensitive   = true
}

output "usage_plan_id" {
  description = "ID of the usage plan"
  value       = aws_api_gateway_usage_plan.astral_usage_plan.id
}

output "first_request_test_command" {
  description = "Command to test the first API call"
  value       = "curl -H 'x-api-key: ${aws_api_gateway_api_key.default.value}' '${aws_api_gateway_stage.astral_api.invoke_url}/health'"
  sensitive   = true
}

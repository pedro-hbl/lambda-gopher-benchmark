#!/bin/bash
# Lambda Gopher Benchmark - AWS Deployment Script
# This script automates the deployment of the Lambda Gopher Benchmark platform to AWS

set -e  # Exit on error

# Configuration
AWS_REGION=${AWS_REGION:-"us-east-1"}
ENVIRONMENT=${ENVIRONMENT:-"prod"}
S3_BUCKET_PREFIX="lambda-gopher-benchmark"
DYNAMODB_READ_CAPACITY=${DYNAMODB_READ_CAPACITY:-50}
DYNAMODB_WRITE_CAPACITY=${DYNAMODB_WRITE_CAPACITY:-50}

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to print section header
section() {
  echo -e "\n${GREEN}==== $1 ====${NC}\n"
}

# Function to print warning
warning() {
  echo -e "${YELLOW}WARNING: $1${NC}"
}

# Function to print error and exit
error() {
  echo -e "${RED}ERROR: $1${NC}"
  exit 1
}

# Check prerequisites
section "Checking Prerequisites"

# Check if AWS CLI is installed
if ! command -v aws &> /dev/null; then
  error "AWS CLI is not installed. Please install it before proceeding."
fi

# Check if Terraform is installed
if ! command -v terraform &> /dev/null; then
  error "Terraform is not installed. Please install it before proceeding."
fi

# Check if AWS credentials are configured
if ! aws sts get-caller-identity &> /dev/null; then
  error "AWS credentials are not configured. Please run 'aws configure' before proceeding."
fi

echo "All prerequisites are met!"

# Build Lambda function
section "Building Lambda Function"
cd "$(dirname "$0")/.."  # Change to project root
echo "Building Lambda function for Linux..."
GOOS=linux GOARCH=amd64 go build -o bootstrap cmd/benchmark/main.go
if [ ! -f bootstrap ]; then
  error "Failed to build Lambda function"
fi

echo "Creating Lambda deployment package..."
zip lambda-function.zip bootstrap
if [ ! -f lambda-function.zip ]; then
  error "Failed to create Lambda deployment package"
fi

echo "Lambda function built successfully!"

# Deploy with Terraform
section "Deploying Infrastructure with Terraform"
cd deployments/terraform

echo "Initializing Terraform..."
terraform init

echo "Planning deployment..."
terraform plan \
  -var="aws_region=${AWS_REGION}" \
  -var="environment=${ENVIRONMENT}" \
  -var="dynamodb_read_capacity=${DYNAMODB_READ_CAPACITY}" \
  -var="dynamodb_write_capacity=${DYNAMODB_WRITE_CAPACITY}"

echo "Applying Terraform configuration..."
terraform apply \
  -auto-approve \
  -var="aws_region=${AWS_REGION}" \
  -var="environment=${ENVIRONMENT}" \
  -var="dynamodb_read_capacity=${DYNAMODB_READ_CAPACITY}" \
  -var="dynamodb_write_capacity=${DYNAMODB_WRITE_CAPACITY}"

if [ $? -ne 0 ]; then
  error "Terraform apply failed"
fi

echo "Infrastructure deployed successfully!"

# Upload Lambda function to S3
section "Uploading Lambda Function"

# Get the S3 bucket name from Terraform output
BUCKET_NAME=$(terraform output -raw lambda_bucket_name)
if [ -z "$BUCKET_NAME" ]; then
  error "Failed to get S3 bucket name from Terraform output"
fi

echo "Uploading Lambda function to S3 bucket: ${BUCKET_NAME}..."
aws s3 cp ../../lambda-function.zip "s3://${BUCKET_NAME}/lambda/lambda-function.zip"

echo "Updating Lambda functions..."
for func in $(terraform output -json lambda_function_names | jq -r 'keys[]'); do
  echo "Updating function: lambda-gopher-benchmark-${func}"
  aws lambda update-function-code \
    --function-name "lambda-gopher-benchmark-${func}" \
    --s3-bucket "${BUCKET_NAME}" \
    --s3-key "lambda/lambda-function.zip" \
    --publish

  # Wait for the update to complete
  aws lambda wait function-updated \
    --function-name "lambda-gopher-benchmark-${func}"
done

echo "Lambda functions updated successfully!"

# Get Lambda function URLs
section "Collecting Lambda Function URLs"

# Create a .env file to store URLs
echo "# Lambda Function URLs" > ../../.env

# Get URLs for each function
for func in $(terraform output -json lambda_function_names | jq -r 'keys[]'); do
  FUNCTION_NAME="lambda-gopher-benchmark-${func}"
  URL=$(aws lambda get-function-url-config --function-name "${FUNCTION_NAME}" --query 'FunctionUrl' --output text 2>/dev/null || echo "URL_NOT_CONFIGURED")
  
  if [ "$URL" = "URL_NOT_CONFIGURED" ]; then
    warning "Function URL not configured for ${FUNCTION_NAME}"
    continue
  fi
  
  echo "${func^^}_FUNCTION_URL=${URL}" >> ../../.env
  echo "Found ${func}: ${URL}"
done

# Add the main Lambda endpoint
MAIN_FUNCTION=$(terraform output -raw main_lambda_function_name)
if [ -n "$MAIN_FUNCTION" ]; then
  MAIN_URL=$(aws lambda get-function-url-config --function-name "${MAIN_FUNCTION}" --query 'FunctionUrl' --output text 2>/dev/null || echo "URL_NOT_CONFIGURED")
  
  if [ "$MAIN_URL" != "URL_NOT_CONFIGURED" ]; then
    echo "LAMBDA_ENDPOINT=${MAIN_URL}" >> ../../.env
    echo "Main Lambda endpoint: ${MAIN_URL}"
  else
    warning "Function URL not configured for ${MAIN_FUNCTION}"
  fi
fi

echo "Function URLs saved to .env file"

# All done!
section "Deployment Complete"
echo "The Lambda Gopher Benchmark platform has been successfully deployed to AWS!"
echo "To run benchmarks, use:"
echo "  source .env  # Load environment variables"
echo "  go run cmd/runner/main.go --config configs/comparison_benchmark.json --lambda-endpoint \${LAMBDA_ENDPOINT} --output results"
echo ""
echo "To clean up resources when you're done:"
echo "  cd deployments/terraform && terraform destroy"

# Clean up local build artifacts
rm -f ../../bootstrap
echo "Deployment script completed successfully!" 
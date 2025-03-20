#!/bin/bash
# Lambda Gopher Benchmark - AWS Resources Cleanup Script
# This script destroys all AWS resources created by the benchmark to avoid additional charges

set -e  # Exit on error

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

# Request user confirmation
section "Confirmation"
echo -e "${RED}ATTENTION: This operation will destroy ALL AWS resources created by Lambda Gopher Benchmark.${NC}"
echo -e "${RED}This action CANNOT be undone!${NC}"
echo ""
read -p "Type 'CONFIRM' to proceed with resource destruction: " confirmation

if [ "$confirmation" != "CONFIRM" ]; then
  echo "Operation canceled by user."
  exit 0
fi

# Change to project directory
cd "$(dirname "$0")/.."  # Change to project root

# Destroy resources using Terraform
section "Destroying Resources via Terraform"

# Extract AWS region from .env file if it exists
AWS_REGION="us-east-1"  # Default value
if [ -f .env ] && grep -q "AWS_REGION" .env; then
  AWS_REGION=$(grep "AWS_REGION" .env | cut -d '=' -f2)
fi

# Extract environment from .env file if it exists
ENVIRONMENT="prod"  # Default value
if [ -f .env ] && grep -q "ENVIRONMENT" .env; then
  ENVIRONMENT=$(grep "ENVIRONMENT" .env | cut -d '=' -f2)
fi

echo "Using AWS Region: ${AWS_REGION}"
echo "Using Environment: ${ENVIRONMENT}"

cd deployments/terraform

echo "Initializing Terraform..."
terraform init

echo "Destroying infrastructure..."
terraform destroy -auto-approve \
  -var="aws_region=${AWS_REGION}" \
  -var="environment=${ENVIRONMENT}"

if [ $? -ne 0 ]; then
  warning "Terraform destroy encountered some issues. Some resources might still exist."
else
  echo "Terraform destroy completed successfully!"
fi

# Check remaining resources
section "Checking Remaining Resources"

echo "Checking Lambda functions..."
LAMBDA_FUNCTIONS=$(aws lambda list-functions --region ${AWS_REGION} --query "Functions[?contains(FunctionName, 'lambda-gopher-benchmark')].FunctionName" --output text)

if [ -n "$LAMBDA_FUNCTIONS" ]; then
  warning "Some Lambda functions still exist. Trying to delete manually..."
  for func in $LAMBDA_FUNCTIONS; do
    echo "Deleting Lambda function: $func"
    aws lambda delete-function --function-name $func --region ${AWS_REGION}
  done
fi

echo "Checking DynamoDB tables..."
DYNAMODB_TABLES=$(aws dynamodb list-tables --region ${AWS_REGION} --query "TableNames[?contains(@, 'LambdaGopherBenchmark')]" --output text)

if [ -n "$DYNAMODB_TABLES" ]; then
  warning "Some DynamoDB tables still exist. Trying to delete manually..."
  for table in $DYNAMODB_TABLES; do
    echo "Deleting DynamoDB table: $table"
    aws dynamodb delete-table --table-name $table --region ${AWS_REGION}
  done
fi

echo "Checking Timestream databases..."
TIMESTREAM_DATABASES=$(aws timestream-write list-databases --region ${AWS_REGION} --query "Databases[?contains(DatabaseName, 'LambdaGopherBenchmark')].DatabaseName" --output text 2>/dev/null || echo "")

if [ -n "$TIMESTREAM_DATABASES" ]; then
  warning "Some Timestream databases still exist. Trying to delete manually..."
  for db in $TIMESTREAM_DATABASES; do
    echo "Deleting Timestream tables in database $db..."
    TIMESTREAM_TABLES=$(aws timestream-write list-tables --database-name $db --region ${AWS_REGION} --query "Tables[].TableName" --output text 2>/dev/null || echo "")
    for table in $TIMESTREAM_TABLES; do
      echo "Deleting Timestream table: $table"
      aws timestream-write delete-table --database-name $db --table-name $table --region ${AWS_REGION}
    done
    
    echo "Deleting Timestream database: $db"
    aws timestream-write delete-database --database-name $db --region ${AWS_REGION}
  done
fi

# Check S3 buckets
echo "Checking S3 buckets..."
S3_BUCKETS=$(aws s3api list-buckets --query "Buckets[?contains(Name, 'lambda-gopher-benchmark')].Name" --output text)

if [ -n "$S3_BUCKETS" ]; then
  warning "Some S3 buckets still exist. Trying to delete manually..."
  for bucket in $S3_BUCKETS; do
    echo "Emptying and deleting S3 bucket: $bucket"
    aws s3 rm s3://$bucket --recursive
    aws s3api delete-bucket --bucket $bucket --region ${AWS_REGION}
  done
fi

# Clean up local files
section "Cleaning Up Local Files"
echo "Removing .env file..."
rm -f ../../.env

echo "Removing terraform.tfstate files..."
rm -f terraform.tfstate*

section "Cleanup Complete"
echo "The AWS resources for Lambda Gopher Benchmark have been destroyed."
echo "Check your AWS dashboard to ensure there are no remaining resources that could generate charges." 
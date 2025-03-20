# Lambda Gopher Benchmark Deployment Guide

This comprehensive guide provides detailed instructions for deploying and running the Lambda Gopher Benchmark platform in both local and AWS environments.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Local Development Setup](#local-development-setup)
3. [AWS Deployment](#aws-deployment)
4. [Running Benchmarks](#running-benchmarks)
5. [Analyzing Results](#analyzing-results)
6. [Troubleshooting](#troubleshooting)
7. [Advanced Configurations](#advanced-configurations)

## Prerequisites

### Required Software

- Go 1.21 or higher
- Docker
- AWS CLI v2
- Terraform 1.0+
- Git

### AWS Account Requirements

- An AWS account with administrative access
- AWS credentials configured locally with the AWS CLI
- Sufficient permissions to create and manage Lambda functions, IAM roles, DynamoDB tables, S3 buckets, and CloudWatch resources

### Installation

1. **Install Go**:
   Download and install from [golang.org](https://golang.org/dl/).

2. **Install Docker**:
   Download and install from [docker.com](https://www.docker.com/get-started).

3. **Install AWS CLI**:
   Follow instructions at [AWS CLI Installation Guide](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html).

   Configure AWS CLI:
   ```bash
   aws configure
   ```

4. **Install Terraform**:
   Download and install from [terraform.io](https://www.terraform.io/downloads.html).

5. **Clone the Repository**:
   ```bash
   git clone https://github.com/yourusername/lambda-gopher-benchmark.git
   cd lambda-gopher-benchmark
   ```

## Local Development Setup

### Install Dependencies

```bash
go mod download
go mod tidy
```

### Running Local Environment with Docker

For local development and testing, you can use Docker to run local versions of the databases:

#### DynamoDB Local

```bash
docker run -d -p 8000:8000 --name dynamodb-local amazon/dynamodb-local
```

#### ImmuDB Local

```bash
docker run -d -p 3322:3322 -p 9497:9497 --name immudb-local codenotary/immudb:latest
```

#### LocalStack (for AWS Services)

```bash
docker run -d -p 4566:4566 --name localstack localstack/localstack
```

### Setting Up Local Databases

#### DynamoDB Local Table Setup

```bash
# Create a table in DynamoDB Local
aws dynamodb create-table \
    --table-name LambdaGopherBenchmark \
    --attribute-definitions AttributeName=PK,AttributeType=S AttributeName=SK,AttributeType=S \
    --key-schema AttributeName=PK,KeyType=HASH AttributeName=SK,KeyType=RANGE \
    --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
    --endpoint-url http://localhost:8000
```

#### ImmuDB Setup

Connect to your ImmuDB container and create a database:

```bash
docker exec -it immudb-local immuadmin login immudb
docker exec -it immudb-local immuclient database create benchmarkdb
```

## AWS Deployment

### Automated Deployment (Recommended)

The Lambda Gopher Benchmark platform includes a deployment script that automates the entire AWS deployment process:

```bash
# Make the script executable (Linux/macOS)
chmod +x scripts/deploy-aws.sh

# Run the deployment script
./scripts/deploy-aws.sh
```

For Windows PowerShell users:
```powershell
# Run the deployment script
.\scripts\deploy-aws.ps1
```

The deployment script will:
1. Check prerequisites
2. Build the Lambda function
3. Deploy infrastructure with Terraform
4. Upload the Lambda function to S3
5. Configure environment variables

### Manual Deployment

If you prefer to deploy manually, follow these steps:

#### 1. Build the Lambda Function

```bash
# Build the Lambda function for Linux
GOOS=linux GOARCH=amd64 go build -o bootstrap cmd/benchmark/main.go

# Create the deployment package
zip lambda-function.zip bootstrap
```

#### 2. Deploy AWS Infrastructure with Terraform

```bash
cd deployments/terraform

# Initialize Terraform
terraform init

# Plan the deployment
terraform plan \
  -var="aws_region=us-east-1" \
  -var="environment=prod" \
  -var="dynamodb_read_capacity=50" \
  -var="dynamodb_write_capacity=50"

# Apply the configuration
terraform apply \
  -auto-approve \
  -var="aws_region=us-east-1" \
  -var="environment=prod" \
  -var="dynamodb_read_capacity=50" \
  -var="dynamodb_write_capacity=50"

# Get the S3 bucket name
BUCKET_NAME=$(terraform output -raw lambda_bucket_name)
```

#### 3. Upload Lambda Function to S3

```bash
aws s3 cp ../lambda-function.zip "s3://${BUCKET_NAME}/lambda/lambda-function.zip"
```

#### 4. Update Lambda Functions

```bash
# Update all Lambda functions
for func in $(terraform output -json lambda_function_names | jq -r 'keys[]'); do
  aws lambda update-function-code \
    --function-name "lambda-gopher-benchmark-${func}" \
    --s3-bucket "${BUCKET_NAME}" \
    --s3-key "lambda/lambda-function.zip" \
    --publish
done
```

#### 5. Collect Lambda Function URLs

Create a `.env` file with function URLs:

```bash
# Create .env file
echo "# Lambda Function URLs" > .env

# Get URLs for each function
for func in $(terraform output -json lambda_function_names | jq -r 'keys[]'); do
  FUNCTION_NAME="lambda-gopher-benchmark-${func}"
  URL=$(aws lambda get-function-url-config --function-name "${FUNCTION_NAME}" --query 'FunctionUrl' --output text)
  
  echo "${func^^}_FUNCTION_URL=${URL}" >> .env
done

# Add the main Lambda endpoint
MAIN_FUNCTION=$(terraform output -raw main_lambda_function_name)
MAIN_URL=$(aws lambda get-function-url-config --function-name "${MAIN_FUNCTION}" --query 'FunctionUrl' --output text)
echo "LAMBDA_ENDPOINT=${MAIN_URL}" >> .env
```

## Running Benchmarks

### Using Configuration Files

The benchmark runner supports JSON configuration files for running benchmarks:

```bash
# Source environment variables
source .env

# Run a benchmark using a configuration file
go run cmd/runner/main.go \
  --config configs/comparison_benchmark.json \
  --lambda-endpoint ${LAMBDA_ENDPOINT} \
  --output results
```

### Available Benchmark Configurations

The platform includes several predefined benchmark configurations:

1. **DynamoDB Benchmark**:
   ```bash
   go run cmd/runner/main.go \
     --config configs/dynamodb_benchmark.json \
     --lambda-endpoint ${LAMBDA_ENDPOINT} \
     --output results/dynamodb
   ```

2. **ImmuDB Benchmark**:
   ```bash
   go run cmd/runner/main.go \
     --config configs/immudb_benchmark.json \
     --lambda-endpoint ${LAMBDA_ENDPOINT} \
     --output results/immudb
   ```

3. **Timestream Benchmark**:
   ```bash
   go run cmd/runner/main.go \
     --config configs/timestream_benchmark.json \
     --lambda-endpoint ${LAMBDA_ENDPOINT} \
     --output results/timestream
   ```

4. **Comparison Benchmark** (all databases):
   ```bash
   go run cmd/runner/main.go \
     --config configs/comparison_benchmark.json \
     --lambda-endpoint ${LAMBDA_ENDPOINT} \
     --output results/comparison
   ```

### Using Database-Specific Endpoints

For improved performance, you can use database-specific Lambda endpoints:

```bash
# Run DynamoDB benchmark with its specific endpoint
go run cmd/runner/main.go \
  --config configs/dynamodb_benchmark.json \
  --lambda-endpoint ${DYNAMODB_FUNCTION_URL} \
  --output results/dynamodb
```

### Custom Parameters

You can override configuration parameters when running benchmarks:

```bash
go run cmd/runner/main.go \
  --config configs/dynamodb_benchmark.json \
  --lambda-endpoint ${LAMBDA_ENDPOINT} \
  --output results/custom \
  --custom-param "operations=1000" \
  --custom-param "dataSize=5120" \
  --custom-param "batchSize=50"
```

## Analyzing Results

### Visualizing Benchmark Results

The platform includes a visualizer tool for analyzing benchmark results:

```bash
# Visualize a single benchmark result
go run cmd/visualizer/main.go \
  --input results/comparison/result_20240601_120000.json \
  --output visualizations

# Compare multiple benchmark results
go run cmd/visualizer/main.go \
  --input-dir results/comparison \
  --output visualizations/comparison \
  --format html
```

### Available Visualization Formats

- **HTML** (`--format html`): Interactive charts and tables
- **CSV** (`--format csv`): Raw data for import into spreadsheet applications
- **PNG** (`--format png`): Static chart images
- **JSON** (`--format json`): Structured data for further processing

### Running Sample Visualizations

The platform includes sample visualizations that you can run:

```bash
# For Linux/macOS
./examples/run_sample_visualization.sh

# For Windows
.\examples\run_sample_visualization.ps1
```

## Troubleshooting

### Common Issues

#### AWS Deployment Failures

1. **IAM Permission Issues**:
   - Error: `User is not authorized to perform: iam:CreateRole`
   - Solution: Ensure your AWS user has the necessary IAM permissions.

2. **Lambda Function Size Limit**:
   - Error: `The unzipped size of your Lambda function exceeds the limit`
   - Solution: Optimize your code and dependencies to reduce the package size.

3. **VPC Configuration Issues**:
   - Error: `The subnet ID xxx does not exist`
   - Solution: Verify your VPC and subnet configurations in the Terraform variables.

#### Benchmark Runner Issues

1. **Connection Timeout**:
   - Error: `context deadline exceeded`
   - Solution: Check your Lambda function URL and network connectivity.

2. **Invalid Configuration**:
   - Error: `invalid database configuration`
   - Solution: Verify your benchmark configuration file for correct parameters.

3. **Missing Environment Variables**:
   - Error: `LAMBDA_ENDPOINT not set`
   - Solution: Ensure you've sourced the `.env` file created during deployment.

### Logs and Diagnostics

#### Viewing Lambda Logs

```bash
# Get Lambda log group name
LOG_GROUP_NAME="/aws/lambda/lambda-gopher-benchmark-dynamodb"

# View recent logs
aws logs get-log-events \
  --log-group-name "${LOG_GROUP_NAME}" \
  --log-stream-name "$(aws logs describe-log-streams --log-group-name ${LOG_GROUP_NAME} --order-by LastEventTime --descending --limit 1 --query 'logStreams[0].logStreamName' --output text)" \
  --limit 100
```

#### Enabling Debug Logging

For more detailed logs, run the benchmark with the `--verbose` flag:

```bash
go run cmd/runner/main.go \
  --config configs/comparison_benchmark.json \
  --lambda-endpoint ${LAMBDA_ENDPOINT} \
  --output results \
  --verbose
```

## Advanced Configurations

### Customizing Terraform Variables

Create a `terraform.tfvars` file in the `deployments/terraform` directory:

```hcl
aws_region             = "eu-west-1"
environment            = "staging"
dynamodb_read_capacity = 100
dynamodb_write_capacity = 100
```

### Configuring VPC and Subnets

Edit `deployments/terraform/variables.tf` to customize VPC settings:

```hcl
variable "lambda_vpc_enabled" {
  description = "Whether to deploy the Lambda function in a VPC"
  type        = bool
  default     = true
}

variable "lambda_vpc_subnet_ids" {
  description = "Subnet IDs for Lambda VPC configuration"
  type        = list(string)
  default     = ["subnet-abc123", "subnet-def456"]
}
```

### Using a Custom IAM Role

To use a custom IAM role for Lambda functions:

1. Create the role in AWS IAM with appropriate permissions
2. Edit `terraform.tfvars`:
   ```hcl
   lambda_role_name = "custom-lambda-role-name"
   ```

### Multi-Region Deployment

For multi-region benchmarking, deploy to multiple regions and update the configuration:

1. Deploy to each region separately
2. Create a custom benchmark configuration with appropriate region settings:
   ```json
   {
     "tests": [
       {
         "name": "us-east-1-write-test",
         "database": {
           "type": "dynamodb",
           "region": "us-east-1",
           "table": "LambdaGopherBenchmark"
         },
         "operation": {
           "type": "write",
           ...
         }
       },
       {
         "name": "eu-west-1-write-test",
         "database": {
           "type": "dynamodb",
           "region": "eu-west-1",
           "table": "LambdaGopherBenchmark"
         },
         "operation": {
           "type": "write",
           ...
         }
       }
     ]
   }
   ```

## Cleaning Up Resources

To avoid incurring AWS charges, clean up resources when finished:

```bash
cd deployments/terraform
terraform destroy -auto-approve \
  -var="aws_region=us-east-1" \
  -var="environment=prod"
```

This command will remove all AWS resources created by the deployment, including Lambda functions, IAM roles, DynamoDB tables, and S3 buckets. 
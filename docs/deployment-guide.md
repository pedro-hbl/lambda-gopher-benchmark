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

### AWS Architecture

The Lambda Gopher Benchmark platform on AWS consists of the following components:

1. **Lambda Functions**: A set of Lambda functions for executing benchmark operations against different databases with varying memory configurations
2. **DynamoDB**: A DynamoDB table for storing transaction data
3. **Timestream**: A Timestream database for time-series data
4. **EC2 Instance**: A t2.micro EC2 instance running ImmuDB
5. **S3 Bucket**: For storing Lambda function code
6. **CloudWatch**: For monitoring and logging

The architecture ensures that all three database types (DynamoDB, Timestream, and ImmuDB) are properly deployed and accessible from the Lambda functions.

### Complete AWS Deployment

The deployment process is fully automated using Terraform and deployment scripts. To deploy the complete benchmark platform:

1. **Prerequisites**:
   - AWS CLI configured with appropriate permissions
   - Terraform installed
   - Go 1.21 or higher installed

2. **Deployment Steps**:

   ```powershell
   # On Windows:
   .\scripts\deploy-aws.ps1
   
   # On Linux/macOS:
   ./scripts/deploy-aws.sh
   ```

   This script will:
   - Build the Lambda function
   - Deploy all AWS resources using Terraform
   - Upload the Lambda function code to S3
   - Update Lambda functions with the latest code
   - Configure Lambda function URLs
   - Set up an EC2 instance with ImmuDB
   - Create a `.env` file with all the connection details

3. **Terraform Resources Created**:
   - IAM roles and policies
   - Lambda functions for each database and operation type
   - DynamoDB table with GSI for queries
   - Timestream database and table
   - S3 bucket for Lambda function code
   - EC2 instance for ImmuDB
   - VPC, subnet, security groups for ImmuDB
   - CloudWatch dashboards and alarms

### ImmuDB on EC2

The ImmuDB database runs on a t2.micro EC2 instance, which is suitable for testing purposes and is eligible for the AWS free tier. The EC2 instance:

- Runs Amazon Linux 2 AMI
- Has ImmuDB installed and configured during instance startup
- Opens necessary ports for ImmuDB access (3322 for API, 9497 for metrics)
- Creates a benchmark database automatically
- Is publicly accessible with default credentials (immudb/immudb)

#### SSH Access to ImmuDB Instance

During deployment, you'll be asked if you want to configure SSH access to the ImmuDB EC2 instance. This is optional but can be useful for troubleshooting:

- If you choose "yes", you'll need to provide your SSH public key (either as a file path or by pasting the content)
- If you choose "no", SSH access will be disabled for the instance
- The key pair is specific to your deployment and won't affect other users

To connect to the instance when SSH access is enabled:

```bash
# Using the public IP from the .env file
ssh -i /path/to/your/private/key ec2-user@$(grep IMMUDB_ADDRESS .env | cut -d= -f2)
```

For Windows PowerShell:
```powershell
$immudbIp = (Get-Content .env | Where-Object { $_ -match "IMMUDB_ADDRESS=" }) -replace "IMMUDB_ADDRESS=", ""
ssh -i C:\path\to\your\private\key ec2-user@$immudbIp
```

### Running Benchmarks on AWS

After deploying the platform to AWS, the deployment script will create a `.env` file with all necessary connection details. To run a benchmark:

1. **Load environment variables**:

   ```powershell
   # PowerShell
   Get-Content .env | ForEach-Object { if ($_ -match '(.+)=(.+)') { $env:$matches[1] = $matches[2] } }
   
   # Bash
   source .env
   ```

2. **Run the comparison benchmark**:

   ```
   go run cmd/runner/main.go --config configs/comparison_benchmark.json --lambda-endpoint $env:LAMBDA_ENDPOINT --output results/aws
   ```

3. **Visualize the results**:

   ```
   go run cmd/visualizer/main.go --input results/aws --output visualizations/aws
   ```

### Resource Cleanup

When done with benchmarking, clean up all AWS resources to avoid unnecessary charges:

```powershell
# On Windows:
.\scripts\destroy-aws-resources.ps1

# On Linux/macOS:
./scripts/destroy-aws-resources.sh
```

This script will use Terraform to destroy all created resources, and perform additional cleanup for any resources that might not have been properly removed.

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
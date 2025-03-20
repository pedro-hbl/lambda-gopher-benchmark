# Lambda Gopher Benchmark - AWS Deployment Script for Windows
# This PowerShell script automates the deployment of the Lambda Gopher Benchmark platform to AWS

# Configuration
$AWS_REGION = if ($env:AWS_REGION) { $env:AWS_REGION } else { "us-east-1" }
$ENVIRONMENT = if ($env:ENVIRONMENT) { $env:ENVIRONMENT } else { "prod" }
$S3_BUCKET_PREFIX = "lambda-gopher-benchmark"
$DYNAMODB_READ_CAPACITY = if ($env:DYNAMODB_READ_CAPACITY) { $env:DYNAMODB_READ_CAPACITY } else { 50 }
$DYNAMODB_WRITE_CAPACITY = if ($env:DYNAMODB_WRITE_CAPACITY) { $env:DYNAMODB_WRITE_CAPACITY } else { 50 }

# Function to print section header
function Write-Section {
    param (
        [string]$Title
    )
    Write-Host "`n==== $Title ====`n" -ForegroundColor Green
}

# Function to print warning
function Write-Warning {
    param (
        [string]$Message
    )
    Write-Host "WARNING: $Message" -ForegroundColor Yellow
}

# Function to print error and exit
function Write-ErrorAndExit {
    param (
        [string]$Message
    )
    Write-Host "ERROR: $Message" -ForegroundColor Red
    exit 1
}

# Check prerequisites
Write-Section "Checking Prerequisites"

# Check if AWS CLI is installed
if (-not (Get-Command aws -ErrorAction SilentlyContinue)) {
    Write-ErrorAndExit "AWS CLI is not installed. Please install it before proceeding."
}

# Check if Terraform is installed
if (-not (Get-Command terraform -ErrorAction SilentlyContinue)) {
    Write-ErrorAndExit "Terraform is not installed. Please install it before proceeding."
}

# Check if AWS credentials are configured
try {
    $null = aws sts get-caller-identity 2>$null
}
catch {
    Write-ErrorAndExit "AWS credentials are not configured. Please run 'aws configure' before proceeding."
}

Write-Host "All prerequisites are met!"

# Build Lambda function
Write-Section "Building Lambda Function"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = Split-Path -Parent $scriptDir
Set-Location $rootDir

Write-Host "Building Lambda function for Linux..."
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o bootstrap cmd/benchmark/main.go
if (-not (Test-Path bootstrap)) {
    Write-ErrorAndExit "Failed to build Lambda function"
}

Write-Host "Creating Lambda deployment package..."
Compress-Archive -Path bootstrap -DestinationPath lambda-function.zip -Force
if (-not (Test-Path lambda-function.zip)) {
    Write-ErrorAndExit "Failed to create Lambda deployment package"
}

Write-Host "Lambda function built successfully!"

# Deploy with Terraform
Write-Section "Deploying Infrastructure with Terraform"
Set-Location "$rootDir\deployments\terraform"

Write-Host "Initializing Terraform..."
terraform init

Write-Host "Planning deployment..."
terraform plan `
  -var="aws_region=$AWS_REGION" `
  -var="environment=$ENVIRONMENT" `
  -var="dynamodb_read_capacity=$DYNAMODB_READ_CAPACITY" `
  -var="dynamodb_write_capacity=$DYNAMODB_WRITE_CAPACITY"

Write-Host "Applying Terraform configuration..."
terraform apply -auto-approve `
  -var="aws_region=$AWS_REGION" `
  -var="environment=$ENVIRONMENT" `
  -var="dynamodb_read_capacity=$DYNAMODB_READ_CAPACITY" `
  -var="dynamodb_write_capacity=$DYNAMODB_WRITE_CAPACITY"

if (-not $?) {
    Write-ErrorAndExit "Terraform apply failed"
}

Write-Host "Infrastructure deployed successfully!"

# Upload Lambda function to S3
Write-Section "Uploading Lambda Function"

# Get the S3 bucket name from Terraform output
$BUCKET_NAME = terraform output -raw lambda_bucket_name
if (-not $BUCKET_NAME) {
    Write-ErrorAndExit "Failed to get S3 bucket name from Terraform output"
}

Write-Host "Uploading Lambda function to S3 bucket: $BUCKET_NAME..."
aws s3 cp ..\..\lambda-function.zip "s3://$BUCKET_NAME/lambda/lambda-function.zip"

Write-Host "Updating Lambda functions..."
$functions = terraform output -json lambda_function_names | ConvertFrom-Json
foreach ($func in $functions.PSObject.Properties.Name) {
    Write-Host "Updating function: lambda-gopher-benchmark-$func"
    aws lambda update-function-code `
        --function-name "lambda-gopher-benchmark-$func" `
        --s3-bucket "$BUCKET_NAME" `
        --s3-key "lambda/lambda-function.zip" `
        --publish
    
    # Wait for the update to complete
    aws lambda wait function-updated `
        --function-name "lambda-gopher-benchmark-$func"
}

Write-Host "Lambda functions updated successfully!"

# Get Lambda function URLs
Write-Section "Collecting Lambda Function URLs"

# Create a .env file to store URLs
Set-Content -Path "..\..\\.env" -Value "# Lambda Function URLs"

# Get URLs for each function
foreach ($func in $functions.PSObject.Properties.Name) {
    $FUNCTION_NAME = "lambda-gopher-benchmark-$func"
    try {
        $URL = aws lambda get-function-url-config --function-name "$FUNCTION_NAME" --query 'FunctionUrl' --output text 2>$null
        if ($URL) {
            Add-Content -Path "..\..\\.env" -Value "$($func.ToUpper())_FUNCTION_URL=$URL"
            Write-Host "Found $func`: $URL"
        } else {
            Write-Warning "Function URL not configured for $FUNCTION_NAME"
        }
    } catch {
        Write-Warning "Function URL not configured for $FUNCTION_NAME"
    }
}

# Add the main Lambda endpoint
$MAIN_FUNCTION = terraform output -raw main_lambda_function_name
if ($MAIN_FUNCTION) {
    try {
        $MAIN_URL = aws lambda get-function-url-config --function-name "$MAIN_FUNCTION" --query 'FunctionUrl' --output text 2>$null
        if ($MAIN_URL) {
            Add-Content -Path "..\..\\.env" -Value "LAMBDA_ENDPOINT=$MAIN_URL"
            Write-Host "Main Lambda endpoint: $MAIN_URL"
        } else {
            Write-Warning "Function URL not configured for $MAIN_FUNCTION"
        }
    } catch {
        Write-Warning "Function URL not configured for $MAIN_FUNCTION"
    }
}

Write-Host "Function URLs saved to .env file"

# All done!
Write-Section "Deployment Complete"
Write-Host "The Lambda Gopher Benchmark platform has been successfully deployed to AWS!"
Write-Host "To run benchmarks, use:"
Write-Host "  # Load environment variables (PowerShell):"
Write-Host "  Get-Content .env | ForEach-Object { if (`$_ -match '(.+)=(.+)') { `$env:`$matches[1] = `$matches[2] } }"
Write-Host "  go run cmd/runner/main.go --config configs/comparison_benchmark.json --lambda-endpoint `$env:LAMBDA_ENDPOINT --output results"
Write-Host ""
Write-Host "To clean up resources when you're done:"
Write-Host "  cd deployments/terraform; terraform destroy"

# Clean up local build artifacts
Remove-Item -Path "..\..\bootstrap" -Force
Write-Host "Deployment script completed successfully!"

# Keep the PowerShell window open
Read-Host -Prompt "Press Enter to exit" 
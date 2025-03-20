# Lambda Gopher Benchmark - AWS Resources Cleanup Script (PowerShell)
# This script destroys all AWS resources created by the benchmark to avoid additional charges

# Function to print section header
function Write-Section {
    param (
        [string]$Title
    )
    Write-Host "`n==== $Title ====`n" -ForegroundColor Green
}

# Function to print warning
function Write-WarningMessage {
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

# Request user confirmation
Write-Section "Confirmation"
Write-Host "ATTENTION: This operation will destroy ALL AWS resources created by Lambda Gopher Benchmark." -ForegroundColor Red
Write-Host "This action CANNOT be undone!" -ForegroundColor Red
Write-Host ""
$confirmation = Read-Host "Type 'CONFIRM' to proceed with resource destruction"

if ($confirmation -ne "CONFIRM") {
    Write-Host "Operation canceled by user."
    exit 0
}

# Change to project directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = Split-Path -Parent $scriptDir
Set-Location $rootDir

# Destroy resources using Terraform
Write-Section "Destroying Resources via Terraform"

# Extract AWS region from .env file if it exists
$AWS_REGION = "us-east-1"  # Default value
if (Test-Path .env) {
    $envContent = Get-Content .env
    $regionLine = $envContent | Where-Object { $_ -match "AWS_REGION=" }
    if ($regionLine) {
        $AWS_REGION = $regionLine -replace "AWS_REGION=", ""
    }
}

# Extract environment from .env file if it exists
$ENVIRONMENT = "prod"  # Default value
if (Test-Path .env) {
    $envContent = Get-Content .env
    $envLine = $envContent | Where-Object { $_ -match "ENVIRONMENT=" }
    if ($envLine) {
        $ENVIRONMENT = $envLine -replace "ENVIRONMENT=", ""
    }
}

Write-Host "Using AWS Region: $AWS_REGION"
Write-Host "Using Environment: $ENVIRONMENT"

Set-Location "$rootDir\deployments\terraform"

Write-Host "Initializing Terraform..."
terraform init

Write-Host "Destroying infrastructure..."
terraform destroy -auto-approve `
  -var="aws_region=$AWS_REGION" `
  -var="environment=$ENVIRONMENT"

if (-not $?) {
    Write-WarningMessage "Terraform destroy encountered some issues. Some resources might still exist."
}
else {
    Write-Host "Terraform destroy completed successfully!"
}

# Check remaining resources
Write-Section "Checking Remaining Resources"

Write-Host "Checking Lambda functions..."
$LAMBDA_FUNCTIONS = (aws lambda list-functions --region $AWS_REGION --query "Functions[?contains(FunctionName, 'lambda-gopher-benchmark')].FunctionName" --output text) -split "\s+"

if ($LAMBDA_FUNCTIONS -and $LAMBDA_FUNCTIONS[0]) {
    Write-WarningMessage "Some Lambda functions still exist. Trying to delete manually..."
    foreach ($func in $LAMBDA_FUNCTIONS) {
        if ($func) {
            Write-Host "Deleting Lambda function: $func"
            aws lambda delete-function --function-name $func --region $AWS_REGION
        }
    }
}

Write-Host "Checking DynamoDB tables..."
$DYNAMODB_TABLES = (aws dynamodb list-tables --region $AWS_REGION --query "TableNames[?contains(@, 'LambdaGopherBenchmark')]" --output text) -split "\s+"

if ($DYNAMODB_TABLES -and $DYNAMODB_TABLES[0]) {
    Write-WarningMessage "Some DynamoDB tables still exist. Trying to delete manually..."
    foreach ($table in $DYNAMODB_TABLES) {
        if ($table) {
            Write-Host "Deleting DynamoDB table: $table"
            aws dynamodb delete-table --table-name $table --region $AWS_REGION
        }
    }
}

Write-Host "Checking Timestream databases..."
try {
    $TIMESTREAM_DATABASES = (aws timestream-write list-databases --region $AWS_REGION --query "Databases[?contains(DatabaseName, 'LambdaGopherBenchmark')].DatabaseName" --output text 2>$null) -split "\s+"
    
    if ($TIMESTREAM_DATABASES -and $TIMESTREAM_DATABASES[0]) {
        Write-WarningMessage "Some Timestream databases still exist. Trying to delete manually..."
        foreach ($db in $TIMESTREAM_DATABASES) {
            if ($db) {
                Write-Host "Deleting Timestream tables in database $db..."
                try {
                    $TIMESTREAM_TABLES = (aws timestream-write list-tables --database-name $db --region $AWS_REGION --query "Tables[].TableName" --output text 2>$null) -split "\s+"
                    foreach ($table in $TIMESTREAM_TABLES) {
                        if ($table) {
                            Write-Host "Deleting Timestream table: $table"
                            aws timestream-write delete-table --database-name $db --table-name $table --region $AWS_REGION
                        }
                    }
                } catch {}
                
                Write-Host "Deleting Timestream database: $db"
                aws timestream-write delete-database --database-name $db --region $AWS_REGION
            }
        }
    }
} catch {}

# Check S3 buckets
Write-Host "Checking S3 buckets..."
$S3_BUCKETS = (aws s3api list-buckets --query "Buckets[?contains(Name, 'lambda-gopher-benchmark')].Name" --output text) -split "\s+"

if ($S3_BUCKETS -and $S3_BUCKETS[0]) {
    Write-WarningMessage "Some S3 buckets still exist. Trying to delete manually..."
    foreach ($bucket in $S3_BUCKETS) {
        if ($bucket) {
            Write-Host "Emptying and deleting S3 bucket: $bucket"
            aws s3 rm "s3://$bucket" --recursive
            aws s3api delete-bucket --bucket $bucket --region $AWS_REGION
        }
    }
}

# Clean up local files
Write-Section "Cleaning Up Local Files"
Write-Host "Removing .env file..."
if (Test-Path "$rootDir\.env") {
    Remove-Item -Path "$rootDir\.env" -Force
}

Write-Host "Removing terraform.tfstate files..."
Remove-Item -Path "terraform.tfstate*" -Force -ErrorAction SilentlyContinue

Write-Section "Cleanup Complete"
Write-Host "The AWS resources for Lambda Gopher Benchmark have been destroyed."
Write-Host "Check your AWS dashboard to ensure there are no remaining resources that could generate charges."

# Keep PowerShell window open
Read-Host -Prompt "Press Enter to exit" 
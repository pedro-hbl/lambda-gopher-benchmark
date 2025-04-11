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
$DYNAMODB_TABLES = (aws dynamodb list-tables --region $AWS_REGION --query "TableNames[?contains(@, 'LambdaGopherBenchmark') || contains(@, 'Transactions')]" --output text) -split "\s+"

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
    $TIMESTREAM_DATABASES = (aws timestream-write list-databases --region $AWS_REGION --query "Databases[?contains(DatabaseName, 'LambdaGopherBenchmark') || contains(DatabaseName, 'TransactionsDB')].DatabaseName" --output text 2>$null) -split "\s+"
    
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
} catch {
    Write-Host "No Timestream databases found or error checking Timestream"
}

Write-Host "Checking EC2 instances..."
try {
    $EC2_INSTANCES = (aws ec2 describe-instances --region $AWS_REGION --filters "Name=tag:Name,Values=immudb-server" "Name=instance-state-name,Values=running,pending,stopping,stopped" --query "Reservations[].Instances[].InstanceId" --output text 2>$null) -split "\s+"
    
    if ($EC2_INSTANCES -and $EC2_INSTANCES[0]) {
        Write-WarningMessage "Some ImmuDB EC2 instances still exist. Trying to terminate manually..."
        foreach ($instanceId in $EC2_INSTANCES) {
            if ($instanceId) {
                Write-Host "Terminating EC2 instance: $instanceId"
                aws ec2 terminate-instances --instance-ids $instanceId --region $AWS_REGION
            }
        }
    }
} catch {
    Write-Host "No ImmuDB EC2 instances found or error checking EC2 instances"
}

# Check CloudWatch dashboards
Write-Host "Checking CloudWatch dashboards..."
try {
    $CW_DASHBOARDS = (aws cloudwatch list-dashboards --region $AWS_REGION --query "DashboardEntries[?contains(DashboardName, 'lambda-gopher-benchmark')].DashboardName" --output text 2>$null) -split "\s+"
    
    if ($CW_DASHBOARDS -and $CW_DASHBOARDS[0]) {
        Write-WarningMessage "Some CloudWatch dashboards still exist. Trying to delete manually..."
        foreach ($dashboard in $CW_DASHBOARDS) {
            if ($dashboard) {
                Write-Host "Deleting CloudWatch dashboard: $dashboard"
                aws cloudwatch delete-dashboards --dashboard-names $dashboard --region $AWS_REGION
            }
        }
    }
} catch {
    Write-Host "No CloudWatch dashboards found or error checking CloudWatch"
}

# Check CloudWatch alarms
Write-Host "Checking CloudWatch alarms..."
try {
    $CW_ALARMS = (aws cloudwatch describe-alarms --region $AWS_REGION --query "MetricAlarms[?contains(AlarmName, 'lambda-gopher-benchmark')].AlarmName" --output text 2>$null) -split "\s+"
    
    if ($CW_ALARMS -and $CW_ALARMS[0]) {
        Write-WarningMessage "Some CloudWatch alarms still exist. Trying to delete manually..."
        foreach ($alarm in $CW_ALARMS) {
            if ($alarm) {
                Write-Host "Deleting CloudWatch alarm: $alarm"
                aws cloudwatch delete-alarms --alarm-names $alarm --region $AWS_REGION
            }
        }
    }
} catch {
    Write-Host "No CloudWatch alarms found or error checking CloudWatch"
}

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

# Check VPC resources
Write-Host "Checking VPC resources..."
try {
    # Find VPCs with benchmark tags
    $VPC_IDS = (aws ec2 describe-vpcs --region $AWS_REGION --filters "Name=tag:Name,Values=*immudb*" --query "Vpcs[].VpcId" --output text 2>$null) -split "\s+"
    
    if ($VPC_IDS -and $VPC_IDS[0]) {
        foreach ($vpcId in $VPC_IDS) {
            if ($vpcId) {
                # Delete route table associations
                Write-Host "Finding route tables for VPC: $vpcId"
                $ROUTE_TABLES = (aws ec2 describe-route-tables --region $AWS_REGION --filters "Name=vpc-id,Values=$vpcId" --query "RouteTables[].RouteTableId" --output text 2>$null) -split "\s+"
                
                foreach ($rtbId in $ROUTE_TABLES) {
                    if ($rtbId) {
                        # Get associations
                        $RTB_ASSOCS = (aws ec2 describe-route-tables --region $AWS_REGION --route-table-ids $rtbId --query "RouteTables[].Associations[].RouteTableAssociationId" --output text 2>$null) -split "\s+"
                        
                        foreach ($assocId in $RTB_ASSOCS) {
                            if ($assocId) {
                                Write-Host "Deleting route table association: $assocId"
                                aws ec2 disassociate-route-table --association-id $assocId --region $AWS_REGION
                            }
                        }
                        
                        # Delete route table if not the main one
                        $IS_MAIN = aws ec2 describe-route-tables --region $AWS_REGION --route-table-ids $rtbId --query "RouteTables[].Associations[].Main" --output text
                        if ($IS_MAIN -ne "True") {
                            Write-Host "Deleting route table: $rtbId"
                            aws ec2 delete-route-table --route-table-id $rtbId --region $AWS_REGION
                        }
                    }
                }
                
                # Delete internet gateway
                $IGW_IDS = (aws ec2 describe-internet-gateways --region $AWS_REGION --filters "Name=attachment.vpc-id,Values=$vpcId" --query "InternetGateways[].InternetGatewayId" --output text 2>$null) -split "\s+"
                
                foreach ($igwId in $IGW_IDS) {
                    if ($igwId) {
                        Write-Host "Detaching internet gateway: $igwId from VPC: $vpcId"
                        aws ec2 detach-internet-gateway --internet-gateway-id $igwId --vpc-id $vpcId --region $AWS_REGION
                        
                        Write-Host "Deleting internet gateway: $igwId"
                        aws ec2 delete-internet-gateway --internet-gateway-id $igwId --region $AWS_REGION
                    }
                }
                
                # Delete security groups (except default)
                $SG_IDS = (aws ec2 describe-security-groups --region $AWS_REGION --filters "Name=vpc-id,Values=$vpcId" "Name=group-name,Values=!default" --query "SecurityGroups[].GroupId" --output text 2>$null) -split "\s+"
                
                foreach ($sgId in $SG_IDS) {
                    if ($sgId) {
                        Write-Host "Deleting security group: $sgId"
                        aws ec2 delete-security-group --group-id $sgId --region $AWS_REGION
                    }
                }
                
                # Delete subnets
                $SUBNET_IDS = (aws ec2 describe-subnets --region $AWS_REGION --filters "Name=vpc-id,Values=$vpcId" --query "Subnets[].SubnetId" --output text 2>$null) -split "\s+"
                
                foreach ($subnetId in $SUBNET_IDS) {
                    if ($subnetId) {
                        Write-Host "Deleting subnet: $subnetId"
                        aws ec2 delete-subnet --subnet-id $subnetId --region $AWS_REGION
                    }
                }
                
                # Delete VPC
                Write-Host "Deleting VPC: $vpcId"
                aws ec2 delete-vpc --vpc-id $vpcId --region $AWS_REGION
            }
        }
    }
} catch {
    Write-Host "No VPC resources found or error checking VPC resources: $_"
}

# Check EC2 key pairs
Write-Host "Checking EC2 key pairs..."
try {
    $KEY_PAIRS = (aws ec2 describe-key-pairs --region $AWS_REGION --filters "Name=key-name,Values=*immudb*" --query "KeyPairs[].KeyName" --output text 2>$null) -split "\s+"
    
    if ($KEY_PAIRS -and $KEY_PAIRS[0]) {
        Write-WarningMessage "Some EC2 key pairs still exist. Trying to delete manually..."
        foreach ($keyName in $KEY_PAIRS) {
            if ($keyName) {
                Write-Host "Deleting EC2 key pair: $keyName"
                aws ec2 delete-key-pair --key-name $keyName --region $AWS_REGION
            }
        }
    }
} catch {
    Write-Host "No EC2 key pairs found or error checking key pairs"
}

# Check IAM roles and policies
Write-Host "Checking IAM roles and policies..."
try {
    $IAM_ROLES = (aws iam list-roles --query "Roles[?contains(RoleName, 'lambda-gopher-benchmark') || contains(RoleName, 'lambda_benchmark')].RoleName" --output text 2>$null) -split "\s+"
    
    if ($IAM_ROLES -and $IAM_ROLES[0]) {
        Write-WarningMessage "Some IAM roles still exist. Trying to delete manually..."
        foreach ($roleName in $IAM_ROLES) {
            if ($roleName) {
                # Detach all policies first
                $ATTACHED_POLICIES = (aws iam list-attached-role-policies --role-name $roleName --query "AttachedPolicies[].PolicyArn" --output text 2>$null) -split "\s+"
                foreach ($policyArn in $ATTACHED_POLICIES) {
                    if ($policyArn) {
                        Write-Host "Detaching policy $policyArn from role $roleName"
                        aws iam detach-role-policy --role-name $roleName --policy-arn $policyArn
                    }
                }
                
                Write-Host "Deleting IAM role: $roleName"
                aws iam delete-role --role-name $roleName
            }
        }
    }
    
    $IAM_POLICIES = (aws iam list-policies --scope Local --query "Policies[?contains(PolicyName, 'lambda-gopher-benchmark') || contains(PolicyName, 'lambda_benchmark')].Arn" --output text 2>$null) -split "\s+"
    
    if ($IAM_POLICIES -and $IAM_POLICIES[0]) {
        Write-WarningMessage "Some IAM policies still exist. Trying to delete manually..."
        foreach ($policyArn in $IAM_POLICIES) {
            if ($policyArn) {
                Write-Host "Deleting IAM policy: $policyArn"
                aws iam delete-policy --policy-arn $policyArn
            }
        }
    }
} catch {
    Write-Host "No IAM resources found or error checking IAM resources: $_"
}

# Clean up local files
Write-Section "Cleaning Up Local Files"
Write-Host "Removing .env file..."
if (Test-Path "$rootDir\.env") {
    Remove-Item -Path "$rootDir\.env" -Force
}

Write-Host "Removing terraform.tfstate files..."
Remove-Item -Path "terraform.tfstate*" -Force -ErrorAction SilentlyContinue

Write-Host "Removing build artifacts..."
if (Test-Path "$rootDir\bootstrap") {
    Remove-Item -Path "$rootDir\bootstrap" -Force
}
if (Test-Path "$rootDir\lambda-function.zip") {
    Remove-Item -Path "$rootDir\lambda-function.zip" -Force
}

Write-Section "Cleanup Complete"
Write-Host "The AWS resources for Lambda Gopher Benchmark have been destroyed."
Write-Host "Check your AWS dashboard to ensure there are no remaining resources that could generate charges."

# Keep PowerShell window open
Read-Host -Prompt "Press Enter to exit" 
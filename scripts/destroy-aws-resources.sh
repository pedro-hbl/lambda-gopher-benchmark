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
DYNAMODB_TABLES=$(aws dynamodb list-tables --region ${AWS_REGION} --query "TableNames[?contains(@, 'LambdaGopherBenchmark') || contains(@, 'Transactions')]" --output text)

if [ -n "$DYNAMODB_TABLES" ]; then
  warning "Some DynamoDB tables still exist. Trying to delete manually..."
  for table in $DYNAMODB_TABLES; do
    echo "Deleting DynamoDB table: $table"
    aws dynamodb delete-table --table-name $table --region ${AWS_REGION}
  done
fi

echo "Checking Timestream databases..."
TIMESTREAM_DATABASES=$(aws timestream-write list-databases --region ${AWS_REGION} --query "Databases[?contains(DatabaseName, 'LambdaGopherBenchmark') || contains(DatabaseName, 'TransactionsDB')].DatabaseName" --output text 2>/dev/null || echo "")

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

echo "Checking EC2 instances..."
EC2_INSTANCES=$(aws ec2 describe-instances --region ${AWS_REGION} --filters "Name=tag:Name,Values=immudb-server" "Name=instance-state-name,Values=running,pending,stopping,stopped" --query "Reservations[].Instances[].InstanceId" --output text 2>/dev/null || echo "")

if [ -n "$EC2_INSTANCES" ]; then
  warning "Some ImmuDB EC2 instances still exist. Trying to terminate manually..."
  for instanceId in $EC2_INSTANCES; do
    echo "Terminating EC2 instance: $instanceId"
    aws ec2 terminate-instances --instance-ids $instanceId --region ${AWS_REGION}
  done
fi

# Check CloudWatch dashboards
echo "Checking CloudWatch dashboards..."
CW_DASHBOARDS=$(aws cloudwatch list-dashboards --region ${AWS_REGION} --query "DashboardEntries[?contains(DashboardName, 'lambda-gopher-benchmark')].DashboardName" --output text 2>/dev/null || echo "")
    
if [ -n "$CW_DASHBOARDS" ]; then
  warning "Some CloudWatch dashboards still exist. Trying to delete manually..."
  for dashboard in $CW_DASHBOARDS; do
    echo "Deleting CloudWatch dashboard: $dashboard"
    aws cloudwatch delete-dashboards --dashboard-names $dashboard --region ${AWS_REGION}
  done
fi

# Check CloudWatch alarms
echo "Checking CloudWatch alarms..."
CW_ALARMS=$(aws cloudwatch describe-alarms --region ${AWS_REGION} --query "MetricAlarms[?contains(AlarmName, 'lambda-gopher-benchmark')].AlarmName" --output text 2>/dev/null || echo "")
    
if [ -n "$CW_ALARMS" ]; then
  warning "Some CloudWatch alarms still exist. Trying to delete manually..."
  for alarm in $CW_ALARMS; do
    echo "Deleting CloudWatch alarm: $alarm"
    aws cloudwatch delete-alarms --alarm-names $alarm --region ${AWS_REGION}
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

# Check VPC resources
echo "Checking VPC resources..."
VPC_IDS=$(aws ec2 describe-vpcs --region ${AWS_REGION} --filters "Name=tag:Name,Values=*immudb*" --query "Vpcs[].VpcId" --output text 2>/dev/null || echo "")
    
if [ -n "$VPC_IDS" ]; then
  for vpcId in $VPC_IDS; do
    # Delete route table associations
    echo "Finding route tables for VPC: $vpcId"
    ROUTE_TABLES=$(aws ec2 describe-route-tables --region ${AWS_REGION} --filters "Name=vpc-id,Values=$vpcId" --query "RouteTables[].RouteTableId" --output text 2>/dev/null || echo "")
    
    for rtbId in $ROUTE_TABLES; do
      # Get associations
      RTB_ASSOCS=$(aws ec2 describe-route-tables --region ${AWS_REGION} --route-table-ids $rtbId --query "RouteTables[].Associations[].RouteTableAssociationId" --output text 2>/dev/null || echo "")
      
      for assocId in $RTB_ASSOCS; do
        echo "Deleting route table association: $assocId"
        aws ec2 disassociate-route-table --association-id $assocId --region ${AWS_REGION}
      done
      
      # Delete route table if not the main one
      IS_MAIN=$(aws ec2 describe-route-tables --region ${AWS_REGION} --route-table-ids $rtbId --query "RouteTables[].Associations[].Main" --output text)
      if [ "$IS_MAIN" != "True" ]; then
        echo "Deleting route table: $rtbId"
        aws ec2 delete-route-table --route-table-id $rtbId --region ${AWS_REGION}
      fi
    done
    
    # Delete internet gateway
    IGW_IDS=$(aws ec2 describe-internet-gateways --region ${AWS_REGION} --filters "Name=attachment.vpc-id,Values=$vpcId" --query "InternetGateways[].InternetGatewayId" --output text 2>/dev/null || echo "")
    
    for igwId in $IGW_IDS; do
      echo "Detaching internet gateway: $igwId from VPC: $vpcId"
      aws ec2 detach-internet-gateway --internet-gateway-id $igwId --vpc-id $vpcId --region ${AWS_REGION}
      
      echo "Deleting internet gateway: $igwId"
      aws ec2 delete-internet-gateway --internet-gateway-id $igwId --region ${AWS_REGION}
    done
    
    # Delete security groups (except default)
    SG_IDS=$(aws ec2 describe-security-groups --region ${AWS_REGION} --filters "Name=vpc-id,Values=$vpcId" "Name=group-name,Values=!default" --query "SecurityGroups[].GroupId" --output text 2>/dev/null || echo "")
    
    for sgId in $SG_IDS; do
      echo "Deleting security group: $sgId"
      aws ec2 delete-security-group --group-id $sgId --region ${AWS_REGION}
    done
    
    # Delete subnets
    SUBNET_IDS=$(aws ec2 describe-subnets --region ${AWS_REGION} --filters "Name=vpc-id,Values=$vpcId" --query "Subnets[].SubnetId" --output text 2>/dev/null || echo "")
    
    for subnetId in $SUBNET_IDS; do
      echo "Deleting subnet: $subnetId"
      aws ec2 delete-subnet --subnet-id $subnetId --region ${AWS_REGION}
    done
    
    # Delete VPC
    echo "Deleting VPC: $vpcId"
    aws ec2 delete-vpc --vpc-id $vpcId --region ${AWS_REGION}
  done
fi

# Check EC2 key pairs
echo "Checking EC2 key pairs..."
KEY_PAIRS=$(aws ec2 describe-key-pairs --region ${AWS_REGION} --filters "Name=key-name,Values=*immudb*" --query "KeyPairs[].KeyName" --output text 2>/dev/null || echo "")

if [ -n "$KEY_PAIRS" ]; then
  warning "Some EC2 key pairs still exist. Trying to delete manually..."
  for keyName in $KEY_PAIRS; do
    echo "Deleting EC2 key pair: $keyName"
    aws ec2 delete-key-pair --key-name $keyName --region ${AWS_REGION}
  done
fi

# Check IAM roles and policies
echo "Checking IAM roles and policies..."
IAM_ROLES=$(aws iam list-roles --query "Roles[?contains(RoleName, 'lambda-gopher-benchmark') || contains(RoleName, 'lambda_benchmark')].RoleName" --output text 2>/dev/null || echo "")

if [ -n "$IAM_ROLES" ]; then
  warning "Some IAM roles still exist. Trying to delete manually..."
  for roleName in $IAM_ROLES; do
    # Detach all policies first
    ATTACHED_POLICIES=$(aws iam list-attached-role-policies --role-name $roleName --query "AttachedPolicies[].PolicyArn" --output text 2>/dev/null || echo "")
    for policyArn in $ATTACHED_POLICIES; do
      echo "Detaching policy $policyArn from role $roleName"
      aws iam detach-role-policy --role-name $roleName --policy-arn $policyArn
    done
    
    echo "Deleting IAM role: $roleName"
    aws iam delete-role --role-name $roleName
  done
fi

IAM_POLICIES=$(aws iam list-policies --scope Local --query "Policies[?contains(PolicyName, 'lambda-gopher-benchmark') || contains(PolicyName, 'lambda_benchmark')].Arn" --output text 2>/dev/null || echo "")

if [ -n "$IAM_POLICIES" ]; then
  warning "Some IAM policies still exist. Trying to delete manually..."
  for policyArn in $IAM_POLICIES; do
    echo "Deleting IAM policy: $policyArn"
    aws iam delete-policy --policy-arn $policyArn
  done
fi

# Clean up local files
section "Cleaning Up Local Files"
echo "Removing .env file..."
rm -f .env

echo "Removing terraform.tfstate files..."
rm -f terraform.tfstate*

echo "Removing build artifacts..."
rm -f ../bootstrap
rm -f ../lambda-function.zip

section "Cleanup Complete"
echo "The AWS resources for Lambda Gopher Benchmark have been destroyed."
echo "Check your AWS dashboard to ensure there are no remaining resources that could generate charges." 
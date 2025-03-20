#!/bin/bash

set -e

# Default values
AWS_REGION=${AWS_REGION:-"us-east-1"}
DYNAMODB_ENDPOINT=${DYNAMODB_ENDPOINT:-"http://localhost:8000"}
TABLE_NAME=${DB_TABLE_NAME:-"Transactions"}

echo "Setting up DynamoDB table: $TABLE_NAME"
echo "Using endpoint: $DYNAMODB_ENDPOINT"

# Configure AWS CLI to use the local DynamoDB
export AWS_ACCESS_KEY_ID="dummy"
export AWS_SECRET_ACCESS_KEY="dummy"
export AWS_DEFAULT_REGION=$AWS_REGION

# Check if table exists
if aws dynamodb describe-table --table-name $TABLE_NAME --endpoint-url $DYNAMODB_ENDPOINT &> /dev/null; then
  echo "Table $TABLE_NAME already exists. Deleting it first..."
  aws dynamodb delete-table --table-name $TABLE_NAME --endpoint-url $DYNAMODB_ENDPOINT
  echo "Waiting for table deletion..."
  aws dynamodb wait table-not-exists --table-name $TABLE_NAME --endpoint-url $DYNAMODB_ENDPOINT
fi

# Create the table
echo "Creating table $TABLE_NAME..."
aws dynamodb create-table \
  --table-name $TABLE_NAME \
  --attribute-definitions \
    AttributeName=AccountID,AttributeType=S \
    AttributeName=UUID,AttributeType=S \
    AttributeName=Timestamp,AttributeType=S \
  --key-schema \
    AttributeName=AccountID,KeyType=HASH \
    AttributeName=UUID,KeyType=RANGE \
  --global-secondary-indexes \
    "IndexName=TimestampIndex,KeySchema=[{AttributeName=AccountID,KeyType=HASH},{AttributeName=Timestamp,KeyType=RANGE}],Projection={ProjectionType=ALL},ProvisionedThroughput={ReadCapacityUnits=5,WriteCapacityUnits=5}" \
  --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
  --endpoint-url $DYNAMODB_ENDPOINT

# Wait for table to be created
echo "Waiting for table to become active..."
aws dynamodb wait table-exists --table-name $TABLE_NAME --endpoint-url $DYNAMODB_ENDPOINT

# Verify table is ready
aws dynamodb describe-table --table-name $TABLE_NAME --endpoint-url $DYNAMODB_ENDPOINT --query "Table.TableStatus"

echo "DynamoDB table $TABLE_NAME is ready for benchmarking." 
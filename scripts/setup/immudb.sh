#!/bin/bash

set -e

# Default values
IMMUDB_ADDRESS=${IMMUDB_ADDRESS:-"127.0.0.1"}
IMMUDB_PORT=${IMMUDB_PORT:-"3322"}
IMMUDB_USERNAME=${IMMUDB_USERNAME:-"immudb"}
IMMUDB_PASSWORD=${IMMUDB_PASSWORD:-"immudb"}
DB_NAME=${DB_NAME:-"defaultdb"}
TABLE_NAME=${DB_TABLE_NAME:-"Transactions"}

echo "Setting up ImmuDB database: $DB_NAME"
echo "Using ImmuDB at: $IMMUDB_ADDRESS:$IMMUDB_PORT"

# Wait for ImmuDB to be available
MAX_RETRIES=30
RETRY_INTERVAL=2

echo "Waiting for ImmuDB to be available..."
for i in $(seq 1 $MAX_RETRIES); do
  if nc -z $IMMUDB_ADDRESS $IMMUDB_PORT; then
    echo "ImmuDB is available!"
    break
  fi
  
  if [ $i -eq $MAX_RETRIES ]; then
    echo "Timeout waiting for ImmuDB"
    exit 1
  fi
  
  echo "Attempt $i/$MAX_RETRIES: ImmuDB not available yet, retrying in ${RETRY_INTERVAL}s..."
  sleep $RETRY_INTERVAL
done

# Use immuadmin and immudb client to set up the database
# This is a simple setup - in a real implementation, you would use the Go client
# to create the tables with proper indexes

echo "Setting up database $DB_NAME and table $TABLE_NAME..."

# Note: For a production implementation, use the ImmuDB Go client to:
# 1. Create the database if it doesn't exist
# 2. Create the table schema with proper indexes
# 3. Set up any additional configuration needed

echo "ImmuDB setup completed. The Go client will handle database and table creation during initialization." 
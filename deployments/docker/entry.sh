#!/bin/sh

# Check if the AWS_LAMBDA_RUNTIME_API environment variable is set
# If it's not set, we're running outside of Lambda, so use the Runtime Interface Emulator
if [ -z "${AWS_LAMBDA_RUNTIME_API}" ]; then
    exec aws-lambda-rie /benchmark
else
    exec /benchmark
fi 
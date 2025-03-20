# Lambda Gopher Benchmark Configuration Guide

This guide provides detailed instructions for creating and customizing benchmark configurations for the Lambda Gopher Benchmark platform.

## Table of Contents

1. [Introduction](#introduction)
2. [Configuration File Structure](#configuration-file-structure)
3. [Database Configurations](#database-configurations)
4. [Operation Types](#operation-types)
5. [Benchmark Parameters](#benchmark-parameters)
6. [Predefined Benchmarks](#predefined-benchmarks)
7. [Creating Custom Benchmarks](#creating-custom-benchmarks)
8. [Parameter Overrides](#parameter-overrides)
9. [Advanced Configuration](#advanced-configuration)
10. [Best Practices](#best-practices)

## Introduction

The Lambda Gopher Benchmark platform uses JSON configuration files to define benchmark scenarios. These configuration files specify:

- Which databases to benchmark (DynamoDB, ImmuDB, Timestream)
- Which operations to test (write, read, query, etc.)
- How many operations to perform
- What data sizes to use
- Various other parameters that affect benchmark performance

By creating custom benchmark configurations, you can tailor the benchmarking process to match your specific use cases and requirements.

## Configuration File Structure

A benchmark configuration file has the following structure:

```json
{
  "tests": [
    {
      "name": "test-name",
      "database": {
        "type": "database-type",
        "region": "aws-region",
        "table": "table-name"
      },
      "operation": {
        "type": "operation-type",
        "operations": 1000,
        "dataSize": 1024
      }
    }
  ]
}
```

### Key Components

- **tests**: An array of test configurations to run
- **name**: A descriptive name for the test
- **database**: Configuration for the database to benchmark
- **operation**: Configuration for the operation to perform

## Database Configurations

The platform supports the following database types:

### DynamoDB

```json
"database": {
  "type": "dynamodb",
  "region": "us-east-1",
  "table": "LambdaGopherBenchmark",
  "endpoint": "https://dynamodb.us-east-1.amazonaws.com"
}
```

Optional parameters:
- **endpoint**: Custom endpoint URL (useful for DynamoDB Local)
- **consistentRead**: Use consistent reads (boolean, default: false)

### ImmuDB

```json
"database": {
  "type": "immudb",
  "address": "immudb:3322",
  "database": "defaultdb",
  "username": "immudb",
  "password": "immudb"
}
```

Optional parameters:
- **verifiedRead**: Use cryptographic verification for reads (boolean, default: false)

### Timestream

```json
"database": {
  "type": "timestream",
  "region": "us-east-1",
  "database": "LambdaGopherBenchmark",
  "table": "DeviceReadings"
}
```

Optional parameters:
- **endpoint**: Custom endpoint URL

## Operation Types

The platform supports the following operation types:

### Write Operations

Single record writes:

```json
"operation": {
  "type": "write",
  "operations": 1000,
  "dataSize": 1024
}
```

Batch writes:

```json
"operation": {
  "type": "batch-write",
  "operations": 1000,
  "batchSize": 25,
  "dataSize": 1024
}
```

Conditional writes:

```json
"operation": {
  "type": "conditional-write",
  "operations": 1000,
  "dataSize": 1024,
  "condition": "attribute_not_exists(PK)"
}
```

### Read Operations

Single record reads:

```json
"operation": {
  "type": "read",
  "operations": 1000
}
```

Parallel reads:

```json
"operation": {
  "type": "read-parallel",
  "operations": 1000,
  "concurrency": 10
}
```

### Query Operations

Basic queries:

```json
"operation": {
  "type": "query",
  "operations": 100,
  "queryField": "accountId",
  "queryValue": "test-account"
}
```

Time range queries:

```json
"operation": {
  "type": "time-range-query",
  "operations": 100,
  "timeRangeMinutes": 60
}
```

## Benchmark Parameters

Common parameters that can be configured for benchmark operations:

### General Parameters

- **operations**: Number of operations to perform (integer)
- **dataSize**: Size of data in bytes for write operations (integer)
- **warmup**: Number of warmup operations to perform before measuring (integer)

### Concurrency Parameters

- **concurrency**: Number of parallel operations (integer)
- **batchSize**: Number of items per batch operation (integer)

### Data Generation Parameters

- **randomData**: Generate random data for each operation (boolean)
- **sequentialIds**: Use sequential IDs instead of random UUIDs (boolean)

### Time-Related Parameters

- **timeRangeMinutes**: Time range for time-range queries (integer)
- **timeoutSeconds**: Operation timeout in seconds (integer)

## Predefined Benchmarks

The platform includes several predefined benchmark configurations:

### DynamoDB Benchmark

Located at `configs/dynamodb_benchmark.json`, this configuration tests all supported DynamoDB operations with various parameters.

### ImmuDB Benchmark

Located at `configs/immudb_benchmark.json`, this configuration tests all supported ImmuDB operations, including verified reads.

### Timestream Benchmark

Located at `configs/timestream_benchmark.json`, this configuration tests all supported Timestream operations, focusing on time-series data.

### Comparison Benchmark

Located at `configs/comparison_benchmark.json`, this configuration compares all supported databases across equivalent operations.

## Creating Custom Benchmarks

To create a custom benchmark configuration:

1. Create a new JSON file in the `configs` directory
2. Define your tests array with the desired database and operation configurations
3. Save the file with a descriptive name (e.g., `custom_benchmark.json`)

### Example: Custom Comparison Benchmark

```json
{
  "tests": [
    {
      "name": "dynamodb-write-test",
      "database": {
        "type": "dynamodb",
        "region": "us-east-1",
        "table": "LambdaGopherBenchmark"
      },
      "operation": {
        "type": "write",
        "operations": 5000,
        "dataSize": 2048
      }
    },
    {
      "name": "immudb-write-test",
      "database": {
        "type": "immudb",
        "address": "immudb:3322",
        "database": "defaultdb"
      },
      "operation": {
        "type": "write",
        "operations": 5000,
        "dataSize": 2048
      }
    }
  ]
}
```

### Example: High-Volume Read Test

```json
{
  "tests": [
    {
      "name": "dynamodb-high-volume-read",
      "database": {
        "type": "dynamodb",
        "region": "us-east-1",
        "table": "LambdaGopherBenchmark",
        "consistentRead": true
      },
      "operation": {
        "type": "read-parallel",
        "operations": 10000,
        "concurrency": 50
      }
    }
  ]
}
```

## Parameter Overrides

When running benchmarks, you can override configuration parameters using command-line flags:

```bash
go run cmd/runner/main.go \
  --config configs/dynamodb_benchmark.json \
  --lambda-endpoint ${LAMBDA_ENDPOINT} \
  --output results \
  --custom-param "operations=5000" \
  --custom-param "dataSize=2048" \
  --custom-param "concurrency=20"
```

These overrides will apply to all tests in the configuration file, unless the test explicitly sets a different value.

## Advanced Configuration

### Multi-Region Testing

You can configure tests to run across multiple AWS regions:

```json
{
  "tests": [
    {
      "name": "dynamodb-us-east-1",
      "database": {
        "type": "dynamodb",
        "region": "us-east-1",
        "table": "LambdaGopherBenchmark"
      },
      "operation": {
        "type": "write",
        "operations": 1000
      }
    },
    {
      "name": "dynamodb-us-west-2",
      "database": {
        "type": "dynamodb",
        "region": "us-west-2",
        "table": "LambdaGopherBenchmark"
      },
      "operation": {
        "type": "write",
        "operations": 1000
      }
    }
  ]
}
```

### Progressive Load Testing

You can create configurations that progressively increase the load:

```json
{
  "tests": [
    {
      "name": "dynamodb-write-10-concurrent",
      "database": {
        "type": "dynamodb",
        "region": "us-east-1",
        "table": "LambdaGopherBenchmark"
      },
      "operation": {
        "type": "write",
        "operations": 1000,
        "concurrency": 10
      }
    },
    {
      "name": "dynamodb-write-20-concurrent",
      "database": {
        "type": "dynamodb",
        "region": "us-east-1",
        "table": "LambdaGopherBenchmark"
      },
      "operation": {
        "type": "write",
        "operations": 1000,
        "concurrency": 20
      }
    },
    {
      "name": "dynamodb-write-50-concurrent",
      "database": {
        "type": "dynamodb",
        "region": "us-east-1",
        "table": "LambdaGopherBenchmark"
      },
      "operation": {
        "type": "write",
        "operations": 1000,
        "concurrency": 50
      }
    }
  ]
}
```

### Data Size Variation

You can test with different data sizes:

```json
{
  "tests": [
    {
      "name": "dynamodb-write-1kb",
      "database": {
        "type": "dynamodb",
        "region": "us-east-1",
        "table": "LambdaGopherBenchmark"
      },
      "operation": {
        "type": "write",
        "operations": 1000,
        "dataSize": 1024
      }
    },
    {
      "name": "dynamodb-write-10kb",
      "database": {
        "type": "dynamodb",
        "region": "us-east-1",
        "table": "LambdaGopherBenchmark"
      },
      "operation": {
        "type": "write",
        "operations": 1000,
        "dataSize": 10240
      }
    },
    {
      "name": "dynamodb-write-100kb",
      "database": {
        "type": "dynamodb",
        "region": "us-east-1",
        "table": "LambdaGopherBenchmark"
      },
      "operation": {
        "type": "write",
        "operations": 1000,
        "dataSize": 102400
      }
    }
  ]
}
```

## Best Practices

### Naming Conventions

- Use descriptive names for your benchmark configurations (e.g., `high_throughput_benchmark.json`)
- Use descriptive names for your tests (e.g., `dynamodb-high-concurrency-write`)
- Include key parameters in the test name for clarity

### Performance Considerations

- Start with small operation counts and increase gradually
- Be mindful of AWS costs when running large benchmarks
- Use appropriate timeouts based on expected performance

### Reproducibility

- Document all parameter values used in your benchmarks
- Use consistent parameter values when comparing different databases
- Include AWS region information in your benchmark results

### AWS Resource Management

- Ensure your AWS account has appropriate permissions
- Monitor AWS resource usage during benchmarks
- Clean up resources after benchmarks complete

### Local Testing

Before running benchmarks in AWS, test your configurations locally:

```bash
# Run DynamoDB locally
docker run -p 8000:8000 amazon/dynamodb-local

# Create table
aws dynamodb create-table \
  --table-name LambdaGopherBenchmark \
  --attribute-definitions AttributeName=PK,AttributeType=S AttributeName=SK,AttributeType=S \
  --key-schema AttributeName=PK,KeyType=HASH AttributeName=SK,KeyType=RANGE \
  --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
  --endpoint-url http://localhost:8000

# Run benchmark with local endpoint
go run cmd/runner/main.go \
  --config configs/dynamodb_benchmark.json \
  --custom-param "endpoint=http://localhost:8000" \
  --output results/local
``` 
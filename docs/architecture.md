# Lambda Gopher Benchmark Architecture

This document provides a detailed overview of the Lambda Gopher Benchmark platform's architecture, including its components, patterns, and design principles.

## Table of Contents

1. [System Overview](#system-overview)
2. [Key Components](#key-components)
3. [Design Patterns](#design-patterns)
4. [Data Flow](#data-flow)
5. [Deployment Architecture](#deployment-architecture)
6. [Performance Considerations](#performance-considerations)
7. [Security Considerations](#security-considerations)
8. [Extensibility](#extensibility)

## System Overview

The Lambda Gopher Benchmark platform is designed to evaluate and compare the performance of various database systems when accessed from AWS Lambda functions. The architecture follows a modular, extensible approach that allows for easy addition of new database types, operations, and benchmark configurations.

### Core Principles

- **Standardization**: All tests run under consistent conditions
- **Reproducibility**: Tests can be run anywhere with consistent results
- **Extensibility**: Easy to add new databases or test scenarios
- **AWS-Centric**: Optimized for AWS Lambda environments
- **Data-Driven**: Configuration-based approach to defining benchmarks

## Key Components

The platform consists of several key components:

### 1. Lambda Function (Benchmark Execution)

Located in `cmd/benchmark/main.go`, this is the core component that executes benchmark operations. It is deployed as an AWS Lambda function and is responsible for:

- Receiving operation requests
- Connecting to specified databases
- Executing benchmark operations
- Collecting performance metrics
- Returning results

The Lambda function uses a unified architecture with adapters for different database types, allowing it to work with any supported database using a consistent interface.

### 2. Benchmark Runner

Located in `cmd/runner/main.go`, this is a client-side tool that:

- Reads benchmark configurations from JSON files
- Makes HTTP requests to the Lambda function
- Collects and aggregates results
- Saves results to output files
- Provides configuration overrides via command-line flags

The runner coordinates the execution of benchmark tests, handling parallelism, retries, and result collection.

### 3. Database Adapters

Located in `pkg/databases/`, these adapters provide a consistent interface for working with different database systems:

- **DynamoDB Adapter** (`pkg/databases/dynamodb/`): Implements operations for Amazon DynamoDB
- **ImmuDB Adapter** (`pkg/databases/immudb/`): Implements operations for ImmuDB
- **Timestream Adapter** (`pkg/databases/timestream/`): Implements operations for Amazon Timestream

Each adapter implements a common interface defined in `pkg/databases/database.go`, which includes methods like:

```go
type Database interface {
    Initialize(config map[string]interface{}) error
    Write(data *Data) error
    BatchWrite(data []*Data) error
    Read(key string) (*Data, error)
    Query(query *Query) ([]*Data, error)
    Close() error
}
```

### 4. Operation Strategies

Located in `pkg/operations/`, these define how different operations are executed:

- **Write Operations** (`pkg/operations/write.go`): Single and batch write operations
- **Read Operations** (`pkg/operations/read.go`): Single and parallel read operations
- **Query Operations** (`pkg/operations/query.go`): Various query types

Operation strategies are implemented using the Strategy pattern, allowing different operation types to be executed with any database adapter.

### 5. Metrics Collection

Located in `pkg/metrics/`, this component is responsible for:

- Collecting timing information
- Calculating statistics (min, max, average, percentiles)
- Measuring throughput
- Tracking error rates

Metrics are collected during benchmark execution and returned as part of the benchmark results.

### 6. Visualizer

Located in `cmd/visualizer/main.go`, this tool processes benchmark results and generates visualizations including:

- Text tables
- CSV reports
- Charts (PNG format)
- HTML reports

The visualizer helps users analyze and compare benchmark results across different databases and operations.

### 7. Configuration and Utilities

- **Configuration** (`pkg/config/`): Handles loading and parsing benchmark configurations
- **Utilities** (`pkg/utils/`): Common utility functions used across the platform

## Design Patterns

The platform employs several design patterns to achieve its architectural goals:

### Adapter Pattern

Used for database interfaces, allowing different database systems to be used with a consistent interface. This pattern:

- Decouples the benchmark logic from specific database implementations
- Makes it easy to add new database types
- Allows for consistent operations across different databases

### Strategy Pattern

Used for operation types, allowing different operations to be executed with any database. This pattern:

- Separates operation logic from database-specific code
- Enables mixing and matching of operations and databases
- Simplifies adding new operation types

### Factory Pattern

Used for creating database instances and operation strategies based on configuration. This pattern:

- Centralizes the creation of objects
- Handles configuration-based instantiation
- Provides a consistent way to create components

### Command Pattern

Used in the benchmark runner to execute operations via Lambda. This pattern:

- Encapsulates requests as objects
- Allows for parameterization of operations
- Enables features like retries and parallel execution

## Data Flow

The data flow through the system follows this sequence:

1. **Configuration Loading**:
   - The benchmark runner loads a JSON configuration file
   - Configuration parameters are parsed and validated

2. **Test Execution**:
   - For each test in the configuration:
     - The runner prepares the request parameters
     - The request is sent to the Lambda function
     - The Lambda function executes the requested operation
     - Results are returned to the runner

3. **Lambda Function Execution**:
   - The Lambda function receives a request
   - It creates the appropriate database adapter
   - It executes the requested operation
   - It collects performance metrics
   - It returns the results

4. **Result Collection**:
   - The runner collects results from each test
   - Results are aggregated and processed
   - Results are saved to output files

5. **Visualization**:
   - The visualizer loads benchmark results
   - It processes the results to extract metrics
   - It generates visualizations based on the results

## Deployment Architecture

The platform is designed for deployment in AWS, with the following components:

### AWS Components

1. **Lambda Functions**:
   - Core benchmark function deployed across multiple configurations
   - One Lambda function per database type for isolation
   - Lambda function URLs for HTTP access

2. **DynamoDB Tables**:
   - Tables for storing benchmark data
   - Configured with appropriate capacity

3. **Timestream Database and Tables**:
   - For time-series benchmark data
   - Configured with appropriate retention

4. **S3 Bucket**:
   - For storing Lambda deployment packages
   - For storing benchmark results (optional)

5. **CloudWatch**:
   - For Lambda function logs and metrics
   - For monitoring benchmark execution

### Infrastructure as Code

The platform uses Terraform for infrastructure deployment, with configurations in `deployments/terraform/`. The Terraform configuration includes:

- Lambda function definitions
- IAM roles and policies
- DynamoDB tables
- Timestream database and tables
- S3 buckets
- CloudWatch dashboards

## Performance Considerations

The platform is designed with performance in mind:

### Lambda Optimization

- Functions are optimized for cold start performance
- Connection pooling is used where applicable
- Resources are properly initialized and cleaned up

### Database Considerations

- Connection pooling for database access
- Proper error handling and retries
- Configurable timeouts

### Benchmark Runner Optimization

- Parallel execution of benchmark tests
- Efficient result collection and aggregation
- Minimal overhead in measurement

## Security Considerations

The platform includes several security features:

### AWS Security

- IAM roles with least privilege
- Lambda function isolation
- Secure configuration of database access

### Data Security

- No sensitive data in benchmark tests
- Option for encryption of benchmark results
- Secure handling of database credentials

## Extensibility

The platform is designed to be extensible in several ways:

### Adding New Database Types

To add a new database type:

1. Create a new adapter in `pkg/databases/`
2. Implement the Database interface
3. Add the new database type to the factory
4. Update configuration handling

### Adding New Operation Types

To add a new operation type:

1. Create a new strategy in `pkg/operations/`
2. Implement the Operation interface
3. Add the new operation type to the factory
4. Update configuration handling

### Enhancing Visualization

To enhance the visualizer:

1. Add new visualization formats
2. Improve existing visualizations
3. Add new metrics or analysis methods

This extensible architecture ensures that the platform can evolve to meet new requirements and support additional database systems and operation types. 
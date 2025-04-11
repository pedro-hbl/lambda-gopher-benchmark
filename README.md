# Lambda Gopher Benchmark

A comprehensive benchmarking platform for testing database performance in AWS Lambda environments. The platform allows you to compare different database systems, including DynamoDB, Timestream, and ImmuDB, to help you make informed decisions for your serverless applications.

## Features

- **Cross-Database Comparison**: Benchmark multiple database types (DynamoDB, Timestream, ImmuDB) with standardized metrics
- **Configurable Operations**: Test various database operations (read, write, query, batch operations)
- **Lambda Integration**: Execute benchmarks directly in AWS Lambda environments with various memory configurations
- **Comprehensive Visualization**: Generate detailed reports with tables, charts, and CSV exports
- **Automation**: Fully automated deployment and cleanup of AWS resources, including ImmuDB on EC2
- **Extensible Architecture**: Easily add new database types and operations
- **Cross-Platform Support**: Works on Windows, macOS, and Linux

## Overview

Lambda Gopher Benchmark is a toolkit designed to evaluate and compare the performance of various database systems when accessed from AWS Lambda functions. The platform supports:

- **Multiple Database Systems**:
  - Amazon DynamoDB
  - ImmuDB
  - Amazon Timestream

- **Various Operations**:
  - Single writes
  - Batch writes
  - Single reads
  - Parallel reads
  - Queries
  - Time range queries
  - Conditional writes

- **Performance Metrics**:
  - Latency (min, max, average, percentiles)
  - Throughput
  - Error rate
  - Cost estimation

## Getting Started

### Prerequisites

- Go 1.21 or higher
- AWS CLI configured with appropriate permissions
- Terraform (for AWS deployment)

### Quick Start

1. **Clone the repository**

```bash
git clone https://github.com/yourusername/lambda-gopher-benchmark.git
cd lambda-gopher-benchmark
```

2. **Deploy to AWS**

```bash
# On Linux/macOS
./scripts/deploy-aws.sh

# On Windows
.\scripts\deploy-aws.ps1
```

3. **Run a benchmark**

```bash
# Load environment variables
source .env  # Linux/macOS
# Or for PowerShell
Get-Content .env | ForEach-Object { if ($_ -match '(.+)=(.+)') { $env:$matches[1] = $matches[2] } }

# Run the benchmark
go run cmd/runner/main.go --config configs/comparison_benchmark.json --lambda-endpoint $LAMBDA_ENDPOINT --output results/aws
```

4. **Visualize results**

```bash
go run cmd/visualizer/main.go --input results/aws --output visualizations/aws
```

5. **Clean up resources**

```bash
# On Linux/macOS
./scripts/destroy-aws-resources.sh

# On Windows
.\scripts\destroy-aws-resources.ps1
```

## AWS Deployment Architecture

The platform creates the following AWS resources:

- Lambda functions for each database and operation type
- DynamoDB table with GSI for queries
- Timestream database and table
- EC2 t2.micro instance running ImmuDB
- S3 bucket for Lambda function code
- CloudWatch dashboards for monitoring

## Documentation

For detailed documentation, see:

- [Architecture Overview](docs/architecture.md)
- [Deployment Guide](docs/deployment-guide.md)
- [Benchmark Configuration](docs/benchmark-configuration.md)
- [Visualization Guide](docs/visualization.md)

## Results and Visualization

The platform generates several types of outputs:

- **Text Reports**: Summary tables showing key metrics
- **CSV Files**: Detailed results for further analysis
- **Charts**: Visual comparison of database performance
- **Markdown Reports**: Easy-to-share documentation of results

Example visualization:

```
| Database  | Operation | Avg Latency (ms) | Throughput (ops/s) |
|-----------|-----------|------------------|-------------------|
| DynamoDB  | Write     | 12.45            | 80.32             |
| ImmuDB    | Write     | 8.76             | 114.15            |
| Timestream| Write     | 15.32            | 65.27             |
```

## Extending the Benchmark

See [Architecture Overview](docs/architecture.md) for details on how to extend the platform with:

- New database types
- Custom operations
- Additional metrics
- Alternative deployment targets

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
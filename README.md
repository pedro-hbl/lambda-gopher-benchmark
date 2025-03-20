# Lambda Gopher Benchmark

A comprehensive benchmarking platform for comparing database performance in AWS Lambda environments.

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

- Go 1.21+
- Docker (for local testing)
- AWS CLI v2
- Terraform 1.0+

### Quick Start

1. **Clone the repository**:
   ```bash
   git clone https://github.com/yourusername/lambda-gopher-benchmark.git
   cd lambda-gopher-benchmark
   ```

2. **Deploy to AWS**:
   ```bash
   chmod +x scripts/deploy-aws.sh
   ./scripts/deploy-aws.sh
   ```
   For Windows PowerShell:
   ```powershell
   .\scripts\deploy-aws.ps1
   ```

3. **Run a benchmark**:
   ```bash
   source .env
   go run cmd/runner/main.go --config configs/comparison_benchmark.json --lambda-endpoint ${LAMBDA_ENDPOINT} --output results
   ```

4. **Visualize results**:
   ```bash
   go run cmd/visualizer/main.go --input results --output visualizations
   ```

## Documentation

Detailed documentation is available in the `/docs` directory:

- [Deployment Guide](docs/deployment-guide.md) - Comprehensive instructions for deploying and running benchmarks
- [Benchmark Configuration](docs/benchmark-configuration.md) - Guide to creating custom benchmark configurations
- [Visualization Guide](docs/visualization.md) - Instructions for visualizing benchmark results
- [Architecture Overview](docs/architecture.md) - Technical details about the platform's design

## Running Benchmarks from Configuration Files

The benchmark runner supports JSON configuration files that allow you to define complex benchmark scenarios:

```bash
go run cmd/runner/main.go --config configs/dynamodb_benchmark.json --lambda-endpoint ${LAMBDA_ENDPOINT} --output results
```

### Configuration File Structure

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
        "operations": 1000,
        "dataSize": 1024
      }
    },
    {
      "name": "dynamodb-read-test",
      "database": {
        "type": "dynamodb",
        "region": "us-east-1",
        "table": "LambdaGopherBenchmark"
      },
      "operation": {
        "type": "read",
        "operations": 1000
      }
    }
  ]
}
```

## Customizing Benchmarks

You can override configuration parameters when running benchmarks:

```bash
go run cmd/runner/main.go --config configs/dynamodb_benchmark.json --lambda-endpoint ${LAMBDA_ENDPOINT} --output results --custom-param "operations=5000" --custom-param "dataSize=2048"
```

## Visualizing Results

The platform includes a visualizer for analyzing benchmark results:

```bash
# Visualize a single result file
go run cmd/visualizer/main.go --input results/result_20240601_120000.json --output visualizations

# Compare multiple results in a directory
go run cmd/visualizer/main.go --input-dir results --output visualizations/comparison --format html
```

Available visualization formats:
- HTML (`--format html`)
- CSV (`--format csv`)
- PNG (`--format png`)
- JSON (`--format json`)

## Running Sample Visualizations

Try the included sample visualizations:

```bash
# For Linux/macOS
./examples/run_sample_visualization.sh

# For Windows
.\examples\run_sample_visualization.ps1
```

## Architecture

The Lambda Gopher Benchmark platform consists of several components:

1. **Lambda Functions** (deployments/terraform): AWS Lambda functions that execute database operations
2. **Benchmark Runner** (cmd/runner): Go application for configuring and running benchmarks
3. **Visualizer** (cmd/visualizer): Tool for visualizing and comparing benchmark results
4. **Configuration Files** (configs): JSON files defining benchmark scenarios
5. **Infrastructure as Code** (deployments/terraform): Terraform configurations for AWS deployment

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
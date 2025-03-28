# Lambda Gopher Benchmark Visualization Guide

This guide provides detailed instructions for visualizing and analyzing benchmark results generated by the Lambda Gopher Benchmark platform.

## Table of Contents

1. [Introduction](#introduction)
2. [Visualizer Tool Overview](#visualizer-tool-overview)
3. [Basic Usage](#basic-usage)
4. [Output Formats](#output-formats)
5. [Visualization Options](#visualization-options)
6. [Sample Visualizations](#sample-visualizations)
7. [Advanced Usage](#advanced-usage)
8. [Programmatic Access](#programmatic-access)
9. [Troubleshooting](#troubleshooting)

## Introduction

The Lambda Gopher Benchmark platform includes a powerful visualization tool designed to help you analyze and compare benchmark results across different database systems, operations, and configurations. The visualizer can generate various output formats including tables, charts, and raw data for further processing.

## Visualizer Tool Overview

The visualizer tool is a Go application located at `cmd/visualizer/main.go`. It processes benchmark result files generated by the benchmark runner and produces various visualizations to help you understand the performance characteristics of the databases under test.

Key capabilities:
- Compare performance across different database systems
- Analyze latency, throughput, and error rates
- Generate visual representations of benchmark results
- Export results in various formats for further analysis

## Basic Usage

### Visualizing a Single Result File

```bash
go run cmd/visualizer/main.go \
  --input results/result_20240601_120000.json \
  --output visualizations
```

### Comparing Multiple Result Files

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations/comparison
```

### Running Sample Visualizations

The platform includes sample visualizations that you can run to see the capabilities of the visualizer:

```bash
# For Linux/macOS
./examples/run_sample_visualization.sh

# For Windows
.\examples\run_sample_visualization.ps1
```

## Output Formats

The visualizer can generate output in several formats:

### HTML Format

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations \
  --format html
```

HTML output provides interactive charts and tables that allow you to:
- Toggle between different metrics
- Sort data by various criteria
- Zoom in on specific parts of charts
- Export data to CSV

### CSV Format

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations \
  --format csv
```

CSV output is useful for importing data into spreadsheet applications or other data analysis tools.

### PNG Format

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations \
  --format png
```

PNG output generates static chart images suitable for inclusion in reports or presentations.

### JSON Format

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations \
  --format json
```

JSON output provides structured data for programmatic processing or custom visualization.

## Visualization Options

### Filtering by Database Type

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations \
  --databases dynamodb,immudb
```

This command generates visualizations only for DynamoDB and ImmuDB results, excluding other database types.

### Filtering by Operation Type

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations \
  --operations write,read
```

This command generates visualizations only for write and read operations, excluding other operation types.

### Focusing on Specific Metrics

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations \
  --metrics latency,throughput
```

This command generates visualizations focusing on latency and throughput metrics.

### Grouping Results

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations \
  --group-by database
```

This command groups results by database type, making it easier to compare performance across different databases.

Other grouping options include:
- `--group-by operation`: Group by operation type
- `--group-by dataSize`: Group by data size
- `--group-by concurrency`: Group by concurrency level

## Sample Visualizations

The platform includes sample result files that demonstrate the expected format and can be used to test the visualization capabilities:

### Database Comparison Chart

This visualization compares the performance of different database systems across various operations:

```bash
go run cmd/visualizer/main.go \
  --input examples/sample_results/comparison_results.json \
  --output visualizations \
  --format png
```

### Operation Performance Chart

This visualization shows the performance of different operations for a specific database:

```bash
go run cmd/visualizer/main.go \
  --input examples/sample_results/dynamodb_results.json \
  --output visualizations \
  --group-by operation \
  --format png
```

### Latency Distribution Chart

This visualization shows the distribution of latency values for different operations:

```bash
go run cmd/visualizer/main.go \
  --input examples/sample_results/latency_distribution.json \
  --output visualizations \
  --metrics latency \
  --format png
```

## Advanced Usage

### Custom Chart Titles and Labels

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations \
  --title "DynamoDB vs ImmuDB Performance Comparison" \
  --x-label "Operation Type" \
  --y-label "Latency (ms)"
```

### Setting Chart Dimensions

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations \
  --width 1200 \
  --height 800
```

### Generating Reports

```bash
go run cmd/visualizer/main.go \
  --input-dir results \
  --output visualizations \
  --generate-report
```

This command generates a comprehensive report including tables, charts, and analysis of the benchmark results.

## Programmatic Access

The visualizer can also be used programmatically in your Go applications:

```go
package main

import (
    "fmt"
    "github.com/yourusername/lambda-gopher-benchmark/pkg/visualizer"
)

func main() {
    // Create a new visualizer
    v := visualizer.New()
    
    // Load benchmark results
    err := v.LoadFromFile("results/result_20240601_120000.json")
    if err != nil {
        fmt.Printf("Error loading benchmark results: %v\n", err)
        return
    }
    
    // Generate visualizations
    err = v.GenerateCharts("visualizations", visualizer.FormatPNG)
    if err != nil {
        fmt.Printf("Error generating visualizations: %v\n", err)
        return
    }
    
    fmt.Println("Visualizations generated successfully!")
}
```

## Troubleshooting

### Missing Dependencies

If you encounter errors related to missing dependencies, make sure you have installed all required Go packages:

```bash
go get github.com/olekukonko/tablewriter
go get github.com/wcharczuk/go-chart/v2
```

### Invalid Result Format

If the visualizer fails to process a result file, check that the file follows the expected format:

```json
{
  "id": "benchmark_id",
  "timestamp": "2024-06-01T12:00:00Z",
  "config": { ... },
  "results": [
    {
      "database": "dynamodb",
      "operation": "write",
      "metrics": {
        "latency": {
          "min": 10.5,
          "max": 100.2,
          "avg": 45.6,
          "p50": 42.1,
          "p90": 75.3,
          "p99": 95.7
        },
        "throughput": 156.2,
        "errorRate": 0.01
      },
      "parameters": { ... }
    },
    ...
  ]
}
```

### No Output Generated

If no output is generated, check that:
1. The input file exists and is readable
2. The output directory exists and is writable
3. The benchmark result file contains valid data

```bash
# Check if the input file exists
ls -l results/result_20240601_120000.json

# Create the output directory if it doesn't exist
mkdir -p visualizations

# Validate the JSON format of the result file
jq . results/result_20240601_120000.json
```

### Low-Quality Charts

If the generated charts have low quality or are difficult to read:
1. Try increasing the dimensions with `--width` and `--height` flags
2. Use a different format like SVG for better scalability
3. Reduce the number of data points being visualized by filtering or grouping 
#!/bin/bash

# Sample visualization script for Lambda Gopher Benchmark
# This script demonstrates how to use the visualizer with sample data

# Create output directory if it doesn't exist
if [ ! -d "sample_visualizations" ]; then
  echo "Creating sample_visualizations directory..."
  mkdir -p sample_visualizations
fi

# Check if sample results directory exists
if [ ! -d "examples/sample_results" ]; then
  echo "Error: Sample results directory not found!"
  echo "Please ensure the examples/sample_results directory exists and contains sample files."
  exit 1
fi

echo "===== Running Sample Visualization ====="
go run cmd/visualizer/main.go --input examples/sample_results/ --output sample_visualizations/

echo ""
echo "===== Sample Visualization Complete ====="
echo "Output files are available in the sample_visualizations/ directory"
echo ""
echo "This demonstration used sample files from examples/sample_results/"
echo "In a real benchmark run, you would typically use actual result files from your benchmarks."
echo ""
echo "Try exploring different visualization options:"
echo "- go run cmd/visualizer/main.go --input examples/sample_results/ --metric latency"
echo "- go run cmd/visualizer/main.go --input examples/sample_results/ --group-by operation"

# For Windows: Pause at the end to keep the command window open
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
  echo ""
  echo "Press any key to continue..."
  read -n1 -s
fi 
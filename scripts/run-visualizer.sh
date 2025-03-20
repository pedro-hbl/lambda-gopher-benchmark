#!/bin/bash

# Lambda Gopher Benchmark Visualizer Script
# This script demonstrates various ways to use the visualization tool

# Create output directories
mkdir -p visualizations/throughput
mkdir -p visualizations/latency
mkdir -p visualizations/database_focus
mkdir -p visualizations/operation_focus

echo "===== Running Basic Visualizations ====="
go run cmd/visualizer/main.go --input results/ --output visualizations/ 

echo ""
echo "===== Running Latency Analysis ====="
go run cmd/visualizer/main.go --input results/ --output visualizations/latency/ --metric latency

echo ""
echo "===== Running Database-Centric Analysis ====="
go run cmd/visualizer/main.go --input results/ --output visualizations/database_focus/ --group-by database

echo ""
echo "===== Running Operation-Centric Analysis ====="
go run cmd/visualizer/main.go --input results/ --output visualizations/operation_focus/ --group-by operation

echo ""
echo "===== Visualizations Complete ====="
echo "Output files are available in the visualizations/ directory"
echo ""
echo "Visualization Types Generated:"
echo "- Text summaries (*.txt)"
echo "- CSV reports (*.csv)"
echo "- Charts (*.png)"
echo ""
echo "Use the following to view your results:"
echo "- Text files: Any text editor"
echo "- CSV files: Any spreadsheet application"
echo "- PNG files: Any image viewer" 
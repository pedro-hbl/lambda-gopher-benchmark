# PowerShell script for running sample visualization in Windows
# Lambda Gopher Benchmark

# Create output directory if it doesn't exist
if (!(Test-Path -Path "sample_visualizations")) {
    Write-Host "Creating sample_visualizations directory..."
    New-Item -ItemType Directory -Path "sample_visualizations" | Out-Null
}

# Check if sample results directory exists
if (!(Test-Path -Path "examples\sample_results")) {
    Write-Host "Error: Sample results directory not found!" -ForegroundColor Red
    Write-Host "Please ensure the examples\sample_results directory exists and contains sample files."
    exit 1
}

Write-Host "===== Running Sample Visualization =====" -ForegroundColor Green
go run cmd/visualizer/main.go --input examples/sample_results/ --output sample_visualizations/

Write-Host ""
Write-Host "===== Sample Visualization Complete =====" -ForegroundColor Green
Write-Host "Output files are available in the sample_visualizations/ directory"
Write-Host ""
Write-Host "This demonstration used sample files from examples\sample_results\"
Write-Host "In a real benchmark run, you would typically use actual result files from your benchmarks."
Write-Host ""
Write-Host "Try exploring different visualization options:" -ForegroundColor Cyan
Write-Host "- go run cmd/visualizer/main.go --input examples/sample_results/ --metric latency"
Write-Host "- go run cmd/visualizer/main.go --input examples/sample_results/ --group-by operation"

Write-Host ""
Write-Host "Press any key to continue..."
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown") 
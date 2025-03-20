package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

// BenchmarkResult holds the result of a benchmark run
type BenchmarkResult struct {
	OperationType          string                 `json:"operationType"`
	DatabaseType           string                 `json:"databaseType"`
	Success                bool                   `json:"success"`
	ErrorMessage           string                 `json:"errorMessage,omitempty"`
	ItemsProcessed         int                    `json:"itemsProcessed"`
	TotalDurationNs        int64                  `json:"totalDurationNs"`
	AvgOperationDurationNs int64                  `json:"avgOperationDurationNs"`
	Throughput             float64                `json:"throughput"`
	Metrics                map[string]interface{} `json:"metrics,omitempty"`
	Timestamp              time.Time              `json:"timestamp"`
}

// ResultsCollection holds all loaded benchmark results
type ResultsCollection struct {
	Results        []BenchmarkResult
	DatabaseTypes  []string
	OperationTypes []string
}

// Filter options for results
type FilterOptions struct {
	Databases  []string
	Operations []string
	StartTime  time.Time
	EndTime    time.Time
}

// OutputOptions for visualization
type OutputOptions struct {
	Format     string // text, csv, chart
	OutputDir  string
	GroupBy    string // database, operation
	MetricType string // throughput, latency
}

// Command line flags
var (
	inputPath  = flag.String("input", "", "Path to benchmark results directory or specific result file")
	outputPath = flag.String("output", "visualizations", "Directory to store visualization outputs")
	format     = flag.String("format", "all", "Output format: text, csv, chart, all")
	groupBy    = flag.String("group-by", "database", "Group results by: database, operation")
	metricType = flag.String("metric", "throughput", "Metric to visualize: throughput, latency")
	databases  = flag.String("databases", "", "Comma-separated list of databases to include")
	operations = flag.String("operations", "", "Comma-separated list of operations to include")
	startDate  = flag.String("start-date", "", "Start date filter (YYYY-MM-DD)")
	endDate    = flag.String("end-date", "", "End date filter (YYYY-MM-DD)")
)

func main() {
	flag.Parse()

	if *inputPath == "" {
		log.Fatal("Input path is required. Use --input flag to specify the directory or file.")
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputPath, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Parse filter options
	filterOpts := parseFilterOptions()

	// Load benchmark results
	resultsCollection, err := loadBenchmarkResults(*inputPath, filterOpts)
	if err != nil {
		log.Fatalf("Failed to load benchmark results: %v", err)
	}

	if len(resultsCollection.Results) == 0 {
		log.Fatal("No benchmark results found.")
	}

	fmt.Printf("Loaded %d benchmark results.\n", len(resultsCollection.Results))
	fmt.Printf("Database types: %s\n", strings.Join(resultsCollection.DatabaseTypes, ", "))
	fmt.Printf("Operation types: %s\n", strings.Join(resultsCollection.OperationTypes, ", "))

	// Output options
	outputOpts := OutputOptions{
		Format:     *format,
		OutputDir:  *outputPath,
		GroupBy:    *groupBy,
		MetricType: *metricType,
	}

	// Generate visualizations
	if *format == "text" || *format == "all" {
		generateTextSummary(resultsCollection, outputOpts)
	}

	if *format == "csv" || *format == "all" {
		generateCSVReport(resultsCollection, outputOpts)
	}

	if *format == "chart" || *format == "all" {
		generateCharts(resultsCollection, outputOpts)
	}
}

// parseFilterOptions parses command line flags into filter options
func parseFilterOptions() FilterOptions {
	var filterOpts FilterOptions

	// Parse databases
	if *databases != "" {
		filterOpts.Databases = strings.Split(*databases, ",")
	}

	// Parse operations
	if *operations != "" {
		filterOpts.Operations = strings.Split(*operations, ",")
	}

	// Parse date range
	if *startDate != "" {
		startTime, err := time.Parse("2006-01-02", *startDate)
		if err != nil {
			log.Fatalf("Invalid start date format. Use YYYY-MM-DD: %v", err)
		}
		filterOpts.StartTime = startTime
	}

	if *endDate != "" {
		endTime, err := time.Parse("2006-01-02", *endDate)
		if err != nil {
			log.Fatalf("Invalid end date format. Use YYYY-MM-DD: %v", err)
		}
		// Set to end of day
		filterOpts.EndTime = endTime.Add(24*time.Hour - time.Second)
	}

	return filterOpts
}

// loadBenchmarkResults loads benchmark results from a file or directory
func loadBenchmarkResults(path string, filterOpts FilterOptions) (ResultsCollection, error) {
	collection := ResultsCollection{
		Results:        []BenchmarkResult{},
		DatabaseTypes:  []string{},
		OperationTypes: []string{},
	}

	// Set of unique database and operation types
	dbTypes := make(map[string]bool)
	opTypes := make(map[string]bool)

	// Check if path is a directory or file
	fileInfo, err := os.Stat(path)
	if err != nil {
		return collection, fmt.Errorf("failed to stat path: %v", err)
	}

	if fileInfo.IsDir() {
		// Walk directory and process all JSON files
		err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
				result, err := loadResultFromFile(filePath)
				if err != nil {
					fmt.Printf("Warning: Skipping file %s: %v\n", filePath, err)
					return nil
				}

				// Apply filters
				if shouldIncludeResult(result, filterOpts) {
					collection.Results = append(collection.Results, result)
					dbTypes[result.DatabaseType] = true
					opTypes[result.OperationType] = true
				}
			}
			return nil
		})
		if err != nil {
			return collection, fmt.Errorf("failed to walk directory: %v", err)
		}
	} else {
		// Process single file
		result, err := loadResultFromFile(path)
		if err != nil {
			return collection, fmt.Errorf("failed to load result file: %v", err)
		}

		// Apply filters
		if shouldIncludeResult(result, filterOpts) {
			collection.Results = append(collection.Results, result)
			dbTypes[result.DatabaseType] = true
			opTypes[result.OperationType] = true
		}
	}

	// Convert maps to slices
	for dbType := range dbTypes {
		collection.DatabaseTypes = append(collection.DatabaseTypes, dbType)
	}
	sort.Strings(collection.DatabaseTypes)

	for opType := range opTypes {
		collection.OperationTypes = append(collection.OperationTypes, opType)
	}
	sort.Strings(collection.OperationTypes)

	return collection, nil
}

// loadResultFromFile loads a benchmark result from a file
func loadResultFromFile(filePath string) (BenchmarkResult, error) {
	var result BenchmarkResult

	file, err := os.Open(filePath)
	if err != nil {
		return result, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return result, fmt.Errorf("failed to read file: %v", err)
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return result, nil
}

// shouldIncludeResult checks if a result should be included based on filters
func shouldIncludeResult(result BenchmarkResult, filterOpts FilterOptions) bool {
	// Filter by database
	if len(filterOpts.Databases) > 0 {
		found := false
		for _, db := range filterOpts.Databases {
			if result.DatabaseType == db {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by operation
	if len(filterOpts.Operations) > 0 {
		found := false
		for _, op := range filterOpts.Operations {
			if result.OperationType == op {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by time range
	if !filterOpts.StartTime.IsZero() && result.Timestamp.Before(filterOpts.StartTime) {
		return false
	}

	if !filterOpts.EndTime.IsZero() && result.Timestamp.After(filterOpts.EndTime) {
		return false
	}

	return true
}

// generateTextSummary generates a text summary of the benchmark results
func generateTextSummary(collection ResultsCollection, opts OutputOptions) {
	fmt.Println("\n=== Benchmark Results Summary ===")

	// Group results by database or operation
	groupedResults := groupResults(collection, opts.GroupBy)

	// Create a table
	table := tablewriter.NewWriter(os.Stdout)

	// Set header based on grouping
	if opts.GroupBy == "database" {
		headers := []string{"Database"}
		for _, op := range collection.OperationTypes {
			if opts.MetricType == "throughput" {
				headers = append(headers, fmt.Sprintf("%s (ops/sec)", op))
			} else {
				headers = append(headers, fmt.Sprintf("%s (ms)", op))
			}
		}
		table.SetHeader(headers)
	} else {
		headers := []string{"Operation"}
		for _, db := range collection.DatabaseTypes {
			if opts.MetricType == "throughput" {
				headers = append(headers, fmt.Sprintf("%s (ops/sec)", db))
			} else {
				headers = append(headers, fmt.Sprintf("%s (ms)", db))
			}
		}
		table.SetHeader(headers)
	}

	// Add rows
	for groupName, results := range groupedResults {
		row := []string{groupName}

		var sortedKeys []string
		if opts.GroupBy == "database" {
			sortedKeys = collection.OperationTypes
		} else {
			sortedKeys = collection.DatabaseTypes
		}

		for _, key := range sortedKeys {
			if val, ok := results[key]; ok {
				if opts.MetricType == "throughput" {
					row = append(row, fmt.Sprintf("%.2f", val))
				} else {
					// Convert nanoseconds to milliseconds
					latencyMs := val / 1000000
					row = append(row, fmt.Sprintf("%.2f", latencyMs))
				}
			} else {
				row = append(row, "N/A")
			}
		}

		table.Append(row)
	}

	table.Render()

	// Save to file
	outputFile := filepath.Join(opts.OutputDir, fmt.Sprintf("summary_%s_%s.txt", opts.GroupBy, opts.MetricType))
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Warning: Failed to create summary file: %v\n", err)
		return
	}
	defer file.Close()

	tableString := table.RenderFormat(tablewriter.FormatMarkdown)
	file.WriteString("# Benchmark Results Summary\n\n")
	file.WriteString(fmt.Sprintf("Grouped by: %s\n", opts.GroupBy))
	file.WriteString(fmt.Sprintf("Metric: %s\n\n", opts.MetricType))
	file.WriteString(tableString)

	fmt.Printf("Text summary saved to: %s\n", outputFile)
}

// generateCSVReport generates a CSV report of the benchmark results
func generateCSVReport(collection ResultsCollection, opts OutputOptions) {
	outputFile := filepath.Join(opts.OutputDir, fmt.Sprintf("benchmark_results_%s_%s.csv", opts.GroupBy, opts.MetricType))
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Warning: Failed to create CSV file: %v\n", err)
		return
	}
	defer file.Close()

	// Group results by database or operation
	groupedResults := groupResults(collection, opts.GroupBy)

	// Write CSV header
	var header string
	if opts.GroupBy == "database" {
		header = "Database"
		for _, op := range collection.OperationTypes {
			header += fmt.Sprintf(",%s", op)
		}
	} else {
		header = "Operation"
		for _, db := range collection.DatabaseTypes {
			header += fmt.Sprintf(",%s", db)
		}
	}
	file.WriteString(header + "\n")

	// Write CSV rows
	for groupName, results := range groupedResults {
		row := groupName

		var sortedKeys []string
		if opts.GroupBy == "database" {
			sortedKeys = collection.OperationTypes
		} else {
			sortedKeys = collection.DatabaseTypes
		}

		for _, key := range sortedKeys {
			if val, ok := results[key]; ok {
				if opts.MetricType == "throughput" {
					row += fmt.Sprintf(",%.2f", val)
				} else {
					// Convert nanoseconds to milliseconds
					latencyMs := val / 1000000
					row += fmt.Sprintf(",%.2f", latencyMs)
				}
			} else {
				row += ",N/A"
			}
		}

		file.WriteString(row + "\n")
	}

	fmt.Printf("CSV report saved to: %s\n", outputFile)
}

// generateCharts generates charts of the benchmark results
func generateCharts(collection ResultsCollection, opts OutputOptions) {
	if opts.GroupBy == "database" {
		// Generate one chart per database comparing operations
		for _, dbType := range collection.DatabaseTypes {
			generateDatabaseChart(collection, dbType, opts)
		}

		// Generate comparison chart across all databases
		generateComparisonChart(collection, opts)
	} else {
		// Generate one chart per operation comparing databases
		for _, opType := range collection.OperationTypes {
			generateOperationChart(collection, opType, opts)
		}
	}
}

// generateDatabaseChart generates a chart for a specific database
func generateDatabaseChart(collection ResultsCollection, dbType string, opts OutputOptions) {
	// Filter results for this database
	var dbResults []BenchmarkResult
	for _, result := range collection.Results {
		if result.DatabaseType == dbType {
			dbResults = append(dbResults, result)
		}
	}

	if len(dbResults) == 0 {
		return
	}

	// Group results by operation
	opData := make(map[string]float64)
	for _, result := range dbResults {
		if opts.MetricType == "throughput" {
			opData[result.OperationType] = result.Throughput
		} else {
			// Convert nanoseconds to milliseconds
			opData[result.OperationType] = float64(result.AvgOperationDurationNs) / 1000000
		}
	}

	// Create bar chart
	var bars []chart.Value
	for op, value := range opData {
		bars = append(bars, chart.Value{
			Label: op,
			Value: value,
		})
	}

	// Sort bars by label for consistency
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].Label < bars[j].Label
	})

	// Create chart
	barChart := chart.BarChart{
		Title: fmt.Sprintf("%s - %s by Operation Type", dbType, strings.Title(opts.MetricType)),
		Background: chart.Style{
			Padding: chart.Box{
				Top:    40,
				Left:   20,
				Right:  20,
				Bottom: 20,
			},
		},
		Width:  800,
		Height: 400,
		Bars:   bars,
	}

	// Set formatting on y-axis
	if opts.MetricType == "latency" {
		barChart.YAxis.ValueFormatter = func(v interface{}) string {
			if vf, isFloat := v.(float64); isFloat {
				return fmt.Sprintf("%.2f ms", vf)
			}
			return ""
		}
	} else {
		barChart.YAxis.ValueFormatter = func(v interface{}) string {
			if vf, isFloat := v.(float64); isFloat {
				return fmt.Sprintf("%.2f ops/sec", vf)
			}
			return ""
		}
	}

	// Save chart to file
	outputFile := filepath.Join(opts.OutputDir, fmt.Sprintf("%s_%s_chart.png", dbType, opts.MetricType))
	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Warning: Failed to create chart file: %v\n", err)
		return
	}
	defer f.Close()

	if err := barChart.Render(chart.PNG, f); err != nil {
		fmt.Printf("Warning: Failed to render chart: %v\n", err)
		return
	}

	fmt.Printf("Chart for %s saved to: %s\n", dbType, outputFile)
}

// generateOperationChart generates a chart for a specific operation
func generateOperationChart(collection ResultsCollection, opType string, opts OutputOptions) {
	// Filter results for this operation
	var opResults []BenchmarkResult
	for _, result := range collection.Results {
		if result.OperationType == opType {
			opResults = append(opResults, result)
		}
	}

	if len(opResults) == 0 {
		return
	}

	// Group results by database
	dbData := make(map[string]float64)
	for _, result := range opResults {
		if opts.MetricType == "throughput" {
			dbData[result.DatabaseType] = result.Throughput
		} else {
			// Convert nanoseconds to milliseconds
			dbData[result.DatabaseType] = float64(result.AvgOperationDurationNs) / 1000000
		}
	}

	// Create bar chart
	var bars []chart.Value
	for db, value := range dbData {
		bars = append(bars, chart.Value{
			Label: db,
			Value: value,
		})
	}

	// Sort bars by label for consistency
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].Label < bars[j].Label
	})

	// Create chart
	barChart := chart.BarChart{
		Title: fmt.Sprintf("%s - %s by Database Type", opType, strings.Title(opts.MetricType)),
		Background: chart.Style{
			Padding: chart.Box{
				Top:    40,
				Left:   20,
				Right:  20,
				Bottom: 20,
			},
		},
		Width:  800,
		Height: 400,
		Bars:   bars,
	}

	// Set formatting on y-axis
	if opts.MetricType == "latency" {
		barChart.YAxis.ValueFormatter = func(v interface{}) string {
			if vf, isFloat := v.(float64); isFloat {
				return fmt.Sprintf("%.2f ms", vf)
			}
			return ""
		}
	} else {
		barChart.YAxis.ValueFormatter = func(v interface{}) string {
			if vf, isFloat := v.(float64); isFloat {
				return fmt.Sprintf("%.2f ops/sec", vf)
			}
			return ""
		}
	}

	// Save chart to file
	outputFile := filepath.Join(opts.OutputDir, fmt.Sprintf("%s_%s_chart.png", opType, opts.MetricType))
	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Warning: Failed to create chart file: %v\n", err)
		return
	}
	defer f.Close()

	if err := barChart.Render(chart.PNG, f); err != nil {
		fmt.Printf("Warning: Failed to render chart: %v\n", err)
		return
	}

	fmt.Printf("Chart for %s saved to: %s\n", opType, outputFile)
}

// generateComparisonChart generates a comparison chart across all databases
func generateComparisonChart(collection ResultsCollection, opts OutputOptions) {
	// Only generate for throughput
	if opts.MetricType != "throughput" {
		return
	}

	// Group by database and operation
	dbOpData := make(map[string]map[string]float64)

	for _, result := range collection.Results {
		if _, ok := dbOpData[result.DatabaseType]; !ok {
			dbOpData[result.DatabaseType] = make(map[string]float64)
		}

		dbOpData[result.DatabaseType][result.OperationType] = result.Throughput
	}

	// Generate multi-series bar chart with go-chart
	series := []chart.Series{}

	// Different colors for each database
	colors := []drawing.Color{
		{R: 77, G: 184, B: 255, A: 255},  // Blue
		{R: 250, G: 134, B: 94, A: 255},  // Orange
		{R: 165, G: 235, B: 91, A: 255},  // Green
		{R: 252, G: 201, B: 100, A: 255}, // Yellow
		{R: 208, G: 134, B: 255, A: 255}, // Purple
	}

	// Create separate bar series for each database
	colorIndex := 0
	for _, dbType := range collection.DatabaseTypes {
		if colorIndex >= len(colors) {
			colorIndex = 0
		}

		var bars []chart.Value
		for _, opType := range collection.OperationTypes {
			if value, ok := dbOpData[dbType][opType]; ok {
				bars = append(bars, chart.Value{
					Label: opType,
					Value: value,
					Style: chart.Style{
						FillColor:   colors[colorIndex],
						StrokeColor: colors[colorIndex].WithAlpha(255),
						StrokeWidth: 0,
					},
				})
			}
		}

		series = append(series, chart.BarSeries{
			Name:  dbType,
			Bars:  bars,
			Style: chart.Style{FillColor: colors[colorIndex]},
		})

		colorIndex++
	}

	// Output file
	outputFile := filepath.Join(opts.OutputDir, "database_comparison_chart.png")
	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Warning: Failed to create comparison chart file: %v\n", err)
		return
	}
	defer f.Close()

	// Create a legend
	graph := chart.Chart{
		Title: "Database Performance Comparison - Throughput (ops/sec)",
		Background: chart.Style{
			Padding: chart.Box{
				Top:    50,
				Left:   20,
				Right:  20,
				Bottom: 30,
			},
		},
		Width:  1000,
		Height: 500,
		Series: series,
	}

	// Render chart
	if err := graph.Render(chart.PNG, f); err != nil {
		fmt.Printf("Warning: Failed to render comparison chart: %v\n", err)
		return
	}

	fmt.Printf("Database comparison chart saved to: %s\n", outputFile)
}

// groupResults groups benchmark results by database or operation
func groupResults(collection ResultsCollection, groupBy string) map[string]map[string]float64 {
	groupedResults := make(map[string]map[string]float64)

	if groupBy == "database" {
		// Group by database type
		for _, result := range collection.Results {
			if result.Success {
				if _, ok := groupedResults[result.DatabaseType]; !ok {
					groupedResults[result.DatabaseType] = make(map[string]float64)
				}

				if *metricType == "throughput" {
					groupedResults[result.DatabaseType][result.OperationType] = result.Throughput
				} else {
					groupedResults[result.DatabaseType][result.OperationType] = float64(result.AvgOperationDurationNs)
				}
			}
		}
	} else {
		// Group by operation type
		for _, result := range collection.Results {
			if result.Success {
				if _, ok := groupedResults[result.OperationType]; !ok {
					groupedResults[result.OperationType] = make(map[string]float64)
				}

				if *metricType == "throughput" {
					groupedResults[result.OperationType][result.DatabaseType] = result.Throughput
				} else {
					groupedResults[result.OperationType][result.DatabaseType] = float64(result.AvgOperationDurationNs)
				}
			}
		}
	}

	return groupedResults
}

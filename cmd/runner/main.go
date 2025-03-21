package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// BenchmarkConfig holds the configuration for a benchmark run
type BenchmarkConfig struct {
	DatabaseType  string                 `json:"databaseType"`
	OperationType string                 `json:"operationType"`
	Parameters    map[string]interface{} `json:"parameters"`
}

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

// BenchmarkDefinition represents a benchmark configuration file
type BenchmarkDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tests       []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Database    struct {
			Type   string                 `json:"type"`
			Config map[string]interface{} `json:"config"`
		} `json:"database"`
		Operation struct {
			Type        string                 `json:"type"`
			Count       int                    `json:"count"`
			Data        map[string]interface{} `json:"data"`
			BatchSize   int                    `json:"batchSize,omitempty"`
			Concurrency int                    `json:"concurrency,omitempty"`
		} `json:"operation"`
	} `json:"tests"`
}

// Command line flags
var (
	lambdaEndpoint = flag.String("lambda-endpoint", "", "Lambda function endpoint URL")
	databases      = flag.String("database", "dynamodb", "Comma-separated list of databases to benchmark")
	operations     = flag.String("operations", "read-sequential,read-parallel,write,write-batch,query", "Comma-separated list of operations to benchmark")
	concurrency    = flag.Int("concurrency", 10, "Concurrency level for parallel operations")
	itemCount      = flag.Int("items", 100, "Number of items to process")
	dataSize       = flag.Int("data-size", 1024, "Size of data in bytes")
	outputDir      = flag.String("output", "", "Directory to store result files")
	runAll         = flag.Bool("all", false, "Run all databases and operations")
	verbose        = flag.Bool("verbose", false, "Enable verbose output")
	configFile     = flag.String("config", "", "Path to benchmark configuration file")
)

var availableDatabases = []string{
	"dynamodb",
	"immudb",
	"timestream",
}

// Map of database types to their specific function URLs
var functionURLs = make(map[string]string)

func main() {
	// Parse command line flags
	flag.Parse()

	// Set up logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime)

	// If config file is specified, use that
	if *configFile != "" {
		runBenchmarkFromConfigFile(*configFile)
		return
	}

	// Otherwise, validate required flags
	if *lambdaEndpoint == "" && *databases == "" {
		log.Fatal("Either --lambda-endpoint, --database flag, or --config file must be provided")
	}

	// Get Lambda endpoint from flag or environment variable
	if *lambdaEndpoint == "" {
		*lambdaEndpoint = os.Getenv("LAMBDA_ENDPOINT")
		if *lambdaEndpoint == "" {
			log.Fatalf("Lambda endpoint not specified. Use --lambda-endpoint flag or LAMBDA_ENDPOINT environment variable")
		}
	}

	// Get output directory from flag or environment variable
	if *outputDir == "" {
		*outputDir = os.Getenv("RESULTS_DIR")
		if *outputDir == "" {
			*outputDir = "./results"
		}
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Initialize function URLs map for different database types
	functionURLs = make(map[string]string)

	// For DynamoDB benchmarks
	dynamoDBFunctionURL := os.Getenv("DYNAMODB_FUNCTION_URL")
	if dynamoDBFunctionURL != "" {
		functionURLs["dynamodb"] = dynamoDBFunctionURL
	}

	// For ImmuDB benchmarks
	immuDBFunctionURL := os.Getenv("IMMUDB_FUNCTION_URL")
	if immuDBFunctionURL != "" {
		functionURLs["immudb"] = immuDBFunctionURL
	}

	// For Timestream benchmarks
	timestreamFunctionURL := os.Getenv("TIMESTREAM_FUNCTION_URL")
	if timestreamFunctionURL != "" {
		functionURLs["timestream"] = timestreamFunctionURL
	}

	// Parse database and operation lists
	var dbList, opList []string
	if *runAll {
		dbList = []string{"dynamodb", "immudb", "timestream"}
		opList = []string{"read", "read-parallel", "write", "batch-write", "query"}
	} else {
		dbList = strings.Split(*databases, ",")
		opList = strings.Split(*operations, ",")
	}

	// Run benchmarks
	for _, db := range dbList {
		for _, op := range opList {
			// Use database-specific endpoint if available
			endpoint := *lambdaEndpoint
			if specificURL, ok := functionURLs[db]; ok && specificURL != "" {
				endpoint = specificURL
			}
			runBenchmarkWithEndpoint(db, op, endpoint, nil)
		}
	}

	log.Println("All benchmarks completed!")
}

// runBenchmarkWithEndpoint runs a single benchmark with a specific endpoint
func runBenchmarkWithEndpoint(dbType, opType, endpoint string, customParams map[string]interface{}) {
	log.Printf("Running benchmark: %s - %s using endpoint %s", dbType, opType, endpoint)

	// Configure the benchmark
	config := BenchmarkConfig{
		DatabaseType:  dbType,
		OperationType: opType,
		Parameters: map[string]interface{}{
			"concurrency":    *concurrency,
			"itemCount":      *itemCount,
			"dataSize":       *dataSize,
			"accountId":      "benchmark-account",
			"consistentRead": true,
			"collectMetrics": true,
		},
	}

	// Override with custom parameters if provided
	if customParams != nil {
		for k, v := range customParams {
			config.Parameters[k] = v
		}
	}

	// Additional parameters based on operation type if not already set
	switch opType {
	case "batch-write":
		if _, ok := config.Parameters["batchSize"]; !ok {
			config.Parameters["batchSize"] = 25
		}
	case "query":
		if _, ok := config.Parameters["limit"]; !ok {
			config.Parameters["limit"] = int64(100)
		}
		if _, ok := config.Parameters["startTime"]; !ok {
			config.Parameters["startTime"] = time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
		}
		if _, ok := config.Parameters["endTime"]; !ok {
			config.Parameters["endTime"] = time.Now().Format(time.RFC3339)
		}
	}

	// Convert config to JSON
	jsonData, err := json.Marshal(config)
	if err != nil {
		log.Fatalf("Failed to marshal config to JSON: %v", err)
	}

	if *verbose {
		log.Printf("Request payload: %s", string(jsonData))
	}

	// Invoke Lambda function
	resp, err := http.Post(endpoint+"/2015-03-31/functions/function/invocations", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Failed to invoke Lambda function: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	if *verbose {
		log.Printf("Response: %s", string(body))
	}

	// Parse result
	var result BenchmarkResult
	if err := json.Unmarshal(body, &result); err != nil {
		log.Fatalf("Failed to parse result: %v", err)
	}

	// Add timestamp
	result.Timestamp = time.Now()

	// Save result to file
	saveResult(dbType, opType, &result)

	// Print summary
	printSummary(&result)
}

// runBenchmarkFromConfigFile runs benchmarks defined in a configuration file
func runBenchmarkFromConfigFile(filePath string) {
	log.Printf("Loading benchmark configuration from file: %s", filePath)

	// Read the configuration file
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read configuration file: %v", err)
	}

	// Replace environment variables in the configuration
	configStr := string(data)
	envVarPattern := regexp.MustCompile(`\${([A-Za-z0-9_]+)}`)
	configStr = envVarPattern.ReplaceAllStringFunc(configStr, func(match string) string {
		// Extract environment variable name (without ${ and })
		envVarName := match[2 : len(match)-1]
		envValue := os.Getenv(envVarName)
		if envValue == "" {
			log.Printf("Warning: Environment variable %s not set", envVarName)
			return match // Keep the original placeholder if env var is not set
		}
		return envValue
	})

	// Parse the configuration
	var benchmarkDef BenchmarkDefinition
	if err := json.Unmarshal([]byte(configStr), &benchmarkDef); err != nil {
		log.Fatalf("Failed to parse configuration file: %v", err)
	}

	log.Printf("Running benchmark: %s - %s", benchmarkDef.ID, benchmarkDef.Name)
	log.Printf("Description: %s", benchmarkDef.Description)
	log.Printf("Found %d tests to run", len(benchmarkDef.Tests))

	// Get output directory
	if *outputDir == "" {
		*outputDir = os.Getenv("RESULTS_DIR")
		if *outputDir == "" {
			*outputDir = "./results"
		}
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Get Lambda endpoint
	if *lambdaEndpoint == "" {
		*lambdaEndpoint = os.Getenv("LAMBDA_ENDPOINT")
		if *lambdaEndpoint == "" {
			log.Fatalf("Lambda endpoint not specified. Use --lambda-endpoint flag or LAMBDA_ENDPOINT environment variable")
		}
	}

	// Run each test
	for _, test := range benchmarkDef.Tests {
		log.Printf("Running test: %s - %s", test.ID, test.Name)

		// Create custom parameters from the test definition
		params := make(map[string]interface{})

		// Add database config
		for k, v := range test.Database.Config {
			params["db."+k] = v
		}

		// Add operation parameters
		params["itemCount"] = test.Operation.Count
		for k, v := range test.Operation.Data {
			params[k] = v
		}

		// Add optional parameters if present
		if test.Operation.BatchSize > 0 {
			params["batchSize"] = test.Operation.BatchSize
		}
		if test.Operation.Concurrency > 0 {
			params["concurrency"] = test.Operation.Concurrency
		}

		// Get database-specific endpoint if available
		endpoint := *lambdaEndpoint
		if specificURL, ok := functionURLs[test.Database.Type]; ok && specificURL != "" {
			endpoint = specificURL
		}

		// Run the benchmark with the configured parameters and specific endpoint
		runBenchmarkWithEndpoint(test.Database.Type, test.Operation.Type, endpoint, params)
	}

	log.Printf("Completed all tests for benchmark: %s", benchmarkDef.ID)
}

// TODO: This function is not currently used directly but kept for future implementation of standalone benchmark runs
func runBenchmark(dbType, opType string, customParams map[string]interface{}) {
	// Get database-specific endpoint if available
	endpoint := *lambdaEndpoint
	if specificURL, ok := functionURLs[dbType]; ok && specificURL != "" {
		endpoint = specificURL
	}
	runBenchmarkWithEndpoint(dbType, opType, endpoint, customParams)
}

func saveResult(dbType, opType string, result *BenchmarkResult) {
	// Create filename
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s-%s.json", dbType, opType, timestamp)
	filepath := filepath.Join(*outputDir, filename)

	// Marshal result to JSON with indentation for readability
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal result to JSON: %v", err)
		return
	}

	// Write to file
	if err := os.WriteFile(filepath, jsonData, 0644); err != nil {
		log.Printf("Failed to write result to file: %v", err)
		return
	}

	log.Printf("Result saved to %s", filepath)
}

func printSummary(result *BenchmarkResult) {
	if !result.Success {
		log.Printf("Benchmark failed: %s", result.ErrorMessage)
		return
	}

	log.Printf("==== Benchmark Summary ====")
	log.Printf("Database:    %s", result.DatabaseType)
	log.Printf("Operation:   %s", result.OperationType)
	log.Printf("Items:       %d", result.ItemsProcessed)
	log.Printf("Total Time:  %.2f ms", float64(result.TotalDurationNs)/1e6)
	log.Printf("Avg Time:    %.2f ms", float64(result.AvgOperationDurationNs)/1e6)
	log.Printf("Throughput:  %.2f ops/sec", result.Throughput)
	log.Printf("==========================")
}

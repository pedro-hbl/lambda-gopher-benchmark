package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pedro-hbl/lambda-gopher-benchmark/cmd/benchmark/operations"
	"github.com/pedro-hbl/lambda-gopher-benchmark/internal/metrics"
	"github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases"
	"github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases/dynamodb"
	"github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases/immudb"
	"github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases/timestream"
	// Import other database packages as they are implemented
	// "github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases/timestream"
)

// BenchmarkRequest represents a configurable benchmark request
type BenchmarkRequest struct {
	DatabaseType  string                 `json:"databaseType"`  // dynamodb, immudb, timestream
	OperationType string                 `json:"operationType"` // read-sequential, read-parallel, write, write-batch, query
	Parameters    map[string]interface{} `json:"parameters"`
}

// BenchmarkResponse represents the result of a benchmark
type BenchmarkResponse struct {
	OperationType          string                 `json:"operationType"`
	DatabaseType           string                 `json:"databaseType"`
	Success                bool                   `json:"success"`
	ErrorMessage           string                 `json:"errorMessage,omitempty"`
	ItemsProcessed         int                    `json:"itemsProcessed"`
	TotalDurationNs        int64                  `json:"totalDurationNs"`
	AvgOperationDurationNs int64                  `json:"avgOperationDurationNs"`
	Throughput             float64                `json:"throughput"` // operations per second
	Metrics                map[string]interface{} `json:"metrics,omitempty"`
}

var (
	// Global metrics collector
	metricsCollector *metrics.Collector

	// Track cold start
	isColdStart = true
)

func init() {
	// Initialize metrics collector
	metricsCollector = metrics.NewCollector()

	// Set up logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Llongfile)

	log.Println("Lambda benchmark function initialized")
}

// createDatabaseAdapter creates the appropriate database adapter based on the request
func createDatabaseAdapter(ctx context.Context, dbType string, params map[string]interface{}) (databases.Database, error) {
	// Default configuration
	config := map[string]interface{}{
		"region":    os.Getenv("AWS_REGION"),
		"tableName": os.Getenv("DB_TABLE_NAME"),
	}

	// Override with request parameters if provided
	for k, v := range params {
		if strings.HasPrefix(k, "db.") {
			configKey := strings.TrimPrefix(k, "db.")
			config[configKey] = v
		}
	}

	// Special handling for local testing endpoints
	if endpoint, ok := os.LookupEnv("DB_ENDPOINT"); ok && endpoint != "" {
		config["endpoint"] = endpoint
	}

	// Create appropriate database adapter
	var (
		db  databases.Database
		err error
	)

	switch strings.ToLower(dbType) {
	case "dynamodb":
		factory := dynamodb.NewDynamoDBFactory()
		db, err = factory.CreateDatabase(config)
	case "immudb":
		factory := immudb.NewImmuDBFactory()
		db, err = factory.CreateDatabase(config)
	case "timestream":
		factory := timestream.NewTimestreamFactory()
		db, err = factory.CreateDatabase(config)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating database adapter: %w", err)
	}

	// Initialize the database
	err = db.Initialize(ctx)
	if err != nil {
		return nil, fmt.Errorf("error initializing database: %w", err)
	}

	return db, nil
}

// createOperationStrategy creates the appropriate operation strategy based on the request
func createOperationStrategy(opType string, params map[string]interface{}) (operations.Operation, error) {
	// Default parameters
	defaultParams := map[string]interface{}{
		"concurrency":    10,
		"itemCount":      100,
		"dataSize":       1024, // 1KB
		"consistentRead": true,
	}

	// Merge with provided parameters
	for k, v := range params {
		if !strings.HasPrefix(k, "db.") {
			defaultParams[k] = v
		}
	}

	// Create appropriate operation strategy
	switch strings.ToLower(opType) {
	case "read-sequential":
		return operations.NewReadOperation(defaultParams, false), nil
	case "read-parallel":
		return operations.NewReadOperation(defaultParams, true), nil
	case "write":
		return operations.NewWriteOperation(defaultParams, false), nil
	case "write-batch":
		return operations.NewWriteOperation(defaultParams, true), nil
	case "query":
		return operations.NewQueryOperation(defaultParams), nil
	default:
		return nil, fmt.Errorf("unsupported operation type: %s", opType)
	}
}

// handleRequest is the Lambda handler function
func handleRequest(ctx context.Context, request BenchmarkRequest) (BenchmarkResponse, error) {
	startTime := time.Now()
	log.Printf("Received benchmark request: %+v", request)

	// Initialize response
	response := BenchmarkResponse{
		OperationType: request.OperationType,
		DatabaseType:  request.DatabaseType,
		Success:       false,
	}

	// Start test for metrics collection
	testName := fmt.Sprintf("%s-%s-%s", request.DatabaseType, request.OperationType, time.Now().Format(time.RFC3339))
	metricsCollector.StartTest(
		testName,
		fmt.Sprintf("%s operations on %s", request.OperationType, request.DatabaseType),
		request.DatabaseType,
		map[string]interface{}{"region": os.Getenv("AWS_REGION")},
		request.Parameters,
	)

	// Create database adapter
	db, err := createDatabaseAdapter(ctx, request.DatabaseType, request.Parameters)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to create database adapter: %v", err)
		log.Println(errMsg)
		response.ErrorMessage = errMsg
		return response, nil
	}
	defer db.Close()

	// Add cold start parameter
	request.Parameters["isColdStart"] = isColdStart

	// Create operation strategy
	op, err := createOperationStrategy(request.OperationType, request.Parameters)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to create operation strategy: %v", err)
		log.Println(errMsg)
		response.ErrorMessage = errMsg
		return response, nil
	}

	// Execute the operation
	result, err := op.Execute(ctx, db, metricsCollector)
	if err != nil {
		errMsg := fmt.Sprintf("Operation execution failed: %v", err)
		log.Println(errMsg)
		response.ErrorMessage = errMsg
		return response, nil
	}

	// Get metrics
	collectMetrics := true
	if v, ok := request.Parameters["collectMetrics"]; ok {
		if b, ok := v.(bool); ok {
			collectMetrics = b
		}
	}

	testResult := metricsCollector.EndTest(testName)
	if testResult != nil && collectMetrics {
		response.Metrics = testResult.Summary
	}

	// Populate response
	response.Success = true
	response.ItemsProcessed = result.ItemsProcessed
	response.TotalDurationNs = result.TotalDuration.Nanoseconds()
	if result.ItemsProcessed > 0 {
		response.AvgOperationDurationNs = result.TotalDuration.Nanoseconds() / int64(result.ItemsProcessed)
		response.Throughput = float64(result.ItemsProcessed) / result.TotalDuration.Seconds()
	}

	// Log execution time
	elapsed := time.Since(startTime)
	log.Printf("Benchmark completed in %v", elapsed)

	// Reset cold start flag after first invocation
	isColdStart = false

	return response, nil
}

func main() {
	// Run as Lambda function if in AWS environment
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(handleRequest)
		return
	}

	// Run locally for testing
	log.Println("Running in local mode")

	// Example request for local testing
	request := BenchmarkRequest{
		DatabaseType:  "dynamodb",
		OperationType: "read-parallel",
		Parameters: map[string]interface{}{
			"concurrency":  10,
			"itemCount":    100,
			"dataSize":     1024,
			"accountId":    "test-account",
			"db.endpoint":  "http://localhost:8000", // Local DynamoDB
			"db.tableName": "Transactions",
			"db.region":    "us-east-1",
		},
	}

	// Parse command line flags for local testing
	// TODO: Add flag parsing

	response, err := handleRequest(context.Background(), request)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Print response as JSON
	jsonResponse, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println(string(jsonResponse))
}

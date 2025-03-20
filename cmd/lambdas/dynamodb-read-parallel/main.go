package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/uuid"
	"github.com/pedro-hbl/lambda-gopher-benchmark/internal/metrics"
	"github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases"
	"github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases/dynamodb"
)

// Request represents the input for the benchmark Lambda function
type Request struct {
	AccountID        string   `json:"accountId"`
	TransactionCount int      `json:"transactionCount"`
	CollectMetrics   bool     `json:"collectMetrics"`
	ConsistentRead   bool     `json:"consistentRead"`
	UseRandomIDs     bool     `json:"useRandomIds"`
	TransactionIDs   []string `json:"transactionIds"`
	IsColdStart      bool     `json:"isColdStart"`
	DataSizeBytes    int64    `json:"dataSizeBytes"`
	Concurrency      int      `json:"concurrency"`
}

// Response represents the output from the benchmark Lambda function
type Response struct {
	TransactionsRead int                    `json:"transactionsRead"`
	TotalDuration    int64                  `json:"totalDurationNs"`
	AvgDuration      int64                  `json:"avgDurationNs"`
	TransactionIDs   []string               `json:"transactionIds,omitempty"`
	Metrics          map[string]interface{} `json:"metrics,omitempty"`
	Errors           []string               `json:"errors,omitempty"`
}

// Result represents the result of a single read operation
type Result struct {
	TransactionID string
	Duration      time.Duration
	Error         error
}

var (
	db               databases.Database
	metricsCollector *metrics.Collector
	isColdStart      = true
)

func init() {
	// Initialize metrics collector
	metricsCollector = metrics.NewCollector()

	// Get configuration from environment variables
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	tableName := os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		tableName = "Transactions"
	}

	endpoint := os.Getenv("DYNAMODB_ENDPOINT")

	// Create DynamoDB factory
	factory := dynamodb.NewDynamoDBFactory()

	// Configure DynamoDB
	config := map[string]interface{}{
		"region":    region,
		"tableName": tableName,
	}

	if endpoint != "" {
		config["endpoint"] = endpoint
	}

	var err error
	db, err = factory.CreateDatabase(config)
	if err != nil {
		fmt.Printf("Error creating database: %v\n", err)
		os.Exit(1)
	}

	// Initialize the database
	err = db.Initialize(context.Background())
	if err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		os.Exit(1)
	}
}

func handleRequest(ctx context.Context, request Request) (Response, error) {
	startTime := time.Now()
	response := Response{
		TransactionsRead: 0,
		Errors:           []string{},
	}

	// Start metrics collection if requested
	if request.CollectMetrics {
		testName := fmt.Sprintf("dynamodb-read-parallel-%s", time.Now().Format(time.RFC3339))
		metricsCollector.StartTest(
			testName,
			"Parallel read operations on DynamoDB",
			"dynamodb",
			map[string]interface{}{"region": os.Getenv("AWS_REGION")},
			map[string]interface{}{"tableName": os.Getenv("DYNAMODB_TABLE")},
		)
	}

	// Read options
	readOptions := &databases.ReadOptions{
		ConsistentRead: request.ConsistentRead,
	}

	// Determine transaction IDs to read
	var transactionIDs []string

	// If transaction IDs are provided, use them
	// Otherwise generate random IDs if requested or sequential IDs
	if len(request.TransactionIDs) > 0 {
		transactionIDs = request.TransactionIDs
	} else if request.UseRandomIDs {
		// Generate random transaction IDs
		for i := 0; i < request.TransactionCount; i++ {
			transactionIDs = append(transactionIDs, uuid.New().String())
		}
	} else {
		// Generate sequential transaction IDs
		for i := 0; i < request.TransactionCount; i++ {
			transactionIDs = append(transactionIDs, fmt.Sprintf("txn-%07d", i))
		}
	}

	// Set concurrency level
	concurrency := request.Concurrency
	if concurrency <= 0 {
		concurrency = 10 // Default concurrency
	}
	if concurrency > len(transactionIDs) {
		concurrency = len(transactionIDs)
	}

	// Create a channel for results
	results := make(chan Result, len(transactionIDs))

	// Create a worker pool
	var wg sync.WaitGroup
	taskChan := make(chan string, len(transactionIDs))

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for transactionID := range taskChan {
				readStart := time.Now()
				var readErr error

				// Use the metrics collector to measure the operation
				err := metricsCollector.MeasureOperation(
					metrics.ReadOperation,
					1,
					request.DataSizeBytes,
					isColdStart && request.IsColdStart,
					func() error {
						_, err := db.ReadTransaction(ctx, request.AccountID, transactionID, readOptions)
						return err
					},
				)
				readErr = err

				readDuration := time.Since(readStart)
				results <- Result{
					TransactionID: transactionID,
					Duration:      readDuration,
					Error:         readErr,
				}
			}
		}()
	}

	// Send tasks to workers
	for _, transactionID := range transactionIDs {
		taskChan <- transactionID
	}
	close(taskChan)

	// Wait for all workers to finish
	wg.Wait()
	close(results)

	// Process results
	var durations []time.Duration
	for result := range results {
		if result.Error != nil {
			errMsg := fmt.Sprintf("Error reading transaction %s: %v", result.TransactionID, result.Error)
			response.Errors = append(response.Errors, errMsg)
		} else {
			response.TransactionsRead++
		}
		durations = append(durations, result.Duration)
	}

	// Calculate total and average durations
	var totalDuration time.Duration
	for _, d := range durations {
		totalDuration += d
	}

	response.TotalDuration = totalDuration.Nanoseconds()
	if len(durations) > 0 {
		response.AvgDuration = totalDuration.Nanoseconds() / int64(len(durations))
	}

	// Include transaction IDs in response if specified
	if len(request.TransactionIDs) == 0 {
		response.TransactionIDs = transactionIDs
	}

	// Include metrics in response if requested
	if request.CollectMetrics {
		testResult := metricsCollector.EndTest(fmt.Sprintf("dynamodb-read-parallel-%s", time.Now().Format(time.RFC3339)))
		if testResult != nil {
			response.Metrics = testResult.Summary
		}
	}

	// Reset cold start flag after first invocation
	isColdStart = false

	// Calculate elapsed time
	elapsed := time.Since(startTime)
	fmt.Printf("Total execution time: %v\n", elapsed)

	return response, nil
}

func main() {
	lambda.Start(handleRequest)
}

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
	AccountID        string `json:"accountId"`
	TransactionCount int    `json:"transactionCount"`
	CollectMetrics   bool   `json:"collectMetrics"`
	UseRandomIDs     bool   `json:"useRandomIds"`
	IsColdStart      bool   `json:"isColdStart"`
	DataSizeBytes    int64  `json:"dataSizeBytes"`
	Concurrency      int    `json:"concurrency"`
	BatchSize        int    `json:"batchSize"`
}

// Response represents the output from the benchmark Lambda function
type Response struct {
	TransactionsWritten int                    `json:"transactionsWritten"`
	TotalDuration       int64                  `json:"totalDurationNs"`
	AvgDuration         int64                  `json:"avgDurationNs"`
	TransactionIDs      []string               `json:"transactionIds,omitempty"`
	Metrics             map[string]interface{} `json:"metrics,omitempty"`
	Errors              []string               `json:"errors,omitempty"`
}

// Result represents the result of a single write operation
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

// generateTransactionData creates a transaction with specified data size
func generateTransactionData(accountID, transactionID string, dataSize int64) *databases.Transaction {
	// Create basic transaction
	tx := &databases.Transaction{
		AccountID:       accountID,
		UUID:            transactionID,
		Timestamp:       time.Now(),
		Amount:          100.00,
		TransactionType: databases.Deposit,
		Metadata:        make(map[string]interface{}),
	}

	// Add more data to reach desired size
	// We'll add a payload field with random data
	if dataSize > 0 {
		// Estimate base size (rough approximation)
		baseSize := int64(len(accountID) + len(transactionID) + 50) // 50 bytes for other fields
		remainingSize := dataSize - baseSize

		if remainingSize > 0 {
			// Create a payload of appropriate size
			payload := make([]byte, remainingSize)
			for i := range payload {
				payload[i] = byte(i % 256) // Pattern to avoid compression in transit
			}
			metadata := tx.Metadata.(map[string]interface{})
			metadata["payload"] = payload
			tx.Metadata = metadata
		}
	}

	return tx
}

func handleRequest(ctx context.Context, request Request) (Response, error) {
	startTime := time.Now()
	response := Response{
		TransactionsWritten: 0,
		Errors:              []string{},
	}

	// Start metrics collection if requested
	if request.CollectMetrics {
		testName := fmt.Sprintf("dynamodb-write-%s", time.Now().Format(time.RFC3339))
		metricsCollector.StartTest(
			testName,
			"Write operations on DynamoDB",
			"dynamodb",
			map[string]interface{}{"region": os.Getenv("AWS_REGION")},
			map[string]interface{}{"tableName": os.Getenv("DYNAMODB_TABLE")},
		)
	}

	// Determine batch size (default to 1 if not specified)
	batchSize := request.BatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	if batchSize > 25 {
		batchSize = 25 // DynamoDB batch write limit
	}

	// Generate transaction IDs
	var transactionIDs []string
	if request.UseRandomIDs {
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

	// Processing in batches
	batches := make([][]string, 0)
	currentBatch := make([]string, 0, batchSize)

	for _, id := range transactionIDs {
		currentBatch = append(currentBatch, id)
		if len(currentBatch) >= batchSize {
			batches = append(batches, currentBatch)
			currentBatch = make([]string, 0, batchSize)
		}
	}

	// Add any remaining transactions
	if len(currentBatch) > 0 {
		batches = append(batches, currentBatch)
	}

	// Create a channel for results
	results := make(chan Result, len(batches))

	// Create a worker pool
	var wg sync.WaitGroup
	batchChan := make(chan []string, len(batches))

	// Write options
	writeOptions := &databases.WriteOptions{}
	batchOptions := &databases.BatchOptions{
		MaxBatchSize: batchSize,
	}

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range batchChan {
				writeStart := time.Now()
				var writeErr error

				if len(batch) == 1 {
					// Single transaction write
					transactionID := batch[0]
					tx := generateTransactionData(request.AccountID, transactionID, request.DataSizeBytes)

					// Use the metrics collector to measure the operation
					err := metricsCollector.MeasureOperation(
						metrics.WriteOperation,
						1,
						request.DataSizeBytes,
						isColdStart && request.IsColdStart,
						func() error {
							return db.WriteTransaction(ctx, tx, writeOptions)
						},
					)
					writeErr = err

					writeDuration := time.Since(writeStart)
					results <- Result{
						TransactionID: transactionID,
						Duration:      writeDuration,
						Error:         writeErr,
					}
				} else {
					// Batch write
					transactions := make([]*databases.Transaction, 0, len(batch))
					for _, id := range batch {
						tx := generateTransactionData(request.AccountID, id, request.DataSizeBytes)
						transactions = append(transactions, tx)
					}

					// Use the metrics collector to measure the operation
					err := metricsCollector.MeasureOperation(
						metrics.BatchOperation,
						int64(len(batch)),
						request.DataSizeBytes*int64(len(batch)),
						isColdStart && request.IsColdStart,
						func() error {
							return db.BatchWriteTransactions(ctx, transactions, batchOptions)
						},
					)
					writeErr = err

					writeDuration := time.Since(writeStart)
					// Associate the duration with the first transaction ID in the batch
					results <- Result{
						TransactionID: batch[0],
						Duration:      writeDuration,
						Error:         writeErr,
					}
				}
			}
		}()
	}

	// Send batches to workers
	for _, batch := range batches {
		batchChan <- batch
	}
	close(batchChan)

	// Wait for all workers to finish
	wg.Wait()
	close(results)

	// Process results
	var durations []time.Duration
	successfulBatches := 0

	for result := range results {
		if result.Error != nil {
			errMsg := fmt.Sprintf("Error writing transaction(s) starting with %s: %v", result.TransactionID, result.Error)
			response.Errors = append(response.Errors, errMsg)
		} else {
			successfulBatches++
		}
		durations = append(durations, result.Duration)
	}

	// Calculate approximate number of transactions written (since batches may have different sizes)
	response.TransactionsWritten = request.TransactionCount - len(response.Errors)

	// Calculate total and average durations
	var totalDuration time.Duration
	for _, d := range durations {
		totalDuration += d
	}

	response.TotalDuration = totalDuration.Nanoseconds()
	if len(durations) > 0 {
		response.AvgDuration = totalDuration.Nanoseconds() / int64(len(durations))
	}

	// Include transaction IDs in response
	response.TransactionIDs = transactionIDs

	// Include metrics in response if requested
	if request.CollectMetrics {
		testResult := metricsCollector.EndTest(fmt.Sprintf("dynamodb-write-%s", time.Now().Format(time.RFC3339)))
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

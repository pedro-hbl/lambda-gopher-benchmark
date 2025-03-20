package operations

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pedro-hbl/lambda-gopher-benchmark/internal/metrics"
	"github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases"
)

// OperationResult contains the results of an operation execution
type OperationResult struct {
	ItemsProcessed int
	TotalDuration  time.Duration
	Errors         []error
	Data           map[string]interface{}
}

// Operation defines the interface for all database operations
type Operation interface {
	Execute(ctx context.Context, db databases.Database, collector *metrics.Collector) (OperationResult, error)
}

// baseOperation contains common parameters and methods for all operations
type baseOperation struct {
	params        map[string]interface{}
	isParallel    bool
	generateUUIDs bool
}

// Common utility functions for operations

// getParam retrieves a parameter with type assertion and default value
func getParam[T any](params map[string]interface{}, key string, defaultValue T) T {
	if val, ok := params[key]; ok {
		if result, ok := val.(T); ok {
			return result
		}
	}
	return defaultValue
}

// generateTransaction creates a transaction with random or specified data
func generateTransaction(params map[string]interface{}, index int) *databases.Transaction {
	accountID := getParam(params, "accountId", "test-account")
	dataSizeBytes := getParam(params, "dataSize", 1024)
	useRandomIDs := getParam(params, "useRandomIDs", false)

	var transactionID string
	if useRandomIDs {
		transactionID = uuid.New().String()
	} else {
		// Deterministic ID for easier testing/verification
		transactionID = fmt.Sprintf("%s-tx-%d", accountID, index)
	}

	// Generate random payload of specified size
	payload := make([]byte, dataSizeBytes)
	rand.Read(payload)

	// Create transaction
	timestamp := time.Now()
	return &databases.Transaction{
		UUID:            transactionID,
		AccountID:       accountID,
		Timestamp:       timestamp,
		Amount:          float64(rand.Intn(10000)) / 100, // Random amount between 0-100
		TransactionType: databases.Deposit,
		Metadata:        payload,
	}
}

// Read Operation
type ReadOperation struct {
	baseOperation
}

// NewReadOperation creates a new read operation (sequential or parallel)
func NewReadOperation(params map[string]interface{}, isParallel bool) *ReadOperation {
	return &ReadOperation{
		baseOperation: baseOperation{
			params:     params,
			isParallel: isParallel,
		},
	}
}

// Execute runs the read operation
func (op *ReadOperation) Execute(ctx context.Context, db databases.Database, collector *metrics.Collector) (OperationResult, error) {
	startTime := time.Now()
	result := OperationResult{
		Errors: []error{},
		Data:   make(map[string]interface{}),
	}

	// Get parameters
	count := getParam(op.params, "itemCount", 100)
	accountID := getParam(op.params, "accountId", "test-account")
	useRandomIDs := getParam(op.params, "useRandomIDs", false)
	consistentRead := getParam(op.params, "consistentRead", true)
	concurrency := getParam(op.params, "concurrency", 10)
	isColdStart := getParam(op.params, "isColdStart", false)
	dataSizeBytes := getParam(op.params, "dataSize", 1024)
	specificIDs, hasSpecificIDs := op.params["transactionIDs"].([]string)

	// Load IDs to read
	var transactionIDs []string
	if hasSpecificIDs {
		transactionIDs = specificIDs
		count = len(transactionIDs)
	} else if useRandomIDs {
		// For random IDs, we need to create transactions first
		return result, fmt.Errorf("reading random IDs requires pre-generating transactions first")
	} else {
		// Generate deterministic IDs
		transactionIDs = make([]string, count)
		for i := 0; i < count; i++ {
			transactionIDs[i] = fmt.Sprintf("%s-tx-%d", accountID, i)
		}
	}

	// Set options for reads
	readOptions := &databases.ReadOptions{
		ConsistentRead: consistentRead,
	}

	// Update result with actual count
	result.ItemsProcessed = count
	result.Data["transactionIDs"] = transactionIDs

	// Execute the reads
	if op.isParallel {
		// Parallel reads with worker pool
		var wg sync.WaitGroup
		errorChan := make(chan error, count)
		semaphore := make(chan struct{}, concurrency)

		for i, id := range transactionIDs {
			wg.Add(1)
			semaphore <- struct{}{}

			go func(index int, txID string) {
				defer wg.Done()
				defer func() { <-semaphore }()

				var readErr error

				err := collector.MeasureOperation(
					metrics.ReadOperation,
					1, // itemCount
					int64(dataSizeBytes),
					isColdStart,
					func() error {
						_, readErr = db.ReadTransaction(ctx, accountID, txID, readOptions)
						return readErr
					},
				)

				if err != nil {
					errorChan <- fmt.Errorf("failed to read transaction %s: %w", txID, err)
				}
			}(i, id)
		}

		// Wait for all reads to complete
		wg.Wait()
		close(errorChan)

		// Collect errors
		for err := range errorChan {
			result.Errors = append(result.Errors, err)
		}
	} else {
		// Sequential reads
		for _, id := range transactionIDs {
			var readErr error

			err := collector.MeasureOperation(
				metrics.ReadOperation,
				1, // itemCount
				int64(dataSizeBytes),
				isColdStart,
				func() error {
					_, readErr = db.ReadTransaction(ctx, accountID, id, readOptions)
					return readErr
				},
			)

			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to read transaction %s: %w", id, err))
			}
		}
	}

	// Calculate total duration
	result.TotalDuration = time.Since(startTime)

	// Return error if all operations failed
	if len(result.Errors) == count {
		return result, fmt.Errorf("all read operations failed")
	}

	return result, nil
}

// Write Operation
type WriteOperation struct {
	baseOperation
}

// NewWriteOperation creates a new write operation (single or batch)
func NewWriteOperation(params map[string]interface{}, isBatch bool) *WriteOperation {
	return &WriteOperation{
		baseOperation: baseOperation{
			params:     params,
			isParallel: isBatch,
		},
	}
}

// Execute runs the write operation
func (op *WriteOperation) Execute(ctx context.Context, db databases.Database, collector *metrics.Collector) (OperationResult, error) {
	startTime := time.Now()
	result := OperationResult{
		Errors: []error{},
		Data:   make(map[string]interface{}),
	}

	// Get parameters
	count := getParam(op.params, "itemCount", 100)
	batchSize := getParam(op.params, "batchSize", 25)
	concurrency := getParam(op.params, "concurrency", 10)
	isColdStart := getParam(op.params, "isColdStart", false)
	dataSizeBytes := getParam(op.params, "dataSize", 1024)

	// Generate transactions
	transactions := make([]*databases.Transaction, count)
	transactionIDs := make([]string, count)

	for i := 0; i < count; i++ {
		transactions[i] = generateTransaction(op.params, i)
		transactionIDs[i] = transactions[i].UUID
	}

	// Set options for writes
	writeOptions := &databases.WriteOptions{}
	batchOptions := &databases.BatchOptions{
		MaxBatchSize: batchSize,
	}

	// Update result with actual count
	result.ItemsProcessed = count
	result.Data["transactionIDs"] = transactionIDs

	// Execute the writes
	if op.isParallel {
		// Batch writes
		numBatches := (count + batchSize - 1) / batchSize
		var wg sync.WaitGroup
		errorChan := make(chan error, numBatches)
		semaphore := make(chan struct{}, concurrency)

		for i := 0; i < numBatches; i++ {
			wg.Add(1)
			semaphore <- struct{}{}

			go func(batchIndex int) {
				defer wg.Done()
				defer func() { <-semaphore }()

				startIdx := batchIndex * batchSize
				endIdx := (batchIndex + 1) * batchSize
				if endIdx > count {
					endIdx = count
				}

				batch := transactions[startIdx:endIdx]
				batchSize := len(batch)

				var writeErr error
				err := collector.MeasureOperation(
					metrics.BatchOperation,
					int64(batchSize),
					int64(batchSize*dataSizeBytes),
					isColdStart,
					func() error {
						writeErr = db.BatchWriteTransactions(ctx, batch, batchOptions)
						return writeErr
					},
				)

				if err != nil {
					errorChan <- fmt.Errorf("failed to write batch %d: %w", batchIndex, err)
				}
			}(i)
		}

		// Wait for all batches to complete
		wg.Wait()
		close(errorChan)

		// Collect errors
		for err := range errorChan {
			result.Errors = append(result.Errors, err)
		}
	} else {
		// Individual writes
		for _, tx := range transactions {
			var writeErr error
			err := collector.MeasureOperation(
				metrics.WriteOperation,
				1, // itemCount
				int64(dataSizeBytes),
				isColdStart,
				func() error {
					writeErr = db.WriteTransaction(ctx, tx, writeOptions)
					return writeErr
				},
			)

			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to write transaction %s: %w", tx.UUID, err))
			}
		}
	}

	// Calculate total duration
	result.TotalDuration = time.Since(startTime)

	// Return error if all operations failed
	if len(result.Errors) == count {
		return result, fmt.Errorf("all write operations failed")
	}

	return result, nil
}

// Query Operation
type QueryOperation struct {
	baseOperation
}

// NewQueryOperation creates a new query operation
func NewQueryOperation(params map[string]interface{}) *QueryOperation {
	return &QueryOperation{
		baseOperation: baseOperation{
			params:     params,
			isParallel: false,
		},
	}
}

// Execute runs the query operation
func (op *QueryOperation) Execute(ctx context.Context, db databases.Database, collector *metrics.Collector) (OperationResult, error) {
	startTime := time.Now()
	result := OperationResult{
		Errors: []error{},
		Data:   make(map[string]interface{}),
	}

	// Get parameters
	accountID := getParam(op.params, "accountId", "test-account")
	isColdStart := getParam(op.params, "isColdStart", false)

	// Get start and end times with proper type conversion
	var startDate, endDate time.Time
	startTimestamp, hasStartTime := op.params["startTime"]
	endTimestamp, hasEndTime := op.params["endTime"]

	if hasStartTime {
		if ts, ok := startTimestamp.(time.Time); ok {
			startDate = ts
		} else if str, ok := startTimestamp.(string); ok {
			// Try to parse RFC3339 time
			if t, err := time.Parse(time.RFC3339, str); err == nil {
				startDate = t
			}
		}
	}

	if hasEndTime {
		if ts, ok := endTimestamp.(time.Time); ok {
			endDate = ts
		} else if str, ok := endTimestamp.(string); ok {
			// Try to parse RFC3339 time
			if t, err := time.Parse(time.RFC3339, str); err == nil {
				endDate = t
			}
		}
	}

	// Set default dates if not provided or parsing failed
	if startDate.IsZero() {
		startDate = time.Now().Add(-24 * time.Hour)
	}
	if endDate.IsZero() {
		endDate = time.Now()
	}

	limit := getParam(op.params, "limit", int64(100))
	consistentRead := getParam(op.params, "consistentRead", true)

	// Set query options
	queryOptions := &databases.QueryOptions{
		Limit:          limit,
		ConsistentRead: consistentRead,
	}

	// Execute the query
	var transactions []*databases.Transaction
	var queryErr error

	// Estimate the data size for metrics - will be updated with actual results
	estimatedItemCount := limit
	estimatedByteCount := estimatedItemCount * int64(getParam(op.params, "dataSize", 1024))

	err := collector.MeasureOperation(
		metrics.QueryOperation,
		estimatedItemCount,
		estimatedByteCount,
		isColdStart,
		func() error {
			transactions, queryErr = db.QueryTransactionsByTimeRange(
				ctx,
				accountID,
				startDate,
				endDate,
				queryOptions,
			)
			return queryErr
		},
	)

	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to execute query: %w", err))
		return result, err
	}

	// Update result with retrieved count
	result.ItemsProcessed = len(transactions)
	transactionIDs := make([]string, len(transactions))
	for i, tx := range transactions {
		transactionIDs[i] = tx.UUID
	}
	result.Data["transactionIDs"] = transactionIDs

	// Calculate total duration
	result.TotalDuration = time.Since(startTime)

	return result, nil
}

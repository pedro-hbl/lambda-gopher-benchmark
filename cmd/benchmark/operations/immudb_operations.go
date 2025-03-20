package operations

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pedro-hbl/lambda-gopher-benchmark/internal/metrics"
	"github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases"
)

// ImmuDBWriteOperation represents a write operation for ImmuDB
type ImmuDBWriteOperation struct {
	baseOperation
	numTransactions int
	accountID       string
}

// NewImmuDBWriteOperation creates a new ImmuDB write operation
func NewImmuDBWriteOperation(params map[string]interface{}) Operation {
	return &ImmuDBWriteOperation{
		baseOperation: baseOperation{
			params:        params,
			isParallel:    getParam(params, "parallel", false),
			generateUUIDs: true,
		},
		numTransactions: getParam(params, "numTransactions", 10),
		accountID:       getParam(params, "accountID", fmt.Sprintf("acct-%s", uuid.New().String()[:8])),
	}
}

// Execute runs the ImmuDB write operation
func (op *ImmuDBWriteOperation) Execute(ctx context.Context, db databases.Database, collector *metrics.Collector) (OperationResult, error) {
	result := OperationResult{
		ItemsProcessed: 0,
		TotalDuration:  0,
		Errors:         []error{},
		Data:           make(map[string]interface{}),
	}

	// Create transactions
	transactions := make([]*databases.Transaction, op.numTransactions)
	for i := 0; i < op.numTransactions; i++ {
		transactions[i] = generateTransaction(op.params, i)
		transactions[i].AccountID = op.accountID
	}

	// Track UUIDs for verification
	uuids := make([]string, len(transactions))
	for i, tx := range transactions {
		uuids[i] = tx.UUID
	}
	result.Data["uuids"] = uuids
	result.Data["accountID"] = op.accountID

	startTime := time.Now()

	// Execute operation based on parallel flag
	if op.isParallel {
		var wg sync.WaitGroup
		errChan := make(chan error, len(transactions))

		for _, tx := range transactions {
			wg.Add(1)
			go func(transaction *databases.Transaction) {
				defer wg.Done()

				// Estimate size of transaction for metrics
				txSize := int64(len(transaction.UUID) + len(transaction.AccountID) +
					len(transaction.TransactionType) + 8) // 8 bytes for timestamp and amount
				if meta, ok := transaction.Metadata.(string); ok {
					txSize += int64(len(meta))
				} else {
					txSize += 100 // Default estimate if not a string
				}

				operationErr := collector.MeasureOperation(
					metrics.WriteOperation,
					1, // One transaction
					txSize,
					false, // Not a cold start
					func() error {
						return db.WriteTransaction(ctx, transaction, &databases.WriteOptions{})
					},
				)
				if operationErr != nil {
					errChan <- operationErr
				}
			}(tx)
		}

		wg.Wait()
		close(errChan)

		for err := range errChan {
			result.Errors = append(result.Errors, err)
		}
	} else {
		// Estimate total size for batch metrics
		totalSize := int64(0)
		for _, tx := range transactions {
			itemSize := int64(len(tx.UUID) + len(tx.AccountID) +
				len(tx.TransactionType) + 8)
			if meta, ok := tx.Metadata.(string); ok {
				itemSize += int64(len(meta))
			} else {
				itemSize += 100
			}
			totalSize += itemSize
		}

		// Batch write all transactions
		err := collector.MeasureOperation(
			metrics.BatchOperation,
			int64(len(transactions)),
			totalSize,
			false, // Not a cold start
			func() error {
				return db.BatchWriteTransactions(ctx, transactions, &databases.BatchOptions{})
			},
		)
		if err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	result.TotalDuration = time.Since(startTime)
	result.ItemsProcessed = op.numTransactions - len(result.Errors)

	return result, nil
}

// ImmuDBReadOperation represents a read operation for ImmuDB
type ImmuDBReadOperation struct {
	baseOperation
	uuids     []string
	accountID string
}

// NewImmuDBReadOperation creates a new ImmuDB read operation
func NewImmuDBReadOperation(params map[string]interface{}) Operation {
	return &ImmuDBReadOperation{
		baseOperation: baseOperation{
			params:     params,
			isParallel: getParam(params, "parallel", false),
		},
		uuids:     getParam(params, "uuids", []string{}),
		accountID: getParam(params, "accountID", ""),
	}
}

// Execute runs the ImmuDB read operation
func (op *ImmuDBReadOperation) Execute(ctx context.Context, db databases.Database, collector *metrics.Collector) (OperationResult, error) {
	result := OperationResult{
		ItemsProcessed: 0,
		TotalDuration:  0,
		Errors:         []error{},
		Data:           make(map[string]interface{}),
	}

	if len(op.uuids) == 0 {
		return result, fmt.Errorf("no UUIDs provided for read operation")
	}

	if op.accountID == "" {
		return result, fmt.Errorf("no account ID provided for read operation")
	}

	startTime := time.Now()
	transactions := make([]*databases.Transaction, 0, len(op.uuids))

	// Execute operation based on parallel flag
	if op.isParallel {
		var wg sync.WaitGroup
		resultLock := sync.Mutex{}
		errChan := make(chan error, len(op.uuids))
		txChan := make(chan *databases.Transaction, len(op.uuids))

		for _, uuid := range op.uuids {
			wg.Add(1)
			go func(txid string) {
				defer wg.Done()
				var tx *databases.Transaction

				// Estimate size for metrics - this is just key size since we don't know result size yet
				keySize := int64(len(txid) + len(op.accountID))

				err := collector.MeasureOperation(
					metrics.ReadOperation,
					1, // One transaction
					keySize,
					false, // Not a cold start
					func() error {
						var opErr error
						tx, opErr = db.ReadTransaction(ctx, op.accountID, txid, &databases.ReadOptions{})
						return opErr
					},
				)
				if err != nil {
					errChan <- err
				} else if tx != nil {
					txChan <- tx
				}
			}(uuid)
		}

		wg.Wait()
		close(errChan)
		close(txChan)

		for err := range errChan {
			result.Errors = append(result.Errors, err)
		}

		for tx := range txChan {
			resultLock.Lock()
			transactions = append(transactions, tx)
			resultLock.Unlock()
		}
	} else {
		// Create keys structure for BatchReadTransactions
		keys := make([]struct{ AccountID, UUID string }, len(op.uuids))
		for i, uuid := range op.uuids {
			keys[i].AccountID = op.accountID
			keys[i].UUID = uuid
		}

		// Calculate total key size for batch metrics
		totalKeySize := int64(0)
		for _, uuid := range op.uuids {
			totalKeySize += int64(len(uuid) + len(op.accountID))
		}

		// Batch read transactions
		err := collector.MeasureOperation(
			metrics.BatchOperation,
			int64(len(op.uuids)),
			totalKeySize,
			false, // Not a cold start
			func() error {
				var opErr error
				transactions, opErr = db.BatchReadTransactions(ctx, keys, &databases.BatchOptions{})
				return opErr
			},
		)
		if err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	result.TotalDuration = time.Since(startTime)
	result.ItemsProcessed = len(transactions)
	result.Data["transactions"] = transactions

	return result, nil
}

// ImmuDBQueryOperation represents a query operation for ImmuDB
type ImmuDBQueryOperation struct {
	baseOperation
	accountID string
	timeRange bool
	startTime time.Time
	endTime   time.Time
}

// NewImmuDBQueryOperation creates a new ImmuDB query operation
func NewImmuDBQueryOperation(params map[string]interface{}) Operation {
	// Determine whether to use time range query
	timeRange := getParam(params, "timeRange", false)

	// Default time range is last 24 hours
	now := time.Now()
	startTime := now.Add(-24 * time.Hour)
	endTime := now

	// Allow custom time ranges
	if startTimeStr, ok := params["startTime"].(string); ok {
		if parsedTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = parsedTime
		}
	}
	if endTimeStr, ok := params["endTime"].(string); ok {
		if parsedTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = parsedTime
		}
	}

	return &ImmuDBQueryOperation{
		baseOperation: baseOperation{
			params: params,
		},
		accountID: getParam(params, "accountID", ""),
		timeRange: timeRange,
		startTime: startTime,
		endTime:   endTime,
	}
}

// Execute runs the ImmuDB query operation
func (op *ImmuDBQueryOperation) Execute(ctx context.Context, db databases.Database, collector *metrics.Collector) (OperationResult, error) {
	result := OperationResult{
		ItemsProcessed: 0,
		TotalDuration:  0,
		Errors:         []error{},
		Data:           make(map[string]interface{}),
	}

	if op.accountID == "" {
		return result, fmt.Errorf("no account ID provided for query operation")
	}

	startTime := time.Now()
	var transactions []*databases.Transaction
	var err error

	// Estimate query size for metrics
	querySize := int64(len(op.accountID))
	if op.timeRange {
		querySize += 16 // Approximate size of two timestamps
	}

	// Choose query type based on parameters
	if op.timeRange {
		// Query by time range
		err = collector.MeasureOperation(
			metrics.QueryOperation,
			0, // We don't know item count yet
			querySize,
			false, // Not a cold start
			func() error {
				var opErr error
				transactions, opErr = db.QueryTransactionsByTimeRange(ctx, op.accountID, op.startTime, op.endTime, &databases.QueryOptions{})
				return opErr
			},
		)
	} else {
		// Query by account only
		err = collector.MeasureOperation(
			metrics.QueryOperation,
			0, // We don't know item count yet
			querySize,
			false, // Not a cold start
			func() error {
				var opErr error
				transactions, opErr = db.QueryTransactionsByAccount(ctx, op.accountID, &databases.QueryOptions{})
				return opErr
			},
		)
	}

	if err != nil {
		result.Errors = append(result.Errors, err)
	}

	result.TotalDuration = time.Since(startTime)
	result.ItemsProcessed = len(transactions)
	result.Data["transactions"] = transactions

	return result, nil
}

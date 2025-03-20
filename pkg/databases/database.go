package databases

import (
	"context"
	"time"
)

// TransactionType represents the type of banking transaction
type TransactionType string

const (
	// Deposit represents a deposit transaction
	Deposit TransactionType = "DEPOSIT"
	// Withdrawal represents a withdrawal transaction
	Withdrawal TransactionType = "WITHDRAWAL"
	// Transfer represents a transfer transaction
	Transfer TransactionType = "TRANSFER"
)

// Transaction represents a banking transaction record
type Transaction struct {
	AccountID       string          `json:"accountId"`       // 12 characters
	UUID            string          `json:"uuid"`            // 36 characters
	Timestamp       time.Time       `json:"timestamp"`       // ISO 8601 format
	Amount          float64         `json:"amount"`          // Decimal with 2 precision points
	TransactionType TransactionType `json:"transactionType"` // DEPOSIT, WITHDRAWAL, TRANSFER
	Metadata        interface{}     `json:"metadata"`        // JSON object, configurable size
}

// ReadOptions represents options for read operations
type ReadOptions struct {
	ConsistentRead bool
	IndexName      string
	Limit          int64
	// Add more options as needed
}

// WriteOptions represents options for write operations
type WriteOptions struct {
	Condition     string
	ReturnOldItem bool
	// Add more options as needed
}

// QueryOptions represents options for query operations
type QueryOptions struct {
	ScanIndexForward bool
	Limit            int64
	ConsistentRead   bool
	// Add more options as needed
}

// BatchOptions represents options for batch operations
type BatchOptions struct {
	MaxBatchSize int
	// Add more options as needed
}

// Database defines the standard interface that all database implementations must satisfy
type Database interface {
	// Core operations
	Initialize(ctx context.Context) error
	Close() error

	// Single-item operations
	ReadTransaction(ctx context.Context, accountID, uuid string, options *ReadOptions) (*Transaction, error)
	WriteTransaction(ctx context.Context, transaction *Transaction, options *WriteOptions) error
	DeleteTransaction(ctx context.Context, accountID, uuid string) error

	// Query operations
	QueryTransactionsByAccount(ctx context.Context, accountID string, options *QueryOptions) ([]*Transaction, error)
	QueryTransactionsByTimeRange(ctx context.Context, accountID string, startTime, endTime time.Time, options *QueryOptions) ([]*Transaction, error)

	// Batch operations
	BatchReadTransactions(ctx context.Context, keys []struct{ AccountID, UUID string }, options *BatchOptions) ([]*Transaction, error)
	BatchWriteTransactions(ctx context.Context, transactions []*Transaction, options *BatchOptions) error

	// Transaction operations
	ExecuteTransactWrite(ctx context.Context, transactions []*Transaction) error

	// Metrics and diagnostics
	GetMetrics() map[string]interface{}
	ResetMetrics()
}

// DatabaseFactory creates and configures a specific database implementation
type DatabaseFactory interface {
	// CreateDatabase creates a new database instance with the given configuration
	CreateDatabase(config map[string]interface{}) (Database, error)
}

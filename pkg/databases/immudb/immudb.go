package immudb

import (
	"context"
	"fmt"
	"time"

	"github.com/codenotary/immudb/pkg/client"
	"github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases"
)

// ImmuDBAdapter implements the Database interface for ImmuDB
type ImmuDBAdapter struct {
	client    client.ImmuClient
	options   *client.Options
	dbName    string
	tableName string
	connected bool
	config    map[string]interface{}
	metrics   map[string]interface{}
}

// ImmuDBFactory creates ImmuDB database instances
type ImmuDBFactory struct{}

// NewImmuDBFactory creates a new factory for ImmuDB
func NewImmuDBFactory() *ImmuDBFactory {
	return &ImmuDBFactory{}
}

// CreateDatabase creates a new ImmuDB database adapter
func (f *ImmuDBFactory) CreateDatabase(config map[string]interface{}) (databases.Database, error) {
	// Default configuration
	defaultConfig := map[string]interface{}{
		"address":   "127.0.0.1",
		"port":      3322,
		"username":  "immudb",
		"password":  "immudb",
		"database":  "defaultdb",
		"tableName": "transactions",
	}

	// Override defaults with provided config
	for k, v := range config {
		defaultConfig[k] = v
	}

	// Extract configuration values
	address := fmt.Sprintf("%v", defaultConfig["address"])
	// Convert port to int for the WithPort method
	portValue := defaultConfig["port"]
	var port int
	switch v := portValue.(type) {
	case int:
		port = v
	case float64:
		port = int(v)
	case uint16:
		port = int(v)
	default:
		port = 3322 // default port
	}
	username := fmt.Sprintf("%v", defaultConfig["username"])
	password := fmt.Sprintf("%v", defaultConfig["password"])
	dbName := fmt.Sprintf("%v", defaultConfig["database"])
	tableName := fmt.Sprintf("%v", defaultConfig["tableName"])

	// Create ImmuDB options
	options := client.DefaultOptions().
		WithAddress(address).
		WithPort(port).
		WithUsername(username).
		WithPassword(password)

	// Create adapter
	adapter := &ImmuDBAdapter{
		options:   options,
		dbName:    dbName,
		tableName: tableName,
		config:    defaultConfig,
		metrics:   make(map[string]interface{}),
	}

	return adapter, nil
}

// Initialize establishes a connection to the ImmuDB database and ensures the required table exists
func (a *ImmuDBAdapter) Initialize(ctx context.Context) error {
	if a.connected {
		return nil
	}

	// Create client
	c := client.NewClient()

	// Connect to server with the right types for username and password ([]byte)
	err := c.OpenSession(ctx, []byte(a.options.Username), []byte(a.options.Password), a.options.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to ImmuDB: %w", err)
	}

	a.client = c
	a.connected = true

	// Create the table if it doesn't exist
	// Determine if table exists
	sqlStmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s ("+
		"uuid VARCHAR[36] NOT NULL, "+
		"account_id VARCHAR[36] NOT NULL, "+
		"timestamp INTEGER NOT NULL, "+
		"amount FLOAT NOT NULL, "+
		"transaction_type VARCHAR[50] NOT NULL, "+
		"metadata VARCHAR, "+
		"PRIMARY KEY uuid"+
		")", a.tableName)

	_, err = c.SQLExec(ctx, sqlStmt, nil)
	if err != nil {
		c.CloseSession(ctx)
		a.connected = false
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create indexes for faster queries
	indexStmts := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_account ON %s(account_id)", a.tableName, a.tableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_timestamp ON %s(timestamp)", a.tableName, a.tableName),
	}

	for _, stmt := range indexStmts {
		_, err = c.SQLExec(ctx, stmt, nil)
		if err != nil {
			// Log the error but continue - index creation is not critical
			fmt.Printf("Warning: failed to create index: %v\n", err)
		}
	}

	return nil
}

// Close closes the ImmuDB connection
func (db *ImmuDBAdapter) Close() error {
	if db.connected && db.client != nil {
		ctx := context.Background()
		err := db.client.CloseSession(ctx)
		if err == nil {
			db.connected = false
		}
		return err
	}
	return nil
}

// ReadTransaction retrieves a transaction by its UUID
func (a *ImmuDBAdapter) ReadTransaction(ctx context.Context, accountID, uuid string, options *databases.ReadOptions) (*databases.Transaction, error) {
	if !a.connected {
		if err := a.Initialize(ctx); err != nil {
			return nil, err
		}
	}

	query := fmt.Sprintf("SELECT uuid, account_id, timestamp, amount, transaction_type, metadata FROM %s WHERE uuid = ?", a.tableName)

	// Execute query
	params := map[string]interface{}{
		"uuid": uuid,
	}

	result, err := a.client.SQLQuery(ctx, query, params, true)
	if err != nil {
		return nil, fmt.Errorf("failed to read transaction: %w", err)
	}

	if len(result.Rows) == 0 {
		return nil, fmt.Errorf("transaction not found: %s", uuid)
	}

	// Parse the result
	row := result.Rows[0]

	// Extract values based on column order
	transaction := &databases.Transaction{
		UUID:            row.Values[0].GetS(),
		AccountID:       row.Values[1].GetS(),
		Timestamp:       time.Unix(row.Values[2].GetN(), 0),
		Amount:          float64(row.Values[3].GetF()),
		TransactionType: databases.TransactionType(row.Values[4].GetS()),
		Metadata:        row.Values[5].GetS(),
	}

	return transaction, nil
}

// WriteTransaction stores a transaction in the database
func (a *ImmuDBAdapter) WriteTransaction(ctx context.Context, transaction *databases.Transaction, options *databases.WriteOptions) error {
	if !a.connected {
		if err := a.Initialize(ctx); err != nil {
			return err
		}
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (uuid, account_id, timestamp, amount, transaction_type, metadata) VALUES (?, ?, ?, ?, ?, ?)",
		a.tableName,
	)

	params := map[string]interface{}{
		"uuid":             transaction.UUID,
		"account_id":       transaction.AccountID,
		"timestamp":        transaction.Timestamp.Unix(),
		"amount":           transaction.Amount,
		"transaction_type": string(transaction.TransactionType),
		"metadata":         transaction.Metadata,
	}

	_, err := a.client.SQLExec(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to write transaction: %w", err)
	}

	return nil
}

// DeleteTransaction removes a transaction by its UUID
func (a *ImmuDBAdapter) DeleteTransaction(ctx context.Context, accountID, uuid string) error {
	if !a.connected {
		if err := a.Initialize(ctx); err != nil {
			return err
		}
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE uuid = ?", a.tableName)

	params := map[string]interface{}{
		"uuid": uuid,
	}

	_, err := a.client.SQLExec(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}

	return nil
}

// QueryTransactionsByAccount retrieves all transactions for a specific account
func (a *ImmuDBAdapter) QueryTransactionsByAccount(ctx context.Context, accountID string, options *databases.QueryOptions) ([]*databases.Transaction, error) {
	if !a.connected {
		if err := a.Initialize(ctx); err != nil {
			return nil, err
		}
	}

	query := fmt.Sprintf("SELECT uuid, account_id, timestamp, amount, transaction_type, metadata FROM %s WHERE account_id = ?", a.tableName)

	params := map[string]interface{}{
		"account_id": accountID,
	}

	result, err := a.client.SQLQuery(ctx, query, params, true)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}

	transactions := make([]*databases.Transaction, 0, len(result.Rows))

	for _, row := range result.Rows {
		transaction := &databases.Transaction{
			UUID:            row.Values[0].GetS(),
			AccountID:       row.Values[1].GetS(),
			Timestamp:       time.Unix(row.Values[2].GetN(), 0),
			Amount:          float64(row.Values[3].GetF()),
			TransactionType: databases.TransactionType(row.Values[4].GetS()),
			Metadata:        row.Values[5].GetS(),
		}

		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

// QueryTransactionsByTimeRange retrieves transactions within a specific time range
func (a *ImmuDBAdapter) QueryTransactionsByTimeRange(ctx context.Context, accountID string, startTime, endTime time.Time, options *databases.QueryOptions) ([]*databases.Transaction, error) {
	if !a.connected {
		if err := a.Initialize(ctx); err != nil {
			return nil, err
		}
	}

	query := fmt.Sprintf("SELECT uuid, account_id, timestamp, amount, transaction_type, metadata FROM %s WHERE account_id = ? AND timestamp >= ? AND timestamp <= ?", a.tableName)

	params := map[string]interface{}{
		"account_id":      accountID,
		"start_timestamp": startTime.Unix(),
		"end_timestamp":   endTime.Unix(),
	}

	result, err := a.client.SQLQuery(ctx, query, params, true)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}

	transactions := make([]*databases.Transaction, 0, len(result.Rows))

	for _, row := range result.Rows {
		transaction := &databases.Transaction{
			UUID:            row.Values[0].GetS(),
			AccountID:       row.Values[1].GetS(),
			Timestamp:       time.Unix(row.Values[2].GetN(), 0),
			Amount:          float64(row.Values[3].GetF()),
			TransactionType: databases.TransactionType(row.Values[4].GetS()),
			Metadata:        row.Values[5].GetS(),
		}

		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

// BatchReadTransactions reads multiple transactions in a single operation
func (db *ImmuDBAdapter) BatchReadTransactions(ctx context.Context, keys []struct{ AccountID, UUID string }, options *databases.BatchOptions) ([]*databases.Transaction, error) {
	if !db.connected {
		if err := db.Initialize(ctx); err != nil {
			return nil, err
		}
	}

	// For now, implement as sequential reads
	transactions := make([]*databases.Transaction, 0, len(keys))
	readOptions := &databases.ReadOptions{
		ConsistentRead: true,
	}

	for _, key := range keys {
		transaction, err := db.ReadTransaction(ctx, key.AccountID, key.UUID, readOptions)
		if err != nil {
			// Log error but continue
			fmt.Printf("Error reading transaction %s: %v\n", key.UUID, err)
			continue
		}
		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

// BatchWriteTransactions writes multiple transactions to the database
func (a *ImmuDBAdapter) BatchWriteTransactions(ctx context.Context, transactions []*databases.Transaction, options *databases.BatchOptions) error {
	if !a.connected {
		if err := a.Initialize(ctx); err != nil {
			return err
		}
	}

	// Start a transaction for batch insert
	tx, err := a.client.NewTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	// Set up the base query
	query := fmt.Sprintf(
		"INSERT INTO %s (uuid, account_id, timestamp, amount, transaction_type, metadata) VALUES (?, ?, ?, ?, ?, ?)",
		a.tableName,
	)

	// Execute batch inserts
	for _, transaction := range transactions {
		params := map[string]interface{}{
			"uuid":             transaction.UUID,
			"account_id":       transaction.AccountID,
			"timestamp":        transaction.Timestamp.Unix(),
			"amount":           transaction.Amount,
			"transaction_type": string(transaction.TransactionType),
			"metadata":         transaction.Metadata,
		}

		// Fixed: SQLExec returns only one value
		err = tx.SQLExec(ctx, query, params)
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to insert transaction: %w", err)
		}
	}

	// Commit the transaction
	// Fixed: Commit returns two values (txID and error)
	_, err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit batch transaction: %w", err)
	}

	return nil
}

// ExecuteTransactWrite executes a transaction with multiple operations
func (db *ImmuDBAdapter) ExecuteTransactWrite(ctx context.Context, transactions []*databases.Transaction) error {
	if !db.connected {
		if err := db.Initialize(ctx); err != nil {
			return err
		}
	}

	// For ImmuDB, we can use the BatchWriteTransactions since it already uses transactions
	return db.BatchWriteTransactions(ctx, transactions, &databases.BatchOptions{})
}

// GetMetrics returns metrics collected by the adapter
func (db *ImmuDBAdapter) GetMetrics() map[string]interface{} {
	return db.metrics
}

// ResetMetrics resets all metrics
func (db *ImmuDBAdapter) ResetMetrics() {
	db.metrics = make(map[string]interface{})
}

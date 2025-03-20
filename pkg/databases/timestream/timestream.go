package timestream

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/timestreamquery"
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite"
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite/types"
	"github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases"
)

// TimestreamDatabase implements the Database interface for AWS Timestream
type TimestreamDatabase struct {
	writeClient  *timestreamwrite.Client
	queryClient  *timestreamquery.Client
	databaseName string
	tableName    string
	metrics      map[string]interface{}
	initialized  bool
}

// TimestreamConfig holds configuration for the Timestream database
type TimestreamConfig struct {
	Region       string
	DatabaseName string
	TableName    string
	Endpoint     string
}

// TimestreamFactory creates Timestream database instances
type TimestreamFactory struct{}

// NewTimestreamFactory creates a new Timestream factory
func NewTimestreamFactory() *TimestreamFactory {
	return &TimestreamFactory{}
}

// CreateDatabase implements the DatabaseFactory interface
func (f *TimestreamFactory) CreateDatabase(config map[string]interface{}) (databases.Database, error) {
	// Extract configuration
	dbConfig := TimestreamConfig{
		Region:       "us-east-1", // Default region
		DatabaseName: "BenchmarkDB",
		TableName:    "Transactions",
	}

	if region, ok := config["region"].(string); ok {
		dbConfig.Region = region
	}
	if databaseName, ok := config["databaseName"].(string); ok {
		dbConfig.DatabaseName = databaseName
	}
	if tableName, ok := config["tableName"].(string); ok {
		dbConfig.TableName = tableName
	}
	if endpoint, ok := config["endpoint"].(string); ok {
		dbConfig.Endpoint = endpoint
	}

	return NewTimestreamDatabase(dbConfig)
}

// NewTimestreamDatabase creates a new AWS Timestream database instance
func NewTimestreamDatabase(config TimestreamConfig) (*TimestreamDatabase, error) {
	db := &TimestreamDatabase{
		databaseName: config.DatabaseName,
		tableName:    config.TableName,
		metrics:      make(map[string]interface{}),
		initialized:  false,
	}

	// Create AWS configuration
	var err error

	// Configure AWS SDK
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(config.Region))

	if config.Endpoint != "" {
		// Use a custom endpoint if provided
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			if service == "timestreamwrite" || service == "timestreamquery" {
				return aws.Endpoint{
					URL:           config.Endpoint,
					SigningRegion: config.Region,
				}, nil
			}
			// Fallback to default resolution
			return aws.Endpoint{}, fmt.Errorf("unknown service %s", service)
		})
		awsCfg.EndpointResolverWithOptions = customResolver
	}

	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	// Create Timestream clients
	db.writeClient = timestreamwrite.NewFromConfig(awsCfg)
	db.queryClient = timestreamquery.NewFromConfig(awsCfg)

	return db, nil
}

// Initialize implements the Database interface
func (db *TimestreamDatabase) Initialize(ctx context.Context) error {
	if db.initialized {
		return nil
	}

	// Check if database exists, create if it doesn't
	err := db.ensureDatabaseExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to ensure database exists: %w", err)
	}

	// Check if table exists, create if it doesn't
	err = db.ensureTableExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to ensure table exists: %w", err)
	}

	db.initialized = true
	db.ResetMetrics()
	return nil
}

// Close implements the Database interface
func (db *TimestreamDatabase) Close() error {
	// Timestream doesn't require explicit connection closing
	db.initialized = false
	return nil
}

// ReadTransaction implements the Database interface
func (db *TimestreamDatabase) ReadTransaction(ctx context.Context, accountID, uuid string, options *databases.ReadOptions) (*databases.Transaction, error) {
	if !db.initialized {
		return nil, errors.New("database not initialized")
	}

	// Build the query to fetch a specific transaction by UUID
	query := fmt.Sprintf(`
		SELECT uuid, account_id, time, measure_value::double AS amount, transaction_type, metadata
		FROM "%s"."%s"
		WHERE account_id = '%s' AND uuid = '%s'
		LIMIT 1
	`, db.databaseName, db.tableName, accountID, uuid)

	// Execute the query
	result, err := db.queryClient.Query(ctx, &timestreamquery.QueryInput{
		QueryString: aws.String(query),
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Check if we got a result
	if len(result.Rows) == 0 {
		return nil, fmt.Errorf("transaction not found")
	}

	// Parse the result
	row := result.Rows[0]
	if len(row.Data) < 6 {
		return nil, fmt.Errorf("invalid result format")
	}

	// Extract fields from the query result
	txUUID := *row.Data[0].ScalarValue
	txAccountID := *row.Data[1].ScalarValue
	txTimestamp, err := parseTimestreamTime(*row.Data[2].ScalarValue)
	if err != nil {
		return nil, err
	}
	txAmount, err := strconv.ParseFloat(*row.Data[3].ScalarValue, 64)
	if err != nil {
		return nil, err
	}
	txType := databases.TransactionType(*row.Data[4].ScalarValue)
	txMetadata := *row.Data[5].ScalarValue

	// Create and return the transaction
	transaction := &databases.Transaction{
		UUID:            txUUID,
		AccountID:       txAccountID,
		Timestamp:       txTimestamp,
		Amount:          txAmount,
		TransactionType: txType,
		Metadata:        txMetadata,
	}

	return transaction, nil
}

// WriteTransaction implements the Database interface
func (db *TimestreamDatabase) WriteTransaction(ctx context.Context, transaction *databases.Transaction, options *databases.WriteOptions) error {
	if !db.initialized {
		return errors.New("database not initialized")
	}

	if transaction == nil {
		return errors.New("transaction cannot be nil")
	}

	// Prepare record for Timestream
	record := types.Record{
		Dimensions: []types.Dimension{
			{
				Name:  aws.String("uuid"),
				Value: aws.String(transaction.UUID),
			},
			{
				Name:  aws.String("account_id"),
				Value: aws.String(transaction.AccountID),
			},
			{
				Name:  aws.String("transaction_type"),
				Value: aws.String(string(transaction.TransactionType)),
			},
			{
				Name:  aws.String("metadata"),
				Value: aws.String(fmt.Sprintf("%v", transaction.Metadata)),
			},
		},
		MeasureName:      aws.String("amount"),
		MeasureValue:     aws.String(fmt.Sprintf("%f", transaction.Amount)),
		MeasureValueType: types.MeasureValueTypeDouble,
		Time:             aws.String(strconv.FormatInt(transaction.Timestamp.UnixNano(), 10)),
		TimeUnit:         types.TimeUnitNanoseconds,
	}

	// Write the record to Timestream
	_, err := db.writeClient.WriteRecords(ctx, &timestreamwrite.WriteRecordsInput{
		DatabaseName: aws.String(db.databaseName),
		TableName:    aws.String(db.tableName),
		Records:      []types.Record{record},
	})
	if err != nil {
		return fmt.Errorf("failed to write record: %w", err)
	}

	return nil
}

// DeleteTransaction implements the Database interface
func (db *TimestreamDatabase) DeleteTransaction(ctx context.Context, accountID, uuid string) error {
	// Timestream doesn't support direct record deletion
	// Typically, time-series databases rely on retention policies for data management
	// This is a limitation of Timestream
	return fmt.Errorf("timestream does not support direct record deletion; use retention policies instead")
}

// QueryTransactionsByAccount implements the Database interface
func (db *TimestreamDatabase) QueryTransactionsByAccount(ctx context.Context, accountID string, options *databases.QueryOptions) ([]*databases.Transaction, error) {
	if !db.initialized {
		return nil, errors.New("database not initialized")
	}

	// Set default options if not provided
	limit := int64(100)
	if options != nil && options.Limit > 0 {
		limit = options.Limit
	}

	// Build the query
	// Note: Timestream doesn't directly support ScanIndexForward or ConsistentRead
	orderBy := "ASC" // Default sort order
	if options != nil && !options.ScanIndexForward {
		orderBy = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT uuid, account_id, time, measure_value::double AS amount, transaction_type, metadata
		FROM "%s"."%s"
		WHERE account_id = '%s'
		ORDER BY time %s
		LIMIT %d
	`, db.databaseName, db.tableName, accountID, orderBy, limit)

	// Execute the query
	result, err := db.queryClient.Query(ctx, &timestreamquery.QueryInput{
		QueryString: aws.String(query),
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Parse the results
	transactions := make([]*databases.Transaction, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row.Data) < 6 {
			continue // Skip invalid rows
		}

		// Extract fields
		txUUID := *row.Data[0].ScalarValue
		txAccountID := *row.Data[1].ScalarValue
		txTimestamp, err := parseTimestreamTime(*row.Data[2].ScalarValue)
		if err != nil {
			continue // Skip rows with invalid timestamps
		}
		txAmount, err := strconv.ParseFloat(*row.Data[3].ScalarValue, 64)
		if err != nil {
			continue // Skip rows with invalid amounts
		}
		txType := databases.TransactionType(*row.Data[4].ScalarValue)
		txMetadata := *row.Data[5].ScalarValue

		// Create transaction and add to results
		transaction := &databases.Transaction{
			UUID:            txUUID,
			AccountID:       txAccountID,
			Timestamp:       txTimestamp,
			Amount:          txAmount,
			TransactionType: txType,
			Metadata:        txMetadata,
		}
		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

// QueryTransactionsByTimeRange implements the Database interface
func (db *TimestreamDatabase) QueryTransactionsByTimeRange(ctx context.Context, accountID string, startTime, endTime time.Time, options *databases.QueryOptions) ([]*databases.Transaction, error) {
	if !db.initialized {
		return nil, errors.New("database not initialized")
	}

	// Set default options if not provided
	limit := int64(100)
	if options != nil && options.Limit > 0 {
		limit = options.Limit
	}

	// Build the query
	orderBy := "ASC" // Default sort order
	if options != nil && !options.ScanIndexForward {
		orderBy = "DESC"
	}

	// Convert timestamps to nanoseconds for Timestream
	startTimeNanos := startTime.UnixNano()
	endTimeNanos := endTime.UnixNano()

	query := fmt.Sprintf(`
		SELECT uuid, account_id, time, measure_value::double AS amount, transaction_type, metadata
		FROM "%s"."%s" 
		WHERE account_id = '%s'
		AND time BETWEEN %d AND %d
		ORDER BY time %s
		LIMIT %d
	`, db.databaseName, db.tableName, accountID, startTimeNanos, endTimeNanos, orderBy, limit)

	// Execute the query
	result, err := db.queryClient.Query(ctx, &timestreamquery.QueryInput{
		QueryString: aws.String(query),
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Parse the results
	transactions := make([]*databases.Transaction, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row.Data) < 6 {
			continue // Skip invalid rows
		}

		// Extract fields
		txUUID := *row.Data[0].ScalarValue
		txAccountID := *row.Data[1].ScalarValue
		txTimestamp, err := parseTimestreamTime(*row.Data[2].ScalarValue)
		if err != nil {
			continue // Skip rows with invalid timestamps
		}
		txAmount, err := strconv.ParseFloat(*row.Data[3].ScalarValue, 64)
		if err != nil {
			continue // Skip rows with invalid amounts
		}
		txType := databases.TransactionType(*row.Data[4].ScalarValue)
		txMetadata := *row.Data[5].ScalarValue

		// Create transaction and add to results
		transaction := &databases.Transaction{
			UUID:            txUUID,
			AccountID:       txAccountID,
			Timestamp:       txTimestamp,
			Amount:          txAmount,
			TransactionType: txType,
			Metadata:        txMetadata,
		}
		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

// BatchReadTransactions implements the Database interface
func (db *TimestreamDatabase) BatchReadTransactions(ctx context.Context, keys []struct{ AccountID, UUID string }, options *databases.BatchOptions) ([]*databases.Transaction, error) {
	if !db.initialized {
		return nil, errors.New("database not initialized")
	}

	if len(keys) == 0 {
		return []*databases.Transaction{}, nil
	}

	// Timestream does not have a native batch read API, so we'll implement this using multiple individual reads
	// For better performance in a production system, you might want to use a more sophisticated approach

	transactions := make([]*databases.Transaction, 0, len(keys))
	readOptions := &databases.ReadOptions{
		ConsistentRead: true,
	}

	// Use a simple sequential implementation for now
	for _, key := range keys {
		transaction, err := db.ReadTransaction(ctx, key.AccountID, key.UUID, readOptions)
		if err != nil {
			// Log the error but continue with other transactions
			fmt.Printf("Error reading transaction %s: %v\n", key.UUID, err)
			continue
		}
		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

// BatchWriteTransactions implements the Database interface
func (db *TimestreamDatabase) BatchWriteTransactions(ctx context.Context, transactions []*databases.Transaction, options *databases.BatchOptions) error {
	if !db.initialized {
		return errors.New("database not initialized")
	}

	if len(transactions) == 0 {
		return nil
	}

	// Timestream has a limit of 100 records per batch write
	const maxBatchSize = 100

	// Process transactions in batches
	for i := 0; i < len(transactions); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(transactions) {
			end = len(transactions)
		}
		batchTransactions := transactions[i:end]

		// Prepare the batch of records
		records := make([]types.Record, 0, len(batchTransactions))
		for _, transaction := range batchTransactions {
			record := types.Record{
				Dimensions: []types.Dimension{
					{
						Name:  aws.String("uuid"),
						Value: aws.String(transaction.UUID),
					},
					{
						Name:  aws.String("account_id"),
						Value: aws.String(transaction.AccountID),
					},
					{
						Name:  aws.String("transaction_type"),
						Value: aws.String(string(transaction.TransactionType)),
					},
					{
						Name:  aws.String("metadata"),
						Value: aws.String(fmt.Sprintf("%v", transaction.Metadata)),
					},
				},
				MeasureName:      aws.String("amount"),
				MeasureValue:     aws.String(fmt.Sprintf("%f", transaction.Amount)),
				MeasureValueType: types.MeasureValueTypeDouble,
				Time:             aws.String(strconv.FormatInt(transaction.Timestamp.UnixNano(), 10)),
				TimeUnit:         types.TimeUnitNanoseconds,
			}
			records = append(records, record)
		}

		// Write the batch to Timestream
		_, err := db.writeClient.WriteRecords(ctx, &timestreamwrite.WriteRecordsInput{
			DatabaseName: aws.String(db.databaseName),
			TableName:    aws.String(db.tableName),
			Records:      records,
		})
		if err != nil {
			return fmt.Errorf("failed to write batch: %w", err)
		}
	}

	return nil
}

// ExecuteTransactWrite implements the Database interface
func (db *TimestreamDatabase) ExecuteTransactWrite(ctx context.Context, transactions []*databases.Transaction) error {
	// Timestream does not support ACID transactions
	// We'll implement this as a batch write with no atomicity guarantees

	// This is a limitation of Timestream - it's optimized for high-throughput time-series data,
	// not for transactional workloads

	return db.BatchWriteTransactions(ctx, transactions, &databases.BatchOptions{})
}

// GetMetrics implements the Database interface
func (db *TimestreamDatabase) GetMetrics() map[string]interface{} {
	// Return a copy to avoid race conditions
	metrics := make(map[string]interface{})
	for k, v := range db.metrics {
		metrics[k] = v
	}
	return metrics
}

// ResetMetrics implements the Database interface
func (db *TimestreamDatabase) ResetMetrics() {
	db.metrics = map[string]interface{}{
		"readOperations":       0,
		"writeOperations":      0,
		"queryOperations":      0,
		"batchReadOperations":  0,
		"batchWriteOperations": 0,
		"failedOperations":     0,
		"totalOperations":      0,
		"totalDataPoints":      0,
		"averageReadLatency":   time.Duration(0),
		"averageWriteLatency":  time.Duration(0),
		"averageQueryLatency":  time.Duration(0),
	}
}

// Helper methods

// ensureDatabaseExists checks if the database exists and creates it if it doesn't
func (db *TimestreamDatabase) ensureDatabaseExists(ctx context.Context) error {
	// Try to describe the database to check if it exists
	_, err := db.writeClient.DescribeDatabase(ctx, &timestreamwrite.DescribeDatabaseInput{
		DatabaseName: aws.String(db.databaseName),
	})

	// If the database doesn't exist, create it
	if err != nil {
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			// Database doesn't exist, create it
			_, err = db.writeClient.CreateDatabase(ctx, &timestreamwrite.CreateDatabaseInput{
				DatabaseName: aws.String(db.databaseName),
			})
			if err != nil {
				return fmt.Errorf("failed to create database: %w", err)
			}
			return nil
		}
		return fmt.Errorf("error checking database existence: %w", err)
	}

	// Database exists
	return nil
}

// ensureTableExists checks if the table exists and creates it if it doesn't
func (db *TimestreamDatabase) ensureTableExists(ctx context.Context) error {
	// Try to describe the table to check if it exists
	_, err := db.writeClient.DescribeTable(ctx, &timestreamwrite.DescribeTableInput{
		DatabaseName: aws.String(db.databaseName),
		TableName:    aws.String(db.tableName),
	})

	// If the table doesn't exist, create it
	if err != nil {
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			// Table doesn't exist, create it with default retention settings
			_, err = db.writeClient.CreateTable(ctx, &timestreamwrite.CreateTableInput{
				DatabaseName: aws.String(db.databaseName),
				TableName:    aws.String(db.tableName),
				RetentionProperties: &types.RetentionProperties{
					MagneticStoreRetentionPeriodInDays: aws.Int64(30), // 30 days in magnetic store
					MemoryStoreRetentionPeriodInHours:  aws.Int64(24), // 24 hours in memory store
				},
			})
			if err != nil {
				return fmt.Errorf("failed to create table: %w", err)
			}
			return nil
		}
		return fmt.Errorf("error checking table existence: %w", err)
	}

	// Table exists
	return nil
}

// parseTimestreamTime converts a Timestream time string to a Go time.Time
func parseTimestreamTime(timeStr string) (time.Time, error) {
	// Try parsing as nanoseconds since epoch
	nanos, err := strconv.ParseInt(timeStr, 10, 64)
	if err == nil {
		return time.Unix(0, nanos), nil
	}

	// Try parsing as RFC3339
	t, err := time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return t, nil
	}

	// Try parsing as RFC3339Nano
	t, err = time.Parse(time.RFC3339Nano, timeStr)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("failed to parse timestamp: %s", timeStr)
}

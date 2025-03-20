package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/pedro-hbl/lambda-gopher-benchmark/pkg/databases"
)

// DynamoDBDatabase is an implementation of the Database interface for AWS DynamoDB
type DynamoDBDatabase struct {
	client      *dynamodb.Client
	tableName   string
	metrics     map[string]interface{}
	initialized bool
}

// DynamoDBConfig holds the configuration for a DynamoDB database
type DynamoDBConfig struct {
	Region          string
	TableName       string
	Endpoint        string
	ProvisionedRCUs int64
	ProvisionedWCUs int64
	CreateTable     bool
}

// DynamoDBFactory creates DynamoDB database instances
type DynamoDBFactory struct{}

// NewDynamoDBFactory creates a new DynamoDB factory
func NewDynamoDBFactory() *DynamoDBFactory {
	return &DynamoDBFactory{}
}

// CreateDatabase implements the DatabaseFactory interface
func (f *DynamoDBFactory) CreateDatabase(config map[string]interface{}) (databases.Database, error) {
	// Extract configuration
	dbConfig := DynamoDBConfig{
		Region:          "us-east-1", // Default region
		TableName:       "Transactions",
		ProvisionedRCUs: 5,
		ProvisionedWCUs: 5,
		CreateTable:     false,
	}

	if region, ok := config["region"].(string); ok {
		dbConfig.Region = region
	}
	if tableName, ok := config["tableName"].(string); ok {
		dbConfig.TableName = tableName
	}
	if endpoint, ok := config["endpoint"].(string); ok {
		dbConfig.Endpoint = endpoint
	}
	if rcus, ok := config["provisionedRCUs"].(int64); ok {
		dbConfig.ProvisionedRCUs = rcus
	}
	if wcus, ok := config["provisionedWCUs"].(int64); ok {
		dbConfig.ProvisionedWCUs = wcus
	}
	if createTable, ok := config["createTable"].(bool); ok {
		dbConfig.CreateTable = createTable
	}

	return NewDynamoDBDatabase(dbConfig)
}

// NewDynamoDBDatabase creates a new DynamoDB database instance
func NewDynamoDBDatabase(dbConfig DynamoDBConfig) (*DynamoDBDatabase, error) {
	db := &DynamoDBDatabase{
		tableName:   dbConfig.TableName,
		metrics:     make(map[string]interface{}),
		initialized: false,
	}

	// Create AWS configuration
	var err error

	// Fix AWS SDK configuration loading with renamed package and variable
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(dbConfig.Region))

	if dbConfig.Endpoint != "" {
		// Use a custom endpoint (e.g., for local DynamoDB)
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           dbConfig.Endpoint,
				SigningRegion: dbConfig.Region,
			}, nil
		})
		awsCfg.EndpointResolverWithOptions = customResolver
	}

	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	// Create DynamoDB client
	db.client = dynamodb.NewFromConfig(awsCfg)

	// Create table if requested
	if dbConfig.CreateTable {
		err = db.createTransactionTable(dbConfig.ProvisionedRCUs, dbConfig.ProvisionedWCUs)
		if err != nil {
			return nil, fmt.Errorf("failed to create table: %w", err)
		}
	}

	return db, nil
}

// Initialize implements the Database interface
func (db *DynamoDBDatabase) Initialize(ctx context.Context) error {
	if db.initialized {
		return nil
	}

	// Check if table exists
	_, err := db.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(db.tableName),
	})

	if err != nil {
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			return fmt.Errorf("table %s does not exist", db.tableName)
		}
		return fmt.Errorf("error checking table: %w", err)
	}

	db.initialized = true
	db.ResetMetrics()
	return nil
}

// Close implements the Database interface
func (db *DynamoDBDatabase) Close() error {
	// DynamoDB doesn't require explicit connection closing
	db.initialized = false
	return nil
}

// ReadTransaction implements the Database interface
func (db *DynamoDBDatabase) ReadTransaction(ctx context.Context, accountID, uuid string, options *databases.ReadOptions) (*databases.Transaction, error) {
	if !db.initialized {
		return nil, errors.New("database not initialized")
	}

	// Set default options if not provided
	if options == nil {
		options = &databases.ReadOptions{
			ConsistentRead: true,
		}
	}

	// Create GetItem input
	input := &dynamodb.GetItemInput{
		TableName: aws.String(db.tableName),
		Key: map[string]types.AttributeValue{
			"accountId": &types.AttributeValueMemberS{Value: accountID},
			"uuid":      &types.AttributeValueMemberS{Value: uuid},
		},
		ConsistentRead: aws.Bool(options.ConsistentRead),
	}

	// Execute GetItem operation
	result, err := db.client.GetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("GetItem operation failed: %w", err)
	}

	// Check if item exists
	if result.Item == nil || len(result.Item) == 0 {
		return nil, fmt.Errorf("transaction not found")
	}

	// Unmarshal DynamoDB item to Transaction struct
	var transaction databases.Transaction
	err = attributevalue.UnmarshalMap(result.Item, &transaction)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	return &transaction, nil
}

// WriteTransaction implements the Database interface
func (db *DynamoDBDatabase) WriteTransaction(ctx context.Context, transaction *databases.Transaction, options *databases.WriteOptions) error {
	if !db.initialized {
		return errors.New("database not initialized")
	}

	if transaction == nil {
		return errors.New("transaction cannot be nil")
	}

	// Marshal transaction to DynamoDB attribute values
	item, err := attributevalue.MarshalMap(transaction)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	// Create PutItem input
	input := &dynamodb.PutItemInput{
		TableName: aws.String(db.tableName),
		Item:      item,
	}

	// Add condition expression if provided
	if options != nil && options.Condition != "" {
		input.ConditionExpression = aws.String(options.Condition)
	}

	// Execute PutItem operation
	_, err = db.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("PutItem operation failed: %w", err)
	}

	return nil
}

// DeleteTransaction implements the Database interface
func (db *DynamoDBDatabase) DeleteTransaction(ctx context.Context, accountID, uuid string) error {
	if !db.initialized {
		return errors.New("database not initialized")
	}

	// Create DeleteItem input
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(db.tableName),
		Key: map[string]types.AttributeValue{
			"accountId": &types.AttributeValueMemberS{Value: accountID},
			"uuid":      &types.AttributeValueMemberS{Value: uuid},
		},
	}

	// Execute DeleteItem operation
	_, err := db.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("DeleteItem operation failed: %w", err)
	}

	return nil
}

// QueryTransactionsByAccount implements the Database interface
func (db *DynamoDBDatabase) QueryTransactionsByAccount(ctx context.Context, accountID string, options *databases.QueryOptions) ([]*databases.Transaction, error) {
	if !db.initialized {
		return nil, errors.New("database not initialized")
	}

	// Set default options if not provided
	if options == nil {
		options = &databases.QueryOptions{
			ScanIndexForward: true,
			ConsistentRead:   true,
			Limit:            100,
		}
	}

	// Create Query input
	input := &dynamodb.QueryInput{
		TableName:              aws.String(db.tableName),
		KeyConditionExpression: aws.String("accountId = :accountId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":accountId": &types.AttributeValueMemberS{Value: accountID},
		},
		ScanIndexForward: aws.Bool(options.ScanIndexForward),
		ConsistentRead:   aws.Bool(options.ConsistentRead),
	}

	if options.Limit > 0 {
		input.Limit = aws.Int32(int32(options.Limit))
	}

	// Execute Query operation
	result, err := db.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("Query operation failed: %w", err)
	}

	// Unmarshal items to Transaction structs
	transactions := make([]*databases.Transaction, 0, len(result.Items))
	for _, item := range result.Items {
		var transaction databases.Transaction
		err = attributevalue.UnmarshalMap(item, &transaction)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
		}
		transactions = append(transactions, &transaction)
	}

	return transactions, nil
}

// QueryTransactionsByTimeRange implements the Database interface
func (db *DynamoDBDatabase) QueryTransactionsByTimeRange(ctx context.Context, accountID string, startTime, endTime time.Time, options *databases.QueryOptions) ([]*databases.Transaction, error) {
	if !db.initialized {
		return nil, errors.New("database not initialized")
	}

	// Set default options if not provided
	if options == nil {
		options = &databases.QueryOptions{
			ScanIndexForward: true,
			ConsistentRead:   true,
			Limit:            100,
		}
	}

	// Format timestamps as ISO8601 strings
	startTimeStr := startTime.Format(time.RFC3339)
	endTimeStr := endTime.Format(time.RFC3339)

	// Create Query input
	input := &dynamodb.QueryInput{
		TableName:              aws.String(db.tableName),
		KeyConditionExpression: aws.String("accountId = :accountId AND timestamp BETWEEN :startTime AND :endTime"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":accountId": &types.AttributeValueMemberS{Value: accountID},
			":startTime": &types.AttributeValueMemberS{Value: startTimeStr},
			":endTime":   &types.AttributeValueMemberS{Value: endTimeStr},
		},
		ScanIndexForward: aws.Bool(options.ScanIndexForward),
		ConsistentRead:   aws.Bool(options.ConsistentRead),
	}

	if options.Limit > 0 {
		input.Limit = aws.Int32(int32(options.Limit))
	}

	// Execute Query operation
	result, err := db.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("Query operation failed: %w", err)
	}

	// Unmarshal items to Transaction structs
	transactions := make([]*databases.Transaction, 0, len(result.Items))
	for _, item := range result.Items {
		var transaction databases.Transaction
		err = attributevalue.UnmarshalMap(item, &transaction)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
		}
		transactions = append(transactions, &transaction)
	}

	return transactions, nil
}

// BatchReadTransactions implements the Database interface
func (db *DynamoDBDatabase) BatchReadTransactions(ctx context.Context, keys []struct{ AccountID, UUID string }, options *databases.BatchOptions) ([]*databases.Transaction, error) {
	if !db.initialized {
		return nil, errors.New("database not initialized")
	}

	if len(keys) == 0 {
		return []*databases.Transaction{}, nil
	}

	// Set default options if not provided
	maxBatchSize := 25 // DynamoDB BatchGetItem limit
	if options != nil && options.MaxBatchSize > 0 && options.MaxBatchSize < maxBatchSize {
		maxBatchSize = options.MaxBatchSize
	}

	var transactions []*databases.Transaction
	var unprocessedKeys []struct{ AccountID, UUID string }

	// Process keys in batches
	for i := 0; i < len(keys); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(keys) {
			end = len(keys)
		}
		batchKeys := keys[i:end]

		// Create BatchGetItem input
		keysMap := make([]map[string]types.AttributeValue, 0, len(batchKeys))
		for _, key := range batchKeys {
			keysMap = append(keysMap, map[string]types.AttributeValue{
				"accountId": &types.AttributeValueMemberS{Value: key.AccountID},
				"uuid":      &types.AttributeValueMemberS{Value: key.UUID},
			})
		}

		input := &dynamodb.BatchGetItemInput{
			RequestItems: map[string]types.KeysAndAttributes{
				db.tableName: {
					Keys: keysMap,
				},
			},
		}

		// Execute BatchGetItem operation
		result, err := db.client.BatchGetItem(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("BatchGetItem operation failed: %w", err)
		}

		// Process results
		if items, ok := result.Responses[db.tableName]; ok {
			for _, item := range items {
				var transaction databases.Transaction
				err = attributevalue.UnmarshalMap(item, &transaction)
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
				}
				transactions = append(transactions, &transaction)
			}
		}

		// Handle unprocessed keys
		if unprocessedKeyMap, ok := result.UnprocessedKeys[db.tableName]; ok && len(unprocessedKeyMap.Keys) > 0 {
			for _, keyMap := range unprocessedKeyMap.Keys {
				accountID := keyMap["accountId"].(*types.AttributeValueMemberS).Value
				uuid := keyMap["uuid"].(*types.AttributeValueMemberS).Value
				unprocessedKeys = append(unprocessedKeys, struct{ AccountID, UUID string }{accountID, uuid})
			}
		}
	}

	// Retry unprocessed keys if any (in a production implementation)
	// Here we just return what we have
	if len(unprocessedKeys) > 0 {
		return transactions, fmt.Errorf("%d keys were not processed", len(unprocessedKeys))
	}

	return transactions, nil
}

// BatchWriteTransactions implements the Database interface
func (db *DynamoDBDatabase) BatchWriteTransactions(ctx context.Context, transactions []*databases.Transaction, options *databases.BatchOptions) error {
	if !db.initialized {
		return errors.New("database not initialized")
	}

	if len(transactions) == 0 {
		return nil
	}

	// Set default options if not provided
	maxBatchSize := 25 // DynamoDB BatchWriteItem limit
	if options != nil && options.MaxBatchSize > 0 && options.MaxBatchSize < maxBatchSize {
		maxBatchSize = options.MaxBatchSize
	}

	var unprocessedItems []*databases.Transaction

	// Process transactions in batches
	for i := 0; i < len(transactions); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(transactions) {
			end = len(transactions)
		}
		batchTransactions := transactions[i:end]

		// Create BatchWriteItem input
		writeRequests := make([]types.WriteRequest, 0, len(batchTransactions))
		for _, transaction := range batchTransactions {
			item, err := attributevalue.MarshalMap(transaction)
			if err != nil {
				return fmt.Errorf("failed to marshal transaction: %w", err)
			}

			writeRequests = append(writeRequests, types.WriteRequest{
				PutRequest: &types.PutRequest{
					Item: item,
				},
			})
		}

		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				db.tableName: writeRequests,
			},
		}

		// Execute BatchWriteItem operation
		result, err := db.client.BatchWriteItem(ctx, input)
		if err != nil {
			return fmt.Errorf("BatchWriteItem operation failed: %w", err)
		}

		// Handle unprocessed items
		if unprocessedItemsMap, ok := result.UnprocessedItems[db.tableName]; ok && len(unprocessedItemsMap) > 0 {
			for _, writeRequest := range unprocessedItemsMap {
				if writeRequest.PutRequest != nil {
					var transaction databases.Transaction
					err = attributevalue.UnmarshalMap(writeRequest.PutRequest.Item, &transaction)
					if err != nil {
						return fmt.Errorf("failed to unmarshal unprocessed transaction: %w", err)
					}
					unprocessedItems = append(unprocessedItems, &transaction)
				}
			}
		}
	}

	// Retry unprocessed items if any (in a production implementation)
	// Here we just return an error
	if len(unprocessedItems) > 0 {
		return fmt.Errorf("%d transactions were not processed", len(unprocessedItems))
	}

	return nil
}

// ExecuteTransactWrite implements the Database interface
func (db *DynamoDBDatabase) ExecuteTransactWrite(ctx context.Context, transactions []*databases.Transaction) error {
	if !db.initialized {
		return errors.New("database not initialized")
	}

	if len(transactions) == 0 {
		return nil
	}

	// DynamoDB TransactWriteItems limit is 25
	if len(transactions) > 25 {
		return fmt.Errorf("too many transactions for a single transact write (limit is 25)")
	}

	// Create TransactWriteItems input
	transactItems := make([]types.TransactWriteItem, 0, len(transactions))
	for _, transaction := range transactions {
		item, err := attributevalue.MarshalMap(transaction)
		if err != nil {
			return fmt.Errorf("failed to marshal transaction: %w", err)
		}

		transactItems = append(transactItems, types.TransactWriteItem{
			Put: &types.Put{
				TableName: aws.String(db.tableName),
				Item:      item,
			},
		})
	}

	input := &dynamodb.TransactWriteItemsInput{
		TransactItems: transactItems,
	}

	// Execute TransactWriteItems operation
	_, err := db.client.TransactWriteItems(ctx, input)
	if err != nil {
		return fmt.Errorf("TransactWriteItems operation failed: %w", err)
	}

	return nil
}

// GetMetrics implements the Database interface
func (db *DynamoDBDatabase) GetMetrics() map[string]interface{} {
	// Return a copy to avoid race conditions
	metrics := make(map[string]interface{})
	for k, v := range db.metrics {
		metrics[k] = v
	}
	return metrics
}

// ResetMetrics implements the Database interface
func (db *DynamoDBDatabase) ResetMetrics() {
	db.metrics = map[string]interface{}{
		"readOperations":         0,
		"writeOperations":        0,
		"queryOperations":        0,
		"batchReadOperations":    0,
		"batchWriteOperations":   0,
		"transactionOperations":  0,
		"totalOperations":        0,
		"readCapacityUnits":      float64(0),
		"writeCapacityUnits":     float64(0),
		"failedOperations":       0,
		"throttledOperations":    0,
		"averageReadLatency":     time.Duration(0),
		"averageWriteLatency":    time.Duration(0),
		"averageQueryLatency":    time.Duration(0),
		"totalItemCount":         0,
		"totalDataSize":          int64(0),
		"largestItemSize":        int64(0),
		"smallestItemSize":       int64(0),
		"coldStartCount":         0,
		"connectionErrorCount":   0,
		"throttlingExceptions":   0,
		"conditionalCheckFailed": 0,
	}
}

// createTransactionTable creates a new DynamoDB table for transactions
func (db *DynamoDBDatabase) createTransactionTable(rcus, wcus int64) error {
	createTableInput := &dynamodb.CreateTableInput{
		TableName: aws.String(db.tableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("accountId"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("uuid"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("timestamp"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("accountId"),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String("uuid"),
				KeyType:       types.KeyTypeRange,
			},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("TimestampIndex"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("accountId"),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: aws.String("timestamp"),
						KeyType:       types.KeyTypeRange,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
				ProvisionedThroughput: &types.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(rcus),
					WriteCapacityUnits: aws.Int64(wcus),
				},
			},
		},
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(rcus),
			WriteCapacityUnits: aws.Int64(wcus),
		},
	}

	_, err := db.client.CreateTable(context.Background(), createTableInput)
	if err != nil {
		var alreadyExistsErr *types.ResourceInUseException
		if errors.As(err, &alreadyExistsErr) {
			// Table already exists, which is fine
			return nil
		}
		return err
	}

	// Wait for table to become active
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: aws.String(db.tableName),
	}

	waiter := dynamodb.NewTableExistsWaiter(db.client)
	err = waiter.Wait(context.Background(), describeTableInput, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to wait for table creation: %w", err)
	}

	return nil
}

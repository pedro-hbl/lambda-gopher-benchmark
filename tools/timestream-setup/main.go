package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite"
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite/types"
)

func main() {
	// Read environment variables
	region := getEnv("AWS_REGION", "us-east-1")
	endpoint := getEnv("TIMESTREAM_ENDPOINT", "")
	databaseName := getEnv("DB_DATABASE_NAME", "BenchmarkDB")
	tableName := getEnv("DB_TABLE_NAME", "Transactions")

	log.Printf("Setting up Timestream database: %s, table: %s", databaseName, tableName)

	// Configure AWS SDK
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		log.Fatalf("Unable to load SDK config: %v", err)
	}

	// Use a custom endpoint if provided (for LocalStack)
	if endpoint != "" {
		log.Printf("Using custom endpoint: %s", endpoint)
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           endpoint,
				SigningRegion: region,
			}, nil
		})
		cfg.EndpointResolverWithOptions = customResolver
	}

	// Create Timestream write client
	writeSvc := timestreamwrite.NewFromConfig(cfg)

	// Create database if it doesn't exist
	if err := createDatabaseIfNotExists(ctx, writeSvc, databaseName); err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	// Create table if it doesn't exist
	if err := createTableIfNotExists(ctx, writeSvc, databaseName, tableName); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	log.Println("Timestream setup completed successfully")
}

// createDatabaseIfNotExists creates the database if it doesn't already exist
func createDatabaseIfNotExists(ctx context.Context, client *timestreamwrite.Client, databaseName string) error {
	// Try to describe the database to check if it exists
	_, err := client.DescribeDatabase(ctx, &timestreamwrite.DescribeDatabaseInput{
		DatabaseName: aws.String(databaseName),
	})

	// If error is not nil, check if it's a ResourceNotFoundException
	if err != nil {
		if isResourceNotFound(err) {
			log.Printf("Database %s does not exist, creating...", databaseName)

			// Database doesn't exist, create it
			_, err = client.CreateDatabase(ctx, &timestreamwrite.CreateDatabaseInput{
				DatabaseName: aws.String(databaseName),
			})
			if err != nil {
				return fmt.Errorf("failed to create database: %w", err)
			}
			log.Printf("Database %s created successfully", databaseName)
			return nil
		}
		return fmt.Errorf("error checking database existence: %w", err)
	}

	log.Printf("Database %s already exists", databaseName)
	return nil
}

// createTableIfNotExists creates the table if it doesn't already exist
func createTableIfNotExists(ctx context.Context, client *timestreamwrite.Client, databaseName, tableName string) error {
	// Try to describe the table to check if it exists
	_, err := client.DescribeTable(ctx, &timestreamwrite.DescribeTableInput{
		DatabaseName: aws.String(databaseName),
		TableName:    aws.String(tableName),
	})

	// If error is not nil, check if it's a ResourceNotFoundException
	if err != nil {
		if isResourceNotFound(err) {
			log.Printf("Table %s does not exist in database %s, creating...", tableName, databaseName)

			// Table doesn't exist, create it
			_, err = client.CreateTable(ctx, &timestreamwrite.CreateTableInput{
				DatabaseName: aws.String(databaseName),
				TableName:    aws.String(tableName),
				RetentionProperties: &types.RetentionProperties{
					MagneticStoreRetentionPeriodInDays: aws.Int64(30), // 30 days in magnetic store
					MemoryStoreRetentionPeriodInHours:  aws.Int64(24), // 24 hours in memory store
				},
			})
			if err != nil {
				return fmt.Errorf("failed to create table: %w", err)
			}
			log.Printf("Table %s created successfully", tableName)
			return nil
		}
		return fmt.Errorf("error checking table existence: %w", err)
	}

	log.Printf("Table %s already exists in database %s", tableName, databaseName)
	return nil
}

// isResourceNotFound checks if an error is a ResourceNotFoundException
func isResourceNotFound(err error) bool {
	return err != nil && (isErrorWithCode(err, "ResourceNotFoundException") ||
		isErrorWithCode(err, "ValidationException") ||
		isErrorWithCode(err, "InvalidEndpointException"))
}

// isErrorWithCode checks if an error contains a specific code
func isErrorWithCode(err error, code string) bool {
	errStr := err.Error()
	return errStr != "" && (errStr == code ||
		errStr == "operation error Timestream: CreateDatabase, "+code ||
		errStr == "operation error Timestream: CreateTable, "+code)
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// retry retries a function with exponential backoff
func retry(attempts int, sleep time.Duration, f func() error) error {
	if err := f(); err != nil {
		if attempts--; attempts > 0 {
			log.Printf("Retrying after error: %v", err)
			time.Sleep(sleep)
			return retry(attempts, 2*sleep, f)
		}
		return err
	}
	return nil
}

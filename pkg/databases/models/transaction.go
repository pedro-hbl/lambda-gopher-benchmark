package models

import (
	"time"
)

// Transaction represents a financial transaction record
type Transaction struct {
	// UUID is the unique identifier for the transaction
	UUID string `json:"uuid" dynamodbav:"UUID"`

	// AccountID is the identifier for the account that owns this transaction
	AccountID string `json:"accountId" dynamodbav:"AccountID"`

	// Timestamp when the transaction occurred
	Timestamp time.Time `json:"timestamp" dynamodbav:"Timestamp"`

	// Amount of the transaction
	Amount float64 `json:"amount" dynamodbav:"Amount"`

	// TransactionType categorizes the transaction (e.g., deposit, withdrawal)
	TransactionType string `json:"transactionType" dynamodbav:"TransactionType"`

	// Metadata contains additional transaction data (used for benchmarking with variable sizes)
	Metadata []byte `json:"metadata" dynamodbav:"Metadata"`
}

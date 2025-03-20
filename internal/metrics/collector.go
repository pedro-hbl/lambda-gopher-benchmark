package metrics

import (
	"fmt"
	"sync"
	"time"
)

// OperationType represents the type of database operation being measured
type OperationType string

const (
	// ReadOperation represents a read from the database
	ReadOperation OperationType = "READ"
	// WriteOperation represents a write to the database
	WriteOperation OperationType = "WRITE"
	// QueryOperation represents a query operation
	QueryOperation OperationType = "QUERY"
	// BatchOperation represents a batch operation
	BatchOperation OperationType = "BATCH"
	// TransactionOperation represents a transaction operation
	TransactionOperation OperationType = "TRANSACTION"
)

// TestResult stores the metrics for a complete test run
type TestResult struct {
	TestName    string                 `json:"testName"`
	Description string                 `json:"description"`
	Database    string                 `json:"database"`
	Config      map[string]interface{} `json:"config"`
	Parameters  map[string]interface{} `json:"parameters"`
	StartTime   time.Time              `json:"startTime"`
	EndTime     time.Time              `json:"endTime"`
	Duration    time.Duration          `json:"duration"`
	Operations  []*OperationMetric     `json:"operations"`
	Summary     map[string]interface{} `json:"summary"`
}

// OperationMetric represents metrics for a single operation
type OperationMetric struct {
	Type          OperationType          `json:"type"`
	StartTime     time.Time              `json:"startTime"`
	EndTime       time.Time              `json:"endTime"`
	Duration      time.Duration          `json:"duration"`
	ItemCount     int64                  `json:"itemCount"`
	ByteCount     int64                  `json:"byteCount"`
	IsColdStart   bool                   `json:"isColdStart"`
	Error         error                  `json:"error,omitempty"`
	ErrorMessage  string                 `json:"errorMessage,omitempty"`
	CustomMetrics map[string]interface{} `json:"customMetrics,omitempty"`
}

// Collector collects and organizes metrics for benchmark tests
type Collector struct {
	mu          sync.Mutex
	currentTest *TestResult
	tests       map[string]*TestResult
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		tests: make(map[string]*TestResult),
	}
}

// StartTest begins a new test and sets it as the current test
func (c *Collector) StartTest(name, description, database string, config, parameters map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.currentTest = &TestResult{
		TestName:    name,
		Description: description,
		Database:    database,
		Config:      config,
		Parameters:  parameters,
		StartTime:   time.Now(),
		Operations:  make([]*OperationMetric, 0),
		Summary:     make(map[string]interface{}),
	}

	c.tests[name] = c.currentTest
}

// MeasureOperation measures a single operation and returns any error from the operation
func (c *Collector) MeasureOperation(
	opType OperationType,
	itemCount int64,
	byteCount int64,
	isColdStart bool,
	operation func() error,
) error {
	if operation == nil {
		return fmt.Errorf("operation function cannot be nil")
	}

	c.mu.Lock()
	if c.currentTest == nil {
		c.mu.Unlock()
		return fmt.Errorf("no test is currently running")
	}
	c.mu.Unlock()

	metric := &OperationMetric{
		Type:        opType,
		StartTime:   time.Now(),
		ItemCount:   itemCount,
		ByteCount:   byteCount,
		IsColdStart: isColdStart,
	}

	err := operation()
	metric.EndTime = time.Now()
	metric.Duration = metric.EndTime.Sub(metric.StartTime)

	if err != nil {
		metric.Error = err
		metric.ErrorMessage = err.Error()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.currentTest != nil {
		c.currentTest.Operations = append(c.currentTest.Operations, metric)
	}

	return err
}

// AddCustomMetric adds a custom metric to the current test
func (c *Collector) AddCustomMetric(name string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.currentTest == nil {
		return fmt.Errorf("no test is currently running")
	}

	c.currentTest.Summary[name] = value
	return nil
}

// EndTest completes the current test, calculates summary metrics, and returns the result
func (c *Collector) EndTest(testName string) *TestResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	test, exists := c.tests[testName]
	if !exists || test != c.currentTest {
		return nil
	}

	test.EndTime = time.Now()
	test.Duration = test.EndTime.Sub(test.StartTime)

	// Calculate summary metrics
	var totalDuration time.Duration
	var totalItems, totalBytes int64
	var successCount, errorCount int64
	var coldStartCount int64

	for _, op := range test.Operations {
		totalDuration += op.Duration
		totalItems += op.ItemCount
		totalBytes += op.ByteCount

		if op.Error != nil {
			errorCount++
		} else {
			successCount++
		}

		if op.IsColdStart {
			coldStartCount++
		}
	}

	opCount := int64(len(test.Operations))

	// Populate summary metrics
	if opCount > 0 {
		test.Summary["operationCount"] = opCount
		test.Summary["totalDuration"] = totalDuration.Nanoseconds()
		test.Summary["avgDuration"] = totalDuration.Nanoseconds() / opCount
		test.Summary["totalItems"] = totalItems
		test.Summary["totalBytes"] = totalBytes
		test.Summary["successCount"] = successCount
		test.Summary["errorCount"] = errorCount
		test.Summary["successRate"] = float64(successCount) / float64(opCount)
		test.Summary["throughputItems"] = float64(totalItems) / test.Duration.Seconds()
		test.Summary["throughputBytes"] = float64(totalBytes) / test.Duration.Seconds()
		test.Summary["coldStartCount"] = coldStartCount

		// Calculate percentiles if we have enough data
		if opCount >= 10 {
			durations := make([]int64, 0, opCount)
			for _, op := range test.Operations {
				durations = append(durations, op.Duration.Nanoseconds())
			}

			// Sort the durations
			for i := int64(0); i < opCount; i++ {
				for j := i + 1; j < opCount; j++ {
					if durations[i] > durations[j] {
						durations[i], durations[j] = durations[j], durations[i]
					}
				}
			}

			// Calculate percentiles
			test.Summary["p50"] = durations[opCount*50/100]
			test.Summary["p90"] = durations[opCount*90/100]
			test.Summary["p99"] = durations[opCount*99/100]
		}
	}

	// Clear current test if this is the one that was active
	if c.currentTest == test {
		c.currentTest = nil
	}

	return test
}

// GetTestResult retrieves a test result by name
func (c *Collector) GetTestResult(name string) *TestResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.tests[name]
}

// ResetCollector clears all test data
func (c *Collector) ResetCollector() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.currentTest = nil
	c.tests = make(map[string]*TestResult)
}

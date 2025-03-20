package operations

import (
	"fmt"
)

// OperationFactory creates operation instances based on type
type OperationFactory struct {
	builders map[string]func(map[string]interface{}) Operation
}

// NewOperationFactory creates a new operation factory
func NewOperationFactory() *OperationFactory {
	factory := &OperationFactory{
		builders: make(map[string]func(map[string]interface{}) Operation),
	}

	// Register standard operations
	factory.Register("read", func(params map[string]interface{}) Operation {
		return NewReadOperation(params, getParam(params, "parallel", false))
	})
	factory.Register("write", func(params map[string]interface{}) Operation {
		return NewWriteOperation(params, getParam(params, "batch", false))
	})
	factory.Register("query", func(params map[string]interface{}) Operation {
		return NewQueryOperation(params)
	})

	// Register ImmuDB-specific operations
	factory.Register("immudb_write", func(params map[string]interface{}) Operation {
		return NewImmuDBWriteOperation(params)
	})
	factory.Register("immudb_read", func(params map[string]interface{}) Operation {
		return NewImmuDBReadOperation(params)
	})
	factory.Register("immudb_query", func(params map[string]interface{}) Operation {
		return NewImmuDBQueryOperation(params)
	})

	return factory
}

// Register adds a new operation builder to the factory
func (f *OperationFactory) Register(opType string, builder func(map[string]interface{}) Operation) {
	f.builders[opType] = builder
}

// CreateOperation creates a new operation instance based on type
func (f *OperationFactory) CreateOperation(opType string, params map[string]interface{}) (Operation, error) {
	builder, ok := f.builders[opType]
	if !ok {
		return nil, fmt.Errorf("unknown operation type: %s", opType)
	}
	return builder(params), nil
}

package ch

import (
	"fmt"
)

var (
	// ErrBufferFull when buffer is full, please retry later
	ErrBufferFull = fmt.Errorf("ch: buffer is full, please retry later")

	// ErrWriterClosed when writer is closed
	ErrWriterClosed = fmt.Errorf("ch: writer is closed")

	// ErrConnectionClosed when connection is closed
	ErrConnectionClosed = fmt.Errorf("ch: connection is closed")

	// ErrInvalidTable invalid table name
	ErrInvalidTable = fmt.Errorf("ch: invalid table name")

	// ErrWriterDisabled when writer is not enabled (WriterConfig is nil)
	ErrWriterDisabled = fmt.Errorf("ch: writer is disabled, please set WriterConfig to enable")
)

// ErrInvalidConfig invalid config
func ErrInvalidConfig(msg string) error {
	return fmt.Errorf("ch: invalid config: %s", msg)
}

// ErrConnection ClickHouse connection error
func ErrConnection(err error) error {
	return fmt.Errorf("ch: connection failed: %w", err)
}

// ErrInsert insert error
func ErrInsert(tableName TableName, err error) error {
	return fmt.Errorf("ch: insert to table %s failed: %w", tableName, err)
}

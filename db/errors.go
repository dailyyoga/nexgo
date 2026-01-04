package db

import "fmt"

var (
	// ErrConnectionNotEstablished database connection not established
	ErrConnectionNotEstablished = fmt.Errorf("db: database connection not established")
)

// ErrInvalidConfig invalid config
func ErrInvalidConfig(msg string) error {
	return fmt.Errorf("db: invalid config: %s", msg)
}

// ErrConnection database connection error
func ErrConnection(err error) error {
	return fmt.Errorf("db: connection failed: %w", err)
}

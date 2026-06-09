package dlq

import "fmt"

// ErrInvalidConfig DLQ configuration error
func ErrInvalidConfig(msg string) error {
	return fmt.Errorf("dlq: invalid config: %s", msg)
}

package logger

import "fmt"

// ErrBuildLogger represents an error building the logger
func ErrBuildLogger(err error) error {
	return fmt.Errorf("logger: failed to build logger: %w", err)
}

// ErrInvalidLevel represents an invalid log level error
func ErrInvalidLevel(level string, err error) error {
	return fmt.Errorf("logger: invalid level %q: %w", level, err)
}

// ErrInvalidEncoding represents an invalid encoding error
func ErrInvalidEncoding(encoding string) error {
	return fmt.Errorf("logger: invalid encoding %q, must be 'json' or 'console'", encoding)
}

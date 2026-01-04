package logger

import (
	"fmt"
	"slices"
	"strings"
)

var (
	validLevels    = []string{"debug", "info", "warn", "error", "dpanic", "panic", "fatal"}
	validEncodings = []string{"json", "console"}
)

// Config is the configuration for the logger
type Config struct {
	// Level, debug, info, warn, error, dpanic, panic, fatal
	// default: "info"
	Level string `mapstructure:"level"`
	// Encoding, json or console
	// default: "json"
	Encoding string `mapstructure:"encoding"`
	// Output paths
	// default: []string{"stdout"}
	OutputPaths []string `mapstructure:"output_paths"`
	// Error output paths
	// default: []string{"stderr"}
	ErrorOutputPaths []string `mapstructure:"error_output_paths"`
}

// DefaultConfig returns the default configuration for the logger
// It is used to initialize the logger with the default configuration when no configuration is provided
func DefaultConfig() *Config {
	return &Config{
		Level:            "info",
		Encoding:         "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
}

// Validate validates the configuration for the logger
func (c *Config) Validate() error {
	if !slices.Contains(validLevels, c.Level) {
		return ErrInvalidLevel(c.Level, fmt.Errorf("must be one of: %s", strings.Join(validLevels, ", ")))
	}
	if !slices.Contains(validEncodings, c.Encoding) {
		return ErrInvalidEncoding(c.Encoding)
	}
	return nil
}

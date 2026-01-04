package cache

import "time"

// SyncableCacheConfig holds configuration for SyncableCache
type SyncableCacheConfig struct {
	// Name is used for logging purposes to identify the cache (required)
	Name string `mapstructure:"name"`
	// SyncInterval is the interval between periodic sync operations
	// default: 5 * time.Minute
	SyncInterval time.Duration `mapstructure:"sync_interval"`
	// SyncTimeout is the timeout for each sync operation
	// default: 30 * time.Second
	SyncTimeout time.Duration `mapstructure:"sync_timeout"`
	// MaxRetries is the maximum number of retry attempts for failed sync operations
	// default: 3
	MaxRetries int `mapstructure:"max_retries"`
}

// DefaultSyncableCacheConfig returns the default configuration for SyncableCache
// It is used to initialize the cache with default configuration when no configuration is provided
// Note: Name field has no default value and must be explicitly set by the user
func DefaultSyncableCacheConfig() *SyncableCacheConfig {
	return &SyncableCacheConfig{
		// Name is required and must be explicitly set
		SyncInterval: 5 * time.Minute,
		SyncTimeout:  30 * time.Second,
		MaxRetries:   3,
	}
}

// Validate validates the configuration
// It checks that all required fields are set and have valid values
func (c *SyncableCacheConfig) Validate() error {
	if c.Name == "" {
		return ErrInvalidName(c.Name)
	}
	if c.SyncInterval <= 0 {
		return ErrInvalidSyncInterval(c.SyncInterval)
	}
	if c.SyncTimeout <= 0 {
		return ErrInvalidSyncTimeout(c.SyncTimeout)
	}
	if c.MaxRetries < 1 {
		return ErrInvalidMaxRetries(c.MaxRetries)
	}
	return nil
}

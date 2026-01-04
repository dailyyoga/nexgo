package cache

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/dailyyoga/nexgo/logger"
	"github.com/dailyyoga/nexgo/routine"
	"go.uber.org/zap"
)

// syncableCache is a generic cache that periodically syncs data from a source
// It implements the SyncableCache interface
type syncableCache[T any] struct {
	// Dependencies
	logger   logger.Logger
	syncFunc SyncFunc[T]

	// Configuration
	name         string
	syncInterval time.Duration
	syncTimeout  time.Duration
	maxRetries   int

	// Runtime state
	mu     sync.RWMutex
	cache  T
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once // Ensures Stop is only executed once
}

// NewSyncableCache creates a new syncable cache
// It returns an error if the configuration is invalid
// The returned SyncableCache must have Start() called before use
func NewSyncableCache[T any](
	log logger.Logger,
	cfg *SyncableCacheConfig,
	syncFunc SyncFunc[T],
) (SyncableCache[T], error) {
	// Use default config if not provided
	if cfg == nil {
		cfg = DefaultSyncableCacheConfig()
	} else {
		// Merge with defaults for zero values
		defaultCfg := DefaultSyncableCacheConfig()
		if cfg.SyncInterval == 0 {
			cfg.SyncInterval = defaultCfg.SyncInterval
		}
		if cfg.SyncTimeout == 0 {
			cfg.SyncTimeout = defaultCfg.SyncTimeout
		}
		if cfg.MaxRetries == 0 {
			cfg.MaxRetries = defaultCfg.MaxRetries
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if syncFunc == nil {
		return nil, ErrInvalidConfig
	}

	return &syncableCache[T]{
		logger:       log,
		syncFunc:     syncFunc,
		name:         cfg.Name,
		syncInterval: cfg.SyncInterval,
		syncTimeout:  cfg.SyncTimeout,
		maxRetries:   cfg.MaxRetries,
	}, nil
}

// Start starts the periodic sync process
// It performs an initial sync before starting the background goroutine
// Returns error if initial sync fails
func (sc *syncableCache[T]) Start() error {
	sc.ctx, sc.cancel = context.WithCancel(context.Background())

	// Initial load
	if err := sc.sync(sc.ctx); err != nil {
		sc.cancel()
		return ErrSync(err)
	}

	// Start periodic sync
	routine.GoNamedWithContext(sc.ctx, sc.logger, sc.name+"-sync", func(ctx context.Context) {
		ticker := time.NewTicker(sc.syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := sc.sync(ctx); err != nil {
					sc.logger.Error("periodic sync failed",
						zap.String("cache", sc.name),
						zap.Error(err),
					)
				}
			case <-ctx.Done():
				sc.logger.Info("stopping sync", zap.String("cache", sc.name))
				return
			}
		}
	})

	return nil
}

// Stop stops the periodic sync
// It can be called multiple times safely
func (sc *syncableCache[T]) Stop() {
	sc.once.Do(func() {
		if sc.cancel != nil {
			sc.cancel()
		}
	})
}

// Get returns the current cache value
// It is safe to call concurrently with sync operations
//
// IMPORTANT: For reference types (slice, map, pointer, chan), this method returns
// a reference to the internal cache data, not a deep copy. The returned value
// MUST be treated as read-only. Modifying it will cause data races.
//
// See the SyncableCache interface documentation for detailed usage examples.
func (sc *syncableCache[T]) Get() T {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.cache
}

// Sync manually triggers a sync operation
// The context can be used to set timeout or cancel the operation
// Returns error if sync fails after all retry attempts
func (sc *syncableCache[T]) Sync(ctx context.Context) error {
	return sc.sync(ctx)
}

// sync performs the sync operation with retry logic
func (sc *syncableCache[T]) sync(ctx context.Context) error {
	return sc.syncWithRetry(ctx, sc.maxRetries)
}

// syncWithRetry performs sync with exponential backoff retry
func (sc *syncableCache[T]) syncWithRetry(ctx context.Context, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Exponential backoff: 1s, 2s, 4s, 8s, ...
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			sc.logger.Warn("retrying sync after backoff",
				zap.String("cache", sc.name),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
			)
			time.Sleep(backoff)
		}

		// Execute sync with timeout
		syncCtx, cancel := context.WithTimeout(ctx, sc.syncTimeout)
		data, err := sc.syncFunc(syncCtx)
		cancel()

		if err == nil {
			sc.mu.Lock()
			sc.cache = data
			sc.mu.Unlock()
			sc.logger.Debug("sync completed successfully",
				zap.String("cache", sc.name),
				zap.Int("attempt", attempt+1),
			)
			return nil
		}

		lastErr = err

		// Determine if retry is possible
		if !isRetryableError(err) {
			sc.logger.Error("non-retryable sync error",
				zap.String("cache", sc.name),
				zap.Error(err),
			)
			return ErrSync(err)
		}

		sc.logger.Warn("sync failed, will retry",
			zap.String("cache", sc.name),
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", maxRetries),
		)
	}

	return ErrSync(lastErr)
}

// isRetryableError checks if an error is retryable
// It checks for common transient errors like timeouts and connection issues
func isRetryableError(err error) bool {
	// Check for context deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check for common retryable error messages
	errStr := err.Error()
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"timeout",
		"too many connections",
		"temporary failure",
		"network is unreachable",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	return false
}

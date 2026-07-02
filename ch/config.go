package ch

import (
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type Config struct {
	// clickhouse connection config
	Hosts       []string      `mapstructure:"hosts"`
	Database    string        `mapstructure:"database"`
	Username    string        `mapstructure:"username"`
	Password    string        `mapstructure:"password"`
	DialTimeout time.Duration `mapstructure:"dial_timeout"`
	Debug       bool          `mapstructure:"debug"`
	// clickhouse settings (https://clickhouse.com/docs/zh/operations/settings/settings)
	Settings clickhouse.Settings `mapstructure:"settings"`
	// batch insert config
	WriterConfig *WriterConfig `mapstructure:"writer"`
}

type WriterConfig struct {
	FlushInterval time.Duration `mapstructure:"flush_interval"`
	FlushSize     int           `mapstructure:"flush_size"`
	// MinFlushSize is the minimum batch size for time-triggered flush.
	// Time-triggered flush will only execute when buffer size >= MinFlushSize.
	// This helps reduce small batch writes to ClickHouse.
	// Set to 0 to disable this feature (flush on every interval).
	MinFlushSize int `mapstructure:"min_flush_size"`
	// MaxWaitTime is the maximum time to wait before flushing, regardless of buffer size.
	// This ensures data freshness even during low traffic periods.
	// Set to 0 to disable this feature (wait indefinitely for MinFlushSize).
	MaxWaitTime time.Duration `mapstructure:"max_wait_time"`
	// table schema cache refresh interval, 0 means not auto refresh
	SchemaRefreshInterval time.Duration `mapstructure:"schema_refresh_interval"`
	// MaxRetries is the maximum number of retries for transient errors during batch insert.
	// Default is 3. Set to 0 to use default.
	MaxRetries int `mapstructure:"max_retries"`
	// OnPermanentFailure is invoked when a batch of rows exhausts all retries and
	// is about to be dropped (optional). When nil the old behaviour is preserved
	// (log only, then drop the whole batch; see writer.go flush()).
	// It has no mapstructure tag on purpose: it is a function injected by code at
	// construction time, not loaded from YAML, so viper/mapstructure ignores it.
	// The callback must be non-blocking and must not let a panic escape; callers
	// should dump the failed rows to a DLQ (or other side channel) inside it.
	OnPermanentFailure func(table TableName, rows []Table, err error) `mapstructure:"-"`
	// MetricsHook, when set, receives per-table flush observability events (batch
	// size, flush duration, success/failure). It is prometheus-free (the adapter
	// lives in nexgo/metrics/chmetrics) and orthogonal to OnPermanentFailure:
	// that callback routes failed rows to a dead-letter sink (business concern),
	// whereas this hook only measures flush outcomes. Like OnPermanentFailure it
	// carries no mapstructure tag — it is injected by code, not loaded from YAML —
	// and must be non-blocking and panic-free. See WriterMetricsHook.
	MetricsHook WriterMetricsHook `mapstructure:"-"`
}

func DefaultConfig() *Config {
	return &Config{
		Database:    "default",
		DialTimeout: 10 * time.Second,
		Debug:       false,
	}
}

// DefaultWriterConfig returns the default writer config
func DefaultWriterConfig() *WriterConfig {
	return &WriterConfig{
		FlushInterval:         10 * time.Second,
		FlushSize:             5000,
		MinFlushSize:          500,
		MaxWaitTime:           60 * time.Second,
		SchemaRefreshInterval: 5 * time.Minute,
		MaxRetries:            3,
	}
}

func (c *Config) Validate() error {
	if len(c.Hosts) == 0 {
		return ErrInvalidConfig("hosts are required")
	}
	if c.Username == "" {
		return ErrInvalidConfig("username is required")
	}
	if c.Password == "" {
		return ErrInvalidConfig("password is required")
	}

	// validate writer config only if it's set
	if c.WriterConfig != nil {
		if c.WriterConfig.FlushInterval <= 0 {
			return ErrInvalidConfig("writer.flush_interval is required")
		}
		if c.WriterConfig.FlushSize <= 0 {
			return ErrInvalidConfig("writer.flush_size is required")
		}
		if c.WriterConfig.MinFlushSize < 0 {
			return ErrInvalidConfig("writer.min_flush_size cannot be negative")
		}
		if c.WriterConfig.MinFlushSize > c.WriterConfig.FlushSize {
			return ErrInvalidConfig("writer.min_flush_size cannot be greater than writer.flush_size")
		}
		if c.WriterConfig.MaxWaitTime < 0 {
			return ErrInvalidConfig("writer.max_wait_time cannot be negative")
		}
		if c.WriterConfig.MaxRetries < 0 {
			return ErrInvalidConfig("writer.max_retries cannot be negative")
		}
	}
	return nil
}

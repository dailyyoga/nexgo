package db

import (
	"fmt"
	"slices"
	"strings"
	"time"

	glogger "gorm.io/gorm/logger"
)

// BaseConfig holds the connection and pool fields shared by every supported
// SQL driver. Driver-specific configs (Config for MySQL, PostgresConfig for
// PostgreSQL, ...) embed BaseConfig with `mapstructure:",squash"` so YAML
// payloads stay flat and field access via promotion (cfg.Host) is preserved.
type BaseConfig struct {
	// Host is the host of the database
	Host string `mapstructure:"host"`
	// Port is the port of the database (driver default applied via MergeDefaults)
	Port int `mapstructure:"port"`
	// User is the user of the database
	User string `mapstructure:"user"`
	// Password is the password of the database
	Password string `mapstructure:"password"`
	// Database is the name of the database
	Database string `mapstructure:"database"`
	// MaxOpenConns is the maximum number of open connections to the database
	// default: 25
	MaxOpenConns int `mapstructure:"max_open_conns"`
	// MaxIdleConns is the maximum number of idle connections to the database
	// default: 10
	MaxIdleConns int `mapstructure:"max_idle_conns"`
	// ConnMaxLifetime is the maximum lifetime of a connection
	// default: 1800 * time.Second
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	// ConnMaxIdleTime is the maximum idle time of a connection
	// default: 600 * time.Second
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	// LogLevel is the log level of the database
	// default: "warn"
	LogLevel string `mapstructure:"log_level"`
	// SlowThreshold is the threshold for slow queries
	// default: 1 * time.Second
	SlowThreshold time.Duration `mapstructure:"slow_threshold"`
}

// shared default values for the pool / logger fields. Port is excluded because
// the canonical value differs between drivers (3306 vs 5432).
var basePoolDefaults = BaseConfig{
	MaxOpenConns:    25,
	MaxIdleConns:    10,
	ConnMaxLifetime: 1800 * time.Second,
	ConnMaxIdleTime: 600 * time.Second,
	LogLevel:        "warn",
	SlowThreshold:   1 * time.Second,
}

// mergePoolDefaults fills in zero-valued pool / logger fields. The driver's
// MergeDefaults wrapper is responsible for the Port default and any
// driver-specific fields.
func (b *BaseConfig) mergePoolDefaults() {
	if b.MaxOpenConns == 0 {
		b.MaxOpenConns = basePoolDefaults.MaxOpenConns
	}
	if b.MaxIdleConns == 0 {
		b.MaxIdleConns = basePoolDefaults.MaxIdleConns
	}
	if b.ConnMaxLifetime == 0 {
		b.ConnMaxLifetime = basePoolDefaults.ConnMaxLifetime
	}
	if b.ConnMaxIdleTime == 0 {
		b.ConnMaxIdleTime = basePoolDefaults.ConnMaxIdleTime
	}
	if b.LogLevel == "" {
		b.LogLevel = basePoolDefaults.LogLevel
	}
	if b.SlowThreshold == 0 {
		b.SlowThreshold = basePoolDefaults.SlowThreshold
	}
}

// validateRequired checks the connection-identity fields every driver needs.
func (b *BaseConfig) validateRequired() error {
	if b.Host == "" {
		return ErrInvalidConfig("host is required")
	}
	if b.Port <= 0 {
		return ErrInvalidConfig("port is required")
	}
	if b.User == "" {
		return ErrInvalidConfig("user is required")
	}
	if b.Password == "" {
		return ErrInvalidConfig("password is required")
	}
	if b.Database == "" {
		return ErrInvalidConfig("database is required")
	}
	return nil
}

var validLogLevels = []string{"silent", "error", "warn", "info"}

// validateLogLevel checks LogLevel against the gorm-supported set.
func (b *BaseConfig) validateLogLevel() error {
	if !slices.ContainsFunc(validLogLevels, func(level string) bool {
		return strings.EqualFold(b.LogLevel, level)
	}) {
		return ErrInvalidConfig(fmt.Sprintf(
			"log_level %q must be one of: %s",
			b.LogLevel, strings.Join(validLogLevels, ", "),
		))
	}
	return nil
}

// gormLogLevel maps the textual LogLevel onto gorm's enum.
func (b *BaseConfig) gormLogLevel() glogger.LogLevel {
	switch strings.ToLower(b.LogLevel) {
	case "silent":
		return glogger.Silent
	case "error":
		return glogger.Error
	case "warn":
		return glogger.Warn
	case "info":
		return glogger.Info
	default:
		return glogger.Warn
	}
}

package db

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Config is the configuration for the database
// It is used to configure the database connection pool and logging
type Config struct {
	// Host is the host of the database
	Host string `mapstructure:"host"`
	// Port is the port of the database
	// default: 3306
	Port int `mapstructure:"port"`
	// User is the user of the database
	User string `mapstructure:"user"`
	// Password is the password of the database
	Password string `mapstructure:"password"`
	// Database is the name of the database
	Database string `mapstructure:"database"`
	// MaxOpenConns is the maximum number of open connections to the database
	// default: 10
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
	// charset is the charset of the database
	// default: "utf8mb4"
	Charset string `mapstructure:"charset"`
	// loc is the location of the database
	// default: "Local"
	Loc string `mapstructure:"loc"`
}

func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=%s",
		c.User, c.Password, c.Host, c.Port, c.Database,
		c.Charset, c.Loc,
	)
}

// DefaultConfig returns the default configuration for the database
func DefaultConfig() *Config {
	return &Config{
		Port:            3306,
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 1800 * time.Second,
		ConnMaxIdleTime: 600 * time.Second,
		LogLevel:        "warn",
		SlowThreshold:   1 * time.Second,
		Charset:         "utf8mb4",
		Loc:             "Local",
	}
}

// Validate validates the configuration for the database
func (c *Config) Validate() error {
	if c.Host == "" {
		return ErrInvalidConfig("host is required")
	}
	if c.Port <= 0 {
		return ErrInvalidConfig("port is required")
	}
	if c.User == "" {
		return ErrInvalidConfig("user is required")
	}
	if c.Password == "" {
		return ErrInvalidConfig("password is required")
	}
	if c.Database == "" {
		return ErrInvalidConfig("database is required")
	}

	validLogLevels := []string{"silent", "error", "warn", "info"}
	if !slices.ContainsFunc(validLogLevels, func(level string) bool {
		return strings.EqualFold(c.LogLevel, level)
	}) {
		return ErrInvalidConfig(fmt.Sprintf("log_level %q must be one of: %s", c.LogLevel, strings.Join(validLogLevels, ", ")))
	}
	return nil
}

// MergeDefaults merges the default configuration with the given configuration
// It returns the merged configuration
func (c *Config) MergeDefaults() *Config {
	defaults := DefaultConfig()
	if c.Port == 0 {
		c.Port = defaults.Port
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = defaults.MaxOpenConns
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = defaults.MaxIdleConns
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = defaults.ConnMaxLifetime
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = defaults.ConnMaxIdleTime
	}
	if c.LogLevel == "" {
		c.LogLevel = defaults.LogLevel
	}
	if c.SlowThreshold == 0 {
		c.SlowThreshold = defaults.SlowThreshold
	}
	if c.Charset == "" {
		c.Charset = defaults.Charset
	}
	if c.Loc == "" {
		c.Loc = defaults.Loc
	}
	return c
}

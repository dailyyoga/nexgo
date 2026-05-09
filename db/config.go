package db

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Config is the configuration for a MySQL database connection.
//
// Shared connection / pool / logging fields live on the embedded BaseConfig.
// Only MySQL-specific knobs (Charset, Loc) are declared here.
type Config struct {
	BaseConfig `mapstructure:",squash"`

	// Charset is the charset of the database
	// default: "utf8mb4"
	Charset string `mapstructure:"charset"`
	// Loc is the location of the database
	// default: "Local"
	Loc string `mapstructure:"loc"`
}

// DefaultConfig returns the default configuration for MySQL.
func DefaultConfig() *Config {
	c := &Config{
		BaseConfig: BaseConfig{Port: 3306},
		Charset:    "utf8mb4",
		Loc:        "Local",
	}
	c.mergePoolDefaults()
	return c
}

// MergeDefaults fills in zero-valued fields with their defaults and returns
// the same pointer for chaining.
func (c *Config) MergeDefaults() *Config {
	if c.Port == 0 {
		c.Port = 3306
	}
	c.mergePoolDefaults()
	if c.Charset == "" {
		c.Charset = "utf8mb4"
	}
	if c.Loc == "" {
		c.Loc = "Local"
	}
	return c
}

// Validate validates the configuration for MySQL.
func (c *Config) Validate() error {
	if err := c.validateRequired(); err != nil {
		return err
	}
	return c.validateLogLevel()
}

// DSN returns the MySQL DSN consumed by gorm.io/driver/mysql.
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=%s",
		c.User, c.Password, c.Host, c.Port, c.Database,
		c.Charset, c.Loc,
	)
}

// Base exposes the shared connection / pool / logging fields.
func (c *Config) Base() *BaseConfig { return &c.BaseConfig }

// Dialector returns the gorm.io/driver/mysql adapter for this Config.
func (c *Config) Dialector() gorm.Dialector { return mysql.Open(c.DSN()) }

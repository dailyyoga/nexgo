package db

import (
	"fmt"
	"slices"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// PostgresConfig is the configuration for a PostgreSQL database connection.
//
// Shared connection / pool / logging fields live on the embedded BaseConfig.
// Only PostgreSQL-specific knobs live here.
type PostgresConfig struct {
	BaseConfig `mapstructure:",squash"`

	// SSLMode is the PostgreSQL sslmode parameter.
	// valid values: disable | require | verify-ca | verify-full
	// default: "disable"
	SSLMode string `mapstructure:"ssl_mode"`
	// TimeZone is the IANA timezone name applied at session start.
	// default: "UTC"
	TimeZone string `mapstructure:"time_zone"`
	// SearchPath is the PostgreSQL search_path (schema lookup order).
	// default: "public"
	SearchPath string `mapstructure:"search_path"`
	// ApplicationName populates the PostgreSQL application_name parameter,
	// useful for `pg_stat_activity` attribution. Empty leaves it unset.
	ApplicationName string `mapstructure:"application_name"`
}

// DefaultPostgresConfig returns the default configuration for PostgreSQL.
func DefaultPostgresConfig() *PostgresConfig {
	c := &PostgresConfig{
		BaseConfig: BaseConfig{Port: 5432},
		SSLMode:    "disable",
		TimeZone:   "UTC",
		SearchPath: "public",
	}
	c.mergePoolDefaults()
	return c
}

// MergeDefaults fills in zero-valued fields with their defaults and returns
// the same pointer for chaining.
func (c *PostgresConfig) MergeDefaults() *PostgresConfig {
	if c.Port == 0 {
		c.Port = 5432
	}
	c.mergePoolDefaults()
	if c.SSLMode == "" {
		c.SSLMode = "disable"
	}
	if c.TimeZone == "" {
		c.TimeZone = "UTC"
	}
	if c.SearchPath == "" {
		c.SearchPath = "public"
	}
	return c
}

var validSSLModes = []string{"disable", "require", "verify-ca", "verify-full"}

// Validate validates the configuration for PostgreSQL.
func (c *PostgresConfig) Validate() error {
	if err := c.validateRequired(); err != nil {
		return err
	}
	if err := c.validateLogLevel(); err != nil {
		return err
	}
	if !slices.ContainsFunc(validSSLModes, func(m string) bool {
		return strings.EqualFold(c.SSLMode, m)
	}) {
		return ErrInvalidConfig(fmt.Sprintf(
			"ssl_mode %q must be one of: %s",
			c.SSLMode, strings.Join(validSSLModes, ", "),
		))
	}
	return nil
}

// DSN returns the libpq-style keyword DSN consumed by gorm.io/driver/postgres.
//
// Values containing whitespace, single quotes or backslashes are wrapped in
// single quotes and escaped per the PostgreSQL connection-string grammar.
func (c *PostgresConfig) DSN() string {
	parts := []string{
		"host=" + pgEscape(c.Host),
		fmt.Sprintf("port=%d", c.Port),
		"user=" + pgEscape(c.User),
		"password=" + pgEscape(c.Password),
		"dbname=" + pgEscape(c.Database),
		"sslmode=" + pgEscape(c.SSLMode),
		"TimeZone=" + pgEscape(c.TimeZone),
		"search_path=" + pgEscape(c.SearchPath),
	}
	if c.ApplicationName != "" {
		parts = append(parts, "application_name="+pgEscape(c.ApplicationName))
	}
	return strings.Join(parts, " ")
}

// Base exposes the shared connection / pool / logging fields.
func (c *PostgresConfig) Base() *BaseConfig { return &c.BaseConfig }

// Dialector returns the gorm.io/driver/postgres adapter for this Config.
func (c *PostgresConfig) Dialector() gorm.Dialector { return postgres.Open(c.DSN()) }

// pgEscape quotes a value for the PostgreSQL keyword/value DSN format when
// it contains characters that would otherwise terminate the value (space) or
// confuse the parser (single quote, backslash).
func pgEscape(v string) string {
	if v == "" {
		return "''"
	}
	if !strings.ContainsAny(v, " '\\") {
		return v
	}
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + r.Replace(v) + "'"
}

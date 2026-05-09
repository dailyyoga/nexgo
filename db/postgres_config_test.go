package db

import (
	"strings"
	"testing"
	"time"
)

func TestPostgresConfig_DefaultPostgresConfig(t *testing.T) {
	c := DefaultPostgresConfig()
	if c.Port != 5432 {
		t.Errorf("default port = %d, want 5432", c.Port)
	}
	if c.SSLMode != "disable" {
		t.Errorf("default ssl_mode = %q, want disable", c.SSLMode)
	}
	if c.TimeZone != "UTC" {
		t.Errorf("default time_zone = %q, want UTC", c.TimeZone)
	}
	if c.SearchPath != "public" {
		t.Errorf("default search_path = %q, want public", c.SearchPath)
	}
	if c.MaxOpenConns != 25 || c.MaxIdleConns != 10 {
		t.Errorf("default pool sizes = %d/%d, want 25/10", c.MaxOpenConns, c.MaxIdleConns)
	}
	if c.ConnMaxLifetime != 1800*time.Second || c.ConnMaxIdleTime != 600*time.Second {
		t.Errorf("default lifetimes = %v/%v", c.ConnMaxLifetime, c.ConnMaxIdleTime)
	}
	if c.LogLevel != "warn" {
		t.Errorf("default log_level = %q, want warn", c.LogLevel)
	}
}

func TestPostgresConfig_MergeDefaults(t *testing.T) {
	c := (&PostgresConfig{
		BaseConfig: BaseConfig{Host: "h", User: "u", Password: "p", Database: "d"},
	}).MergeDefaults()
	if c.Port != 5432 || c.SSLMode != "disable" || c.TimeZone != "UTC" || c.SearchPath != "public" {
		t.Errorf("MergeDefaults did not fill defaults: %+v", c)
	}

	// non-zero fields must not be overwritten
	c2 := (&PostgresConfig{
		BaseConfig: BaseConfig{
			Host:     "h",
			Port:     6543,
			User:     "u",
			Password: "p",
			Database: "d",
		},
		SSLMode:    "require",
		TimeZone:   "Asia/Shanghai",
		SearchPath: "myschema",
	}).MergeDefaults()
	if c2.Port != 6543 || c2.SSLMode != "require" || c2.TimeZone != "Asia/Shanghai" || c2.SearchPath != "myschema" {
		t.Errorf("MergeDefaults overwrote non-zero fields: %+v", c2)
	}
}

func TestPostgresConfig_Validate(t *testing.T) {
	base := func() *PostgresConfig {
		return (&PostgresConfig{
			BaseConfig: BaseConfig{Host: "localhost", User: "u", Password: "p", Database: "d"},
		}).MergeDefaults()
	}

	tests := []struct {
		name    string
		mut     func(*PostgresConfig)
		wantErr bool
	}{
		{"valid", func(*PostgresConfig) {}, false},
		{"missing host", func(c *PostgresConfig) { c.Host = "" }, true},
		{"missing port", func(c *PostgresConfig) { c.Port = 0 }, true},
		{"missing user", func(c *PostgresConfig) { c.User = "" }, true},
		{"missing password", func(c *PostgresConfig) { c.Password = "" }, true},
		{"missing database", func(c *PostgresConfig) { c.Database = "" }, true},
		{"bad log_level", func(c *PostgresConfig) { c.LogLevel = "verbose" }, true},
		{"bad ssl_mode", func(c *PostgresConfig) { c.SSLMode = "yes-please" }, true},
		{"ssl_mode require ok", func(c *PostgresConfig) { c.SSLMode = "require" }, false},
		{"ssl_mode verify-full ok", func(c *PostgresConfig) { c.SSLMode = "verify-full" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base()
			tt.mut(c)
			err := c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPostgresConfig_DSN(t *testing.T) {
	c := (&PostgresConfig{
		BaseConfig: BaseConfig{
			Host:     "10.0.0.1",
			Port:     5432,
			User:     "invoker",
			Password: "secret",
			Database: "agent",
		},
		ApplicationName: "invoker-server",
	}).MergeDefaults()

	dsn := c.DSN()
	mustContain(t, dsn, "host=10.0.0.1")
	mustContain(t, dsn, "port=5432")
	mustContain(t, dsn, "user=invoker")
	mustContain(t, dsn, "password=secret")
	mustContain(t, dsn, "dbname=agent")
	mustContain(t, dsn, "sslmode=disable")
	mustContain(t, dsn, "TimeZone=UTC")
	mustContain(t, dsn, "search_path=public")
	mustContain(t, dsn, "application_name=invoker-server")
}

func TestPostgresConfig_DSN_EscapesSpecials(t *testing.T) {
	c := (&PostgresConfig{
		BaseConfig: BaseConfig{
			Host:     "localhost",
			User:     "u",
			Password: `p ass'wo\rd`, // space, single quote, backslash
			Database: "d",
		},
	}).MergeDefaults()

	dsn := c.DSN()
	if !strings.Contains(dsn, `password='p ass\'wo\\rd'`) {
		t.Errorf("password not escaped correctly in DSN: %s", dsn)
	}
}

func TestPostgresConfig_DSN_OmitsApplicationNameWhenEmpty(t *testing.T) {
	c := (&PostgresConfig{
		BaseConfig: BaseConfig{Host: "localhost", User: "u", Password: "p", Database: "d"},
	}).MergeDefaults()
	if strings.Contains(c.DSN(), "application_name=") {
		t.Errorf("DSN should omit application_name when empty: %s", c.DSN())
	}
}

func TestPostgresConfig_ImplementsConnector(t *testing.T) {
	var _ Connector = (*PostgresConfig)(nil)
}

func TestPgEscape(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "''"},
		{"plain", "plain"},
		{"with space", "'with space'"},
		{"with'quote", `'with\'quote'`},
		{`with\back`, `'with\\back'`},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := pgEscape(tt.in); got != tt.want {
				t.Errorf("pgEscape(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("expected DSN to contain %q, got: %s", sub, s)
	}
}

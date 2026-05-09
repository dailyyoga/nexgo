package db

import (
	"strings"
	"testing"
	"time"
)

func TestConfig_DefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.Port != 3306 {
		t.Errorf("default port = %d, want 3306", c.Port)
	}
	if c.Charset != "utf8mb4" {
		t.Errorf("default charset = %q, want utf8mb4", c.Charset)
	}
	if c.Loc != "Local" {
		t.Errorf("default loc = %q, want Local", c.Loc)
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

func TestConfig_MergeDefaults(t *testing.T) {
	c := (&Config{
		BaseConfig: BaseConfig{Host: "h", User: "u", Password: "p", Database: "d"},
	}).MergeDefaults()
	if c.Port != 3306 || c.Charset != "utf8mb4" || c.Loc != "Local" {
		t.Errorf("MergeDefaults did not fill defaults: %+v", c)
	}
	if c.MaxOpenConns != 25 {
		t.Errorf("MergeDefaults did not fill pool defaults: %+v", c)
	}

	c2 := (&Config{
		BaseConfig: BaseConfig{
			Host:     "h",
			Port:     3307,
			User:     "u",
			Password: "p",
			Database: "d",
		},
		Charset: "latin1",
		Loc:     "UTC",
	}).MergeDefaults()
	if c2.Port != 3307 || c2.Charset != "latin1" || c2.Loc != "UTC" {
		t.Errorf("MergeDefaults overwrote non-zero fields: %+v", c2)
	}
}

func TestConfig_Validate(t *testing.T) {
	base := func() *Config {
		return (&Config{
			BaseConfig: BaseConfig{Host: "localhost", User: "u", Password: "p", Database: "d"},
		}).MergeDefaults()
	}
	tests := []struct {
		name    string
		mut     func(*Config)
		wantErr bool
	}{
		{"valid", func(*Config) {}, false},
		{"missing host", func(c *Config) { c.Host = "" }, true},
		{"missing port", func(c *Config) { c.Port = 0 }, true},
		{"missing user", func(c *Config) { c.User = "" }, true},
		{"missing password", func(c *Config) { c.Password = "" }, true},
		{"missing database", func(c *Config) { c.Database = "" }, true},
		{"bad log_level", func(c *Config) { c.LogLevel = "verbose" }, true},
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

func TestConfig_DSN(t *testing.T) {
	c := (&Config{
		BaseConfig: BaseConfig{
			Host:     "10.0.0.1",
			Port:     3306,
			User:     "invoker",
			Password: "secret",
			Database: "agent",
		},
	}).MergeDefaults()

	dsn := c.DSN()
	if !strings.Contains(dsn, "invoker:secret@tcp(10.0.0.1:3306)/agent") {
		t.Errorf("unexpected DSN: %s", dsn)
	}
	if !strings.Contains(dsn, "charset=utf8mb4") {
		t.Errorf("missing charset in DSN: %s", dsn)
	}
	if !strings.Contains(dsn, "loc=Local") {
		t.Errorf("missing loc in DSN: %s", dsn)
	}
	if !strings.Contains(dsn, "parseTime=True") {
		t.Errorf("missing parseTime in DSN: %s", dsn)
	}
}

func TestConfig_ImplementsConnector(t *testing.T) {
	var _ Connector = (*Config)(nil)
}

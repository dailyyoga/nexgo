package db

import (
	"testing"
	"time"

	glogger "gorm.io/gorm/logger"
)

func TestBaseConfig_MergePoolDefaults(t *testing.T) {
	b := &BaseConfig{}
	b.mergePoolDefaults()

	if b.MaxOpenConns != 25 {
		t.Errorf("MaxOpenConns = %d, want 25", b.MaxOpenConns)
	}
	if b.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %d, want 10", b.MaxIdleConns)
	}
	if b.ConnMaxLifetime != 1800*time.Second {
		t.Errorf("ConnMaxLifetime = %v, want 30m", b.ConnMaxLifetime)
	}
	if b.ConnMaxIdleTime != 600*time.Second {
		t.Errorf("ConnMaxIdleTime = %v, want 10m", b.ConnMaxIdleTime)
	}
	if b.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want warn", b.LogLevel)
	}
	if b.SlowThreshold != time.Second {
		t.Errorf("SlowThreshold = %v, want 1s", b.SlowThreshold)
	}
}

func TestBaseConfig_MergePoolDefaults_PreservesNonZero(t *testing.T) {
	b := &BaseConfig{
		MaxOpenConns:    50,
		MaxIdleConns:    20,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
		LogLevel:        "info",
		SlowThreshold:   200 * time.Millisecond,
	}
	b.mergePoolDefaults()

	if b.MaxOpenConns != 50 || b.MaxIdleConns != 20 ||
		b.ConnMaxLifetime != 5*time.Minute || b.ConnMaxIdleTime != 1*time.Minute ||
		b.LogLevel != "info" || b.SlowThreshold != 200*time.Millisecond {
		t.Errorf("mergePoolDefaults overwrote non-zero fields: %+v", b)
	}
}

func TestBaseConfig_GormLogLevel(t *testing.T) {
	tests := map[string]glogger.LogLevel{
		"silent":   glogger.Silent,
		"SILENT":   glogger.Silent,
		"error":    glogger.Error,
		"warn":     glogger.Warn,
		"info":     glogger.Info,
		"":         glogger.Warn, // unknown -> warn
		"verbose":  glogger.Warn,
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) {
			b := &BaseConfig{LogLevel: in}
			if got := b.gormLogLevel(); got != want {
				t.Errorf("gormLogLevel(%q) = %v, want %v", in, got, want)
			}
		})
	}
}

func TestBaseConfig_ValidateRequired(t *testing.T) {
	full := func() *BaseConfig {
		return &BaseConfig{Host: "h", Port: 1, User: "u", Password: "p", Database: "d"}
	}
	tests := []struct {
		name    string
		mut     func(*BaseConfig)
		wantErr bool
	}{
		{"ok", func(*BaseConfig) {}, false},
		{"no host", func(b *BaseConfig) { b.Host = "" }, true},
		{"no port", func(b *BaseConfig) { b.Port = 0 }, true},
		{"neg port", func(b *BaseConfig) { b.Port = -1 }, true},
		{"no user", func(b *BaseConfig) { b.User = "" }, true},
		{"no password", func(b *BaseConfig) { b.Password = "" }, true},
		{"no database", func(b *BaseConfig) { b.Database = "" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := full()
			tt.mut(b)
			err := b.validateRequired()
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRequired() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

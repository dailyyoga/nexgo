package logger

import (
	"testing"
)

func TestNew_NilConfig(t *testing.T) {
	l, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil) failed: %v", err)
	}
	if l == nil {
		t.Fatal("New(nil) returned nil logger")
	}
	l.Info("test")
	if err := l.Sync(); err != nil {
		t.Logf("Sync returned error (may be expected for stdout): %v", err)
	}
}

func TestNew_PartialConfig(t *testing.T) {
	cfg := &Config{
		Level:    "info",
		Encoding: "json",
		// OutputPaths and ErrorOutputPaths are nil
	}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New with partial config failed: %v", err)
	}
	if l == nil {
		t.Fatal("New returned nil logger")
	}
	l.Info("test from partial config")
	if err := l.Sync(); err != nil {
		t.Logf("Sync returned error (may be expected for stdout): %v", err)
	}
}

func TestNew_InvalidLevel(t *testing.T) {
	cfg := &Config{
		Level:            "invalid",
		Encoding:         "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid level, got nil")
	}
}

func TestNew_InvalidEncoding(t *testing.T) {
	cfg := &Config{
		Level:            "info",
		Encoding:         "invalid",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid encoding, got nil")
	}
}

func TestNew_EmptyLevel(t *testing.T) {
	cfg := &Config{
		Level:            "",
		Encoding:         "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	l, err := New(cfg)
	if err != nil {
		t.Fatalf("New with empty level failed: %v", err)
	}
	if l == nil {
		t.Fatal("New returned nil logger")
	}
	// Empty level should be filled with default "info"
	l.Info("test empty level defaults to info")
	if err := l.Sync(); err != nil {
		t.Logf("Sync returned error (may be expected for stdout): %v", err)
	}
}

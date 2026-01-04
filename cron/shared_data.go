package cron

import (
	"context"
	"sync"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// sharedDataKey is the context key for storing SharedData
const sharedDataKey contextKey = "cron:shared_data"

// SharedData provides thread-safe data sharing between tasks in a chain
// It uses sync.Map internally for concurrent access without explicit locking
type SharedData struct {
	data sync.Map
}

// GetSharedData retrieves SharedData from the context
// Returns nil if the context does not contain SharedData
func GetSharedData(ctx context.Context) *SharedData {
	if val, ok := ctx.Value(sharedDataKey).(*SharedData); ok {
		return val
	}
	return nil
}

// Set stores a key-value pair in the SharedData
// This method is safe for concurrent use by multiple tasks
func (s *SharedData) Set(key string, value any) {
	s.data.Store(key, value)
}

// Get retrieves a value from the SharedData by key
// Returns the value and true if the key exists, otherwise returns nil and false
func (s *SharedData) Get(key string) (any, bool) {
	return s.data.Load(key)
}

// Delete removes a key-value pair from the SharedData
func (s *SharedData) Delete(key string) {
	s.data.Delete(key)
}

// Range iterates over all key-value pairs in the SharedData
// The iteration stops if the function f returns false
func (s *SharedData) Range(f func(key string, value any) bool) {
	s.data.Range(func(k, v any) bool {
		return f(k.(string), v)
	})
}

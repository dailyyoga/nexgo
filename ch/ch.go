package ch

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type TableName string

type Table interface {
	TableName() TableName
	ToValueMap() map[string]any
}

type Writer interface {
	Start() error
	Close() error
	Write(ctx context.Context, rows []Table) error
	// RefreshTableSchema 手动刷新指定表的结构缓存（用于表结构变更后立即生效）
	RefreshTableSchema(ctx context.Context, table TableName) error
}

// Client is the unified ClickHouse client interface for both query and batch write operations
type Client interface {
	// Writer returns the Writer interface for batch writing
	Writer() (Writer, error)
	// Query executes a ClickHouse query and returns driver.Rows
	Query(ctx context.Context, query string, args ...any) (driver.Rows, error)
	// QueryRow executes a query that is expected to return at most one row
	QueryRow(ctx context.Context, query string, args ...any) driver.Row
	// Close closes the client and all associated resources
	Close() error
}

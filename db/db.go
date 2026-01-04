package db

import (
	"context"

	"gorm.io/gorm"
)

// Database is the interface for the database
type Database interface {
	DB() (*gorm.DB, error)
	Ping(ctx context.Context) error
	Close() error
}

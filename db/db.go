package db

import (
	"context"

	"github.com/dailyyoga/nexgo/logger"
	"gorm.io/gorm"
)

// Database is the interface for the database
type Database interface {
	DB() (*gorm.DB, error)
	Ping(ctx context.Context) error
	Close() error
}

// gormDatabase is the unified Database implementation backed by *gorm.DB.
// All driver-specific constructors funnel into this type via openGorm.
type gormDatabase struct {
	logger logger.Logger
	db     *gorm.DB
}

func (g *gormDatabase) DB() (*gorm.DB, error) {
	if g.db == nil {
		return nil, ErrConnectionNotEstablished
	}
	return g.db, nil
}

func (g *gormDatabase) Ping(ctx context.Context) error {
	sqldb, err := g.db.DB()
	if err != nil {
		return ErrConnection(err)
	}
	return sqldb.PingContext(ctx)
}

func (g *gormDatabase) Close() error {
	sqldb, err := g.db.DB()
	if err != nil {
		return ErrConnection(err)
	}
	return sqldb.Close()
}

// openGorm builds a *gorm.DB from any Connector and applies the shared pool
// settings, custom logger and connectivity probe. Driver-specific
// constructors are reduced to validation + a call to this helper + a
// human-readable log line.
func openGorm(log logger.Logger, c Connector) (*gormDatabase, error) {
	base := c.Base()

	customLogger := &gormLogger{
		logger:        log,
		level:         base.gormLogLevel(),
		slowThreshold: base.SlowThreshold,
	}

	gdb, err := gorm.Open(c.Dialector(), &gorm.Config{
		Logger:                                   customLogger,
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, ErrConnection(err)
	}

	sqldb, err := gdb.DB()
	if err != nil {
		return nil, ErrConnection(err)
	}

	sqldb.SetMaxOpenConns(base.MaxOpenConns)
	sqldb.SetMaxIdleConns(base.MaxIdleConns)
	sqldb.SetConnMaxLifetime(base.ConnMaxLifetime)
	sqldb.SetConnMaxIdleTime(base.ConnMaxIdleTime)

	if err := sqldb.Ping(); err != nil {
		return nil, ErrConnection(err)
	}

	return &gormDatabase{logger: log, db: gdb}, nil
}

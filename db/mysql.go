package db

import (
	"context"
	"strings"

	"github.com/dailyyoga/go-kit/logger"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

type defaultMySQLDatabase struct {
	logger logger.Logger
	db     *gorm.DB
}

func NewMySQL(log logger.Logger, cfg *Config) (Database, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	} else {
		// merge default values for empty fields
		cfg = cfg.MergeDefaults()
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	dd := &defaultMySQLDatabase{
		logger: log,
	}

	// set gorm logger level
	var gormLogLevel glogger.LogLevel
	switch strings.ToLower(cfg.LogLevel) {
	case "silent":
		gormLogLevel = glogger.Silent
	case "error":
		gormLogLevel = glogger.Error
	case "warn":
		gormLogLevel = glogger.Warn
	case "info":
		gormLogLevel = glogger.Info
	default:
		gormLogLevel = glogger.Warn
	}

	// create custom gorm logger with zap backend
	customLogger := &gormLogger{
		logger:        dd.logger,
		level:         gormLogLevel,
		slowThreshold: cfg.SlowThreshold,
	}

	// connection
	var err error
	dd.db, err = gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger:                                   customLogger,
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, ErrConnection(err)
	}
	sqldb, err := dd.db.DB()
	if err != nil {
		return nil, ErrConnection(err)
	}

	// set connection pool settings
	sqldb.SetMaxOpenConns(cfg.MaxOpenConns)
	sqldb.SetMaxIdleConns(cfg.MaxIdleConns)
	sqldb.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqldb.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// test connection
	if err := sqldb.Ping(); err != nil {
		return nil, ErrConnection(err)
	}

	dd.logger.Info("database connection established",
		zap.String("host", cfg.Host),
		zap.String("database", cfg.Database),
		zap.Int("max_open_conns", cfg.MaxOpenConns),
		zap.Int("max_idle_conns", cfg.MaxIdleConns),
		zap.Duration("conn_max_lifetime", cfg.ConnMaxLifetime),
		zap.Duration("conn_max_idle_time", cfg.ConnMaxIdleTime),
	)

	return dd, nil
}

func (dd *defaultMySQLDatabase) DB() (*gorm.DB, error) {
	if dd.db == nil {
		return nil, ErrConnectionNotEstablished
	}
	return dd.db, nil
}

func (dd *defaultMySQLDatabase) Ping(ctx context.Context) error {
	sqldb, err := dd.db.DB()
	if err != nil {
		return ErrConnection(err)
	}
	return sqldb.PingContext(ctx)
}

func (dd *defaultMySQLDatabase) Close() error {
	sqldb, err := dd.db.DB()
	if err != nil {
		return ErrConnection(err)
	}
	return sqldb.Close()
}

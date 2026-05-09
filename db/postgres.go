package db

import (
	"github.com/dailyyoga/nexgo/logger"
	"go.uber.org/zap"
)

// NewPostgres returns a Database backed by PostgreSQL.
//
// The returned Database satisfies the same interface as NewMySQL so call
// sites can swap drivers without touching downstream code; only the wiring
// layer chooses which constructor to invoke.
func NewPostgres(log logger.Logger, cfg *PostgresConfig) (Database, error) {
	if cfg == nil {
		cfg = DefaultPostgresConfig()
	} else {
		cfg = cfg.MergeDefaults()
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	dd, err := openGorm(log, cfg)
	if err != nil {
		return nil, err
	}

	log.Info("postgres connection established",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Database),
		zap.String("ssl_mode", cfg.SSLMode),
		zap.String("search_path", cfg.SearchPath),
		zap.Int("max_open_conns", cfg.MaxOpenConns),
		zap.Int("max_idle_conns", cfg.MaxIdleConns),
		zap.Duration("conn_max_lifetime", cfg.ConnMaxLifetime),
		zap.Duration("conn_max_idle_time", cfg.ConnMaxIdleTime),
	)

	return dd, nil
}

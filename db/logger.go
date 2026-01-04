package db

import (
	"context"
	"fmt"
	"time"

	"github.com/dailyyoga/nexgo/logger"
	"go.uber.org/zap"
	glogger "gorm.io/gorm/logger"
)

// gormLogger is a custom GORM logger that uses the project's zap logger
type gormLogger struct {
	logger        logger.Logger
	level         glogger.LogLevel
	slowThreshold time.Duration
}

// LogMode sets the log level and returns a new logger
func (g *gormLogger) LogMode(level glogger.LogLevel) glogger.Interface {
	return &gormLogger{
		logger:        g.logger,
		level:         level,
		slowThreshold: g.slowThreshold,
	}
}

// Info logs info level messages
func (g *gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if g.level >= glogger.Info {
		g.logger.Info(fmt.Sprintf(msg, data...), zap.String("component", "gorm"))
	}
}

// Warn logs warn level messages
func (g *gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if g.level >= glogger.Warn {
		g.logger.Warn(fmt.Sprintf(msg, data...), zap.String("component", "gorm"))
	}
}

// Error logs error level messages
func (g *gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if g.level >= glogger.Error {
		g.logger.Error(fmt.Sprintf(msg, data...), zap.String("component", "gorm"))
	}
}

// Trace logs SQL execution details
func (g *gormLogger) Trace(
	ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error,
) {
	if g.level <= glogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := []zap.Field{
		zap.String("component", "gorm"),
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
		zap.String("sql", sql),
	}

	switch {
	case err != nil && g.level >= glogger.Error:
		g.logger.Error("sql error", append(fields, zap.Error(err))...)
	case elapsed > g.slowThreshold && g.slowThreshold != 0 && g.level >= glogger.Warn:
		g.logger.Warn("slow sql", append(fields, zap.Duration("threshold", g.slowThreshold))...)
	case g.level >= glogger.Info:
		g.logger.Info("sql trace", fields...)
	}
}

package logger

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	globalMu     sync.RWMutex
	initOnce     sync.Once
)

// setGlobalLoggerInternal sets the global logger (internal use by New function).
func setGlobalLoggerInternal(l *zap.Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = l
}

// getGlobalLogger returns the global logger, initializing it with defaults if necessary.
// This function is concurrency-safe.
func getGlobalLogger() *zap.Logger {
	globalMu.RLock()
	if globalLogger != nil {
		defer globalMu.RUnlock()
		return globalLogger
	}
	globalMu.RUnlock()

	initOnce.Do(func() {
		globalMu.Lock()
		defer globalMu.Unlock()
		if globalLogger == nil {
			globalLogger = mustBuildDefaultLogger()
		}
	})

	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalLogger
}

// mustBuildDefaultLogger creates a default logger for global functions.
// It uses CallerSkip(1) to report the correct caller location.
func mustBuildDefaultLogger() *zap.Logger {
	cfg := DefaultConfig()

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	zapConfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Development:      false,
		Encoding:         cfg.Encoding,
		EncoderConfig:    encoderConfig,
		OutputPaths:      cfg.OutputPaths,
		ErrorOutputPaths: cfg.ErrorOutputPaths,
	}

	// CallerSkip(1) to skip the wrapper function (Info, Debug, etc.)
	logger, err := zapConfig.Build(
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.DPanicLevel),
	)
	if err != nil {
		// Fallback to nop logger if build fails
		return zap.NewNop()
	}
	return logger
}

// SetGlobalLogger sets the global logger.
// The provided logger should be created with AddCallerSkip(1) if you want
// correct caller information when using package-level functions.
// This function is concurrency-safe.
func SetGlobalLogger(l *zap.Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = l
}

// GetGlobalLogger returns the current global logger.
// This function is concurrency-safe.
func GetGlobalLogger() *zap.Logger {
	return getGlobalLogger()
}

// Debug logs a message at debug level using the global logger.
func Debug(msg string, fields ...zap.Field) {
	getGlobalLogger().Debug(msg, fields...)
}

// Info logs a message at info level using the global logger.
func Info(msg string, fields ...zap.Field) {
	getGlobalLogger().Info(msg, fields...)
}

// Warn logs a message at warn level using the global logger.
func Warn(msg string, fields ...zap.Field) {
	getGlobalLogger().Warn(msg, fields...)
}

// Error logs a message at error level using the global logger.
func Error(msg string, fields ...zap.Field) {
	getGlobalLogger().Error(msg, fields...)
}

// Sync flushes any buffered log entries from the global logger.
func Sync() error {
	return getGlobalLogger().Sync()
}

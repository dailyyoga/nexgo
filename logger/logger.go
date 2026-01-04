// Package logger provides a unified logging interface based on zap.
//
// It offers configurable log levels, encoding formats (JSON/Console),
// and output paths, while maintaining interface compatibility with *zap.Logger.
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger defines the interface for logging operations
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Sync() error
}

// New creates a new logger with the given configuration
func New(cfg *Config) (Logger, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	} else {
		// Merge default values for empty fields
		defaults := DefaultConfig()
		if cfg.Level == "" {
			cfg.Level = defaults.Level
		}
		if cfg.Encoding == "" {
			cfg.Encoding = defaults.Encoding
		}
		if len(cfg.OutputPaths) == 0 {
			cfg.OutputPaths = defaults.OutputPaths
		}
		if len(cfg.ErrorOutputPaths) == 0 {
			cfg.ErrorOutputPaths = defaults.ErrorOutputPaths
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	// parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, ErrInvalidLevel(cfg.Level, err)
	}

	// create encoder config
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

	// create logger config
	zapConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       cfg.Encoding == "console",
		Encoding:          cfg.Encoding,
		EncoderConfig:     encoderConfig,
		OutputPaths:       cfg.OutputPaths,
		ErrorOutputPaths:  cfg.ErrorOutputPaths,
		DisableCaller:     false,
		DisableStacktrace: false,
	}

	logger, err := zapConfig.Build(
		zap.AddCallerSkip(0),
		zap.AddStacktrace(zapcore.DPanicLevel),
	)
	if err != nil {
		return nil, ErrBuildLogger(err)
	}

	// Set global logger with CallerSkip(1) for package-level functions
	setGlobalLoggerInternal(logger.WithOptions(zap.AddCallerSkip(1)))

	return logger, nil
}

package logging

import (
	"strings"

	"s3s/info"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logging configuration params.
type Config struct {
	// Log level
	Level string
	// Write logs into json format
	JsonFormat bool
	// Enable log colors
	EnableColors bool
	// Application id
	AppId string
}

func NewZapLogger(cfg *Config) (*zap.Logger, error) {
	var zapcfg zap.Config

	switch cfg.Level {
	case "info":
		zapcfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "debug":
		zapcfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "warning":
		zapcfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapcfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "fatal":
		zapcfg.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	default:
		zapcfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	zapcfg.Development = false

	zapcfg.Sampling = &zap.SamplingConfig{
		Initial:    100,
		Thereafter: 100,
	}

	zapcfg.EncoderConfig = zapcore.EncoderConfig{
		NameKey:        "logger",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LevelKey:       "level",
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		TimeKey:        "logtime",
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		LineEnding:     zapcore.DefaultLineEnding,
	}

	if cfg.EnableColors {
		zapcfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	zapcfg.Encoding = "console"
	if cfg.JsonFormat {
		zapcfg.Encoding = "json"

		zapcfg.EncoderConfig.CallerKey = "caller"

		zapcfg.InitialFields = map[string]interface{}{
			"app":         strings.ToLower(info.NameSpace),
			"app_id":      cfg.AppId,
			"app_version": info.Version,
		}
	}

	zapcfg.OutputPaths = []string{"stdout"}
	zapcfg.ErrorOutputPaths = []string{"stderr"}

	return zapcfg.Build()
}

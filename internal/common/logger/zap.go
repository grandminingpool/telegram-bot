package logger

import (
	"github.com/grandminingpool/telegram-bot/internal/common/flags"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LoggerConfig struct {
	AppMode    flags.AppMode
	OutputPath string
	Level      flags.LogLevel
}

func getZapLevel(level flags.LogLevel) zapcore.Level {
	switch level {
	case flags.LogLevelDebug:
		return zapcore.DebugLevel
	case flags.LogLevelWarn:
		return zapcore.WarnLevel
	default:
		return zapcore.InfoLevel
	}
}

func getProductionLogger(outputPath string, level zapcore.Level) (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{outputPath, "stdout"}
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.Level = zap.NewAtomicLevelAt(level)

	return config.Build()
}

func SetupLogger(config *LoggerConfig) (*zap.Logger, error) {
	if config.AppMode == flags.AppModeProd {
		level := getZapLevel(config.Level)

		return getProductionLogger(config.OutputPath, level)
	}

	return zap.NewDevelopment()
}

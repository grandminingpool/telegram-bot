package logger

import (
	"github.com/grandminingpool/telegram-bot/internal/common/flags"
	"go.uber.org/zap"
)

type LoggerConfig struct {
	AppMode         flags.AppMode
	OutputPath      string
	ErrorOutputPath string
}

func getProductionLogger(outputPath, errorOutputPath string) (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{outputPath, "stdout"}
	config.ErrorOutputPaths = []string{errorOutputPath, "stderr"}

	return config.Build()
}

func SetupLogger(config *LoggerConfig) (*zap.Logger, error) {
	if config.AppMode == flags.AppModeProd {
		return getProductionLogger(config.OutputPath, config.ErrorOutputPath)
	}

	return zap.NewDevelopment()
}

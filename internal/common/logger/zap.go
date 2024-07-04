package logger

import (
	"github.com/grandminingpool/telegram-bot/internal/common/flags"
	"go.uber.org/zap"
)

func SetupLogger(appMode flags.AppMode) (*zap.Logger, error) {
	if appMode == flags.AppModeProd {
		return zap.NewProduction()
	}

	return zap.NewDevelopment()

}
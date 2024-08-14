package handlers

import "go.uber.org/zap"

func ErrorsHandler(err error) {
	zap.L().Error("bot error", zap.Error(err))
}

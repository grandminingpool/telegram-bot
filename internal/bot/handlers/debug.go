package handlers

import (
	"fmt"

	"go.uber.org/zap"
)

func DebugHandler(format string, args ...any) {
	zap.L().Debug(fmt.Sprintf(format, args...))
}

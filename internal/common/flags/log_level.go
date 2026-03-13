package flags

const LogLevelFlag = "log-level"

type LogLevel string

const (
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelDebug LogLevel = "debug"
)

func checkLogLevel(newLevel string) LogLevel {
	switch newLevel {
	case string(LogLevelDebug):
		return LogLevelDebug
	case string(LogLevelWarn):
		return LogLevelWarn
	default:
		return LogLevelInfo
	}
}

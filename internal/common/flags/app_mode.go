package flags

const APP_MODE_FLAG = "mode"

type AppMode string

const (
	AppModeDev  AppMode = "dev"
	AppModeProd AppMode = "prod"
)

func checkAppMode(newMode string) AppMode {
	switch newMode {
	case string(AppModeProd):
		return AppModeProd
	default:
		return AppModeDev
	}
}

package flags

import (
	"flag"

	"golang.org/x/text/language"
)

const (
	CONFIGS_FLAG      = "configs"
	CERTS_FLAG        = "certs"
	LOCALES_PATH_FLAG = "locales_path"
	LOCALES_PATH      = "locales"
)

type ParsedFlags struct {
	AppMode     *string
	ConfigsPath *string
	CertsPath   *string
	LocalesPath *string
	Locales     *Locales
}

type FlagsConfig struct {
	Mode        AppMode
	ConfigsPath string
	CertsPath   string
	LocalesPath string
	Locales     Locales
}

func ParseFlags() *ParsedFlags {
	appModeFlag := flag.String(APP_MODE_FLAG, string(AppModeDev), "application mode")
	configsPathFlag := flag.String(CONFIGS_FLAG, "configs", "configs path")
	certsPathFlag := flag.String(CERTS_FLAG, "certs", "api certificates path")
	localesPathFlag := flag.String(LOCALES_PATH_FLAG, "localesPath", "locales path")
	var localesFlag Locales
	flag.Var(&localesFlag, LOCALES_PATH, "comma-separated list of bot locales")
	parsedFlags := &ParsedFlags{
		AppMode:     appModeFlag,
		ConfigsPath: configsPathFlag,
		CertsPath:   certsPathFlag,
		LocalesPath: localesPathFlag,
		Locales:     &localesFlag,
	}

	flag.Parse()

	return parsedFlags
}

func SetupFlags(parsedFlags *ParsedFlags) *FlagsConfig {
	appMode := AppModeDev
	configsPath := "configs"
	certsPath := "certs"
	localesPath := "locales"
	locales := []language.Tag{language.English}

	if parsedFlags.AppMode != nil {
		appMode = checkAppMode(*parsedFlags.AppMode)
	}

	if parsedFlags.ConfigsPath != nil {
		configsPath = *parsedFlags.ConfigsPath
	}

	if parsedFlags.CertsPath != nil {
		certsPath = *parsedFlags.CertsPath
	}

	if parsedFlags.LocalesPath != nil {
		localesPath = *parsedFlags.LocalesPath
	}

	if parsedFlags.Locales != nil {
		locales = *parsedFlags.Locales
	}

	return &FlagsConfig{
		Mode:        appMode,
		ConfigsPath: configsPath,
		CertsPath:   certsPath,
		LocalesPath: localesPath,
		Locales:     locales,
	}
}

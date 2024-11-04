package flags

import (
	"flag"

	"golang.org/x/text/language"
)

const (
	CONFIGS_FLAG                  = "configs"
	CERTS_FLAG                    = "certs"
	LOGGER_OUTPUT_PATH_FLAG       = "logger-output-path"
	LOGGER_ERROR_OUTPUT_PATH_FLAG = "logger-error-output-path"
	LOCALES_PATH_FLAG             = "locales_path"
	LOCALES_PATH                  = "locales"

	CONFIGS_PATH_DEFAULT             = "configs"
	CERTS_PATH_DEFAULT               = "certs"
	LOGGER_OUTPUT_PATH_DEFAULT       = "logs/output.log"
	LOGGER_ERROR_OUTPUT_PATH_DEFAULT = "logs/error.log"
	LOCALES_PATH_DEFAULT             = "locales"
)

type ParsedFlags struct {
	AppMode               *string
	ConfigsPath           *string
	CertsPath             *string
	LoggerOutputPath      *string
	LoggerErrorOutputPath *string
	LocalesPath           *string
	Locales               *Locales
}

type FlagsLoggerConfig struct {
	OutputPath      string
	ErrorOutputPath string
}

type FlagsConfig struct {
	Mode        AppMode
	ConfigsPath string
	CertsPath   string
	Logger      FlagsLoggerConfig
	LocalesPath string
	Locales     Locales
}

func ParseFlags() *ParsedFlags {
	appModeFlag := flag.String(APP_MODE_FLAG, string(AppModeDev), "application mode")
	configsPathFlag := flag.String(CONFIGS_FLAG, CONFIGS_PATH_DEFAULT, "configs path")
	certsPathFlag := flag.String(CERTS_FLAG, CERTS_PATH_DEFAULT, "api certificates path")
	loggerOutputPath := flag.String(LOGGER_OUTPUT_PATH_FLAG, LOGGER_OUTPUT_PATH_DEFAULT, "logger output logs file path")
	loggerErrorOutputPath := flag.String(LOGGER_ERROR_OUTPUT_PATH_FLAG, LOGGER_ERROR_OUTPUT_PATH_DEFAULT, "logger output error logs file path")
	localesPathFlag := flag.String(LOCALES_PATH_FLAG, LOCALES_PATH_DEFAULT, "locales path")
	var localesFlag Locales
	flag.Var(&localesFlag, LOCALES_PATH, "comma-separated list of bot locales")
	parsedFlags := &ParsedFlags{
		AppMode:               appModeFlag,
		ConfigsPath:           configsPathFlag,
		CertsPath:             certsPathFlag,
		LoggerOutputPath:      loggerOutputPath,
		LoggerErrorOutputPath: loggerErrorOutputPath,
		LocalesPath:           localesPathFlag,
		Locales:               &localesFlag,
	}

	flag.Parse()

	return parsedFlags
}

func SetupFlags(parsedFlags *ParsedFlags) *FlagsConfig {
	appMode := AppModeDev
	configsPath := CONFIGS_PATH_DEFAULT
	certsPath := CERTS_PATH_DEFAULT
	loggerConfig := FlagsLoggerConfig{
		OutputPath:      LOGGER_OUTPUT_PATH_DEFAULT,
		ErrorOutputPath: LOGGER_ERROR_OUTPUT_PATH_DEFAULT,
	}
	localesPath := LOCALES_PATH_DEFAULT
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

	if parsedFlags.LoggerOutputPath != nil {
		loggerConfig.OutputPath = *parsedFlags.LoggerOutputPath
	}

	if parsedFlags.LoggerErrorOutputPath != nil {
		loggerConfig.ErrorOutputPath = *parsedFlags.LoggerErrorOutputPath
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

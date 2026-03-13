package flags

import (
	"flag"

	"golang.org/x/text/language"
)

const (
	ConfigsFlag          = "configs"
	PoolAPICertsFlag     = "pool-api-certs"
	LoggerOutputPathFlag = "logger-output-path"
	LocalesPathFlag      = "locales-path"
	LocalesFlag          = "locales"

	ConfigsPathDefault      = "configs"
	PoolAPICertsPathDefault = "certs"
	LocalesPathDefault      = "locales"
	LoggerOutputPathDefault = "logs/output.log"
)

type ParsedFlags struct {
	AppMode          *string
	ConfigsPath      *string
	PoolAPICertsPath *string
	LoggerOutputPath *string
	LogLevel         *string
	LocalesPath      *string
	Locales          *Locales
}

type FlagsLoggerConfig struct {
	OutputPath string
	Level      LogLevel
}

type FlagsConfig struct {
	Mode             AppMode
	ConfigsPath      string
	PoolAPICertsPath string
	Logger           FlagsLoggerConfig
	LocalesPath      string
	Locales          Locales
}

func DefineFlags() *ParsedFlags {
	appModeFlag := flag.String(AppModeFlag, string(AppModeDev), "application mode")
	configsPathFlag := flag.String(ConfigsFlag, ConfigsPathDefault, "configs path")
	poolAPICertsPathFlag := flag.String(PoolAPICertsFlag, PoolAPICertsPathDefault, "pool api certificates path")
	loggerOutputPath := flag.String(LoggerOutputPathFlag, LoggerOutputPathDefault, "logger output logs file path")
	logLevelFlag := flag.String(LogLevelFlag, string(LogLevelInfo), "log level")
	localesPathFlag := flag.String(LocalesPathFlag, LocalesPathDefault, "locales path")
	var localesFlag Locales
	flag.Var(&localesFlag, LocalesFlag, "comma-separated list of bot locales")
	parsedFlags := &ParsedFlags{
		AppMode:          appModeFlag,
		ConfigsPath:      configsPathFlag,
		PoolAPICertsPath: poolAPICertsPathFlag,
		LoggerOutputPath: loggerOutputPath,
		LocalesPath:      localesPathFlag,
		Locales:          &localesFlag,
		LogLevel:         logLevelFlag,
	}

	flag.Parse()

	return parsedFlags
}

func SetupFlags(parsedFlags *ParsedFlags) *FlagsConfig {
	appMode := AppModeDev
	configsPath := ConfigsPathDefault
	poolAPICertsPath := PoolAPICertsPathDefault
	loggerConfig := FlagsLoggerConfig{
		OutputPath: LoggerOutputPathDefault,
	}
	localesPath := LocalesPathDefault
	locales := []language.Tag{language.English}

	if parsedFlags.AppMode != nil {
		appMode = checkAppMode(*parsedFlags.AppMode)
	}

	if parsedFlags.ConfigsPath != nil {
		configsPath = *parsedFlags.ConfigsPath
	}

	if parsedFlags.PoolAPICertsPath != nil {
		poolAPICertsPath = *parsedFlags.PoolAPICertsPath
	}

	if parsedFlags.LoggerOutputPath != nil {
		loggerConfig.OutputPath = *parsedFlags.LoggerOutputPath
	}

	if parsedFlags.LogLevel != nil {
		loggerConfig.Level = checkLogLevel(*parsedFlags.LogLevel)
	}

	if parsedFlags.LocalesPath != nil {
		localesPath = *parsedFlags.LocalesPath
	}

	if parsedFlags.Locales != nil && len(*parsedFlags.Locales) > 0 {
		locales = *parsedFlags.Locales
	}

	return &FlagsConfig{
		Mode:             appMode,
		ConfigsPath:      configsPath,
		PoolAPICertsPath: poolAPICertsPath,
		Logger:           loggerConfig,
		LocalesPath:      localesPath,
		Locales:          locales,
	}
}

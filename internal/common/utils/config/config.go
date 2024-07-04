package configUtils

import (
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	configErrors "github.com/grandminingpool/telegram-bot/internal/common/errors/config"
)

func ReadConfig(viper *viper.Viper, configName string) error {
	if err := viper.ReadInConfig(); err != nil {
		return &configErrors.ReadConfigError{ConfigName: configName, Err: err}
	}

	return nil
}

func LoadConfig[T any](viper *viper.Viper, validate *validator.Validate, configName string) (*T, error) {
	var config T
	if err := viper.Unmarshal(&config); err != nil {
		return nil, &configErrors.UnmarshalError{ConfigName: configName, Err: err}
	}

	if err := validate.Struct(config); err != nil {
		return nil, &configErrors.ValidationError{ConfigName: configName, Err: err}
	}

	return &config, nil
}
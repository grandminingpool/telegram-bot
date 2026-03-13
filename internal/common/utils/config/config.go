package config

import (
	"github.com/go-playground/validator/v10"
	config_errors "github.com/grandminingpool/telegram-bot/internal/common/errors/config"
	"github.com/spf13/viper"
)

func ReadConfig(viper *viper.Viper, configName string) error {
	if err := viper.ReadInConfig(); err != nil {
		return &config_errors.ReadConfigError{ConfigName: configName, Err: err}
	}

	return nil
}

func LoadConfig[T any](viper *viper.Viper, validate *validator.Validate, configName string) (*T, error) {
	var config T
	if err := viper.Unmarshal(&config); err != nil {
		return nil, &config_errors.UnmarshalError{ConfigName: configName, Err: err}
	}

	if err := validate.Struct(config); err != nil {
		return nil, &config_errors.ValidationError{ConfigName: configName, Err: err}
	}

	return &config, nil
}

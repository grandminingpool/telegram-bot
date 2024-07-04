package botConfig

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	configUtils "github.com/grandminingpool/telegram-bot/internal/common/utils/config"
	"github.com/spf13/viper"
)

type Config struct {
	BotToken string `mapstructure:"botToken" validate:"required"`
}

const configName = "bot"

func New(configsPath string, validate *validator.Validate) (*Config, error) {
	botViper := viper.New()
	botViper.AddConfigPath(fmt.Sprintf("%s/bot", configsPath))
	botViper.SetConfigType("yaml")

	if err := configUtils.ReadConfig(botViper, configName); err != nil {
		return nil, err
	}

	config, err := configUtils.LoadConfig[Config](botViper, validate, configName)
	if err != nil {
		return nil, err
	}

	return config, nil
}

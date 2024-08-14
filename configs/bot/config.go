package botConfig

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	configUtils "github.com/grandminingpool/telegram-bot/internal/common/utils/config"
	"github.com/spf13/viper"
)

type CheckIntervalsConfig struct {
	Workers  int8 `mapstructure:"workers"`
	Blocks   int8 `mapstructure:"blocks"`
	Payments int8 `mapstructure:"payments"`
}

type SupportBotConfig struct {
	UserID   int64  `mapstructure:"userID" validate:"required"`
	Username string `mapstructure:"username" validate:"required"`
}

type NotifyConfig struct {
	MaxWalletsInRequest  int `mapstructure:"maxWalletsInRequest"`
	MaxUsersChangesLimit int `mapstructure:"maxUsersChangesLimit"`
}

type Config struct {
	BotToken            string               `mapstructure:"botToken" validate:"required"`
	CheckIntervals      CheckIntervalsConfig `mapstructure:"checkIntervals"`
	PoolURL             string               `mapstructure:"poolURL" validate:"required"`
	SupportBot          SupportBotConfig     `mapstructure:"supportBot" validate:"required"`
	WalletsLimitPerUser int                  `mapstructure:"walletsLimitPerUser"`
	Notify              NotifyConfig         `mapstructure:"notify"`
}

const configName = "bot"

func New(configsPath string, validate *validator.Validate) (*Config, error) {
	botViper := viper.New()
	botViper.AddConfigPath(fmt.Sprintf("%s/bot", configsPath))
	botViper.SetConfigType("yaml")

	botViper.SetDefault("checkIntervals.workers", 5)
	botViper.SetDefault("checkIntervals.payments", 60)
	botViper.SetDefault("checkIntervals.blocks", 120)
	botViper.SetDefault("walletsLimitPerUser", 50)
	botViper.SetDefault("notify.maxWalletsInRequest", 200)
	botViper.SetDefault("notify.maxUsersChangesLimit", 50)

	if err := configUtils.ReadConfig(botViper, configName); err != nil {
		return nil, err
	}

	config, err := configUtils.LoadConfig[Config](botViper, validate, configName)
	if err != nil {
		return nil, err
	}

	return config, nil
}

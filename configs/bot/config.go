package bot

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	config_utils "github.com/grandminingpool/telegram-bot/internal/common/utils/config"
	"github.com/spf13/viper"
)

type CheckIntervalsConfig struct {
	Workers int `mapstructure:"workers"`
	Payouts int `mapstructure:"payouts"`
}

func (c CheckIntervalsConfig) WorkersDuration() time.Duration {
	return time.Duration(c.Workers) * time.Minute
}

func (c CheckIntervalsConfig) PayoutsDuration() time.Duration {
	return time.Duration(c.Payouts) * time.Minute
}

type NotifyConfig struct {
	MaxWalletsInPayoutsRequest int                  `mapstructure:"maxWalletsInPayoutsRequest"`
	MaxWalletsInWorkersRequest int                  `mapstructure:"maxWalletsInWorkersRequest"`
	MaxUsersDBChangesLimit     int                  `mapstructure:"maxUsersDBChangesLimit"`
	ParallelNotificationsCount int                  `mapstructure:"parallelNotificationsCount"`
	CheckIntervals             CheckIntervalsConfig `mapstructure:"checkIntervals"`
}

type Config struct {
	BotToken            string       `mapstructure:"botToken" validate:"required"`
	PoolURL             string       `mapstructure:"poolURL" validate:"required"`
	PoolChatLink        string       `mapstructure:"poolChatLink" validate:"required"`
	SupportChatID       int64        `mapstructure:"supportChatID" validate:"required"`
	WalletsLimitPerUser int          `mapstructure:"walletsLimitPerUser"`
	Notify              NotifyConfig `mapstructure:"notify"`
}

const configName = "bot"

func New(configsPath string, validate *validator.Validate) (*Config, error) {
	botViper := viper.New()
	botViper.AddConfigPath(fmt.Sprintf("%s/bot", configsPath))
	botViper.SetConfigType("yaml")

	botViper.SetDefault("walletsLimitPerUser", 50)
	botViper.SetDefault("notify.maxWalletsInWorkersRequest", 200)
	botViper.SetDefault("notify.maxWalletsInPayoutsRequest", 250)
	botViper.SetDefault("notify.maxUsersDBChangesLimit", 50)
	botViper.SetDefault("notify.parallelNotificationsCount", 40)
	botViper.SetDefault("notify.paymentsInterval", 60)
	botViper.SetDefault("notify.soloPaymentsInterval", 200)
	botViper.SetDefault("notify.checkIntervals.workers", 5)
	botViper.SetDefault("notify.checkIntervals.payouts", 60)

	if err := config_utils.ReadConfig(botViper, configName); err != nil {
		return nil, err
	}

	config, err := config_utils.LoadConfig[Config](botViper, validate, configName)
	if err != nil {
		return nil, err
	}

	return config, nil
}

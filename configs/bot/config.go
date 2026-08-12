package bot

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	config_utils "github.com/grandminingpool/telegram-bot/internal/common/utils/config"
	"github.com/spf13/viper"
)

// CheckIntervalsConfig holds the background-check intervals in minutes.
// Values are converted to clock-aligned cron expressions at scheduler setup
// (see internal/utils/cron.FromMinutes) so :00, :05, :10… run on the hour
// boundary regardless of when the bot was last restarted.
//
// Workers must divide 60 (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30, 60).
// Payouts must divide 60 OR be a multiple of 60 that divides 1440
// (60, 120, 180, 240, 360, 480, 720, 1440).
type CheckIntervalsConfig struct {
	Workers int `mapstructure:"workers" validate:"required,gt=0"`
	Payouts int `mapstructure:"payouts" validate:"required,gt=0"`
}

type NotifyConfig struct {
	MaxWalletsInPayoutsRequest              int                  `mapstructure:"maxWalletsInPayoutsRequest"`
	MaxWalletsInWorkersRequest              int                  `mapstructure:"maxWalletsInWorkersRequest"`
	MaxUsersDBChangesLimit                  int                  `mapstructure:"maxUsersDBChangesLimit"`
	ParallelNotificationsCount              int                  `mapstructure:"parallelNotificationsCount"`
	SentNotificationsRetentionDays          int                  `mapstructure:"sentNotificationsRetentionDays" validate:"gt=0"`
	ImmatureBlockNotificationsRetentionDays int                  `mapstructure:"immatureBlockNotificationsRetentionDays" validate:"gt=0"`
	CheckIntervals                          CheckIntervalsConfig `mapstructure:"checkIntervals"`
}

type Config struct {
	BotToken            string        `mapstructure:"botToken" validate:"required"`
	PoolURL             string        `mapstructure:"poolURL" validate:"required"`
	PoolChatLink        string        `mapstructure:"poolChatLink" validate:"required"`
	SupportChatID       int64         `mapstructure:"supportChatID" validate:"required"`
	WalletsLimitPerUser int           `mapstructure:"walletsLimitPerUser"`
	PoolAPITimeout      time.Duration `mapstructure:"poolAPITimeout" validate:"gt=0"`
	Notify              NotifyConfig  `mapstructure:"notify"`
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
	botViper.SetDefault("notify.sentNotificationsRetentionDays", 30)
	botViper.SetDefault("notify.immatureBlockNotificationsRetentionDays", 3)
	botViper.SetDefault("notify.checkIntervals.workers", 5)
	botViper.SetDefault("notify.checkIntervals.payouts", 60)
	botViper.SetDefault("poolAPITimeout", "30s")

	if err := config_utils.ReadConfig(botViper, configName); err != nil {
		return nil, err
	}

	config, err := config_utils.LoadConfig[Config](botViper, validate, configName)
	if err != nil {
		return nil, err
	}

	return config, nil
}

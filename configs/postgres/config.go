package postgresConfig

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	configUtils "github.com/grandminingpool/telegram-bot/internal/common/utils/config"
	"github.com/spf13/viper"
)

type Config struct {
	Host     string `mapstructure:"host"`
	Port     int16  `mapstructure:"port"`
	User     string `mapstructure:"user" validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
	Database string `mapstructure:"database" validate:"required"`
}

const configName = "postgres"

func (c *Config) DSN() string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", c.User, c.Password, c.Host, c.Port, c.Database)
}

func New(configsPath string, validate *validator.Validate) (*Config, error) {
	postgresViper := viper.New()
	postgresViper.AddConfigPath(fmt.Sprintf("%s/postgres", configsPath))
	postgresViper.SetConfigType("yaml")

	postgresViper.SetDefault("host", "127.0.0.1")
	postgresViper.SetDefault("port", 5432)

	if err := configUtils.ReadConfig(postgresViper, configName); err != nil {
		return nil, err
	}

	config, err := configUtils.LoadConfig[Config](postgresViper, validate, configName)
	if err != nil {
		return nil, err
	}

	return config, nil
}

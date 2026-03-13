package postgres

import (
	"fmt"
	"net/url"

	"github.com/go-playground/validator/v10"
	config_utils "github.com/grandminingpool/telegram-bot/internal/common/utils/config"
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
	userInfo := url.UserPassword(c.User, c.Password)

	return fmt.Sprintf("postgresql://%s@%s:%d/%s?sslmode=disable", userInfo.String(), c.Host, c.Port, c.Database)
}

func New(configsPath string, validate *validator.Validate) (*Config, error) {
	postgresViper := viper.New()
	postgresViper.AddConfigPath(fmt.Sprintf("%s/postgres", configsPath))
	postgresViper.SetConfigType("yaml")

	postgresViper.SetDefault("host", "127.0.0.1")
	postgresViper.SetDefault("port", 5432)

	if err := config_utils.ReadConfig(postgresViper, configName); err != nil {
		return nil, err
	}

	config, err := config_utils.LoadConfig[Config](postgresViper, validate, configName)
	if err != nil {
		return nil, err
	}

	return config, nil
}

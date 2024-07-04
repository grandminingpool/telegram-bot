package poolBot

import (
	"fmt"

	"github.com/go-telegram/bot"
	botConfig "github.com/grandminingpool/telegram-bot/configs/bot"
	"github.com/grandminingpool/telegram-bot/internal/bot/handlers"
)

func CreateBot(defaultHandler *handlers.DefaultHandler, config *botConfig.Config) (*bot.Bot, error) {
	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler.Handler),
	}
	b, err := bot.New(config.BotToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot instance: %w", err)
	}

	return b, nil
}

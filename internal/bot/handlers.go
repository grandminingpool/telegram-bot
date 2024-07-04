package poolBot

import (
	"github.com/go-telegram/bot"
	"github.com/grandminingpool/telegram-bot/internal/common/constants"
)

func RegisterHandlers(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeMessageText, string(constants.StartCommand), bot.MatchTypeExact, nil)
}

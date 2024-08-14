package botKeyboards

import (
	"github.com/go-telegram/bot"
	"github.com/go-telegram/ui/keyboard/reply"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const BACK_KEYBOARD_PREFIX = "back"

func CreateBackReplyKeyboard(b *bot.Bot, backHandler middlewares.UserHandlerFunc, localizer *i18n.Localizer) *reply.ReplyKeyboard {
	return reply.New(b, reply.IsSelective(), reply.WithPrefix(BACK_KEYBOARD_PREFIX)).Row().Button(localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "BackButton",
	}), b, bot.MatchTypeExact, middlewares.WithUserHandler(backHandler)).Row()
}

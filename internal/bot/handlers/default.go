package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	botKeyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/common/constants"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type DefaultHandler struct {
	languages *languages.Languages
}

func (h *DefaultHandler) Handler(ctx context.Context, user *middlewares.User, startKeyboard *botKeyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "DefaultMessage",
			TemplateData: map[string]string{
				"Command": string(constants.StartCommand),
			},
		}),
		ReplyMarkup: botKeyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})

	b.SetChatMenuButton(ctx, &bot.SetChatMenuButtonParams{
		ChatID: update.Message.Chat.ID,
		MenuButton: models.MenuButtonCommands{
			Type: models.MenuButtonTypeCommands,
		},
	})
}

func NewDefaultHandler(languages *languages.Languages) *DefaultHandler {
	return &DefaultHandler{
		languages: languages,
	}
}

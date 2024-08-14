package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type FAQHandler struct {
	poolURL              string
	checkWorkersInterval int8
	supportBotUsername   string
}

func (h *FAQHandler) Handler(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "FAQText",
			TemplateData: map[string]string{
				"PoolURL":              h.poolURL,
				"CheckWorkersInterval": fmt.Sprintf("%d", h.checkWorkersInterval),
				"SupportBotUsername":   h.supportBotUsername,
			},
		}),
	})
}

func NewFAQHandler(poolURL string, checkWorkersInterval int8, supportBotUsername string) *FAQHandler {
	return &FAQHandler{
		poolURL:              poolURL,
		checkWorkersInterval: checkWorkersInterval,
		supportBotUsername:   supportBotUsername,
	}
}

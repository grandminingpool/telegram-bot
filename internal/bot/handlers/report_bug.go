package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	bot_keyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

type ReportBugHandler struct {
	userActionService  *services.UserActionService
	supportChatID      int64
	poolChatLink string
}

func (h *ReportBugHandler) Back(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
	if err := h.userActionService.Clear(ctx, user.ID); err != nil {
		zap.L().Error("error clearing user action before returning to main menu",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ReturningToMenu",
		}),
		ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})
}

func (h *ReportBugHandler) Enter(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	if err := h.userActionService.Set(ctx, user.ID, services.ReportBugAction, nil); err != nil {
		zap.L().Error("set user report bug action error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ReportBugMessage",
		}),
		ReplyMarkup: bot_keyboards.CreateBackReplyKeyboard(user.Localizer),
	})
}

func (h *ReportBugHandler) SendFeedback(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
	feedbackText := fmt.Sprintf("Feedback from user %d", user.ID)
	if update.Message.From != nil {
		name := update.Message.From.FirstName
		if update.Message.From.LastName != "" {
			name += " " + update.Message.From.LastName
		}

		if name != "" {
			feedbackText = fmt.Sprintf("Feedback from %s (ID: %d)", name, user.ID)
		}

		if update.Message.From.Username != "" {
			feedbackText += fmt.Sprintf(" @%s", update.Message.From.Username)
		}
	}

	feedbackText += fmt.Sprintf("\n\n%s", update.Message.Text)

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: h.supportChatID,
		Text:      feedbackText,
	}); err != nil {
		zap.L().Error("failed to send feedback to support user",
			zap.Int64("user_id", user.ID),
			zap.Int64("support_chat_id", h.supportChatID),
			zap.String("message", update.Message.Text),
			zap.Error(err),
		)

		return
	}

	if err := h.userActionService.Clear(ctx, user.ID); err != nil {
		zap.L().Error("error clearing user action after sending feedback",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "UserFeedbackSent",
			TemplateData: map[string]string{
				"PoolChatLink": h.poolChatLink,
			},
		}),
		ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})
}

func NewReportBugHandler(
	userActionService *services.UserActionService,
	supportChatID int64,
	poolChatLink string,
) *ReportBugHandler {
	return &ReportBugHandler{
		userActionService:  userActionService,
		supportChatID:      supportChatID,
		poolChatLink: poolChatLink,
	}
}

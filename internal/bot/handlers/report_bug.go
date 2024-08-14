package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	botKeyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

type ReportBugHandler struct {
	feedbackService    *services.FeedbackService
	userActionService  *services.UserActionService
	supportBotUsername string
}

func (h *ReportBugHandler) Back(ctx context.Context, user *middlewares.User, startKeyboard *botKeyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
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
		ReplyMarkup: botKeyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
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
		ReplyMarkup: botKeyboards.CreateBackReplyKeyboard(b, botKeyboards.WithStartKeyboardHandler(h.Back), user.Localizer),
	})
}

func (h *ReportBugHandler) SendFeedback(ctx context.Context, user *middlewares.User, startKeyboard *botKeyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
	payload := &services.AddFeedbackPayload{
		ReportMessage: update.Message.Text,
	}

	if update.Message.From != nil {
		if update.Message.From.FirstName != "" {
			payload.FirstName = &update.Message.From.FirstName
		}

		if update.Message.From.LastName != "" {
			payload.LastName = &update.Message.From.LastName
		}

		if update.Message.From.Username != "" {
			payload.Username = &update.Message.From.Username
		}
	}

	if err := h.feedbackService.Add(ctx, user.ID, payload); err != nil {
		zap.L().Error("add feedback error",
			zap.Int64("user_id", user.ID),
			zap.String("message", payload.ReportMessage),
			zap.Stringp("first_name", payload.FirstName),
			zap.Stringp("last_name", payload.LastName),
			zap.Stringp("username", payload.Username),
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
				"SupportBotUsername": h.supportBotUsername,
			},
		}),
		ReplyMarkup: botKeyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})
}

func NewReportBugHandler(
	feedbackService *services.FeedbackService,
	userActionService *services.UserActionService,
	supportBotUsername string,
) *ReportBugHandler {
	return &ReportBugHandler{
		feedbackService:    feedbackService,
		userActionService:  userActionService,
		supportBotUsername: supportBotUsername,
	}
}

package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	botKeyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

type EnterWalletHandler struct {
	userActionService *services.UserActionService
}

func (h *EnterWalletHandler) Back(
	ctx context.Context,
	user *middlewares.User,
	startKeyboard *botKeyboards.StartKeyboard,
	b *bot.Bot,
	update *models.Update,
) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ReturningToMenu",
		}),
		ReplyMarkup: botKeyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})
}

func (h *EnterWalletHandler) BackToBlockchainSelect(
	ctx context.Context,
	user *middlewares.User,
	addWalletKeyboard *botKeyboards.BlockchainsKeyboard,
	b *bot.Bot,
	update *models.Update,
) {
	if err := h.userActionService.Clear(ctx, user.ID); err != nil {
		zap.L().Error("error clearing user action before returning to add wallet select blockchains menu",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "SelectBlockchain",
		}),
		ReplyMarkup: botKeyboards.CreateBlockchainsReplyKeyboard(b, addWalletKeyboard, user.Localizer),
	})
}

func (h *EnterWalletHandler) Handler(
	ctx context.Context,
	user *middlewares.User,
	blockchain *blockchains.BlockchainInfo,
	b *bot.Bot,
	update *models.Update,
) {
	if err := h.userActionService.Set(ctx, user.ID, services.UserAddWalletAction, &blockchain.Coin); err != nil {
		zap.L().Error("set user add wallet action error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "EnterWallet",
			TemplateData: map[string]string{
				"ExampleWallet": blockchain.ExampleWallet,
			},
		}),
		ReplyMarkup: botKeyboards.CreateBackReplyKeyboard(b, botKeyboards.WithBlockchainsKeyboardHandler(h.BackToBlockchainSelect, botKeyboards.ADD_WALLET_KEYBOARD_CTX_KEY), user.Localizer),
	})
}

func NewEnterWalletHandler(userActionService *services.UserActionService) *EnterWalletHandler {
	return &EnterWalletHandler{
		userActionService: userActionService,
	}
}

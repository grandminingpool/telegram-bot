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

type WalletHandler struct {
	userActionService *services.UserActionService
	addWalletKeyboard *botKeyboards.BlockchainsKeyboard
}

func (h *WalletHandler) Back(ctx context.Context, b *bot.Bot, update *models.Update) {
	user, ok := ctx.Value(middlewares.USER_CTX_KEY).(*middlewares.User)
	if ok {
		if err := h.userActionService.Clear(ctx, user.ID); err != nil {
			zap.L().Error("error clearing user action before returning to add wallet menu",
				zap.Error(err),
				zap.Int64("user_id", user.ID),
			)

			return
		}

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "SelectBlockchain",
			}),
			ReplyMarkup: botKeyboards.CreateBlockchainsReplyKeyboard(b, h.addWalletKeyboard, user.Localizer),
		})
	}
}

func (h *WalletHandler) Enter(ctx context.Context, blockchain *blockchains.BlockchainInfo, b *bot.Bot, update *models.Update) {
	user, ok := ctx.Value(middlewares.USER_CTX_KEY).(*middlewares.User)
	if ok {
		if err := h.userActionService.Set(ctx, user.ID, services.UserAddWalletAction, &blockchain.Coin); err != nil {
			zap.L().Error("set user add wallet action error", zap.Error(err), zap.Int64("user_id", user.ID))

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
			ReplyMarkup: botKeyboards.CreateBackReplyKeyboard(b, h.Back, user.Localizer),
		})
	}
}

func (h *WalletHandler) Add(ctx context.Context, bot *bot.Bot, update *models.Update) {

}

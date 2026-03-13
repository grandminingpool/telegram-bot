package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	bot_keyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

type RemoveWalletHandler struct {
	userWalletService *services.UserWalletService
	userActionService *services.UserActionService
}

func (h *RemoveWalletHandler) Back(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,

		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ReturningToMenu",
		}),
		ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})
}

func (h *RemoveWalletHandler) BackToBlockchainSelect(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	userBlockchains, err := h.userWalletService.FindBlockchains(ctx, user.ID)
	if err != nil {
		zap.L().Error("find user blockchains error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	if err := h.userActionService.Set(ctx, user.ID, services.UserRemoveWalletAction, nil); err != nil {
		zap.L().Error("set user select blockchain remove wallet action error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,

		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "SelectBlockchain",
		}),
		ReplyMarkup: bot_keyboards.CreateBlockchainsReplyKeyboard(userBlockchains, user.Localizer),
	})
}

func (h *RemoveWalletHandler) OnBlockchainSelected(
	ctx context.Context,
	user *middlewares.User,
	blockchain blockchains.BlockchainInfo,
	b *bot.Bot,
	update *models.Update,
) {
	userWallets, err := h.userWalletService.FindBlockchainWallets(ctx, user.ID, blockchain.Coin)
	if err != nil {
		zap.L().Error("find user blockchain wallets error",
			zap.Int64("user_id", user.ID),
			zap.String("coin", blockchain.Coin),
			zap.Error(err),
		)

		return
	}

	if err := h.userActionService.Set(ctx, user.ID, services.UserRemoveWalletSelectWalletAction, &blockchain.Coin); err != nil {
		zap.L().Error("set user remove wallet select wallet action error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "SelectWallet",
		}),
		ReplyMarkup: bot_keyboards.CreateWalletsReplyKeyboard(userWallets, user.Localizer),
	})
}

func (h *RemoveWalletHandler) Remove(
	ctx context.Context,
	user *middlewares.User,
	startKeyboard *bot_keyboards.StartKeyboard,
	wallet services.UserWalletInfo,
	b *bot.Bot,
	update *models.Update,
) {
	if err := h.userWalletService.Remove(ctx, wallet.ID); err != nil {
		zap.L().Error("remove user wallet error",
			zap.Int64("user_id", user.ID),
			zap.Int64("wallet_id", wallet.ID),
			zap.Error(err),
		)

		return
	}

	if err := h.userActionService.Clear(ctx, user.ID); err != nil {
		zap.L().Error("clear user action after removing wallet error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,

		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "WalletRemoved",
		}),
		ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})
}

func NewRemoveWalletHandler(userWalletService *services.UserWalletService, userActionService *services.UserActionService) *RemoveWalletHandler {
	return &RemoveWalletHandler{
		userWalletService: userWalletService,
		userActionService: userActionService,
	}
}

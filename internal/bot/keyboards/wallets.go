package botKeyboards

import (
	"context"
	"slices"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/ui/keyboard/reply"

	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const WALLETS_KEYBOARD_PREFIX = "wallets"

type WalletsKeyboardHandlerFunc func(context.Context, *middlewares.User, *WalletsKeyboard, *bot.Bot, *models.Update)
type OnWalletSelectedHandlerFunc func(context.Context, *middlewares.User, services.UserWalletInfo, *bot.Bot, *models.Update)
type OnWalletSelectedWithStartKeyboardHandlerFunc func(context.Context, *middlewares.User, *StartKeyboard, services.UserWalletInfo, *bot.Bot, *models.Update)

type WalletsKeyboard struct {
	wallets         []services.UserWalletInfo
	onSelectHandler OnWalletSelectedHandlerFunc
	onBackHandler   middlewares.UserHandlerFunc
}

func (k *WalletsKeyboard) OnWalletSelected(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	idx := slices.IndexFunc(k.wallets, func(wallet services.UserWalletInfo) bool {
		return wallet.Wallet == update.Message.Text
	})
	if idx != -1 {
		wallet := k.wallets[idx]

		k.onSelectHandler(ctx, user, wallet, b, update)
	}
}

func CreateWalletsKeyboard(
	wallets []services.UserWalletInfo,
	onSelectHandler OnWalletSelectedHandlerFunc,
	onBackHandler middlewares.UserHandlerFunc) *WalletsKeyboard {
	return &WalletsKeyboard{
		wallets:         wallets,
		onSelectHandler: onSelectHandler,
		onBackHandler:   onBackHandler,
	}
}

func CreateWalletsReplyKeyboard(b *bot.Bot, walletsKeyboard *WalletsKeyboard, localizer *i18n.Localizer) *reply.ReplyKeyboard {
	replyKeyboard := reply.New(b, reply.IsSelective(), reply.WithPrefix(WALLETS_KEYBOARD_PREFIX)).Row()
	for _, wallet := range walletsKeyboard.wallets {
		replyKeyboard = replyKeyboard.Button(wallet.Wallet, b, bot.MatchTypeExact, middlewares.WithUserHandler(walletsKeyboard.OnWalletSelected)).Row()
	}

	return replyKeyboard.Button(localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "BackButton",
	}), b, bot.MatchTypeExact, middlewares.WithUserHandler(walletsKeyboard.onBackHandler)).Row()
}

func OnWalletSelectedWithStartKeyboardHandler(handler OnWalletSelectedWithStartKeyboardHandlerFunc) OnWalletSelectedHandlerFunc {
	return func(ctx context.Context, user *middlewares.User, wallet services.UserWalletInfo, b *bot.Bot, update *models.Update) {
		startKeyboard, ok := ctx.Value(START_KEYBOARD_CTX_KEY).(*StartKeyboard)
		if ok {
			handler(ctx, user, startKeyboard, wallet, b, update)
		}
	}
}

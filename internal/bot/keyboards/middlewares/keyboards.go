package keyboardsMiddlewares

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	botKeyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
)

type KeyboardsMiddleware struct {
	addWalletKayboard *botKeyboards.BlockchainsKeyboard
	startKayboard     *botKeyboards.StartKeyboard
}

func (m *KeyboardsMiddleware) Middleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		addWalletWeyboardCtx := context.WithValue(ctx, botKeyboards.ADD_WALLET_KEYBOARD_CTX_KEY, m.addWalletKayboard)
		startKeyboardCtx := context.WithValue(addWalletWeyboardCtx, botKeyboards.START_KEYBOARD_CTX_KEY, m.startKayboard)

		next(startKeyboardCtx, b, update)
	}
}

func CreateKeyboardsMiddleware(
	addWalletKayboard *botKeyboards.BlockchainsKeyboard,
	startKayboard *botKeyboards.StartKeyboard,
) *KeyboardsMiddleware {
	return &KeyboardsMiddleware{
		addWalletKayboard: addWalletKayboard,
		startKayboard:     startKayboard,
	}
}

package middlewares

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	bot_keyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
)

type KeyboardsMiddleware struct {
	startKeyboard *bot_keyboards.StartKeyboard
}

func (m *KeyboardsMiddleware) Middleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		startKeyboardCtx := context.WithValue(ctx, bot_keyboards.StartKeyboardCtxKey, m.startKeyboard)

		next(startKeyboardCtx, b, update)
	}
}

func CreateKeyboardsMiddleware(
	startKeyboard *bot_keyboards.StartKeyboard,
) *KeyboardsMiddleware {
	return &KeyboardsMiddleware{
		startKeyboard: startKeyboard,
	}
}

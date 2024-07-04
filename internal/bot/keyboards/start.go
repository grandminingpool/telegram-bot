package botKeyboards

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/ui/keyboard/reply"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const START_KEYBOARD_PREFIX = "start"

type StartKeyboard struct {
	addWalletKeyboard *BlockchainsKeyboard
}

func (k *StartKeyboard) OnAddWallet(ctx context.Context, b *bot.Bot, update *models.Update) {
	user, ok := ctx.Value(middlewares.USER_CTX_KEY).(*middlewares.User)
	if ok {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "SelectBlockchain",
			}),
			ReplyMarkup: CreateBlockchainsReplyKeyboard(b, k.addWalletKeyboard, user.Localizer),
		})
	}
}

func (k *StartKeyboard) OnRemoveWallet(ctx context.Context, b *bot.Bot, update *models.Update) {

}

func CreateStartReplyKeyboard(b *bot.Bot, startKeyboard *StartKeyboard, localizer *i18n.Localizer) *reply.ReplyKeyboard {
	return reply.New(b, reply.IsSelective(), reply.WithPrefix(START_KEYBOARD_PREFIX)).
		Row().
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "AddWalletButton",
		}), b, bot.MatchTypeExact, startKeyboard.OnAddWallet).
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "RemoveWalletButton",
		}), b, bot.MatchTypeExact, startKeyboard.OnRemoveWallet).
		Row()
}

package botKeyboards

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/ui/keyboard/reply"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/grandminingpool/telegram-bot/internal/common/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

const (
	SETTINGS_KEYBOARD_PREFIX               = "settings"
	SETTINGS_KEYBOARD_CTX_KEY types.CtxKey = "settingsKeyboard"
)

type SettingsKeyboardHandlerFunc func(context.Context, *middlewares.User, *SettingsKeyboard, *bot.Bot, *models.Update)

type SettingsKeyboard struct {
	userService       *services.UserService
	startKeyboard     *StartKeyboard
	languagesKeyboard *LanguagesKeyboard
	payoutNotify      bool
	blockNotify       bool
}

func (k *SettingsKeyboard) IsPayoutNotify() bool {
	return k.payoutNotify
}

func (k *SettingsKeyboard) IsBlockNotify() bool {
	return k.blockNotify
}

func (k *SettingsKeyboard) TogglePayoutNotify(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	newPayoutNotify := !k.payoutNotify

	if err := k.userService.SetPayoutNotify(ctx, user.ID, newPayoutNotify); err != nil {
		zap.L().Error("update user payout notify error",
			zap.Int64("user_id", user.ID),
			zap.Bool("payout_notify", newPayoutNotify),
		)

		return
	}

	k.payoutNotify = newPayoutNotify

	var msgID string
	if newPayoutNotify {
		msgID = "PayoutNotificationsEnabled"
	} else {
		msgID = "PayoutNotificationsDisabled"
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: msgID,
		}),
		ReplyMarkup: CreateSettingsReplyKeyboard(b, k, user.Localizer),
	})
}

func (k *SettingsKeyboard) ToggleBlockNotify(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	newBlockNotify := !k.blockNotify

	if err := k.userService.SetBlockNotify(ctx, user.ID, newBlockNotify); err != nil {
		zap.L().Error("update user block notify error",
			zap.Int64("user_id", user.ID),
			zap.Bool("block_notify", newBlockNotify),
		)

		return
	}

	k.blockNotify = newBlockNotify

	var msgID string
	if newBlockNotify {
		msgID = "BlockNotificationsEnabled"
	} else {
		msgID = "BlockNotificationsDisabled"
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: msgID,
		}),
		ReplyMarkup: CreateSettingsReplyKeyboard(b, k, user.Localizer),
	})
}

func (k *SettingsKeyboard) ShowLanguages(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ChooseLanguage",
		}),
		ReplyMarkup: CreateLanguagesReplyKeyboard(b, k.languagesKeyboard, user.Localizer),
	})
}

func (k *SettingsKeyboard) Back(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ReturningToMenu",
		}),
		ReplyMarkup: CreateStartReplyKeyboard(b, k.startKeyboard, user.Localizer),
	})
}

func CreateSettingsReplyKeyboard(b *bot.Bot, settingsKeyboard *SettingsKeyboard, localizer *i18n.Localizer) *reply.ReplyKeyboard {
	var payoutNotifyMsgID, blockNotifyMsgID string

	if settingsKeyboard.IsPayoutNotify() {
		payoutNotifyMsgID = "SettingsDisablePayoutNotifyButton"
	} else {
		payoutNotifyMsgID = "SettingsEnablePayoutNotifyButton"
	}

	if settingsKeyboard.IsBlockNotify() {
		blockNotifyMsgID = "SettingsEnablePayoutsNotifyButton"
	} else {
		blockNotifyMsgID = "SettingsEnableBlockNotifyButton"
	}

	return reply.New(b, reply.IsSelective(), reply.WithPrefix(SETTINGS_KEYBOARD_PREFIX)).Row().
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: payoutNotifyMsgID,
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(settingsKeyboard.TogglePayoutNotify)).
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: blockNotifyMsgID,
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(settingsKeyboard.ToggleBlockNotify)).Row().
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "SettingsLanguageButton",
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(settingsKeyboard.ShowLanguages)).Row().
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "BackButton",
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(settingsKeyboard.Back)).Row()
}

func WithSettingsKeyboardHandler(handler SettingsKeyboardHandlerFunc) middlewares.UserHandlerFunc {
	return func(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
		settingsKeyboard, ok := ctx.Value(SETTINGS_KEYBOARD_CTX_KEY).(*SettingsKeyboard)
		if ok {
			handler(ctx, user, settingsKeyboard, b, update)
		}
	}
}

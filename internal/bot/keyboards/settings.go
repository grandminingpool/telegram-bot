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
	payoutsNotify     bool
	blocksNotify      bool
}

func (k *SettingsKeyboard) IsPayoutsNotify() bool {
	return k.payoutsNotify
}

func (k *SettingsKeyboard) IsBlocksNotify() bool {
	return k.blocksNotify
}

func (k *SettingsKeyboard) TogglePayoutsNotify(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	newPayoutsNotify := !k.payoutsNotify

	if err := k.userService.SetPayoutsNotify(ctx, user.ID, newPayoutsNotify); err != nil {
		zap.L().Error("update user payout notify error",
			zap.Int64("user_id", user.ID),
			zap.Bool("payouts_notify", newPayoutsNotify),
		)

		return
	}

	k.payoutsNotify = newPayoutsNotify

	var msgID string
	if newPayoutsNotify {
		msgID = "PayoutsNotificationsEnabled"
	} else {
		msgID = "PayoutsNotificationsDisabled"
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: msgID,
		}),
		ReplyMarkup: CreateSettingsReplyKeyboard(b, k, user.Localizer),
	})
}

func (k *SettingsKeyboard) ToggleBlocksNotify(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	newBlocksNotify := !k.blocksNotify

	if err := k.userService.SetBlocksNotify(ctx, user.ID, newBlocksNotify); err != nil {
		zap.L().Error("update user blocks notify error",
			zap.Int64("user_id", user.ID),
			zap.Bool("block_notify", newBlocksNotify),
		)

		return
	}

	k.blocksNotify = newBlocksNotify

	var msgID string
	if newBlocksNotify {
		msgID = "BlocksNotificationsEnabled"
	} else {
		msgID = "BlocksNotificationsDisabled"
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
	var payoutsNotifyMsgID, blocksNotifyMsgID string

	if settingsKeyboard.IsPayoutsNotify() {
		payoutsNotifyMsgID = "SettingsDisablePayoutsNotifyButton"
	} else {
		payoutsNotifyMsgID = "SettingsEnablePayoutsNotifyButton"
	}

	if settingsKeyboard.IsBlocksNotify() {
		blocksNotifyMsgID = "SettingsEnablePayoutsNotifyButton"
	} else {
		blocksNotifyMsgID = "SettingsEnableBlocksNotifyButton"
	}

	return reply.New(b, reply.IsSelective(), reply.WithPrefix(SETTINGS_KEYBOARD_PREFIX)).Row().
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: payoutsNotifyMsgID,
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(settingsKeyboard.TogglePayoutsNotify)).
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: blocksNotifyMsgID,
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(settingsKeyboard.ToggleBlocksNotify)).Row().
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

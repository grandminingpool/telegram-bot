package botKeyboards

import (
	"context"
	"slices"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/ui/keyboard/reply"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	LANGUAGES_KEYBOARD_PREFIX = "languages"
	LANGUAGES_KEYBOARD_COLS   = 2
)

type LanguagesKeyboard struct {
	userService *services.UserService
	localizers  []languages.LocalizersItem
}

func (k *LanguagesKeyboard) OnLocaleSelected(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	idx := slices.IndexFunc(k.localizers, func(l languages.LocalizersItem) bool {
		localeMsg, err := l.Localizer.Localize(&i18n.LocalizeConfig{
			MessageID: "Language",
		})

		return err == nil && localeMsg == update.Message.Text
	})

	if idx != -1 {
		l := k.localizers[idx]

		k.userService.SetLang(ctx, user.ID, l.Tag)
	}
}

func (k *LanguagesKeyboard) Back(ctx context.Context, user *middlewares.User, settingsKeyboard *SettingsKeyboard, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ReturningToSettingsMenu",
		}),
		ReplyMarkup: CreateSettingsReplyKeyboard(b, settingsKeyboard, user.Localizer),
	})
}

func CreateLanguagesKeyboard(
	userService *services.UserService,
	localizers []languages.LocalizersItem,
) *LanguagesKeyboard {
	return &LanguagesKeyboard{
		userService: userService,
		localizers:  localizers,
	}
}

func CreateLanguagesReplyKeyboard(b *bot.Bot, languagesKeyboard *LanguagesKeyboard, localizer *i18n.Localizer) *reply.ReplyKeyboard {
	replyKeyboard := reply.New(b, reply.IsSelective(), reply.WithPrefix(LANGUAGES_KEYBOARD_PREFIX)).Row()
	cols := 0
	for _, l := range languagesKeyboard.localizers {
		replyKeyboard = replyKeyboard.Button(l.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "Language",
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(languagesKeyboard.OnLocaleSelected))
		if cols == LANGUAGES_KEYBOARD_COLS {
			replyKeyboard = replyKeyboard.Row()
			cols = 0
		} else {
			cols++
		}
	}

	if cols < LANGUAGES_KEYBOARD_COLS {
		replyKeyboard = replyKeyboard.Row()
	}

	return replyKeyboard.Button(localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "BackButton",
	}), b, bot.MatchTypeExact, middlewares.WithUserHandler(WithSettingsKeyboardHandler(languagesKeyboard.Back))).Row()
}

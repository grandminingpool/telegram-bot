package poolBot

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func SetBotDescription(ctx context.Context, b *bot.Bot, localizers []languages.LocalizersItem) error {
	for _, l := range localizers {
		ok, err := b.SetMyDescription(ctx, &bot.SetMyDescriptionParams{
			Description: l.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "BotDescription",
			}),
			LanguageCode: l.Tag.String(),
		})
		if err != nil {
			return fmt.Errorf("failed to set bot description for locale: %s, error: %w", l.Tag.String(), err)
		} else if !ok {
			return fmt.Errorf("unsuccessful set bot description for locale: %s", l.Tag.String())
		}
	}

	return nil
}

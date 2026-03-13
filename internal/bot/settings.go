package pool_bot

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/grandminingpool/telegram-bot/internal/common/constants"
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

func SetBotCommands(ctx context.Context, b *bot.Bot, localizers []languages.LocalizersItem) error {
	for _, l := range localizers {
		ok, err := b.SetMyCommands(ctx, &bot.SetMyCommandsParams{
			Commands: []models.BotCommand{
				{
					Command:     string(constants.StartCommand),
					Description: l.Localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CommandStartDescription"}),
				},
				{
					Command:     string(constants.FAQCommand),
					Description: l.Localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CommandFAQDescription"}),
				},
				{
					Command:     string(constants.ReportBugCommand),
					Description: l.Localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CommandReportBugDescription"}),
				},
			},
			LanguageCode: l.Tag.String(),
		})
		if err != nil {
			return fmt.Errorf("failed to set bot commands for locale: %s, error: %w", l.Tag.String(), err)
		} else if !ok {
			return fmt.Errorf("unsuccessful set bot commands for locale: %s", l.Tag.String())
		}
	}

	return nil
}

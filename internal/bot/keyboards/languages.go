package keyboards

import (
	"github.com/go-telegram/bot/models"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const languagesKeyboardCols = 2

func FindLocaleByName(localizers []languages.LocalizersItem, text string) *languages.LocalizersItem {
	for _, l := range localizers {
		localeMsg, err := l.Localizer.Localize(&i18n.LocalizeConfig{
			MessageID: "Language",
		})
		if err == nil && localeMsg == text {
			return &l
		}
	}

	return nil
}

func CreateLanguagesReplyKeyboard(localizers []languages.LocalizersItem, localizer *i18n.Localizer) *models.ReplyKeyboardMarkup {
	rows := [][]models.KeyboardButton{}
	row := []models.KeyboardButton{}

	for _, l := range localizers {
		row = append(row, models.KeyboardButton{
			Text: l.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "Language",
			}),
		})

		if len(row) == languagesKeyboardCols {
			rows = append(rows, row)
			row = []models.KeyboardButton{}
		}
	}

	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, []models.KeyboardButton{
		{Text: localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "BackButton",
		})},
	})

	return &models.ReplyKeyboardMarkup{
		Keyboard:  rows,
		ResizeKeyboard: true,
	}
}

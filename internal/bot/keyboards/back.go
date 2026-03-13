package keyboards

import (
	"github.com/go-telegram/bot/models"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func CreateBackReplyKeyboard(localizer *i18n.Localizer) *models.ReplyKeyboardMarkup {
	return &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "BackButton",
				})},
			},
		},
		ResizeKeyboard: true,
	}
}

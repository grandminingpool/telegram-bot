package keyboards

import (
	"github.com/go-telegram/bot/models"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func CreateSettingsReplyKeyboard(payoutsNotify, blocksNotify bool, localizer *i18n.Localizer) *models.ReplyKeyboardMarkup {
	var payoutsNotifyMsgID, blocksNotifyMsgID string

	if payoutsNotify {
		payoutsNotifyMsgID = "SettingsDisablePayoutsNotifyButton"
	} else {
		payoutsNotifyMsgID = "SettingsEnablePayoutsNotifyButton"
	}

	if blocksNotify {
		blocksNotifyMsgID = "SettingsDisableBlocksNotifyButton"
	} else {
		blocksNotifyMsgID = "SettingsEnableBlocksNotifyButton"
	}

	return &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: payoutsNotifyMsgID,
				})},
				{Text: localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: blocksNotifyMsgID,
				})},
			},
			{
				{Text: localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "SettingsLanguageButton",
				})},
			},
			{
				{Text: localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "BackButton",
				})},
			},
		},
		ResizeKeyboard: true,
	}
}

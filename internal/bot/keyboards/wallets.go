package keyboards

import (
	"slices"

	"github.com/go-telegram/bot/models"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func FindWalletByAddress(wallets []services.UserWalletInfo, text string) *services.UserWalletInfo {
	idx := slices.IndexFunc(wallets, func(wallet services.UserWalletInfo) bool {
		return wallet.Wallet == text
	})
	if idx != -1 {
		return &wallets[idx]
	}

	return nil
}

func CreateWalletsReplyKeyboard(wallets []services.UserWalletInfo, localizer *i18n.Localizer) *models.ReplyKeyboardMarkup {
	rows := [][]models.KeyboardButton{}

	for _, wallet := range wallets {
		rows = append(rows, []models.KeyboardButton{
			{Text: wallet.Wallet},
		})
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

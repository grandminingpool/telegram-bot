package keyboards

import (
	"github.com/go-telegram/bot/models"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const blockchainsKeyboardCols = 3

func FindBlockchainByName(blockchainsInfo []blockchains.BlockchainInfo, name string) *blockchains.BlockchainInfo {
	for _, b := range blockchainsInfo {
		if b.Name == name {
			return &b
		}
	}

	return nil
}

func CreateBlockchainsReplyKeyboard(blockchainsInfo []blockchains.BlockchainInfo, localizer *i18n.Localizer) *models.ReplyKeyboardMarkup {
	rows := [][]models.KeyboardButton{}
	row := []models.KeyboardButton{}

	for _, blockchain := range blockchainsInfo {
		row = append(row, models.KeyboardButton{Text: blockchain.Name})
		if len(row) == blockchainsKeyboardCols {
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

package botKeyboards

import (
	"context"
	"slices"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/ui/keyboard/reply"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	BLOCKCHAINS_KEYBOARD_PREFIX = "blockchains"
	BLOCKCHAINS_KEYBOARD_COLS   = 3
)

type BlockchainKeyboardHandlerFunc func(context.Context, *blockchains.BlockchainInfo, *bot.Bot, *models.Update)

type BlockchainsKeyboard struct {
	blockchains     []blockchains.BlockchainInfo
	onSelectHandler BlockchainKeyboardHandlerFunc
	onBackHandler   bot.HandlerFunc
}

func (k *BlockchainsKeyboard) OnBlockchainSelected(ctx context.Context, b *bot.Bot, update *models.Update) {
	ind := slices.IndexFunc(k.blockchains, func(blockchain blockchains.BlockchainInfo) bool {
		return blockchain.Name == update.Message.Text
	})
	if ind != -1 {
		blockchain := k.blockchains[ind]

		k.onSelectHandler(ctx, &blockchain, b, update)
	}
}

func CreateBlockchainsReplyKeyboard(b *bot.Bot, blockchainsKeyboard *BlockchainsKeyboard, localizer *i18n.Localizer) *reply.ReplyKeyboard {
	replyKeyboard := reply.New(b, reply.IsSelective(), reply.WithPrefix(BLOCKCHAINS_KEYBOARD_PREFIX)).Row()
	cols := 0
	for _, blockchain := range blockchainsKeyboard.blockchains {
		replyKeyboard = replyKeyboard.Button(blockchain.Name, b, bot.MatchTypeExact, blockchainsKeyboard.OnBlockchainSelected)
		if cols == BLOCKCHAINS_KEYBOARD_COLS {
			replyKeyboard = replyKeyboard.Row()
			cols = 0
		} else {
			cols++
		}
	}

	if cols < BLOCKCHAINS_KEYBOARD_COLS {
		replyKeyboard = replyKeyboard.Row()
	}

	return replyKeyboard.Button(localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "BackButton",
	}), b, bot.MatchTypeExact, blockchainsKeyboard.onBackHandler).Row()
}

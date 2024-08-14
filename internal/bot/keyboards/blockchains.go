package botKeyboards

import (
	"context"
	"slices"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/ui/keyboard/reply"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/common/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	BLOCKCHAINS_KEYBOARD_PREFIX                 = "blockchains"
	BLOCKCHAINS_KEYBOARD_COLS                   = 3
	ADD_WALLET_KEYBOARD_CTX_KEY    types.CtxKey = "addWalletKeyboard"
	REMOVE_WALLET_KEYBOARD_CTX_KEY types.CtxKey = "removeWalletKeyboard"
)

type BlockchainsKeyboardHandlerFunc func(context.Context, *middlewares.User, *BlockchainsKeyboard, *bot.Bot, *models.Update)
type OnBlockchainSelectedHandlerFunc func(context.Context, *middlewares.User, *blockchains.BlockchainInfo, *bot.Bot, *models.Update)
type OnBlockchainSelectedWithStartKeyboardHandlerFunc func(context.Context, *middlewares.User, *StartKeyboard, *blockchains.BlockchainInfo, *bot.Bot, *models.Update)

type BlockchainsKeyboard struct {
	blockchains     []*blockchains.BlockchainInfo
	onSelectHandler OnBlockchainSelectedHandlerFunc
	onBackHandler   middlewares.UserHandlerFunc
}

func (k *BlockchainsKeyboard) OnBlockchainSelected(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	idx := slices.IndexFunc(k.blockchains, func(blockchain *blockchains.BlockchainInfo) bool {
		return blockchain.Name == update.Message.Text
	})
	if idx != -1 {
		blockchain := k.blockchains[idx]

		k.onSelectHandler(ctx, user, blockchain, b, update)
	}
}

func CreateBlockchainsKeyboard(
	blockchains []*blockchains.BlockchainInfo,
	onSelectHandler OnBlockchainSelectedHandlerFunc,
	onBackHandler middlewares.UserHandlerFunc) *BlockchainsKeyboard {
	return &BlockchainsKeyboard{
		blockchains:     blockchains,
		onSelectHandler: onSelectHandler,
		onBackHandler:   onBackHandler,
	}
}

func CreateBlockchainsReplyKeyboard(b *bot.Bot, blockchainsKeyboard *BlockchainsKeyboard, localizer *i18n.Localizer) *reply.ReplyKeyboard {
	replyKeyboard := reply.New(b, reply.IsSelective(), reply.WithPrefix(BLOCKCHAINS_KEYBOARD_PREFIX)).Row()
	cols := 0
	for _, blockchain := range blockchainsKeyboard.blockchains {
		replyKeyboard = replyKeyboard.Button(blockchain.Name, b, bot.MatchTypeExact, middlewares.WithUserHandler(blockchainsKeyboard.OnBlockchainSelected))
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
	}), b, bot.MatchTypeExact, middlewares.WithUserHandler(blockchainsKeyboard.onBackHandler)).Row()
}

func WithBlockchainsKeyboardHandler(handler BlockchainsKeyboardHandlerFunc, ctxKey types.CtxKey) middlewares.UserHandlerFunc {
	return func(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
		blockchainsKeyboard, ok := ctx.Value(ctxKey).(*BlockchainsKeyboard)
		if ok {
			handler(ctx, user, blockchainsKeyboard, b, update)
		}
	}
}

func OnBlockchainSelectedWithStartKeyboardHandler(handler OnBlockchainSelectedWithStartKeyboardHandlerFunc) OnBlockchainSelectedHandlerFunc {
	return func(ctx context.Context, user *middlewares.User, blockchain *blockchains.BlockchainInfo, b *bot.Bot, update *models.Update) {
		startKeyboard, ok := ctx.Value(START_KEYBOARD_CTX_KEY).(*StartKeyboard)
		if ok {
			handler(ctx, user, startKeyboard, blockchain, b, update)
		}
	}
}

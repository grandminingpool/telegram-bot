package botKeyboards

import (
	"bytes"
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/ui/keyboard/reply"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/grandminingpool/telegram-bot/internal/common/types"
	formatUtils "github.com/grandminingpool/telegram-bot/internal/utils/format"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

const (
	START_KEYBOARD_PREFIX               = "start"
	START_KEYBOARD_CTX_KEY types.CtxKey = "startKeyboard"
)

type StartKeyboardHandlerFunc func(context.Context, *middlewares.User, *StartKeyboard, *bot.Bot, *models.Update)

type StartKeyboard struct {
	userService                 *services.UserService
	userWalletService           *services.UserWalletService
	addWalletKeyboard           *BlockchainsKeyboard
	poolStatsKeyboard           *BlockchainsKeyboard
	languagesKeyboard           *LanguagesKeyboard
	onRemoveWalletSelectHandler OnBlockchainSelectedHandlerFunc
	onRemoveWalletBackHandler   middlewares.UserHandlerFunc
}

func (k *StartKeyboard) AddWallet(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "SelectBlockchain",
		}),
		ReplyMarkup: CreateBlockchainsReplyKeyboard(b, k.addWalletKeyboard, user.Localizer),
	})
}

func (k *StartKeyboard) RemoveWallet(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	userBlockchains, err := k.userWalletService.FindBlockchains(ctx, user.ID)
	if err != nil {
		zap.L().Error("find user blockchains error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	if len(userBlockchains) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "UserHasNoWallets",
			}),
		})
	} else {
		userRemoveWalletKeyboard := &BlockchainsKeyboard{
			blockchains:     userBlockchains,
			onSelectHandler: k.onRemoveWalletSelectHandler,
			onBackHandler:   k.onRemoveWalletBackHandler,
		}

		newCtx := context.WithValue(ctx, REMOVE_WALLET_KEYBOARD_CTX_KEY, userRemoveWalletKeyboard)

		b.SendMessage(newCtx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "SelectBlockchain",
			}),
			ReplyMarkup: CreateBlockchainsReplyKeyboard(b, userRemoveWalletKeyboard, user.Localizer),
		})
	}
}

func (k *StartKeyboard) ShowWallets(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	wallets, err := k.userWalletService.FindWallets(ctx, user.ID)
	if err != nil {
		zap.L().Error("find user wallets error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	if len(wallets) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "UserHasNoWallets",
			}),
		})
	} else {
		var msgBuf bytes.Buffer
		for _, wallet := range wallets {
			balanceText := formatUtils.WalletBalance(wallet.Balance, wallet.Pool.Blockchain.AtomicUnit)
			msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "WalletBalance",
				TemplateData: map[string]string{
					"Wallet":             wallet.Wallet,
					"PoolBlockchainName": wallet.Pool.Blockchain.Name,
					"Balance":            balanceText,
					"Ticker":             wallet.Pool.Blockchain.Ticker,
				},
			}))

			if wallet.Pool.MinPayout != nil {
				minPayoutText := formatUtils.WalletBalance(*wallet.Pool.MinPayout, wallet.Pool.Blockchain.AtomicUnit)
				msgBuf.WriteString("\n\n")
				msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "WalletLeftForPayment",
					TemplateData: map[string]string{
						"Balance":   balanceText,
						"MinPayout": minPayoutText,
					},
				}))
			}

			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   msgBuf.String(),
			})

			msgBuf.Reset()
		}
	}
}

func (k *StartKeyboard) ShowWorkers(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	workers, err := k.userWalletService.FindWorkers(ctx, user.ID)
	if err != nil {
		zap.L().Error("find user workers error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	if len(workers) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "UserHasNoActiveWorkers",
			}),
		})
	} else {
		for _, worker := range workers {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "WorkerInfo",
					TemplateData: map[string]string{
						"Wallet":             worker.Wallet,
						"PoolBlockchainName": worker.Pool.Blockchain.Name,
						"Region":             worker.Region,
						"Worker":             worker.Worker,
						"Solo":               formatUtils.BoolText(worker.Solo, user.Localizer),
						"Hashrate":           formatUtils.Hashrate(worker.Hashrate),
						"Uptime":             formatUtils.UptimeText(worker.ConnectedAt, user.Localizer),
					},
				}),
			})
		}
	}
}

func (k *StartKeyboard) ShowPoolStatistics(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "SelectBlockchain",
		}),
		ReplyMarkup: CreateBlockchainsReplyKeyboard(b, k.poolStatsKeyboard, user.Localizer),
	})
}

func (k *StartKeyboard) ShowSettings(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	userSettingsKeyboard := &SettingsKeyboard{
		userService:       k.userService,
		startKeyboard:     k,
		languagesKeyboard: k.languagesKeyboard,
		payoutNotify:      user.Settings.PayoutNotify,
		blockNotify:       user.Settings.BlockNotify,
	}

	newCtx := context.WithValue(ctx, SETTINGS_KEYBOARD_CTX_KEY, userSettingsKeyboard)

	b.SendMessage(newCtx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ChooseSetting",
		}),
		ReplyMarkup: CreateSettingsReplyKeyboard(b, userSettingsKeyboard, user.Localizer),
	})
}

func CreateStartKeyboard(
	userService *services.UserService,
	userWalletService *services.UserWalletService,
	addWalletKeyboard *BlockchainsKeyboard,
	poolStatsKeyboard *BlockchainsKeyboard,
	languagesKeyboard *LanguagesKeyboard,
	onRemoveWalletSelectHandler OnBlockchainSelectedHandlerFunc,
	onRemoveWalletBackHandler middlewares.UserHandlerFunc,
) *StartKeyboard {
	return &StartKeyboard{
		userService:                 userService,
		userWalletService:           userWalletService,
		addWalletKeyboard:           addWalletKeyboard,
		poolStatsKeyboard:           poolStatsKeyboard,
		languagesKeyboard:           languagesKeyboard,
		onRemoveWalletSelectHandler: onRemoveWalletSelectHandler,
		onRemoveWalletBackHandler:   onRemoveWalletBackHandler,
	}
}

func CreateStartReplyKeyboard(b *bot.Bot, startKeyboard *StartKeyboard, localizer *i18n.Localizer) *reply.ReplyKeyboard {
	return reply.New(b, reply.IsSelective(), reply.WithPrefix(START_KEYBOARD_PREFIX)).
		Row().
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "AddWalletButton",
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(startKeyboard.AddWallet)).
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "RemoveWalletButton",
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(startKeyboard.RemoveWallet)).
		Row().
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "WalletsButton",
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(startKeyboard.ShowWallets)).
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "WorkersButton",
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(startKeyboard.ShowWorkers)).
		Row().
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "PoolStatsButton",
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(startKeyboard.ShowPoolStatistics)).
		Button(localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "SettingsButton",
		}), b, bot.MatchTypeExact, middlewares.WithUserHandler(startKeyboard.ShowSettings)).Row()
}

func WithStartKeyboardHandler(handler StartKeyboardHandlerFunc) middlewares.UserHandlerFunc {
	return func(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
		startKeyboard, ok := ctx.Value(START_KEYBOARD_CTX_KEY).(*StartKeyboard)
		if ok {
			handler(ctx, user, startKeyboard, b, update)
		}
	}
}

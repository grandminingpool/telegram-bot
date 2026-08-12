package keyboards

import (
	"bytes"
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/go-telegram/ui/keyboard/reply"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/grandminingpool/telegram-bot/internal/common/types"
	format_utils "github.com/grandminingpool/telegram-bot/internal/utils/format"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

const (
	startKeyboardPrefix               = "start"
	StartKeyboardCtxKey types.CtxKey = "startKeyboard"
)

type StartKeyboardHandlerFunc func(context.Context, *middlewares.User, *StartKeyboard, *bot.Bot, *models.Update)

type StartKeyboard struct {
	userService       *services.UserService
	userWalletService *services.UserWalletService
	userActionService *services.UserActionService
	blockchainsInfo   []blockchains.BlockchainInfo
}

func (k *StartKeyboard) AddWallet(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	if err := k.userActionService.Set(ctx, user.ID, services.UserAddWalletSelectBlockchainAction, nil); err != nil {
		zap.L().Error("set user select blockchain add wallet action error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "SelectBlockchain",
		}),
		ReplyMarkup: CreateBlockchainsReplyKeyboard(k.blockchainsInfo, user.Localizer),
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
		if err := k.userActionService.Set(ctx, user.ID, services.UserRemoveWalletAction, nil); err != nil {
			zap.L().Error("set user select blockchain remove wallet action error",
				zap.Int64("user_id", user.ID),
				zap.Error(err),
			)

			return
		}

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "SelectBlockchain",
			}),
			ReplyMarkup: CreateBlockchainsReplyKeyboard(userBlockchains, user.Localizer),
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
			msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "WalletInfo",
				TemplateData: map[string]string{
					"Wallet":             wallet.Wallet,
					"PoolBlockchainName": wallet.Pool.Blockchain.Name,
				},
			}))
			msgBuf.WriteString("\n\n")
			balanceText := format_utils.WalletBalance(wallet.Balance, wallet.Pool.Blockchain.AtomicUnit)
			msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "WalletBalance",
				TemplateData: map[string]string{
					"Balance": balanceText,
					"Ticker":  wallet.Pool.Blockchain.Ticker,
				},
			}))

			if wallet.Pool.MinPayout != nil {
				minPayoutText := format_utils.WalletBalance(*wallet.Pool.MinPayout, wallet.Pool.Blockchain.AtomicUnit)
				msgBuf.WriteString("\n\n")
				msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "WalletLeftForPayment",
					TemplateData: map[string]string{
						"Balance":   balanceText,
						"MinPayout": minPayoutText,
						"Ticker":    wallet.Pool.Blockchain.Ticker,
					},
				}))
			}

			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    update.Message.Chat.ID,
				ParseMode: models.ParseModeHTML,
				Text:      msgBuf.String(),
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
		var msgBuf bytes.Buffer
		for _, worker := range workers {
			msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "WalletInfo",
				TemplateData: map[string]string{
					"Wallet":             worker.Wallet,
					"PoolBlockchainName": worker.Pool.Blockchain.Name,
				},
			}))
			msgBuf.WriteString("\n\n")
			msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "WorkerInfo",
				TemplateData: map[string]string{
					"Worker":   worker.Worker,
					"Solo":     format_utils.BoolText(worker.Solo, user.Localizer),
					"Hashrate": format_utils.Hashrate(worker.Hashrate, worker.Pool.Blockchain.Coin),
					"Uptime":   format_utils.UptimeText(worker.ConnectedAt, user.Localizer),
				},
			}))

			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    update.Message.Chat.ID,
				ParseMode: models.ParseModeHTML,
				Text:      msgBuf.String(),
			})

			msgBuf.Reset()
		}
	}
}

func (k *StartKeyboard) ShowPoolStatistics(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	if err := k.userActionService.Set(ctx, user.ID, services.ShowPoolStatsAction, nil); err != nil {
		zap.L().Error("set user select blockchain pool stats action error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "SelectBlockchain",
		}),
		ReplyMarkup: CreateBlockchainsReplyKeyboard(k.blockchainsInfo, user.Localizer),
	})
}

func (k *StartKeyboard) ShowSettings(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
	if err := k.userActionService.Set(ctx, user.ID, services.UserSettingsAction, nil); err != nil {
		zap.L().Error("set user settings action error",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ChooseSetting",
		}),
		ReplyMarkup: CreateSettingsReplyKeyboard(user.Settings.PayoutsNotify, user.Settings.BlocksNotify, user.Localizer),
	})
}

func CreateStartKeyboard(
	userService *services.UserService,
	userWalletService *services.UserWalletService,
	userActionService *services.UserActionService,
	blockchainsInfo []blockchains.BlockchainInfo,
) *StartKeyboard {
	return &StartKeyboard{
		userService:       userService,
		userWalletService: userWalletService,
		userActionService: userActionService,
		blockchainsInfo:   blockchainsInfo,
	}
}

func CreateStartReplyKeyboard(b *bot.Bot, startKeyboard *StartKeyboard, localizer *i18n.Localizer) *reply.ReplyKeyboard {
	return reply.New(reply.ResizableKeyboard(), reply.WithPrefix(startKeyboardPrefix)).
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
		startKeyboard, ok := ctx.Value(StartKeyboardCtxKey).(*StartKeyboard)
		if ok {
			handler(ctx, user, startKeyboard, b, update)
		}
	}
}

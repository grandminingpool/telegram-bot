package pool_bot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	bot_config "github.com/grandminingpool/telegram-bot/configs/bot"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/grandminingpool/telegram-bot/internal/bot/handlers"
	bot_keyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/grandminingpool/telegram-bot/internal/common/constants"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

type HandlerMatcher struct {
	userActionService *services.UserActionService
	serviceCtx        context.Context
}

func (m *HandlerMatcher) MatchUserAction(action services.UserAction) bot.MatchFunc {
	return func(update *models.Update) bool {
		if update.Message != nil {
			userAction, err := m.userActionService.Get(m.serviceCtx, update.Message.From.ID)
			if err != nil {
				zap.L().Error("get user action error while match handler",
					zap.Int64("user_id", update.Message.From.ID),
					zap.String("action", string(action)),
					zap.Error(err),
				)
			}

			return userAction != nil && userAction.Action == action
		}

		return false
	}
}

func NewHandlerMatcher(
	ctx context.Context,
	userActionService *services.UserActionService,
) *HandlerMatcher {
	return &HandlerMatcher{
		userActionService: userActionService,
		serviceCtx:        ctx,
	}
}

func createSelectBlockchainHandler(
	blockchainsInfo []blockchains.BlockchainInfo,
	userActionService *services.UserActionService,
	onSelected func(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, blockchain blockchains.BlockchainInfo, b *bot.Bot, update *models.Update),
	onBack func(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, b *bot.Bot, update *models.Update),
) middlewares.UserHandlerFunc {
	return func(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
		startKeyboard, ok := ctx.Value(bot_keyboards.StartKeyboardCtxKey).(*bot_keyboards.StartKeyboard)
		if !ok {
			return
		}

		backText := user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "BackButton",
		})
		if update.Message.Text == backText {
			if err := userActionService.Clear(ctx, user.ID); err != nil {
				zap.L().Error("clear user action error",
					zap.Int64("user_id", user.ID),
					zap.Error(err),
				)
			}

			onBack(ctx, user, startKeyboard, b, update)

			return
		}

		blockchain := bot_keyboards.FindBlockchainByName(blockchainsInfo, update.Message.Text)
		if blockchain != nil {
			onSelected(ctx, user, startKeyboard, *blockchain, b, update)
		}
	}
}

func RegisterHandlers(
	b *bot.Bot,
	hm *HandlerMatcher,
	defaultHandler *handlers.DefaultHandler,
	userService *services.UserService,
	userActionService *services.UserActionService,
	userWalletService *services.UserWalletService,
	blockchainsService *blockchains.Service,
	enterWalletHandler *handlers.EnterWalletHandler,
	poolStatsHandler *handlers.PoolStatsHandler,
	removeWalletHandler *handlers.RemoveWalletHandler,
	localizers []languages.LocalizersItem,
	config *bot_config.Config,
	botUsername string,
) {
	blockchainsInfo := blockchainsService.GetBlockchainsInfo()

	//	init handlers
	faqHandler := handlers.NewFAQHandler(config.PoolURL, config.Notify.CheckIntervals.Workers, config.PoolChatLink)
	reportBugHandler := handlers.NewReportBugHandler(userActionService, config.SupportChatID, config.PoolChatLink)
	addWalletHandler := handlers.NewAddWalletHandler(
		userActionService,
		userWalletService,
		blockchainsService,
		config.Notify.CheckIntervals.Workers,
		config.WalletsLimitPerUser,
	)

	//	deep link handler
	deepLinkHandler := handlers.NewDeepLinkHandler(
		defaultHandler,
		addWalletHandler,
		blockchainsService,
		botUsername,
	)

	//	command handlers
	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		string(constants.StartCommand),
		bot.MatchTypePrefix,
		middlewares.WithUserHandler(bot_keyboards.WithStartKeyboardHandler(deepLinkHandler.Handler)),
	)
	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		string(constants.FAQCommand),
		bot.MatchTypeExact,
		middlewares.WithUserHandler(faqHandler.Handler),
	)
	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		string(constants.ReportBugCommand),
		bot.MatchTypeExact,
		middlewares.WithUserHandler(reportBugHandler.Enter),
	)

	//	match handlers
	b.RegisterHandlerMatchFunc(
		hm.MatchUserAction(services.UserAddWalletAction),
		middlewares.WithUserHandler(func(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
			backText := user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "BackButton",
			})
			if update.Message.Text == backText {
				enterWalletHandler.BackToBlockchainSelect(ctx, user, b, update)

				return
			}

			startKeyboard, ok := ctx.Value(bot_keyboards.StartKeyboardCtxKey).(*bot_keyboards.StartKeyboard)
			if ok {
				addWalletHandler.Handler(ctx, user, startKeyboard, b, update)
			}
		}),
	)
	b.RegisterHandlerMatchFunc(
		hm.MatchUserAction(services.ReportBugAction),
		middlewares.WithUserHandler(func(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
			backText := user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "BackButton",
			})
			if update.Message.Text == backText {
				startKeyboard, ok := ctx.Value(bot_keyboards.StartKeyboardCtxKey).(*bot_keyboards.StartKeyboard)
				if ok {
					reportBugHandler.Back(ctx, user, startKeyboard, b, update)
				}

				return
			}

			startKeyboard, ok := ctx.Value(bot_keyboards.StartKeyboardCtxKey).(*bot_keyboards.StartKeyboard)
			if ok {
				reportBugHandler.SendFeedback(ctx, user, startKeyboard, b, update)
			}
		}),
	)

	//	settings match handlers
	b.RegisterHandlerMatchFunc(
		hm.MatchUserAction(services.UserSettingsAction),
		middlewares.WithUserHandler(func(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
			startKeyboard, ok := ctx.Value(bot_keyboards.StartKeyboardCtxKey).(*bot_keyboards.StartKeyboard)
			if !ok {
				return
			}

			backText := user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "BackButton",
			})
			if update.Message.Text == backText {
				if err := userActionService.Clear(ctx, user.ID); err != nil {
					zap.L().Error("clear user action error",
						zap.Int64("user_id", user.ID),
						zap.Error(err),
					)
				}

				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "ReturningToMenu",
					}),
					ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
				})

				return
			}

			languageText := user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "SettingsLanguageButton",
			})
			if update.Message.Text == languageText {
				if err := userActionService.Set(ctx, user.ID, services.UserSettingsSelectLanguageAction, nil); err != nil {
					zap.L().Error("set user settings language action error",
						zap.Int64("user_id", user.ID),
						zap.Error(err),
					)

					return
				}

				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "ChooseLanguage",
					}),
					ReplyMarkup: bot_keyboards.CreateLanguagesReplyKeyboard(localizers, user.Localizer),
				})

				return
			}

			// Toggle payouts notifications
			enablePayoutsText := user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "SettingsEnablePayoutsNotifyButton",
			})
			disablePayoutsText := user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "SettingsDisablePayoutsNotifyButton",
			})
			if update.Message.Text == enablePayoutsText || update.Message.Text == disablePayoutsText {
				newPayoutsNotify := !user.Settings.PayoutsNotify

				if err := userService.SetPayoutsNotify(ctx, user.ID, newPayoutsNotify); err != nil {
					zap.L().Error("update user payout notify error",
						zap.Int64("user_id", user.ID),
						zap.Bool("payouts_notify", newPayoutsNotify),
						zap.Error(err),
					)

					return
				}

				var msgID string
				if newPayoutsNotify {
					msgID = "PayoutsNotificationsEnabled"
				} else {
					msgID = "PayoutsNotificationsDisabled"
				}

				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: msgID,
					}),
					ReplyMarkup: bot_keyboards.CreateSettingsReplyKeyboard(newPayoutsNotify, user.Settings.BlocksNotify, user.Localizer),
				})

				return
			}

			// Toggle blocks notifications
			enableBlocksText := user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "SettingsEnableBlocksNotifyButton",
			})
			disableBlocksText := user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "SettingsDisableBlocksNotifyButton",
			})
			if update.Message.Text == enableBlocksText || update.Message.Text == disableBlocksText {
				newBlocksNotify := !user.Settings.BlocksNotify

				if err := userService.SetBlocksNotify(ctx, user.ID, newBlocksNotify); err != nil {
					zap.L().Error("update user blocks notify error",
						zap.Int64("user_id", user.ID),
						zap.Bool("blocks_notify", newBlocksNotify),
						zap.Error(err),
					)

					return
				}

				var msgID string
				if newBlocksNotify {
					msgID = "BlocksNotificationsEnabled"
				} else {
					msgID = "BlocksNotificationsDisabled"
				}

				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: msgID,
					}),
					ReplyMarkup: bot_keyboards.CreateSettingsReplyKeyboard(user.Settings.PayoutsNotify, newBlocksNotify, user.Localizer),
				})

				return
			}
		}),
	)

	//	language selection match handler
	b.RegisterHandlerMatchFunc(
		hm.MatchUserAction(services.UserSettingsSelectLanguageAction),
		middlewares.WithUserHandler(func(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
			backText := user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "BackButton",
			})
			if update.Message.Text == backText {
				if err := userActionService.Set(ctx, user.ID, services.UserSettingsAction, nil); err != nil {
					zap.L().Error("set user settings action error",
						zap.Int64("user_id", user.ID),
						zap.Error(err),
					)

					return
				}

				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "ReturningToSettingsMenu",
					}),
					ReplyMarkup: bot_keyboards.CreateSettingsReplyKeyboard(user.Settings.PayoutsNotify, user.Settings.BlocksNotify, user.Localizer),
				})

				return
			}

			locale := bot_keyboards.FindLocaleByName(localizers, update.Message.Text)
			if locale != nil {
				if err := userService.SetLang(ctx, user.ID, locale.Tag); err != nil {
					zap.L().Error("failed to set user language",
						zap.Int64("user_id", user.ID),
						zap.Error(err),
					)

					return
				}

				if err := userActionService.Set(ctx, user.ID, services.UserSettingsAction, nil); err != nil {
					zap.L().Error("set user settings action error",
						zap.Int64("user_id", user.ID),
						zap.Error(err),
					)

					return
				}

				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: locale.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "LanguageChanged",
					}),
					ReplyMarkup: bot_keyboards.CreateSettingsReplyKeyboard(user.Settings.PayoutsNotify, user.Settings.BlocksNotify, locale.Localizer),
				})
			}
		}),
	)

	//	remove wallet - select wallet match handler
	b.RegisterHandlerMatchFunc(
		hm.MatchUserAction(services.UserRemoveWalletSelectWalletAction),
		middlewares.WithUserHandler(func(ctx context.Context, user *middlewares.User, b *bot.Bot, update *models.Update) {
			backText := user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "BackButton",
			})
			if update.Message.Text == backText {
				removeWalletHandler.BackToBlockchainSelect(ctx, user, b, update)

				return
			}

			if user.Action == nil || user.Action.Payload == nil {
				return
			}

			coin := *user.Action.Payload
			wallets, err := userWalletService.FindBlockchainWallets(ctx, user.ID, coin)
			if err != nil {
				zap.L().Error("find user blockchain wallets error",
					zap.Int64("user_id", user.ID),
					zap.String("coin", coin),
					zap.Error(err),
				)

				return
			}

			wallet := bot_keyboards.FindWalletByAddress(wallets, update.Message.Text)
			if wallet != nil {
				startKeyboard, ok := ctx.Value(bot_keyboards.StartKeyboardCtxKey).(*bot_keyboards.StartKeyboard)
				if ok {
					removeWalletHandler.Remove(ctx, user, startKeyboard, *wallet, b, update)
				}
			}
		}),
	)

	//	blockchain selection match handlers
	b.RegisterHandlerMatchFunc(
		hm.MatchUserAction(services.UserAddWalletSelectBlockchainAction),
		middlewares.WithUserHandler(createSelectBlockchainHandler(
			blockchainsInfo,
			userActionService,
			func(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, blockchain blockchains.BlockchainInfo, b *bot.Bot, update *models.Update) {
				enterWalletHandler.Handler(ctx, user, blockchain, b, update)
			},
			func(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
				enterWalletHandler.Back(ctx, user, startKeyboard, b, update)
			},
		)),
	)
	b.RegisterHandlerMatchFunc(
		hm.MatchUserAction(services.ShowPoolStatsAction),
		middlewares.WithUserHandler(createSelectBlockchainHandler(
			blockchainsInfo,
			userActionService,
			func(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, blockchain blockchains.BlockchainInfo, b *bot.Bot, update *models.Update) {
				if err := userActionService.Clear(ctx, user.ID); err != nil {
					zap.L().Error("clear user action error",
						zap.Int64("user_id", user.ID),
						zap.Error(err),
					)
				}

				poolStatsHandler.OnBlockchainSelected(ctx, user, startKeyboard, blockchain, b, update)
			},
			func(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
				poolStatsHandler.Back(ctx, user, startKeyboard, b, update)
			},
		)),
	)
	b.RegisterHandlerMatchFunc(
		hm.MatchUserAction(services.UserRemoveWalletAction),
		middlewares.WithUserHandler(createSelectBlockchainHandler(
			blockchainsInfo,
			userActionService,
			func(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, blockchain blockchains.BlockchainInfo, b *bot.Bot, update *models.Update) {
				removeWalletHandler.OnBlockchainSelected(ctx, user, blockchain, b, update)
			},
			func(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
				removeWalletHandler.Back(ctx, user, startKeyboard, b, update)
			},
		)),
	)
}

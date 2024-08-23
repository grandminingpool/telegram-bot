package poolBot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	botConfig "github.com/grandminingpool/telegram-bot/configs/bot"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/grandminingpool/telegram-bot/internal/bot/handlers"
	botKeyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/grandminingpool/telegram-bot/internal/common/constants"
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

func RegisterHandlers(
	b *bot.Bot,
	hm *HandlerMatcher,
	defaultHandler *handlers.DefaultHandler,
	userActionService *services.UserActionService,
	userWalletService *services.UserWalletService,
	feedbackService *services.FeedbackService,
	blockchainsService *blockchains.Service,
	config *botConfig.Config,
) {
	//	init handlers
	faqHandler := handlers.NewFAQHandler(config.PoolURL, config.Notify.CheckIntervals.Workers, config.SupportBot.Username)
	reportBugHandler := handlers.NewReportBugHandler(feedbackService, userActionService, config.SupportBot.Username)
	addWalletHandler := handlers.NewAddWalletHandler(
		userActionService,
		userWalletService,
		blockchainsService,
		config.Notify.CheckIntervals.Workers,
	)

	//	command handlers
	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		string(constants.StartCommand),
		bot.MatchTypeExact,
		middlewares.WithUserHandler(botKeyboards.WithStartKeyboardHandler(defaultHandler.Handler)),
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
		middlewares.WithUserHandler(botKeyboards.WithStartKeyboardHandler(addWalletHandler.Handler)),
	)
	b.RegisterHandlerMatchFunc(
		hm.MatchUserAction(services.ReportBugAction),
		middlewares.WithUserHandler(botKeyboards.WithStartKeyboardHandler(reportBugHandler.SendFeedback)),
	)
}

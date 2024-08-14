package poolBot

import (
	"fmt"

	"github.com/go-telegram/bot"
	botConfig "github.com/grandminingpool/telegram-bot/configs/bot"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/grandminingpool/telegram-bot/internal/bot/handlers"
	botKeyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	keyboardsMiddlewares "github.com/grandminingpool/telegram-bot/internal/bot/keyboards/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/grandminingpool/telegram-bot/internal/common/flags"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
)

func CreateBotOptions(
	appMode flags.AppMode,
	blockchainsService *blockchains.Service,
	userService *services.UserService,
	userActionService *services.UserActionService,
	userWalletService *services.UserWalletService,
	languages *languages.Languages,
	defaultHandler *handlers.DefaultHandler,
	config *botConfig.Config,
) []bot.Option {
	//	init main handlers
	enterWalletHandler := handlers.NewEnterWalletHandler(userActionService)
	removeWalletHandler := handlers.NewRemoveWalletHandler(userWalletService, userActionService)
	poolStatsHandler := handlers.NewPoolStatsHandler(blockchainsService)

	//	init main keyboards
	addWalletKeyboard := botKeyboards.CreateBlockchainsKeyboard(
		blockchainsService.GetBlockchainsInfo(),
		enterWalletHandler.Handler,
		botKeyboards.WithStartKeyboardHandler(enterWalletHandler.Back),
	)
	poolStatsKeyboard := botKeyboards.CreateBlockchainsKeyboard(
		blockchainsService.GetBlockchainsInfo(),
		botKeyboards.OnBlockchainSelectedWithStartKeyboardHandler(poolStatsHandler.OnBlockchainSelected),
		botKeyboards.WithStartKeyboardHandler(poolStatsHandler.Back),
	)
	languagesKeyboard := botKeyboards.CreateLanguagesKeyboard(userService, languages.GetLocalizers())
	startKeyboard := botKeyboards.CreateStartKeyboard(
		userService,
		userWalletService,
		addWalletKeyboard,
		poolStatsKeyboard,
		languagesKeyboard,
		removeWalletHandler.OnBlockchainSelected,
		botKeyboards.WithStartKeyboardHandler(removeWalletHandler.Back),
	)
	userMiddleware := middlewares.CreateUserMiddleware(userService, userActionService, languages)
	keyboardsMiddleware := keyboardsMiddlewares.CreateKeyboardsMiddleware(addWalletKeyboard, startKeyboard)

	options := []bot.Option{
		bot.WithDefaultHandler(middlewares.WithUserHandler(botKeyboards.WithStartKeyboardHandler(defaultHandler.Handler))),
		bot.WithMiddlewares(userMiddleware.Middleware),
		bot.WithMiddlewares(keyboardsMiddleware.Middleware),
		bot.WithErrorsHandler(handlers.ErrorsHandler),
	}

	if appMode == flags.AppModeDev {
		options = append(options, bot.WithDebug(), bot.WithDebugHandler(handlers.DebugHandler))
	}

	return options
}

func CreateBot(
	options []bot.Option,
	token string,
) (*bot.Bot, error) {
	b, err := bot.New(token, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot instance: %w", err)
	}

	return b, nil
}

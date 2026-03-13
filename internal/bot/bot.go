package pool_bot

import (
	"fmt"

	"github.com/go-telegram/bot"
	bot_config "github.com/grandminingpool/telegram-bot/configs/bot"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	"github.com/grandminingpool/telegram-bot/internal/bot/handlers"
	bot_keyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	keyboards_middlewares "github.com/grandminingpool/telegram-bot/internal/bot/keyboards/middlewares"
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
	config *bot_config.Config,
) ([]bot.Option, *handlers.EnterWalletHandler, *handlers.PoolStatsHandler, *handlers.RemoveWalletHandler) {
	blockchainsInfo := blockchainsService.GetBlockchainsInfo()

	//	init main handlers
	enterWalletHandler := handlers.NewEnterWalletHandler(userActionService, blockchainsInfo)
	removeWalletHandler := handlers.NewRemoveWalletHandler(userWalletService, userActionService)
	poolStatsHandler := handlers.NewPoolStatsHandler(blockchainsService)

	//	init main keyboards
	startKeyboard := bot_keyboards.CreateStartKeyboard(
		userService,
		userWalletService,
		userActionService,
		blockchainsInfo,
	)
	userMiddleware := middlewares.CreateUserMiddleware(userService, userActionService, languages)
	keyboardsMiddleware := keyboards_middlewares.CreateKeyboardsMiddleware(startKeyboard)

	options := []bot.Option{
		bot.WithDefaultHandler(middlewares.WithUserHandler(bot_keyboards.WithStartKeyboardHandler(defaultHandler.Handler))),
		bot.WithMiddlewares(userMiddleware.Middleware),
		bot.WithMiddlewares(keyboardsMiddleware.Middleware),
		bot.WithErrorsHandler(handlers.ErrorsHandler),
	}

	if appMode == flags.AppModeDev {
		options = append(options, bot.WithDebug(), bot.WithDebugHandler(handlers.DebugHandler))
	}

	return options, enterWalletHandler, poolStatsHandler, removeWalletHandler
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

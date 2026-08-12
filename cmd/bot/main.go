package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"syscall"

	"github.com/go-playground/validator/v10"
	bot_config "github.com/grandminingpool/telegram-bot/configs/bot"
	postgres_config "github.com/grandminingpool/telegram-bot/configs/postgres"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	pool_bot "github.com/grandminingpool/telegram-bot/internal/bot"
	"github.com/grandminingpool/telegram-bot/internal/bot/handlers"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/grandminingpool/telegram-bot/internal/common/flags"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	"github.com/grandminingpool/telegram-bot/internal/common/logger"
	bot_notify "github.com/grandminingpool/telegram-bot/internal/notify"
	postgres_provider "github.com/grandminingpool/telegram-bot/internal/providers/postgres"
	"go.uber.org/zap"
)

func main() {
	//	Init context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	//	Parse flags
	parsedFlags := flags.DefineFlags()

	//	Setup flags
	flagsConf := flags.SetupFlags(parsedFlags)

	//	Setup logger
	zapLogger, err := logger.SetupLogger(&logger.LoggerConfig{
		AppMode:    flagsConf.Mode,
		OutputPath: flagsConf.Logger.OutputPath,
		Level:      flagsConf.Logger.Level,
	})
	if err != nil {
		log.Fatal(fmt.Errorf("failed to setup zap logger: %w", err))
	}
	defer zapLogger.Sync()

	zap.ReplaceGlobals(zapLogger)

	//	Init validator
	validate := validator.New()

	//	Load languages
	languages, err := languages.LoadLanguages(flagsConf.LocalesPath, flagsConf.Locales)
	if err != nil {
		zap.L().Fatal("failed to load languages", zap.Error(err))
	}

	//	Init postgres config
	postgresConf, err := postgres_config.New(flagsConf.ConfigsPath, validate)
	if err != nil {
		zap.L().Fatal("failed to load postgres config", zap.Error(err))
	}

	//	Init postgres connection
	pgConn, err := postgres_provider.NewConnection(ctx, postgresConf)
	if err != nil {
		zap.L().Fatal("failed to create postgres connection", zap.Error(err))
	}

	zap.L().Info("successfully connected to postgres database")

	//	Init bot config
	botConf, err := bot_config.New(flagsConf.ConfigsPath, validate)
	if err != nil {
		zap.L().Fatal("failed to load bot config", zap.Error(err))
	}

	//	Init blockchains service and start
	blockchainsService := blockchains.NewService(pgConn, botConf.PoolAPITimeout)
	if err := blockchainsService.Start(ctx, flagsConf.PoolAPICertsPath); err != nil {
		zap.L().Fatal("failed to start blockchains service", zap.Error(err))
	}

	//	Init bot services
	userService := services.NewUserService(pgConn)
	userActionService := services.NewUserActionService(pgConn)
	userWalletService := services.NewUserWalletService(pgConn, blockchainsService)

	//	Create bot
	defaultHandler := handlers.NewDefaultHandler(languages, userActionService)
	botOptions, enterWalletHandler, poolStatsHandler, removeWalletHandler := pool_bot.CreateBotOptions(
		flagsConf.Mode,
		blockchainsService,
		userService,
		userActionService,
		userWalletService,
		languages,
		defaultHandler,
		botConf,
	)
	b, err := pool_bot.CreateBot(botOptions, botConf.BotToken)
	if err != nil {
		zap.L().Fatal("failed to create bot", zap.Error(err))
	}

	//	Get bot info
	botUser, err := b.GetMe(ctx)
	if err != nil {
		zap.L().Fatal("failed to get bot info", zap.Error(err))
	}

	handlerMatcher := pool_bot.NewHandlerMatcher(ctx, userActionService)
	pool_bot.RegisterHandlers(
		b,
		handlerMatcher,
		defaultHandler,
		userService,
		userActionService,
		userWalletService,
		blockchainsService,
		enterWalletHandler,
		poolStatsHandler,
		removeWalletHandler,
		languages.GetLocalizers(),
		botConf,
		botUser.Username,
	)

	if err := pool_bot.SetBotDescription(ctx, b, languages.GetLocalizers()); err != nil {
		zap.L().Warn("failed to set bot description", zap.Error(err))
	}

	if err := pool_bot.SetBotCommands(ctx, b, languages.GetLocalizers()); err != nil {
		zap.L().Warn("failed to set bot commands", zap.Error(err))
	}

	//	Create botify service
	notifyService := bot_notify.NewService(pgConn, blockchainsService, b, languages, &botConf.Notify)

	//	Subscribe to system signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	//	Start notify service
	if err := notifyService.Start(ctx); err != nil {
		zap.L().Fatal("failed to start notify service", zap.Error(err))
	}

	go func() {
		stop := <-signalChan

		zap.L().Info("waiting for all processes to stop", zap.String("signal", stop.String()))

		if stopErr := notifyService.Stop(); stopErr != nil {
			zap.L().Error("failed to stop notify service", zap.Error(stopErr))
		}

		ok, stopErr := b.Close(ctx)
		if stopErr != nil {
			zap.L().Error("failed to close bot instance", zap.Error(stopErr))
		} else if !ok {
			zap.L().Warn("unsuccessful bot instance close")
		} else {
			zap.L().Info("closed bot instance")
		}

		cancel()
	}()

	//	Run bot (blocks until context is cancelled)
	zap.L().Info("starting bot")

	b.Start(ctx)

	//	Cleanup after bot stops
	blockchainsService.Close()
	zap.L().Info("closed blockchains pool api connections")

	pgConn.Close()
	zap.L().Info("closed postgres connection")

	zap.L().Info("bot stopped")
}

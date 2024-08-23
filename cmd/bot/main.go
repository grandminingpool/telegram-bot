package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/go-playground/validator/v10"
	botConfig "github.com/grandminingpool/telegram-bot/configs/bot"
	postgresConfig "github.com/grandminingpool/telegram-bot/configs/postgres"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	poolBot "github.com/grandminingpool/telegram-bot/internal/bot"
	"github.com/grandminingpool/telegram-bot/internal/bot/handlers"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/grandminingpool/telegram-bot/internal/common/flags"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	"github.com/grandminingpool/telegram-bot/internal/common/logger"
	botNotify "github.com/grandminingpool/telegram-bot/internal/notify"
	postgresProvider "github.com/grandminingpool/telegram-bot/internal/providers/postgres"
	"go.uber.org/zap"
)

func main() {
	//	Init context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	//	Parse flags
	parsedFlags := flags.ParseFlags()

	//	Setup flags
	flagsConf := flags.SetupFlags(parsedFlags)

	//	Setup logger
	zapLogger, err := logger.SetupLogger(flagsConf.Mode)
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
	postgresConf, err := postgresConfig.New(flagsConf.ConfigsPath, validate)
	if err != nil {
		zap.L().Fatal("failed to load postgres config", zap.Error(err))
	}

	//	Init postgres connection
	pgConn, err := postgresProvider.NewConnection(ctx, postgresConf)
	if err != nil {
		zap.L().Fatal("failed to create postgres connection", zap.Error(err))
	}

	//	Init blockchains service and start
	blockchainsService := blockchains.NewService(pgConn)
	if err := blockchainsService.Start(ctx, flagsConf.CertsPath); err != nil {
		zap.L().Fatal("failed to start blockchains service", zap.Error(err))
	}

	//	Init bot services
	userService := services.NewUserService(pgConn)
	userActionService := services.NewUserActionService(pgConn)
	userWalletService := services.NewUserWalletService(pgConn, blockchainsService)
	feedbackService := services.NewFeedbackService(pgConn)

	//	Init bot config
	botConf, err := botConfig.New(flagsConf.ConfigsPath, validate)
	if err != nil {
		zap.L().Fatal("failed to load bot config", zap.Error(err))
	}

	//	Create bot
	defaultHandler := handlers.NewDefaultHandler(languages)
	botOptions := poolBot.CreateBotOptions(
		flagsConf.Mode,
		blockchainsService,
		userService,
		userActionService,
		userWalletService,
		languages,
		defaultHandler,
		botConf,
	)
	b, err := poolBot.CreateBot(botOptions, botConf.BotToken)
	if err != nil {
		zap.L().Fatal("failed to create bot", zap.Error(err))
	}

	poolBotHandlerMatcher := poolBot.NewHandlerMatcher(ctx, userActionService)
	poolBot.RegisterHandlers(
		b,
		poolBotHandlerMatcher,
		defaultHandler,
		userActionService,
		userWalletService,
		feedbackService,
		blockchainsService,
		botConf,
	)

	if err := poolBot.SetBotDescription(ctx, b, languages.GetLocalizers()); err != nil {
		zap.L().Warn("failed to set bot description", zap.Error(err))
	}

	//	Create botify service
	notifyService := botNotify.NewService(pgConn, blockchainsService, b, languages, &botConf.Notify)

	//	Subscribe to system signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		stop := <-signalChan

		zap.L().Info("waiting for all processes to stop", zap.String("signal", stop.String()))

		var stopErr error
		if stopErr = notifyService.Stop(); stopErr != nil {
			zap.L().Fatal("failed to stop notify service", zap.Error(stopErr))
		}

		ok, stopErr := b.Close(ctx)
		if stopErr != nil {
			zap.L().Fatal("failed to close bot instance", zap.Error(stopErr))
		} else if !ok {
			zap.L().Warn("unsuccessful bot instance close")
		} else {
			zap.L().Info("closed bot instance")
		}

		cancel()

		blockchainsService.Close()
		zap.L().Info("closed blockchains pool api connections")

		if stopErr = pgConn.Close(); stopErr != nil {
			zap.L().Fatal("failed to close postgres connection", zap.Error(stopErr))
		}

		zap.L().Info("closed postgres connection")
	}()

	//	Run bot
	zap.L().Info("starting bot")

	b.Start(ctx)

	//	Start notify service
	if err := notifyService.Start(ctx); err != nil {
		zap.L().Fatal("failed to start notify service", zap.Error(err))
	}

	wg.Wait()
	zap.L().Info("bot stopped")
}

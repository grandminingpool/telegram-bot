package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pool_miners_proto "github.com/grandminingpool/pool-api-proto/generated/pool_miners"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	bot_keyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

type AddWalletHandler struct {
	userActionService    *services.UserActionService
	userWalletService    *services.UserWalletService
	blockchainsService   *blockchains.Service
	checkWorkersInterval int
	walletsLimitPerUser  int
}

// ValidateAndAdd validates wallet address via Pool API, checks limits and duplicates,
// and adds the wallet. Returns a non-empty errorMessageID for user-facing errors.
func (h *AddWalletHandler) ValidateAndAdd(ctx context.Context, userID int64, coin, wallet string) (errorMessageID string, err error) {
	conn, err := h.blockchainsService.GetConnection(coin)
	if err != nil {
		return "", fmt.Errorf("get blockchain connection (coin: %s): %w", coin, err)
	}

	client := pool_miners_proto.NewPoolMinersServiceClient(conn)
	response, err := client.ValidateAddress(ctx, &pool_miners_proto.MinerAddressRequest{
		Address: wallet,
	})
	if err != nil {
		return "", fmt.Errorf("validate address (coin: %s, wallet: %s): %w", coin, wallet, err)
	}

	if !response.Valid {
		return "InvalidWallet", nil
	}

	walletsCount, err := h.userWalletService.Count(ctx, userID, coin)
	if err != nil {
		return "", fmt.Errorf("count wallets (coin: %s): %w", coin, err)
	}

	if walletsCount+1 > h.walletsLimitPerUser {
		return "ExceededWalletsLimit", nil
	}

	hasDuplicates, err := h.userWalletService.CheckDuplicates(ctx, userID, coin, wallet)
	if err != nil {
		return "", fmt.Errorf("check duplicates (coin: %s, wallet: %s): %w", coin, wallet, err)
	}

	if hasDuplicates {
		return "WalletAlreadyAdded", nil
	}

	if err := h.userWalletService.Add(ctx, userID, coin, wallet); err != nil {
		return "", fmt.Errorf("add wallet (coin: %s, wallet: %s): %w", coin, wallet, err)
	}

	return "", nil
}

func (h *AddWalletHandler) Handler(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
	if user.Action == nil {
		return
	}

	coin := *user.Action.Payload
	if _, err := h.blockchainsService.GetInfo(coin); err != nil {
		zap.L().Error("get blockchain info error",
			zap.Int64("user_id", user.ID),
			zap.String("coin", coin),
			zap.Error(err),
		)

		return
	}

	wallet := update.Message.Text

	errorMessageID, err := h.ValidateAndAdd(ctx, user.ID, coin, wallet)
	if err != nil {
		zap.L().Error("add wallet error",
			zap.Int64("user_id", user.ID),
			zap.String("coin", coin),
			zap.String("wallet", wallet),
			zap.Error(err),
		)

		return
	}

	if errorMessageID != "" {
		replyMarkup := bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer)
		if errorMessageID == "InvalidWallet" {
			replyMarkup = nil
		}

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: errorMessageID,
			}),
			ReplyMarkup: replyMarkup,
		})

		return
	}

	if err := h.userActionService.Clear(ctx, user.ID); err != nil {
		zap.L().Error("error clearing user action after adding wallet",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "WalletAdded",
			TemplateData: map[string]string{
				"CheckWorkersInterval": fmt.Sprintf("%d", h.checkWorkersInterval),
			},
		}),
		ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})
}

func NewAddWalletHandler(
	userActionService *services.UserActionService,
	userWalletService *services.UserWalletService,
	blockchainsService *blockchains.Service,
	checkWorkersInterval int,
	walletsLimitPerUser int,
) *AddWalletHandler {
	return &AddWalletHandler{
		userActionService:    userActionService,
		userWalletService:    userWalletService,
		blockchainsService:   blockchainsService,
		checkWorkersInterval: checkWorkersInterval,
		walletsLimitPerUser:  walletsLimitPerUser,
	}
}

package handlers

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	poolMinersProto "github.com/grandminingpool/pool-api-proto/generated/pool_miners"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	botKeyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
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

func (h *AddWalletHandler) Handler(ctx context.Context, user *middlewares.User, startKeyboard *botKeyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
	if user.Action != nil {
		coin := *user.Action.Payload
		blockchain, err := h.blockchainsService.GetInfo(coin)
		if err != nil {
			zap.L().Error("get blockchain info error",
				zap.Int64("user_id", user.ID),
				zap.String("coin", coin),
				zap.Error(err),
			)

			return
		}

		conn, err := h.blockchainsService.GetConnection(coin)
		if err != nil {
			zap.L().Error("get blockchain pool connection error",
				zap.Int64("user_id", user.ID),
				zap.String("coin", coin),
				zap.Error(err),
			)

			return
		}

		wallet := update.Message.Text
		client := poolMinersProto.NewPoolMinersServiceClient(conn)
		response, err := client.ValidateAddress(ctx, &poolMinersProto.MinerAddressRequest{
			Address: wallet,
		})
		if err != nil {
			zap.L().Error("wallet address validation error",
				zap.Int64("user_id", user.ID),
				zap.String("coin", coin),
				zap.String("wallet", wallet),
				zap.Error(err),
			)

			return
		}

		if !response.Valid {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "InvalidWallet",
				}),
			})
		} else {
			walletsCount, err := h.userWalletService.Count(ctx, user.ID, blockchain.Coin)
			if err != nil {
				zap.L().Error("count user wallets error",
					zap.Int64("user_id", user.ID),
					zap.String("coin", coin),
					zap.Error(err),
				)

				return
			}

			if walletsCount+1 > h.walletsLimitPerUser {
				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "ExceededWalletsLimit",
					}),
				})

				return
			}

			hasDuplicates, err := h.userWalletService.CheckDuplicates(ctx, user.ID, blockchain.Coin, wallet)
			if err != nil {
				zap.L().Error("check user wallet duplicates error",
					zap.Int64("user_id", user.ID),
					zap.String("coin", coin),
					zap.String("wallet", wallet),
					zap.Error(err),
				)

				return
			}

			if hasDuplicates {
				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "WalletAlreadyAdded",
					}),
					ReplyMarkup: botKeyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
				})

				return
			}

			if err := h.userWalletService.Add(ctx, user.ID, blockchain.Coin, wallet); err != nil {
				zap.L().Error("add user wallet error",
					zap.Int64("user_id", user.ID),
					zap.String("coin", coin),
					zap.String("wallet", wallet),
					zap.Error(err),
				)

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
				ReplyMarkup: botKeyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
			})
		}
	}
}

func NewAddWalletHandler(
	userActionService *services.UserActionService,
	userWalletService *services.UserWalletService,
	blockchainsService *blockchains.Service,
	checkWorkersInterval int,
) *AddWalletHandler {
	return &AddWalletHandler{
		userActionService:    userActionService,
		userWalletService:    userWalletService,
		blockchainsService:   blockchainsService,
		checkWorkersInterval: checkWorkersInterval,
	}
}

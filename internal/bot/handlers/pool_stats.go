package handlers

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	pool_proto "github.com/grandminingpool/pool-api-proto/generated/pool"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	bot_keyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	format_utils "github.com/grandminingpool/telegram-bot/internal/utils/format"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
)

type PoolStatsHandler struct {
	blockchainsService *blockchains.Service
}

func (h *PoolStatsHandler) Back(
	ctx context.Context,
	user *middlewares.User,
	startKeyboard *bot_keyboards.StartKeyboard,
	b *bot.Bot,
	update *models.Update,
) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ReturningToMenu",
		}),
		ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})
}

func (h *PoolStatsHandler) OnBlockchainSelected(
	ctx context.Context,
	user *middlewares.User,
	startKeyboard *bot_keyboards.StartKeyboard,
	blockchain blockchains.BlockchainInfo,
	b *bot.Bot,
	update *models.Update,
) {
	conn, err := h.blockchainsService.GetConnection(blockchain.Coin)
	if err != nil {
		zap.L().Error("get blockchain pool connection error",
			zap.Int64("user_id", user.ID),
			zap.String("coin", blockchain.Coin),
			zap.Error(err),
		)

		return
	}

	client := pool_proto.NewPoolServiceClient(conn)
	apiCtx, cancel := h.blockchainsService.WithAPITimeout(ctx)
	defer cancel()

	poolInfo, err := client.GetPoolInfo(apiCtx, &emptypb.Empty{})
	if err != nil {
		zap.L().Error("get blockchain pool info error",
			zap.Int64("user_id", user.ID),
			zap.String("coin", blockchain.Coin),
			zap.Error(err),
		)

		return
	}

	poolStats, err := client.GetPoolStats(apiCtx, &pool_proto.GetPoolAssetRequest{Solo: false})
	if err != nil {
		zap.L().Error("get blockchain pool stats error",
			zap.Int64("user_id", user.ID),
			zap.String("coin", blockchain.Coin),
			zap.Error(err),
		)

		return
	}

	var msgBuf bytes.Buffer
	msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "PoolStatsMainInfo",
		TemplateData: map[string]string{
			"PoolBlockchainName": blockchain.Name,
			"Algos":              strings.Join(poolInfo.Algos, ", "),
			"PayoutMode":         poolInfo.PayoutMode.String(),
			"Solo":               format_utils.BoolText(poolInfo.Solo, user.Localizer),
		},
	}))
	msgBuf.WriteString("\n\n")
	msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "PoolStatsFeeInfo",
		TemplateData: map[string]string{
			"Fee": fmt.Sprintf("%.1f", poolInfo.Fee.Fee),
		},
	}))

	if poolInfo.Solo && poolInfo.Fee.SoloFee != nil {
		msgBuf.WriteString("\n")
		msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "PoolStatsSoloFeeInfo",
			TemplateData: map[string]string{
				"Fee": fmt.Sprintf("%.1f", *poolInfo.Fee.SoloFee),
			},
		}))
	}

	msgBuf.WriteString("\n\n")
	msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "PoolStatsMiningInfoCaption",
	}))
	msgBuf.WriteString("\n")
	msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "PoolStatsMiningInfo",
		TemplateData: map[string]string{
			"MinersCount":   fmt.Sprintf("%d", poolStats.MinersCount),
			"Hashrate": format_utils.Hashrate(new(big.Int).SetBytes(poolStats.Hashrate), blockchain.Coin),
			"AvgHashrate":   format_utils.Hashrate(new(big.Int).SetBytes(poolStats.AvgHashrate), blockchain.Coin),
		},
	}))

	if poolInfo.Solo {
		soloPoolStats, err := client.GetPoolStats(apiCtx, &pool_proto.GetPoolAssetRequest{Solo: true})
		if err != nil {
			zap.L().Warn("get blockchain solo pool stats error",
				zap.Int64("user_id", user.ID),
				zap.String("coin", blockchain.Coin),
				zap.Error(err),
			)
		} else {
			msgBuf.WriteString("\n\n")
			msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "PoolStatsSoloMiningInfoCaption",
			}))
			msgBuf.WriteString("\n")
			msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "PoolStatsMiningInfo",
				TemplateData: map[string]string{
					"MinersCount":   fmt.Sprintf("%d", soloPoolStats.MinersCount),
					"Hashrate": format_utils.Hashrate(new(big.Int).SetBytes(soloPoolStats.Hashrate), blockchain.Coin),
					"AvgHashrate":   format_utils.Hashrate(new(big.Int).SetBytes(soloPoolStats.AvgHashrate), blockchain.Coin),
				},
			}))
		}
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		ParseMode:   models.ParseModeHTML,
		Text:        msgBuf.String(),
		ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})
}

func NewPoolStatsHandler(blockchainsService *blockchains.Service) *PoolStatsHandler {
	return &PoolStatsHandler{
		blockchainsService: blockchainsService,
	}
}

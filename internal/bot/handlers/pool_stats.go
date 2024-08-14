package handlers

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	poolProto "github.com/grandminingpool/pool-api-proto/generated/pool"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	botKeyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	formatUtils "github.com/grandminingpool/telegram-bot/internal/utils/format"
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
	startKeyboard *botKeyboards.StartKeyboard,
	b *bot.Bot,
	update *models.Update,
) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ReturningToMenu",
		}),
		ReplyMarkup: botKeyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})
}

func (h *PoolStatsHandler) OnBlockchainSelected(
	ctx context.Context,
	user *middlewares.User,
	startKeyboard *botKeyboards.StartKeyboard,
	blockchain *blockchains.BlockchainInfo,
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

	client := poolProto.NewPoolServiceClient(conn)
	poolInfo, err := client.GetPoolInfo(ctx, &emptypb.Empty{})
	if err != nil {
		zap.L().Error("get blockchain pool info error",
			zap.Int64("user_id", user.ID),
			zap.String("coin", blockchain.Coin),
			zap.Error(err),
		)

		return
	}

	poolStats, err := client.GetPoolStats(ctx, &emptypb.Empty{})
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
			"Solo":               formatUtils.BoolText(poolInfo.Solo, user.Localizer),
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
			"TotalHashrate": formatUtils.Hashrate(new(big.Int).SetBytes(poolStats.Hashrate)),
			"AvgHashrate":   formatUtils.Hashrate(new(big.Int).SetBytes(poolStats.AvgHashrate)),
		},
	}))

	if poolInfo.Solo && poolStats.SoloMinersCount != nil && poolStats.SoloHashrate != nil && poolStats.SoloAvgHashrate != nil {
		msgBuf.WriteString("\n\n")
		msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "PoolStatsSoloMiningInfoCaption",
		}))
		msgBuf.WriteString("\n")
		msgBuf.WriteString(user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "PoolStatsMiningInfo",
			TemplateData: map[string]string{
				"MinersCount":   fmt.Sprintf("%d", *poolStats.SoloMinersCount),
				"TotalHashrate": formatUtils.Hashrate(new(big.Int).SetBytes(poolStats.SoloHashrate)),
				"AvgHashrate":   formatUtils.Hashrate(new(big.Int).SetBytes(poolStats.SoloAvgHashrate)),
			},
		}))
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        msgBuf.String(),
		ReplyMarkup: botKeyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})
}

func NewPoolStatsHandler(blockchainsService *blockchains.Service) *PoolStatsHandler {
	return &PoolStatsHandler{
		blockchainsService: blockchainsService,
	}
}

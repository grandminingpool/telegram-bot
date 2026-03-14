package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/grandminingpool/telegram-bot/internal/blockchains"
	bot_keyboards "github.com/grandminingpool/telegram-bot/internal/bot/keyboards"
	"github.com/grandminingpool/telegram-bot/internal/bot/middlewares"
	"github.com/grandminingpool/telegram-bot/internal/common/constants"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

type DeepLinkHandler struct {
	defaultHandler     *DefaultHandler
	addWalletHandler   *AddWalletHandler
	blockchainsService *blockchains.Service
	botUsername         string
}

func (h *DeepLinkHandler) Handler(ctx context.Context, user *middlewares.User, startKeyboard *bot_keyboards.StartKeyboard, b *bot.Bot, update *models.Update) {
	payload := strings.TrimPrefix(update.Message.Text, string(constants.StartCommand)+" ")

	if payload == update.Message.Text || payload == "" {
		h.defaultHandler.Handler(ctx, user, startKeyboard, b, update)

		return
	}

	parts := strings.SplitN(payload, constants.DeepLinkSeparator, 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "DeepLinkInvalidFormat",
				TemplateData: map[string]string{
					"BotUsername": h.botUsername,
				},
			}),
			ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
		})

		return
	}

	coin := parts[0]
	wallet := parts[1]

	if _, err := h.blockchainsService.GetInfo(coin); err != nil {
		zap.L().Debug("deep link: blockchain not found",
			zap.String("coin", coin),
		)

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "DeepLinkBlockchainNotFound",
			}),
			ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
		})

		return
	}

	errorMessageID, err := h.addWalletHandler.ValidateAndAdd(ctx, user.ID, coin, wallet)
	if err != nil {
		zap.L().Error("deep link: add wallet error",
			zap.Int64("user_id", user.ID),
			zap.String("coin", coin),
			zap.String("wallet", wallet),
			zap.Error(err),
		)

		return
	}

	if errorMessageID != "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: errorMessageID,
			}),
			ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
		})

		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: user.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "WalletAdded",
			TemplateData: map[string]string{
				"CheckWorkersInterval": fmt.Sprintf("%d", h.addWalletHandler.checkWorkersInterval),
			},
		}),
		ReplyMarkup: bot_keyboards.CreateStartReplyKeyboard(b, startKeyboard, user.Localizer),
	})

	b.SetChatMenuButton(ctx, &bot.SetChatMenuButtonParams{
		ChatID: update.Message.Chat.ID,
		MenuButton: models.MenuButtonCommands{
			Type: models.MenuButtonTypeCommands,
		},
	})
}

func NewDeepLinkHandler(
	defaultHandler *DefaultHandler,
	addWalletHandler *AddWalletHandler,
	blockchainsService *blockchains.Service,
	botUsername string,
) *DeepLinkHandler {
	return &DeepLinkHandler{
		defaultHandler:     defaultHandler,
		addWalletHandler:   addWalletHandler,
		blockchainsService: blockchainsService,
		botUsername:         botUsername,
	}
}

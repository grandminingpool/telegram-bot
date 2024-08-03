package handlersMatchers

import (
	"context"

	"github.com/go-telegram/bot/models"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"go.uber.org/zap"
)

type WalletMather struct {
	userActionService *services.UserActionService
	serviceCtx        context.Context
}

func (m *WalletMather) MatchAdd(update *models.Update) bool {
	if update.Message != nil {
		userAction, err := m.userActionService.Get(m.serviceCtx, update.Message.From.ID)
		if err != nil {
			zap.L().Error("get user action error while match add wallet handler",
				zap.Error(err),
				zap.Int64("user_id", update.Message.From.ID),
			)
		}

		return userAction.Action == services.UserAddWalletAction
	}

	return false
}

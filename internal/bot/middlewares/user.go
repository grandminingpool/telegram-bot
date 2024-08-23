package middlewares

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	"github.com/grandminingpool/telegram-bot/internal/common/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"
)

const USER_CTX_KEY types.CtxKey = "botUser"

type UserSettings struct {
	PayoutsNotify bool
	BlocksNotify  bool
}

type UserAction struct {
	Action  services.UserAction
	Payload *string
}

type User struct {
	ID        int64
	ChatID    int64
	Lang      string
	Localizer *i18n.Localizer
	Settings  UserSettings
	Action    *UserAction
}

type UserHandlerFunc func(context.Context, *User, *bot.Bot, *models.Update)

type UserMiddleware struct {
	userService       *services.UserService
	userActionService *services.UserActionService
	languages         *languages.Languages
}

func (m *UserMiddleware) Middleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message != nil {
			user, err := m.userService.Init(ctx, update.Message.From, update.Message.Chat.ID)
			if err != nil {
				zap.L().Error("init user error",
					zap.Int64("user_id", update.Message.From.ID),
					zap.Error(err),
				)

				next(ctx, b, update)

				return
			}

			userAction, err := m.userActionService.Get(ctx, user.ID)
			if err != nil {
				zap.L().Error("get user action error",
					zap.Int64("user_id", user.ID),
					zap.Error(err),
				)
			}

			userLocalizer := m.languages.GetLocalizer(user.Lang)

			userCtx := &User{
				ID:        user.ID,
				Lang:      user.Lang,
				Localizer: userLocalizer,
				Settings: UserSettings{
					PayoutsNotify: user.PayoutsNotify,
					BlocksNotify:  user.BlocksNotify,
				},
				Action: nil,
			}

			if userAction != nil {
				userCtx.Action = &UserAction{
					Action:  userAction.Action,
					Payload: userAction.Payload,
				}
			}

			newCtx := context.WithValue(ctx, USER_CTX_KEY, userCtx)

			next(newCtx, b, update)
		} else {
			next(ctx, b, update)
		}
	}
}

func CreateUserMiddleware(
	userService *services.UserService,
	userActionService *services.UserActionService,
	languages *languages.Languages,
) *UserMiddleware {
	return &UserMiddleware{
		userService:       userService,
		userActionService: userActionService,
		languages:         languages,
	}
}

func WithUserHandler(handler UserHandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		user, ok := ctx.Value(USER_CTX_KEY).(*User)
		if ok {
			handler(ctx, user, b, update)
		}
	}
}

package middlewares

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/grandminingpool/telegram-bot/internal/bot/services"
	"github.com/grandminingpool/telegram-bot/internal/common/languages"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const USER_CTX_KEY = "botUser"

type UserSettings struct {
	PayoutsNotify bool
	BlockNotify   bool
}

type User struct {
	ID        int64
	Lang      string
	Localizer *i18n.Localizer
	Settings  UserSettings
}

type UserMiddleware struct {
	userService *services.UserService
	languages   *languages.Languages
}

func (m *UserMiddleware) Middleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message != nil {
			userSetting, err := m.userService.InitSettings(ctx, update.Message.From)
			if err != nil {

			}

			userLocalizer := m.languages.GetLocalizer(userSetting.Lang)

			user := &User{
				ID:        userSetting.ID,
				Lang:      userSetting.Lang,
				Localizer: userLocalizer,
				Settings: UserSettings{
					PayoutsNotify: userSetting.PayoutsNotify,
					BlockNotify:   userSetting.BlockNotify,
				},
			}

			newCtx := context.WithValue(ctx, USER_CTX_KEY, user)

			next(newCtx, b, update)
		} else {
			next(ctx, b, update)
		}
	}
}

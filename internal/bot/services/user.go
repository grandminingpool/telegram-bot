package services

import (
	"context"
	"database/sql"

	"github.com/go-telegram/bot/models"
	"github.com/jmoiron/sqlx"
)

type UserSettingDB struct {
	ID            int64  `db:"id"`
	Lang          string `db:"lang"`
	PayoutsNotify bool   `db:"payouts_notify"`
	BlockNotify   bool   `db:"block_notify"`
}

type UserService struct {
	pgConn *sqlx.DB
}

func (s *UserService) FindSettings(ctx context.Context, id int64) (*UserSettingDB, error) {
	var userSetting UserSettingDB
	err := s.pgConn.Select(&userSetting, "SELECT * FROM user_settings WHERE id = ?", id)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {

	}

	return &userSetting, nil
}

func (s *UserService) InitSettings(ctx context.Context, user *models.User) (*UserSettingDB, error) {
	userSetting, err := s.FindSettings(ctx, user.ID)
	if err != nil {

	}

	if userSetting == nil {
		userSetting = &UserSettingDB{
			ID:            user.ID,
			Lang:          user.LanguageCode,
			PayoutsNotify: true,
			BlockNotify:   true,
		}

		if _, err := s.pgConn.ExecContext(ctx, `INSERT INTO user_settings (
			id, 
			lang, 
			payouts_notify, 
			block_notify
		) VALUES (?, ?, ?, ?)`,
			userSetting.ID,
			userSetting.Lang,
			userSetting.PayoutsNotify,
			userSetting.BlockNotify,
		); err != nil {

		}
	}

	return userSetting, nil
}

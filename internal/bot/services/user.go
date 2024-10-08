package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-telegram/bot/models"
	"github.com/jmoiron/sqlx"
	"golang.org/x/text/language"
)

type UserDB struct {
	ID            int64  `db:"id"`
	ChatID        int64  `db:"chat_id"`
	Lang          string `db:"lang"`
	PayoutsNotify bool   `db:"payouts_notify"`
	BlocksNotify  bool   `db:"blocks_notify"`
}

type UserService struct {
	pgConn *sqlx.DB
}

func (s *UserService) SetPayoutsNotify(ctx context.Context, id int64, value bool) error {
	if _, err := s.pgConn.ExecContext(ctx, "UPDATE users SET payouts_notify = $1 WHERE user_id = $2", value, id); err != nil {
		return fmt.Errorf("failed to update user (id: %d) payouts notify: %w", id, err)
	}

	return nil
}

func (s *UserService) SetBlocksNotify(ctx context.Context, id int64, value bool) error {
	if _, err := s.pgConn.ExecContext(ctx, "UPDATE users SET blocks_notify = $1 WHERE user_id = $2", value, id); err != nil {
		return fmt.Errorf("failed to update user (id: %d) blocks notify: %w", id, err)
	}

	return nil
}

func (s *UserService) SetLang(ctx context.Context, id int64, languageTag language.Tag) error {
	if _, err := s.pgConn.ExecContext(ctx, "UPDATE users SET lang = $1 WHERE user_id = $2", languageTag.String(), id); err != nil {
		return fmt.Errorf("failed to update user (id: %d) lang: %w", id, err)
	}

	return nil
}

func (s *UserService) Find(ctx context.Context, id int64) (*UserDB, error) {
	var user UserDB
	err := s.pgConn.SelectContext(ctx, &user, "SELECT * FROM users WHERE id = ?", id)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to find user (id: %d), error: %w", id, err)
	}

	return &user, nil
}

func (s *UserService) Init(ctx context.Context, botUser *models.User, chatID int64) (*UserDB, error) {
	user, err := s.Find(ctx, botUser.ID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		user = &UserDB{
			ID:            botUser.ID,
			ChatID:        chatID,
			Lang:          botUser.LanguageCode,
			PayoutsNotify: true,
			BlocksNotify:  true,
		}

		if _, err := s.pgConn.ExecContext(ctx, `INSERT INTO users (
			id, 
			chat_id,
			lang, 
			payouts_notify, 
			blocks_notify
		) VALUES (?, ?, ?, ?)`,
			user.ID,
			user.ChatID,
			user.Lang,
			user.PayoutsNotify,
			user.BlocksNotify,
		); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else if user.ChatID != chatID {
		user.ChatID = chatID
		_, err := s.pgConn.ExecContext(ctx, `UPDATE users SET user_id = $1 WHERE id = $2`, user.ID, chatID)
		if err != nil {
			return user, fmt.Errorf("failed to update user (id: %d) chat id (new value: %d), error: %w", user.ID, chatID, err)
		}
	}

	return user, nil
}

func NewUserService(pgConn *sqlx.DB) *UserService {
	return &UserService{
		pgConn: pgConn,
	}
}

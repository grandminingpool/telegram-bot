package services

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type UserAction string

const (
	UserAddWalletAction UserAction = "add_wallet"
)

func (ua UserAction) Scan(val any) error {
	switch v := val.(type) {
	case string:
		switch v {
		case string(UserAddWalletAction):
			ua = UserAddWalletAction
		default:
			return fmt.Errorf("invalid user action value: %s", v)
		}

		return nil
	default:
		return fmt.Errorf("unsupported user action type: %T", v)
	}
}

func (ua UserAction) Value() (driver.Value, error) {
	return string(ua), nil
}

type UserActionDB struct {
	UserID  int64      `db:"user_id"`
	Action  UserAction `db:"action"`
	Payload *string    `db:"payload"`
}

type UserActionService struct {
	pgConn *sqlx.DB
}

func (a *UserActionService) Set(ctx context.Context, userID int64, action UserAction, payload *string) error {
	if _, err := a.pgConn.ExecContext(ctx, `INSERT INTO user_actions (
		id, 
		action, 
		payload
	) VALUES ($1, $2, $3)`, userID, action, payload); err != nil {
		return fmt.Errorf("failed to set user (id: %d) action (name: %s, payload: %v), error: %w", userID, string(action), payload, err)
	}

	return nil
}

func (a *UserActionService) Get(ctx context.Context, userID int64) (*UserActionDB, error) {
	var userAction UserActionDB
	err := a.pgConn.SelectContext(ctx, userAction, "SELECT * FROM user_actions WHERE user_id = $1", userID)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get user (id: %d) action: %w", userID, err)
	}

	return &userAction, nil
}

func (a *UserActionService) Clear(ctx context.Context, userID int64) error {
	if _, err := a.pgConn.ExecContext(ctx, `DELETE FROM user_actions WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("failed to clear user (id: %d) actions: %w", userID, err)
	}

	return nil
}

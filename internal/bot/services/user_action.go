package services

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserAction string

const (
	UserAddWalletAction                 UserAction = "add_wallet"
	ReportBugAction                     UserAction = "report_bug"
	UserAddWalletSelectBlockchainAction UserAction = "add_wallet_select_blockchain"
	ShowPoolStatsAction                 UserAction = "show_pool_stats"
	UserRemoveWalletAction              UserAction = "remove_wallet"
	UserSettingsAction                  UserAction = "settings"
	UserSettingsSelectLanguageAction    UserAction = "settings_select_language"
	UserRemoveWalletSelectWalletAction  UserAction = "remove_wallet_select_wallet"
)

var validUserActions = map[UserAction]struct{}{
	UserAddWalletAction:                 {},
	ReportBugAction:                     {},
	UserAddWalletSelectBlockchainAction: {},
	ShowPoolStatsAction:                 {},
	UserRemoveWalletAction:              {},
	UserSettingsAction:                  {},
	UserSettingsSelectLanguageAction:    {},
	UserRemoveWalletSelectWalletAction:  {},
}

func (ua *UserAction) Scan(val any) error {
	switch v := val.(type) {
	case string:
		action := UserAction(v)
		if _, ok := validUserActions[action]; !ok {
			return fmt.Errorf("invalid user action value: %s", v)
		}

		*ua = action

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
	pgConn *pgxpool.Pool
}

func (a *UserActionService) Set(ctx context.Context, userID int64, action UserAction, payload *string) error {
	if _, err := a.pgConn.Exec(ctx, `INSERT INTO user_actions (
		user_id,
		action,
		payload
	) VALUES ($1, $2, $3)
	ON CONFLICT (user_id) DO UPDATE SET action = $2, payload = $3`, userID, action, payload); err != nil {
		return fmt.Errorf("failed to set user (id: %d) action (name: %s, payload: %v), error: %w", userID, string(action), payload, err)
	}

	return nil
}

func (a *UserActionService) Get(ctx context.Context, userID int64) (*UserActionDB, error) {
	var userAction UserActionDB
	err := a.pgConn.QueryRow(ctx, "SELECT user_id, action, payload FROM user_actions WHERE user_id = $1", userID).
		Scan(&userAction.UserID, &userAction.Action, &userAction.Payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get user (id: %d) action: %w", userID, err)
	}

	return &userAction, nil
}

func (a *UserActionService) Clear(ctx context.Context, userID int64) error {
	if _, err := a.pgConn.Exec(ctx, `DELETE FROM user_actions WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("failed to clear user (id: %d) actions: %w", userID, err)
	}

	return nil
}

func NewUserActionService(pgConn *pgxpool.Pool) *UserActionService {
	return &UserActionService{
		pgConn: pgConn,
	}
}

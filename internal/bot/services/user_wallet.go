package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type UserWalletDB struct {
	UserID       int64  `db:"user_id"`
	BlockchainID int16  `db:"blockchain_id"`
	Wallet       string `db:"wallet"`
}

type UserWalletService struct {
	pgConn *sqlx.DB
}

func (w *UserWalletService) FindWallets(ctx context.Context, userID int64) (*[]UserWalletDB, error) {
	wallets := []UserWalletDB{}
	err := w.pgConn.SelectContext(ctx, wallets, "SELECT * FROM user_wallets WHERE user_id = ?", userID)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to find user (id: %d) wallets: %w", userID, err)
	}

	return &wallets, nil
}

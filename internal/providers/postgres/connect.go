package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	postgres_config "github.com/grandminingpool/telegram-bot/configs/postgres"
)

func NewConnection(ctx context.Context, config *postgres_config.Config) (*pgxpool.Pool, error) {
	conn, err := pgxpool.New(ctx, config.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	if err = conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres connection: %w", err)
	}

	return conn, nil
}

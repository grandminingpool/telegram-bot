package postgresProvider

import (
	"context"
	"fmt"

	postgresConfig "github.com/grandminingpool/telegram-bot/configs/postgres"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func NewConnection(ctx context.Context, config *postgresConfig.Config) (*sqlx.DB, error) {
	conn, err := sqlx.Connect("postgres", config.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	if err = conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres connection: %w", err)
	}

	return conn, nil
}

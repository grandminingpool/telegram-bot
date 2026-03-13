# Grand Pool Telegram Bot

Telegram bot for monitoring [Grand Pool](https://grandpool.io) mining process: wallet balances, worker statuses, pool statistics, payout notifications and block discovery alerts.

Communicates with pool nodes via gRPC using [pool-api-proto](https://github.com/grandminingpool/pool-api-proto).

## Features

- Add/remove mining wallets and view balances
- Monitor active workers with hashrate and uptime
- Pool mining statistics (miners count, hashrate)
- Notifications when a worker goes offline
- Payout and solo block notifications
- Multi-language support
- Telegram Menu with `/start`, `/faq`, `/reportbug` commands

## Requirements

- Go 1.25+
- PostgreSQL 14+
- Access to a [Pool API](https://github.com/grandminingpool/pool-api-proto) gRPC endpoint with TLS certificates

## Configuration

### Bot config (`configs/bot/config.yaml`)

```yaml
botToken: "YOUR_TELEGRAM_BOT_TOKEN"
poolURL: "https://grandpool.io"
poolChatLink: "https://t.me/grandpoolio"
supportChatID: -1001234567890
walletsLimitPerUser: 50  # optional, default: 50
notify:
  maxWalletsInWorkersRequest: 200
  maxWalletsInPayoutsRequest: 250
  parallelNotificationsCount: 40
  checkIntervals:
    workers: 5    # minutes
    payouts: 60   # minutes
```

### PostgreSQL config (`configs/postgres/config.yaml`)

```yaml
host: 127.0.0.1
port: 5432
user: pool_telegram_bot
password: "YOUR_DB_PASSWORD"
database: pool_telegram_bot
```

## Build

```bash
go build -o pool_telegram_bot ./cmd/bot
```

## Database setup

```bash
# Create database and user
psql -U postgres -f scripts/init_db.sql

# Run migrations (using golang-migrate)
migrate -path migrations -database "postgres://pool_telegram_bot:password@localhost:5432/pool_telegram_bot?sslmode=disable" up
```

## Run

### Development

```bash
./pool_telegram_bot
```

### Production

```bash
./pool_telegram_bot \
  --mode prod \
  --configs /etc/pool-telegram-bot/ \
  --pool-api-certs /etc/ssl/certs/ \
  --locales-path /var/lib/pool-telegram-bot/locales/ \
  --locales en,ru,es \
  --logger-output-path /var/log/pool-telegram-bot/output.log \
  --log-level info
```

### CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `--mode` | `dev` | Application mode (`dev` or `prod`) |
| `--configs` | `configs` | Path to configuration directory |
| `--pool-api-certs` | `certs` | Path to Pool API TLS certificates |
| `--locales-path` | `locales` | Path to locale files directory |
| `--locales` | `en` | Comma-separated list of locales |
| `--logger-output-path` | `logs/output.log` | Log file path |
| `--log-level` | `info` | Log level (`debug`, `info`, `warn`, `error`) |

## Deployment

Setup filesystem permissions for the service user:

```bash
sudo bash scripts/setup_permissions.sh
```

This configures:
- `/etc/pool-telegram-bot/` — read-only config access
- `/var/lib/pool-telegram-bot/locales/` — read-only locale files
- `/var/log/pool-telegram-bot/` — writable log directory

## Project structure

```
cmd/bot/                  — Application entry point
configs/                  — Configuration loaders
internal/
  blockchains/            — Blockchain service (Pool API gRPC clients)
  bot/
    handlers/             — Telegram bot message handlers
    keyboards/            — Reply keyboard builders
    middlewares/           — User middleware
    services/             — User, wallet, action services
  clients/pool_api/       — Pool API gRPC client
  common/                 — Shared utilities (flags, logger, errors)
  notify/                 — Scheduled worker/payout check notifications
  utils/format/           — Formatting helpers (hashrate, wallet, uptime)
locales/                  — Translation files (TOML)
migrations/               — PostgreSQL migrations
scripts/                  — Setup scripts
```

## License

[MIT](LICENSE)

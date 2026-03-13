#!/bin/bash
# Setup filesystem permissions for pool-telegram-bot service.
# Run as root.
# Usage: sudo bash scripts/setup_permissions.sh

set -euo pipefail

USER="pool_api"
GROUP="pool_api"
CONFIG_DIR="/etc/pool-telegram-bot"
LOCALES_DIR="/var/lib/pool-telegram-bot/locales"
LOG_DIR="/var/log/pool-telegram-bot"

# Create directories if they don't exist
mkdir -p "$CONFIG_DIR" "$LOCALES_DIR" "$LOG_DIR"

# Config: read-only access via group
chown -R root:"$GROUP" "$CONFIG_DIR"
chmod 750 "$CONFIG_DIR"
find "$CONFIG_DIR" -type f -exec chmod 640 {} +

# Locales: read-only access via group
chown -R root:"$GROUP" "$LOCALES_DIR"
chmod 750 "$LOCALES_DIR"
find "$LOCALES_DIR" -type f -exec chmod 640 {} +

# Logs: write access
chown "$USER":"$GROUP" "$LOG_DIR"
chmod 755 "$LOG_DIR"

echo "Permissions configured for user '$USER':"
echo "  $CONFIG_DIR  — read (group)"
echo "  $LOCALES_DIR — read (group)"
echo "  $LOG_DIR     — read/write (owner)"

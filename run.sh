#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_PATH="${CONFIG_PATH:-$ROOT_DIR/config.yaml}"
GOCACHE_DIR="${GOCACHE:-$ROOT_DIR/.gocache}"
GOMODCACHE_DIR="${GOMODCACHE:-$ROOT_DIR/.gomodcache}"

cd "$ROOT_DIR"

if [[ ! -f "$CONFIG_PATH" ]]; then
  echo "Config file not found: $CONFIG_PATH" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Go is not installed or not available in PATH." >&2
  exit 1
fi

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is not installed or not available in PATH." >&2
  exit 1
fi

if grep -q "REPLACE_WITH_YOUR_TELEGRAM_BOT_TOKEN" "$CONFIG_PATH"; then
  echo "Please update telegram.bot_token in $CONFIG_PATH before running the bot." >&2
  exit 1
fi

DB_ADDRESS="$(sed -n 's/^  address: "\(.*\)"/\1/p' "$CONFIG_PATH" | head -n 1)"
if [[ -z "$DB_ADDRESS" ]]; then
  echo "database.address was not found in $CONFIG_PATH." >&2
  exit 1
fi

if [[ "$DB_ADDRESS" =~ ^postgres://([^:]+):([^@]+)@([^:/?#]+):([0-9]+)/([^?]+) ]]; then
  DB_USER="${DB_USER:-${BASH_REMATCH[1]}}"
  DB_PASSWORD="${DB_PASSWORD:-${BASH_REMATCH[2]}}"
  DB_HOST="${DB_HOST:-${BASH_REMATCH[3]}}"
  DB_PORT="${DB_PORT:-${BASH_REMATCH[4]}}"
  DB_NAME="${DB_NAME:-${BASH_REMATCH[5]}}"
else
  echo "database.address must look like postgres://user:password@host:port/dbname" >&2
  exit 1
fi

db_is_reachable() {
  (echo >/dev/tcp/"$DB_HOST"/"$DB_PORT") >/dev/null 2>&1
}

ensure_database_exists() {
  local escaped_db_name exists
  escaped_db_name="${DB_NAME//\"/\"\"}"

  exists="$(
    PGPASSWORD="$DB_PASSWORD" \
      psql \
      -h "$DB_HOST" \
      -p "$DB_PORT" \
      -U "$DB_USER" \
      -d postgres \
      -tAc "SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}'" \
      2>/dev/null || true
  )"

  if [[ "$exists" == "1" ]]; then
    return
  fi

  echo "Database '$DB_NAME' does not exist. Creating it..."
  PGPASSWORD="$DB_PASSWORD" \
    psql \
    -h "$DB_HOST" \
    -p "$DB_PORT" \
    -U "$DB_USER" \
    -d postgres \
    -v ON_ERROR_STOP=1 \
    -c "CREATE DATABASE \"$escaped_db_name\""
}

mkdir -p "$GOCACHE_DIR" "$GOMODCACHE_DIR"

if ! db_is_reachable; then
  echo "PostgreSQL is not reachable on ${DB_HOST}:${DB_PORT}." >&2
  echo "Start your database first, then run ./run.sh again." >&2
  exit 1
fi

ensure_database_exists

echo "Starting Telegram exchange bot..."
echo "Using config: $CONFIG_PATH"

exec env \
  GOCACHE="$GOCACHE_DIR" \
  GOMODCACHE="$GOMODCACHE_DIR" \
  GOSUMDB="${GOSUMDB:-off}" \
  GOFLAGS="${GOFLAGS:--mod=mod}" \
  go run ./cmd/exchangebot -config "$CONFIG_PATH"

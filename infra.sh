#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_PATH="${CONFIG_PATH:-$ROOT_DIR/config.yaml}"
GO_VERSION="${GO_VERSION:-$(awk '/^go / { print $2; exit }' "$ROOT_DIR/go.mod")}"
GOCACHE_DIR="${GOCACHE:-$ROOT_DIR/.gocache}"
GOMODCACHE_DIR="${GOMODCACHE:-$ROOT_DIR/.gomodcache}"
BINARY_PATH="${BINARY_PATH:-$ROOT_DIR/exchangebot}"

cd "$ROOT_DIR"

if [[ ! -f "$CONFIG_PATH" ]]; then
  echo "Config file not found: $CONFIG_PATH" >&2
  exit 1
fi

if [[ ! -f "$ROOT_DIR/go.mod" ]]; then
  echo "go.mod not found in $ROOT_DIR." >&2
  exit 1
fi

if ! command -v apt-get >/dev/null 2>&1; then
  echo "infra.sh currently supports Debian/Ubuntu servers with apt-get." >&2
  exit 1
fi

if [[ $EUID -eq 0 ]]; then
  SUDO=""
else
  if ! command -v sudo >/dev/null 2>&1; then
    echo "sudo is required when running infra.sh as a non-root user." >&2
    exit 1
  fi
  SUDO="sudo"
fi

run_root() {
  if [[ -n "$SUDO" ]]; then
    "$SUDO" "$@"
    return
  fi
  "$@"
}

run_postgres() {
  if [[ -n "$SUDO" ]]; then
    sudo -u postgres "$@"
    return
  fi
  runuser -u postgres -- "$@"
}

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

db_user_ident="${DB_USER//\"/\"\"}"
db_name_ident="${DB_NAME//\"/\"\"}"
db_user_literal="${DB_USER//\'/\'\'}"
db_password_literal="${DB_PASSWORD//\'/\'\'}"
db_name_literal="${DB_NAME//\'/\'\'}"

detect_go_arch() {
  case "$(uname -m)" in
    x86_64|amd64)
      echo "amd64"
      ;;
    aarch64|arm64)
      echo "arm64"
      ;;
    *)
      echo "Unsupported CPU architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

install_system_packages() {
  echo "Installing system packages..."
  run_root apt-get update
  run_root env DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates \
    curl \
    git \
    tar \
    build-essential \
    postgresql \
    postgresql-client \
    postgresql-contrib
}

install_go() {
  local go_arch archive_url archive_path current_version

  export PATH="/usr/local/go/bin:$PATH"

  if command -v go >/dev/null 2>&1; then
    current_version="$(go version | awk '{print $3}' | sed 's/^go//')"
    if [[ "$current_version" == "$GO_VERSION" ]]; then
      echo "Go $GO_VERSION is already installed."
      return
    fi
  fi

  go_arch="$(detect_go_arch)"
  archive_url="https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz"
  archive_path="/tmp/go${GO_VERSION}.linux-${go_arch}.tar.gz"

  echo "Installing Go $GO_VERSION..."
  curl -fsSL "$archive_url" -o "$archive_path"
  run_root rm -rf /usr/local/go
  run_root tar -C /usr/local -xzf "$archive_path"
  printf 'export PATH=/usr/local/go/bin:$PATH\n' | run_root tee /etc/profile.d/exchange-go.sh >/dev/null
  export PATH="/usr/local/go/bin:$PATH"
}

find_postgres_conf() {
  find /etc/postgresql -path '*/main/postgresql.conf' | sort | head -n 1
}

configure_postgres_service() {
  local postgres_conf

  postgres_conf="$(find_postgres_conf)"
  if [[ -z "$postgres_conf" ]]; then
    echo "Could not find postgresql.conf under /etc/postgresql." >&2
    exit 1
  fi

  echo "Configuring PostgreSQL on port $DB_PORT..."
  run_root sed -Ei "s/^#?port = .*/port = ${DB_PORT}/" "$postgres_conf"
  run_root sed -Ei "s/^#?listen_addresses = .*/listen_addresses = 'localhost'/" "$postgres_conf"

  run_root systemctl enable postgresql
  run_root systemctl restart postgresql
}

configure_database_access() {
  echo "Provisioning PostgreSQL role and database..."

  run_postgres psql postgres -v ON_ERROR_STOP=1 <<SQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${db_user_literal}') THEN
    CREATE ROLE "${db_user_ident}" LOGIN PASSWORD '${db_password_literal}' CREATEDB;
  ELSE
    ALTER ROLE "${db_user_ident}" WITH LOGIN PASSWORD '${db_password_literal}' CREATEDB;
  END IF;
END
\$\$;
SQL

  if [[ "$(run_postgres psql postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '${db_name_literal}'")" != "1" ]]; then
    run_postgres psql postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"${db_name_ident}\" OWNER \"${db_user_ident}\""
  fi

  run_postgres psql postgres -v ON_ERROR_STOP=1 -c "ALTER DATABASE \"${db_name_ident}\" OWNER TO \"${db_user_ident}\""
}

warm_go_modules() {
  echo "Downloading Go modules..."
  mkdir -p "$GOCACHE_DIR" "$GOMODCACHE_DIR"
  env \
    PATH="/usr/local/go/bin:$PATH" \
    GOCACHE="$GOCACHE_DIR" \
    GOMODCACHE="$GOMODCACHE_DIR" \
    GOSUMDB="${GOSUMDB:-off}" \
    GOFLAGS="${GOFLAGS:--mod=mod}" \
    go mod download
}

build_bot_binary() {
  echo "Building exchange bot binary..."
  mkdir -p "$GOCACHE_DIR" "$GOMODCACHE_DIR"
  env \
    PATH="/usr/local/go/bin:$PATH" \
    GOCACHE="$GOCACHE_DIR" \
    GOMODCACHE="$GOMODCACHE_DIR" \
    GOSUMDB="${GOSUMDB:-off}" \
    GOFLAGS="${GOFLAGS:--mod=mod}" \
    go build -o "$BINARY_PATH" ./cmd/exchangebot
  chmod +x "$BINARY_PATH"
}

main() {
  install_system_packages
  install_go
  configure_postgres_service
  configure_database_access
  warm_go_modules
  build_bot_binary

  echo
  echo "Infrastructure bootstrap completed."
  echo "PostgreSQL is configured for ${DB_HOST}:${DB_PORT}/${DB_NAME}."
  echo "You can now run ./run.sh"
  echo "Built bot binary: $BINARY_PATH"
  echo "If this shell still cannot find Go, run: source /etc/profile.d/exchange-go.sh"
}

main "$@"

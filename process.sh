#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_NAME="${SERVICE_NAME:-exchangebot}"
SERVICE_USER="${SERVICE_USER:-exchangebot}"
SERVICE_GROUP="${SERVICE_GROUP:-$SERVICE_USER}"
WORK_DIR="${WORK_DIR:-$ROOT_DIR}"
CONFIG_PATH="${CONFIG_PATH:-$ROOT_DIR/config.yaml}"
SERVICE_PATH="/etc/systemd/system/${SERVICE_NAME}.service"
RUN_SCRIPT="${RUN_SCRIPT:-$ROOT_DIR/run.sh}"

if [[ $EUID -eq 0 ]]; then
  SUDO=""
else
  if ! command -v sudo >/dev/null 2>&1; then
    echo "sudo is required when running process.sh as a non-root user." >&2
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

usage() {
  cat <<EOF
Usage: ./process.sh <command>

Commands:
  install   Create or update the systemd service and enable it
  start     Start the bot service
  stop      Stop the bot service
  restart   Restart the bot service
  status    Show service status
  logs      Tail service logs
  uninstall Disable and remove the service

Environment overrides:
  SERVICE_NAME   Default: ${SERVICE_NAME}
  SERVICE_USER   Default: ${SERVICE_USER}
  SERVICE_GROUP  Default: ${SERVICE_GROUP}
  WORK_DIR       Default: ${WORK_DIR}
  CONFIG_PATH    Default: ${CONFIG_PATH}
  RUN_SCRIPT     Default: ${RUN_SCRIPT}
EOF
}

require_systemd() {
  if ! command -v systemctl >/dev/null 2>&1; then
    echo "systemctl is not available on this server." >&2
    exit 1
  fi
}

ensure_service_user() {
  if id -u "$SERVICE_USER" >/dev/null 2>&1; then
    return
  fi

  echo "Creating service user: $SERVICE_USER"
  run_root useradd --system --create-home --shell /bin/bash "$SERVICE_USER"
}

ensure_paths() {
  if [[ ! -f "$RUN_SCRIPT" ]]; then
    echo "run script not found: $RUN_SCRIPT" >&2
    exit 1
  fi

  if [[ ! -f "$CONFIG_PATH" ]]; then
    echo "config file not found: $CONFIG_PATH" >&2
    exit 1
  fi

  run_root mkdir -p "$WORK_DIR"
  run_root chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "$WORK_DIR"
}

install_service() {
  require_systemd
  ensure_service_user
  ensure_paths

  echo "Installing systemd service at $SERVICE_PATH"
  run_root tee "$SERVICE_PATH" >/dev/null <<EOF
[Unit]
Description=Telegram Exchange Bot
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
WorkingDirectory=${WORK_DIR}
Environment=CONFIG_PATH=${CONFIG_PATH}
ExecStart=/bin/bash ${RUN_SCRIPT}
Restart=always
RestartSec=5
TimeoutStopSec=20
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

  run_root systemctl daemon-reload
  run_root systemctl enable "$SERVICE_NAME"
  echo "Service installed. Use ./process.sh start to launch it."
}

start_service() {
  require_systemd
  run_root systemctl start "$SERVICE_NAME"
}

stop_service() {
  require_systemd
  run_root systemctl stop "$SERVICE_NAME"
}

restart_service() {
  require_systemd
  run_root systemctl restart "$SERVICE_NAME"
}

status_service() {
  require_systemd
  run_root systemctl status "$SERVICE_NAME" --no-pager
}

logs_service() {
  require_systemd
  run_root journalctl -u "$SERVICE_NAME" -f
}

uninstall_service() {
  require_systemd

  if run_root systemctl list-unit-files "${SERVICE_NAME}.service" >/dev/null 2>&1; then
    run_root systemctl disable --now "$SERVICE_NAME" || true
  fi

  run_root rm -f "$SERVICE_PATH"
  run_root systemctl daemon-reload
}

COMMAND="${1:-}"

case "$COMMAND" in
  install)
    install_service
    ;;
  start)
    start_service
    ;;
  stop)
    stop_service
    ;;
  restart)
    restart_service
    ;;
  status)
    status_service
    ;;
  logs)
    logs_service
    ;;
  uninstall)
    uninstall_service
    ;;
  *)
    usage
    exit 1
    ;;
esac

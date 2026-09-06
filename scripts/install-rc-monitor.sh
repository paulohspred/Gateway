#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="/opt/rc-gateway"
SERVICE="rc-monitor.service"
CONFIG_TARGET="/etc/rc-monitor.json"
ENV_TARGET="/etc/rc-monitor.env"
UNIT_TARGET="/etc/systemd/system/rc-monitor.service"
HEALTH_URL="${RC_MONITOR_HEALTH_URL:-http://127.0.0.1:18100/readyz}"
HEALTH_ATTEMPTS="${RC_MONITOR_HEALTH_ATTEMPTS:-30}"
HEALTH_DELAY="${RC_MONITOR_HEALTH_DELAY_SECONDS:-1}"

usage(){
  echo "Uso: $0 [--dry-run] CONFIG.json [ENV_FILE]" >&2
  exit 64
}

DRY_RUN=0
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
  shift
fi
[[ $# -ge 1 && $# -le 2 ]] || usage
[[ "$HEALTH_ATTEMPTS" =~ ^[0-9]+$ ]] && (( HEALTH_ATTEMPTS >= 1 && HEALTH_ATTEMPTS <= 300 )) || { echo "ERRO: RC_MONITOR_HEALTH_ATTEMPTS deve ser 1..300." >&2; exit 64; }
[[ "$HEALTH_DELAY" =~ ^[0-9]+$ ]] && (( HEALTH_DELAY >= 1 && HEALTH_DELAY <= 60 )) || { echo "ERRO: RC_MONITOR_HEALTH_DELAY_SECONDS deve ser 1..60." >&2; exit 64; }

CONFIG_SOURCE="$(realpath "$1")"
ENV_SOURCE="${2:-}"
[[ -f "$CONFIG_SOURCE" ]] || { echo "ERRO: configuração ausente: $CONFIG_SOURCE" >&2; exit 2; }
if [[ -n "$ENV_SOURCE" ]]; then
  ENV_SOURCE="$(realpath "$ENV_SOURCE")"
  [[ -f "$ENV_SOURCE" ]] || { echo "ERRO: arquivo de ambiente ausente: $ENV_SOURCE" >&2; exit 2; }
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PKG_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MONITOR_BIN="$PKG_ROOT/bin/rc-monitor"
UNIT_SOURCE="$PKG_ROOT/systemd/rc-monitor.service"
[[ -x "$MONITOR_BIN" ]] || { echo "ERRO: rc-monitor ausente ou não executável em $MONITOR_BIN" >&2; exit 4; }
[[ -f "$UNIT_SOURCE" ]] || { echo "ERRO: unit rc-monitor.service ausente." >&2; exit 4; }

"$MONITOR_BIN" --check-config --config "$CONFIG_SOURCE"
"$MONITOR_BIN" --version

if [[ $DRY_RUN -eq 1 ]]; then
  echo "DRY-RUN OK: rc-monitor config=$CONFIG_SOURCE"
  exit 0
fi

[[ $EUID -eq 0 ]] || { echo "ERRO: instalação exige root." >&2; exit 77; }
command -v systemctl >/dev/null || { echo "ERRO: systemctl não encontrado." >&2; exit 69; }
command -v curl >/dev/null || { echo "ERRO: curl não encontrado." >&2; exit 69; }
[[ -x "$ROOT/current/bin/rc-monitor" ]] || { echo "ERRO: $ROOT/current/bin/rc-monitor ausente; instale primeiro a release RC." >&2; exit 4; }

getent group rc-monitor >/dev/null || groupadd --system rc-monitor
if ! id rc-monitor >/dev/null 2>&1; then
  useradd --system --gid rc-monitor --home-dir /nonexistent --shell /usr/sbin/nologin rc-monitor
fi

install -o root -g rc-monitor -m 0640 "$CONFIG_SOURCE" "$CONFIG_TARGET"
if [[ -n "$ENV_SOURCE" ]]; then
  install -o root -g root -m 0600 "$ENV_SOURCE" "$ENV_TARGET"
elif [[ ! -f "$ENV_TARGET" ]]; then
  install -o root -g root -m 0600 /dev/null "$ENV_TARGET"
fi
install -o root -g root -m 0644 "$UNIT_SOURCE" "$UNIT_TARGET"

systemctl daemon-reload
systemctl enable "$SERVICE" >/dev/null
systemctl restart "$SERVICE"

healthy=0
for ((i=1; i<=HEALTH_ATTEMPTS; i++)); do
  if systemctl is-active --quiet "$SERVICE" && curl -fsS --max-time 2 "$HEALTH_URL" >/dev/null; then
    healthy=1
    break
  fi
  sleep "$HEALTH_DELAY"
done

if [[ $healthy -ne 1 ]]; then
  echo "ERRO: rc-monitor não ficou ready em $HEALTH_URL" >&2
  systemctl --no-pager --full status "$SERVICE" >&2 || true
  exit 1
fi

echo "INSTALL RC MONITOR OK: config=$CONFIG_TARGET health=$HEALTH_URL"

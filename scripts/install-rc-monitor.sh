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
  cat <<'EOF'
Uso:
  install-rc-monitor.sh [--dry-run] CONFIG.json [ENV_FILE]

Opções:
  --dry-run   Valida binário/configuração sem alterar o sistema.
  -h, --help  Mostra esta ajuda e encerra com sucesso.

CONFIG.json deve usar caminhos absolutos para profileDir/rapidBinding quando
provider=rapid-web, pois a configuração instalada é movida para /etc.
EOF
  exit "${1:-64}"
}

DRY_RUN=0
case "${1:-}" in
  -h|--help)
    usage 0
    ;;
  --dry-run)
    DRY_RUN=1
    shift
    ;;
esac
[[ $# -ge 1 && $# -le 2 ]] || usage 64
[[ "$HEALTH_ATTEMPTS" =~ ^[0-9]+$ ]] && (( HEALTH_ATTEMPTS >= 1 && HEALTH_ATTEMPTS <= 300 )) || { echo "ERRO: RC_MONITOR_HEALTH_ATTEMPTS deve ser 1..300." >&2; exit 64; }
[[ "$HEALTH_DELAY" =~ ^[0-9]+$ ]] && (( HEALTH_DELAY >= 1 && HEALTH_DELAY <= 60 )) || { echo "ERRO: RC_MONITOR_HEALTH_DELAY_SECONDS deve ser 1..60." >&2; exit 64; }

CONFIG_SOURCE="$(realpath -- "$1")"
ENV_SOURCE="${2:-}"
[[ -f "$CONFIG_SOURCE" ]] || { echo "ERRO: configuração ausente: $CONFIG_SOURCE" >&2; exit 2; }
if [[ -n "$ENV_SOURCE" ]]; then
  ENV_SOURCE="$(realpath -- "$ENV_SOURCE")"
  [[ -f "$ENV_SOURCE" ]] || { echo "ERRO: arquivo de ambiente ausente: $ENV_SOURCE" >&2; exit 2; }
fi

for cmd in python3; do command -v "$cmd" >/dev/null 2>&1 || { echo "ERRO: comando obrigatório ausente: $cmd" >&2; exit 69; }; done

# A configuração instalada é movida para /etc. Caminhos relativos que eram válidos
# junto do arquivo fonte mudariam de significado após a cópia. Recuse-os antes de
# alterar o serviço em vez de instalar uma configuração que só falha no restart.
python3 - "$CONFIG_SOURCE" <<'PY'
import json
import os
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as f:
    cfg = json.load(f)
if cfg.get("provider") == "rapid-web":
    for i, gen in enumerate(cfg.get("generators") or []):
        for key in ("profileDir", "rapidBinding"):
            value = gen.get(key)
            if value and not os.path.isabs(value):
                raise SystemExit(f"ERRO: generators[{i}].{key} deve ser absoluto para instalação em /etc: {value}")
PY

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

backup_dir="$(mktemp -d /var/lib/.rc-monitor-install.XXXXXX)"
cleanup(){ rm -rf "$backup_dir"; }
trap cleanup EXIT
had_config=0; had_env=0; had_unit=0
[[ -f "$CONFIG_TARGET" ]] && { cp --preserve=mode,ownership,timestamps "$CONFIG_TARGET" "$backup_dir/config"; had_config=1; }
[[ -f "$ENV_TARGET" ]] && { cp --preserve=mode,ownership,timestamps "$ENV_TARGET" "$backup_dir/env"; had_env=1; }
[[ -f "$UNIT_TARGET" ]] && { cp --preserve=mode,ownership,timestamps "$UNIT_TARGET" "$backup_dir/unit"; had_unit=1; }

rollback(){
  echo "ERRO: nova configuração do RC Monitor não ficou ready; restaurando estado anterior." >&2
  if [[ $had_config -eq 1 ]]; then cp --preserve=mode,ownership,timestamps "$backup_dir/config" "$CONFIG_TARGET"; else rm -f "$CONFIG_TARGET"; fi
  if [[ $had_env -eq 1 ]]; then cp --preserve=mode,ownership,timestamps "$backup_dir/env" "$ENV_TARGET"; else rm -f "$ENV_TARGET"; fi
  if [[ $had_unit -eq 1 ]]; then cp --preserve=mode,ownership,timestamps "$backup_dir/unit" "$UNIT_TARGET"; else rm -f "$UNIT_TARGET"; fi
  systemctl daemon-reload || true
  if [[ $had_config -eq 1 && $had_unit -eq 1 ]]; then
    systemctl restart "$SERVICE" || true
  else
    systemctl stop "$SERVICE" || true
  fi
}
trap rollback ERR

config_candidate="$(mktemp /etc/.rc-monitor-config.XXXXXX)"
install -o root -g rc-monitor -m 0640 "$CONFIG_SOURCE" "$config_candidate"
"$ROOT/current/bin/rc-monitor" --check-config --config "$config_candidate"
mv -f "$config_candidate" "$CONFIG_TARGET"

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
  systemctl --no-pager --full status "$SERVICE" >&2 || true
  false
fi

trap - ERR
echo "INSTALL RC MONITOR OK: config=$CONFIG_TARGET health=$HEALTH_URL"

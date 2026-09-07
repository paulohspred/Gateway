#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_DIR="$SCRIPT_DIR"
RAPID_ENV=""
FRONTEND_BIND="127.0.0.1:80"
ALLOW_UPGRADE=0

usage(){
  cat >&2 <<'USAGE'
Uso:
  sudo ./install-rc-lab-stack.sh --dir KIT_DIR --rapid-env ENV_FILE [opções]

Opções:
  --dir DIR                    Kit com release RC, checksum, Rapid SCADA e install-scada-stack.sh.
  --rapid-env FILE             Arquivo root-only com RC_RAPID_USER e RC_RAPID_PASSWORD.
  --frontend-bind ENDPOINT     127.0.0.1:80 (padrão) ou 0.0.0.0:80 para laboratório em LAN.
  --upgrade                    Permite rerun/upgrade do instalador-base.
  -h, --help                   Ajuda.

Este instalador é SOMENTE para laboratório/simulação. Ele não configura controladora
física, não habilita escrita industrial e mantém AllowCommandApi=false.
USAGE
  exit "${1:-64}"
}

while (($#)); do
  case "$1" in
    --dir) [[ $# -ge 2 ]] || usage; SOURCE_DIR="$2"; shift 2 ;;
    --rapid-env) [[ $# -ge 2 ]] || usage; RAPID_ENV="$2"; shift 2 ;;
    --frontend-bind) [[ $# -ge 2 ]] || usage; FRONTEND_BIND="$2"; shift 2 ;;
    --upgrade) ALLOW_UPGRADE=1; shift ;;
    -h|--help) usage 0 ;;
    *) echo "ERRO: opção desconhecida: $1" >&2; usage ;;
  esac
done

[[ $EUID -eq 0 ]] || { echo "ERRO: instalação exige root." >&2; exit 77; }
SOURCE_DIR="$(realpath "$SOURCE_DIR")"
RAPID_ENV="$(realpath "$RAPID_ENV")"
[[ -d "$SOURCE_DIR" ]] || { echo "ERRO: kit ausente: $SOURCE_DIR" >&2; exit 2; }
[[ -f "$RAPID_ENV" ]] || { echo "ERRO: arquivo Rapid env ausente: $RAPID_ENV" >&2; exit 2; }
case "$FRONTEND_BIND" in 127.0.0.1:80|0.0.0.0:80) ;; *) echo "ERRO: frontend bind inválido" >&2; exit 64 ;; esac

mode="$(stat -c '%a' "$RAPID_ENV")"
case "$mode" in 600|400) ;; *) echo "ERRO: $RAPID_ENV deve ter modo 0600 ou 0400; atual=$mode" >&2; exit 4 ;; esac
for name in RC_RAPID_USER RC_RAPID_PASSWORD; do
  count="$(grep -Ec "^${name}=.+$" "$RAPID_ENV" || true)"
  [[ "$count" == "1" ]] || { echo "ERRO: $RAPID_ENV deve conter exatamente uma variável $name não vazia" >&2; exit 4; }
done

STACK_INSTALLER="$SOURCE_DIR/install-scada-stack.sh"
[[ -x "$STACK_INSTALLER" || -f "$STACK_INSTALLER" ]] || { echo "ERRO: install-scada-stack.sh ausente no kit" >&2; exit 2; }

stack_args=(--dir "$SOURCE_DIR")
[[ $ALLOW_UPGRADE -eq 1 ]] && stack_args+=(--upgrade)
bash "$STACK_INSTALLER" "${stack_args[@]}"

CURRENT="/opt/rc-gateway/current"
for file in \
  "$CURRENT/scripts/configure-rapid-web-api.sh" \
  "$CURRENT/scripts/install-rc-monitor.sh" \
  "$CURRENT/scripts/install-rc-frontend.sh" \
  "$CURRENT/scripts/rc-frontend-acceptance.sh" \
  "$CURRENT/configs/monitor/rc-monitor.rapid-demo-lab.json"
do
  [[ -f "$file" ]] || { echo "ERRO: release instalada não contém $file" >&2; exit 4; }
done

bash "$CURRENT/scripts/configure-rapid-web-api.sh"
bash "$CURRENT/scripts/install-rc-monitor.sh" "$CURRENT/configs/monitor/rc-monitor.rapid-demo-lab.json" "$RAPID_ENV"
RC_FRONTEND_BIND="$FRONTEND_BIND" bash "$CURRENT/scripts/install-rc-frontend.sh"
bash "$CURRENT/scripts/rc-frontend-acceptance.sh"

install -d -o root -g root -m 0750 /var/lib/rc-scada-stack
cat > /var/lib/rc-scada-stack/lab-mode.env <<EOF
mode=SIMULATION_TEST_ONLY
installed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
frontend_bind=$FRONTEND_BIND
rapid_provider=rapid-web
rapid_commands=false
physical_controller=false
production_validated=false
EOF
chmod 0640 /var/lib/rc-scada-stack/lab-mode.env

echo "RC LAB STACK INSTALL PASSED"
echo "mode=SIMULATION_TEST_ONLY rapid_commands=false physical_controller=false production_validated=false"

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
O pacote Rapid SCADA deve possuir sidecar .sha256 no kit para impedir instalação
acidental de um arquivo diferente do artifact previamente congelado.
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

mapfile -t rapid_sources < <(find "$SOURCE_DIR" -maxdepth 1 -type f \( -name 'rapidscada_*_all.deb' -o -name 'rapidscada_*_linux*.zip' \) -print | sort)
[[ ${#rapid_sources[@]} -eq 1 ]] || { echo "ERRO: kit LAB deve conter exatamente um pacote Rapid SCADA; encontrados ${#rapid_sources[@]}" >&2; exit 2; }
RAPID_SOURCE="${rapid_sources[0]}"
RAPID_CHECKSUM="$RAPID_SOURCE.sha256"
[[ -f "$RAPID_CHECKSUM" ]] || { echo "ERRO: checksum Rapid obrigatório no LAB: $RAPID_CHECKSUM" >&2; exit 3; }
expected_rapid_sha="$(awk 'NF {print $1; exit}' "$RAPID_CHECKSUM")"
actual_rapid_sha="$(sha256sum "$RAPID_SOURCE" | awk '{print $1}')"
[[ "$expected_rapid_sha" =~ ^[0-9a-fA-F]{64}$ ]] || { echo "ERRO: checksum Rapid inválido: $RAPID_CHECKSUM" >&2; exit 3; }
[[ "${expected_rapid_sha,,}" == "${actual_rapid_sha,,}" ]] || { echo "ERRO: SHA256 do Rapid SCADA não confere" >&2; exit 3; }

STACK_INSTALLER="$SOURCE_DIR/install-scada-stack.sh"
[[ -x "$STACK_INSTALLER" || -f "$STACK_INSTALLER" ]] || { echo "ERRO: install-scada-stack.sh ausente no kit" >&2; exit 2; }

stack_args=(--dir "$SOURCE_DIR")
[[ $ALLOW_UPGRADE -eq 1 ]] && stack_args+=(--upgrade)
bash "$STACK_INSTALLER" "${stack_args[@]}"

CURRENT="/opt/rc-gateway/current"
for file in \
  "$CURRENT/scripts/configure-rapid-web-api.sh" \
  "$CURRENT/scripts/install-rc-monitor.sh" \
  "$CURRENT/scripts/install-rc-admin.sh" \
  "$CURRENT/scripts/install-rc-frontend.sh" \
  "$CURRENT/scripts/rc-frontend-acceptance.sh" \
  "$CURRENT/scripts/rc-admin-acceptance.sh" \
  "$CURRENT/configs/monitor/rc-monitor.rapid-demo-lab.json"
do
  [[ -f "$file" ]] || { echo "ERRO: release instalada não contém $file" >&2; exit 4; }
done

bash "$CURRENT/scripts/configure-rapid-web-api.sh"
bash "$CURRENT/scripts/install-rc-monitor.sh" "$CURRENT/configs/monitor/rc-monitor.rapid-demo-lab.json" "$RAPID_ENV"
if [[ -f /var/lib/rc-admin/state.json ]]; then RC_ADMIN_COOKIE_SECURE=false bash "$CURRENT/scripts/install-rc-admin.sh"; else RC_ADMIN_COOKIE_SECURE=false bash "$CURRENT/scripts/install-rc-admin.sh" --lab-generate-bootstrap; fi
RC_FRONTEND_BIND="$FRONTEND_BIND" bash "$CURRENT/scripts/install-rc-frontend.sh"
bash "$CURRENT/scripts/rc-frontend-acceptance.sh"
bash "$CURRENT/scripts/rc-admin-acceptance.sh"

install -d -o root -g root -m 0750 /var/lib/rc-scada-stack
cat > /var/lib/rc-scada-stack/lab-mode.env <<EOF
mode=SIMULATION_TEST_ONLY
installed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
frontend_bind=$FRONTEND_BIND
rapid_provider=rapid-web
rapid_source=$(basename "$RAPID_SOURCE")
rapid_source_sha256=$actual_rapid_sha
rapid_commands=false
physical_controller=false
production_validated=false
EOF
chmod 0640 /var/lib/rc-scada-stack/lab-mode.env

echo "RC LAB STACK INSTALL PASSED"
echo "mode=SIMULATION_TEST_ONLY rapid_commands=false physical_controller=false production_validated=false"

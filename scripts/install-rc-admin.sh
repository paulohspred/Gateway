#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="${RC_GATEWAY_ROOT:-/opt/rc-gateway}"
CURRENT="$ROOT/current"
UNIT_TARGET="/etc/systemd/system/rc-admin.service"
ENV_TARGET="/etc/rc-admin.env"
STATE_FILE="/var/lib/rc-admin/state.json"
HEALTH="http://127.0.0.1:18110/healthz"
BOOTSTRAP_ENV=""
LAB_GENERATE=0
DRY_RUN=0

usage(){ cat >&2 <<'USAGE'
Uso: install-rc-admin.sh [--bootstrap-env FILE | --lab-generate-bootstrap] [--dry-run]

Primeira instalação exige credenciais bootstrap explícitas. Em laboratório descartável,
--lab-generate-bootstrap cria senha aleatória e grava uma cópia root-only em
/root/rc-admin-bootstrap.txt. Depois do primeiro start, a senha é removida de
/etc/rc-admin.env; o hash permanece apenas no state privado do rc-admin.
USAGE
exit "${1:-64}"; }

while (($#)); do case "$1" in
  --bootstrap-env) [[ $# -ge 2 ]] || usage; BOOTSTRAP_ENV="$2"; shift 2;;
  --lab-generate-bootstrap) LAB_GENERATE=1; shift;;
  --dry-run) DRY_RUN=1; shift;;
  -h|--help) usage 0;;
  *) echo "ERRO: opção desconhecida: $1" >&2; usage;;
esac; done
[[ ! ( -n "$BOOTSTRAP_ENV" && $LAB_GENERATE -eq 1 ) ]] || { echo "ERRO: escolha bootstrap-env OU lab-generate-bootstrap" >&2; exit 64; }
[[ -x "$CURRENT/bin/rc-admin" ]] || { echo "ERRO: rc-admin ausente/inexecutável em $CURRENT/bin" >&2; exit 4; }
[[ -f "$CURRENT/systemd/rc-admin.service" ]] || { echo "ERRO: unit rc-admin ausente" >&2; exit 4; }
"$CURRENT/bin/rc-admin" --check-config --bind 127.0.0.1:18110 --state "$STATE_FILE"
if [[ $DRY_RUN -eq 1 ]]; then echo "DRY-RUN OK: rc-admin loopback=127.0.0.1:18110 state=$STATE_FILE"; exit 0; fi
[[ $EUID -eq 0 ]] || { echo "ERRO: instalação exige root" >&2; exit 77; }

getent group rc-admin >/dev/null || groupadd --system rc-admin
id rc-admin >/dev/null 2>&1 || useradd --system --gid rc-admin --home-dir /nonexistent --shell /usr/sbin/nologin rc-admin
install -d -o rc-admin -g rc-admin -m 0700 /var/lib/rc-admin

first_install=0
[[ -f "$STATE_FILE" ]] || first_install=1
bootstrap_user=""; bootstrap_password=""; cookie_secure="${RC_ADMIN_COOKIE_SECURE:-true}"
if [[ $first_install -eq 1 ]]; then
  if [[ -n "$BOOTSTRAP_ENV" ]]; then
    BOOTSTRAP_ENV="$(realpath "$BOOTSTRAP_ENV")"; [[ -f "$BOOTSTRAP_ENV" ]] || { echo "ERRO: bootstrap env ausente" >&2; exit 2; }
    mode="$(stat -c '%a' "$BOOTSTRAP_ENV")"; case "$mode" in 400|600) ;; *) echo "ERRO: bootstrap env deve ser 0400/0600" >&2; exit 4;; esac
    bootstrap_user="$(sed -n 's/^RC_ADMIN_BOOTSTRAP_USER=//p' "$BOOTSTRAP_ENV" | tail -1)"
    bootstrap_password="$(sed -n 's/^RC_ADMIN_BOOTSTRAP_PASSWORD=//p' "$BOOTSTRAP_ENV" | tail -1)"
    configured_cookie="$(sed -n 's/^RC_ADMIN_COOKIE_SECURE=//p' "$BOOTSTRAP_ENV" | tail -1)"; [[ -z "$configured_cookie" ]] || cookie_secure="$configured_cookie"
  elif [[ $LAB_GENERATE -eq 1 ]]; then
    bootstrap_user="admin"
    bootstrap_password="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')"
    cat > /root/rc-admin-bootstrap.txt <<EOF
RC_ADMIN_BOOTSTRAP_USER=$bootstrap_user
RC_ADMIN_BOOTSTRAP_PASSWORD=$bootstrap_password
Generated for LAB only. Change password at first login and enroll MFA.
EOF
    chmod 0600 /root/rc-admin-bootstrap.txt
  else
    echo "ERRO: primeira instalação exige --bootstrap-env ou --lab-generate-bootstrap" >&2; exit 4
  fi
  [[ -n "$bootstrap_user" && ${#bootstrap_password} -ge 12 ]] || { echo "ERRO: bootstrap inválido" >&2; exit 4; }
fi

install -o root -g root -m 0644 "$CURRENT/systemd/rc-admin.service" "$UNIT_TARGET"
if [[ $first_install -eq 1 ]]; then
  tmp="$(mktemp /etc/.rc-admin-env.XXXXXX)"; trap 'rm -f "$tmp"' EXIT
  printf 'RC_ADMIN_BOOTSTRAP_USER=%s\nRC_ADMIN_BOOTSTRAP_PASSWORD=%s\nRC_ADMIN_COOKIE_SECURE=%s\n' "$bootstrap_user" "$bootstrap_password" "$cookie_secure" > "$tmp"
  install -o root -g rc-admin -m 0640 "$tmp" "$ENV_TARGET"
else
  printf 'RC_ADMIN_COOKIE_SECURE=%s\n' "$cookie_secure" | install -o root -g rc-admin -m 0640 /dev/stdin "$ENV_TARGET"
fi
systemctl daemon-reload
systemctl enable rc-admin.service >/dev/null
systemctl restart rc-admin.service
ready=0; for _ in $(seq 1 30); do if systemctl is-active --quiet rc-admin.service && curl -fsS --max-time 2 "$HEALTH" >/dev/null; then ready=1; break; fi; sleep 1; done
[[ $ready -eq 1 ]] || { journalctl -u rc-admin.service -n 30 --no-pager >&2 || true; echo "ERRO: rc-admin não ficou healthy" >&2; exit 5; }
# Bootstrap secrets are not needed after state creation. Remove them from service environment.
printf 'RC_ADMIN_COOKIE_SECURE=%s\n' "$cookie_secure" | install -o root -g rc-admin -m 0640 /dev/stdin "$ENV_TARGET"
systemctl restart rc-admin.service
ready=0; for _ in $(seq 1 20); do if curl -fsS --max-time 2 "$HEALTH" >/dev/null; then ready=1; break; fi; sleep 1; done
[[ $ready -eq 1 ]] || { echo "ERRO: rc-admin falhou após scrub do bootstrap" >&2; exit 5; }
echo "INSTALL RC ADMIN OK: bind=127.0.0.1:18110 state=$STATE_FILE bootstrap_scrubbed=true"
[[ $LAB_GENERATE -eq 1 && $first_install -eq 1 ]] && echo "LAB bootstrap root-only: /root/rc-admin-bootstrap.txt"

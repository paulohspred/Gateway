#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATEWAY_SERVICE="${RC_GATEWAY_SERVICE:-rc-gateway.service}"
SCADA_SERVICE="${RAPID_SCADA_COMM_SERVICE:-scadacomm6.service}"
ADMIN_URL="${RC_GATEWAY_ADMIN_URL:-http://127.0.0.1:18080}"
PORTS="${RAPID_SCADA_GATEWAY_PORTS:-}"
WAIT_SECONDS="${RAPID_SCADA_WAIT_SECONDS:-60}"
MIN_FREE_MB="${RAPID_SCADA_MIN_FREE_MB:-1024}"
MAX_RESTARTS="${RAPID_SCADA_MAX_RESTARTS:-3}"
REQUIRE_NTP="${RAPID_SCADA_REQUIRE_NTP:-1}"
REQUIRE_NON_ROOT="${RAPID_SCADA_REQUIRE_NON_ROOT:-0}"
DISRUPTIVE="${RAPID_SCADA_DISRUPTIVE:-0}"

fail() {
  echo "ERRO: $*" >&2
  exit 1
}

warn() {
  echo "AVISO: $*" >&2
}

for value_name in REQUIRE_NTP REQUIRE_NON_ROOT DISRUPTIVE; do
  value="${!value_name}"
  case "$value" in
    0|1) ;;
    *) echo "ERRO: $value_name deve ser 0 ou 1." >&2; exit 64 ;;
  esac
done

for pair in "WAIT_SECONDS:$WAIT_SECONDS:1:600" "MIN_FREE_MB:$MIN_FREE_MB:128:1048576" "MAX_RESTARTS:$MAX_RESTARTS:0:1000000"; do
  IFS=: read -r name value min max <<<"$pair"
  [[ "$value" =~ ^[0-9]+$ ]] || { echo "ERRO: $name deve ser inteiro." >&2; exit 64; }
  (( value >= min && value <= max )) || { echo "ERRO: $name deve estar entre $min e $max." >&2; exit 64; }
done

[[ -n "$PORTS" ]] || {
  echo "ERRO: defina RAPID_SCADA_GATEWAY_PORTS com as portas consumer do Gateway, por exemplo: 25020 ou '25020 25021'." >&2
  exit 64
}
PORTS="${PORTS//,/ }"

required=(curl systemctl ss awk grep df)
if [[ "$REQUIRE_NTP" == "1" ]]; then
  required+=(timedatectl)
fi
for cmd in "${required[@]}"; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "ERRO: comando obrigatório ausente: $cmd" >&2; exit 69; }
done

[[ -x "$SCRIPT_DIR/rapid-scada-acceptance.sh" ]] || fail "rapid-scada-acceptance.sh não encontrado ou sem permissão de execução."

systemctl is-active --quiet "$GATEWAY_SERVICE" || fail "$GATEWAY_SERVICE não está ativo."
systemctl is-enabled --quiet "$GATEWAY_SERVICE" || fail "$GATEWAY_SERVICE não está habilitado no boot."
systemctl is-active --quiet "$SCADA_SERVICE" || fail "$SCADA_SERVICE não está ativo."
systemctl is-enabled --quiet "$SCADA_SERVICE" || fail "$SCADA_SERVICE não está habilitado no boot."

check_restarts() {
  local service="$1"
  local restarts
  restarts="$(systemctl show "$service" -p NRestarts --value)"
  [[ "$restarts" =~ ^[0-9]+$ ]] || fail "não foi possível ler NRestarts de $service."
  (( restarts <= MAX_RESTARTS )) || fail "$service possui NRestarts=$restarts acima do limite $MAX_RESTARTS."
}

check_restarts "$GATEWAY_SERVICE"
check_restarts "$SCADA_SERVICE"

scada_user="$(systemctl show "$SCADA_SERVICE" -p User --value)"
if [[ -z "$scada_user" || "$scada_user" == "root" ]]; then
  if [[ "$REQUIRE_NON_ROOT" == "1" ]]; then
    fail "$SCADA_SERVICE está executando como root/sem User= explícito. Homologue um override non-root antes deste gate."
  else
    warn "$SCADA_SERVICE está executando como root/sem User= explícito. O unit upstream padrão não define User=. Trate isso como item de hardening antes da promoção final."
  fi
fi

for port in $PORTS; do
  [[ "$port" =~ ^[0-9]+$ ]] && (( port >= 1 && port <= 65535 )) || { echo "ERRO: porta inválida em RAPID_SCADA_GATEWAY_PORTS: $port" >&2; exit 64; }
  matches="$(ss -H -ltn | awk -v p="$port" '$4 ~ (":" p "$") {print $4}')"
  [[ -n "$matches" ]] || fail "nenhum listener TCP encontrado na porta $port."
  while IFS= read -r addr; do
    [[ -n "$addr" ]] || continue
    if [[ "$addr" != "127.0.0.1:$port" && "$addr" != "[::1]:$port" && "$addr" != "::1:$port" ]]; then
      fail "porta consumer $port exposta fora de loopback: $addr"
    fi
  done <<<"$matches"
done

if [[ "$REQUIRE_NTP" == "1" ]]; then
  ntp="$(timedatectl show -p NTPSynchronized --value 2>/dev/null || true)"
  [[ "$ntp" == "yes" ]] || fail "relógio não está sincronizado por NTP (NTPSynchronized=$ntp)."
fi

for path in /opt/rc-gateway/current /opt/scada; do
  [[ -e "$path" ]] || fail "caminho de produção ausente: $path"
  free_mb="$(df -Pm "$path" | awk 'NR==2 {print $4}')"
  [[ "$free_mb" =~ ^[0-9]+$ ]] || fail "não foi possível medir espaço livre em $path."
  (( free_mb >= MIN_FREE_MB )) || fail "espaço livre insuficiente em $path: ${free_mb} MiB < ${MIN_FREE_MB} MiB."
done

RAPID_SCADA_REQUIRE_LOCAL_SERVICE=1 \
RAPID_SCADA_EXPECT_SESSION=1 \
RAPID_SCADA_WAIT_SECONDS="$WAIT_SECONDS" \
RC_GATEWAY_SERVICE="$GATEWAY_SERVICE" \
RC_GATEWAY_ADMIN_URL="$ADMIN_URL" \
RAPID_SCADA_COMM_SERVICE="$SCADA_SERVICE" \
  "$SCRIPT_DIR/rapid-scada-acceptance.sh"

wait_active() {
  local service="$1"
  local deadline=$((SECONDS + WAIT_SECONDS))
  while (( SECONDS < deadline )); do
    if systemctl is-active --quiet "$service"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

if [[ "$DISRUPTIVE" == "1" ]]; then
  echo "Executando testes disruptivos de restart em ambiente autorizado..."

  systemctl restart "$SCADA_SERVICE"
  wait_active "$SCADA_SERVICE" || fail "$SCADA_SERVICE não recuperou após restart."
  RAPID_SCADA_REQUIRE_LOCAL_SERVICE=1 RAPID_SCADA_EXPECT_SESSION=1 RAPID_SCADA_WAIT_SECONDS="$WAIT_SECONDS" \
    RC_GATEWAY_SERVICE="$GATEWAY_SERVICE" RC_GATEWAY_ADMIN_URL="$ADMIN_URL" RAPID_SCADA_COMM_SERVICE="$SCADA_SERVICE" \
    "$SCRIPT_DIR/rapid-scada-acceptance.sh"

  systemctl restart "$GATEWAY_SERVICE"
  wait_active "$GATEWAY_SERVICE" || fail "$GATEWAY_SERVICE não recuperou após restart."
  RAPID_SCADA_REQUIRE_LOCAL_SERVICE=1 RAPID_SCADA_EXPECT_SESSION=1 RAPID_SCADA_WAIT_SECONDS="$WAIT_SECONDS" \
    RC_GATEWAY_SERVICE="$GATEWAY_SERVICE" RC_GATEWAY_ADMIN_URL="$ADMIN_URL" RAPID_SCADA_COMM_SERVICE="$SCADA_SERVICE" \
    "$SCRIPT_DIR/rapid-scada-acceptance.sh"
fi

printf 'Rapid SCADA production preflight OK\n'
printf 'Gateway service: %s\n' "$GATEWAY_SERVICE"
printf 'Rapid SCADA Communicator: %s\n' "$SCADA_SERVICE"
printf 'Consumer loopback ports: %s\n' "$PORTS"
printf 'NTP required: %s\n' "$REQUIRE_NTP"
printf 'Non-root Rapid SCADA required: %s\n' "$REQUIRE_NON_ROOT"
printf 'Disruptive restart gate executed: %s\n' "$DISRUPTIVE"
printf 'Observação: este preflight não substitui HIL com gerador/controladora real nem o soak de produção.\n'

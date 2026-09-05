#!/usr/bin/env bash
set -Eeuo pipefail

SERVICE="rc-gateway.service"
ROOT="/opt/rc-gateway"
CONFIG="/etc/rc-gateway.json"
HEALTH_URL="${RC_GATEWAY_HEALTH_URL:-http://127.0.0.1:18080/readyz}"
HEALTH_ATTEMPTS="${RC_GATEWAY_HEALTH_ATTEMPTS:-30}"
HEALTH_DELAY="${RC_GATEWAY_HEALTH_DELAY_SECONDS:-1}"

[[ $EUID -eq 0 ]] || { echo "ERRO: rollback exige root." >&2; exit 77; }
[[ "$HEALTH_ATTEMPTS" =~ ^[0-9]+$ ]] && (( HEALTH_ATTEMPTS >= 1 && HEALTH_ATTEMPTS <= 300 )) || { echo "ERRO: RC_GATEWAY_HEALTH_ATTEMPTS deve ser 1..300." >&2; exit 64; }
[[ "$HEALTH_DELAY" =~ ^[0-9]+$ ]] && (( HEALTH_DELAY >= 1 && HEALTH_DELAY <= 60 )) || { echo "ERRO: RC_GATEWAY_HEALTH_DELAY_SECONDS deve ser 1..60." >&2; exit 64; }
[[ -L "$ROOT/current" && -L "$ROOT/previous" ]] || { echo "ERRO: current/previous não disponíveis." >&2; exit 2; }
[[ -f "$CONFIG" ]] || { echo "ERRO: configuração ausente: $CONFIG" >&2; exit 2; }

current="$(readlink -f "$ROOT/current")"
previous="$(readlink -f "$ROOT/previous")"
[[ -d "$current" && -d "$previous" && "$current" != "$previous" ]] || { echo "ERRO: release anterior inválida/igual à atual." >&2; exit 2; }
[[ -x "$previous/bin/rc-gateway" && -f "$previous/MANIFEST" ]] || { echo "ERRO: release anterior incompleta." >&2; exit 2; }
grep -qx 'product=rc-gateway' "$previous/MANIFEST" || { echo "ERRO: release anterior não pertence ao produto rc-gateway." >&2; exit 2; }
"$previous/bin/rc-gateway" --check-config --config "$CONFIG"

ln -sfn "$previous" "$ROOT/current.rollback"
mv -Tf "$ROOT/current.rollback" "$ROOT/current"
systemctl daemon-reload
if ! systemctl restart "$SERVICE"; then
  ln -sfn "$current" "$ROOT/current.revert"
  mv -Tf "$ROOT/current.revert" "$ROOT/current"
  systemctl restart "$SERVICE" || true
  exit 1
fi

healthy=0
for ((i=1; i<=HEALTH_ATTEMPTS; i++)); do
  if systemctl is-active --quiet "$SERVICE" && curl -fsS --max-time 2 "$HEALTH_URL" >/dev/null; then
    healthy=1
    break
  fi
  sleep "$HEALTH_DELAY"
done
if [[ $healthy -ne 1 ]]; then
  echo "ERRO: rollback não ficou saudável; restaurando release original." >&2
  ln -sfn "$current" "$ROOT/current.revert"
  mv -Tf "$ROOT/current.revert" "$ROOT/current"
  systemctl restart "$SERVICE" || true
  exit 1
fi

ln -sfn "$current" "$ROOT/previous.new"
mv -Tf "$ROOT/previous.new" "$ROOT/previous"
echo "ROLLBACK OK: current=$previous previous=$current"

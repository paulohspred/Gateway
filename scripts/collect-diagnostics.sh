#!/usr/bin/env bash
set -Eeuo pipefail

SERVICE="${RC_GATEWAY_SERVICE:-rc-gateway.service}"
ROOT="${RC_GATEWAY_ROOT:-/opt/rc-gateway}"
ADMIN_URL="${RC_GATEWAY_ADMIN_URL:-http://127.0.0.1:18080}"
OUTPUT="${1:-rc-gateway-diagnostics-$(date -u +%Y%m%dT%H%M%SZ).tar.gz}"

umask 077
tmp="$(mktemp -d)"
cleanup(){ rm -rf "$tmp"; }
trap cleanup EXIT

capture(){
  local name="$1"; shift
  {
    printf '$'
    printf ' %q' "$@"
    printf '\n\n'
    "$@"
  } >"$tmp/$name.txt" 2>&1 || true
}

cat >"$tmp/README.txt" <<'EOF'
RC Universal Gateway diagnostics bundle.

This bundle intentionally does NOT copy the gateway configuration, TLS keys,
VPN credentials or /sessions payload. It may still contain operationally
sensitive IP addresses, interface names, device identifiers and journal lines.
Review and sanitize it before sharing outside the trusted support channel.
EOF

if [[ -x "$ROOT/current/bin/rc-gateway" ]]; then
  capture version "$ROOT/current/bin/rc-gateway" --version
else
  capture version sh -c 'command -v rc-gateway && rc-gateway --version'
fi
capture uname uname -a
capture os-release cat /etc/os-release
capture service-status systemctl status "$SERVICE" --no-pager
capture service-properties systemctl show "$SERVICE" -p ActiveState -p SubState -p MainPID -p User -p Group -p NRestarts -p WatchdogUSec -p MemoryCurrent -p TasksCurrent
capture service-unit systemctl cat "$SERVICE"
capture journal journalctl -u "$SERVICE" -n 500 --no-pager --output=short-iso
capture status curl -fsS --max-time 3 "$ADMIN_URL/status"
capture readyz curl -fsS --max-time 3 "$ADMIN_URL/readyz"
capture metrics curl -fsS --max-time 3 "$ADMIN_URL/metrics"
capture sockets ss -lntup
capture ip-address ip -brief address
capture ip-route ip route show table all
capture links ip -details link show
capture runtime-dir ls -la /run/rc-gateway
capture serial-devices sh -c 'ls -l /dev/ttyUSB* /dev/ttyACM* /dev/ttyS* 2>/dev/null || true'
capture hid-devices sh -c 'ls -l /dev/hidraw* 2>/dev/null || true'
capture can-links sh -c 'ip -details link show type can 2>/dev/null || true'

tar -C "$tmp" -czf "$OUTPUT" .
printf 'Diagnostics bundle created: %s\n' "$OUTPUT"
printf 'Review README.txt and sanitize operational metadata before sharing.\n'

#!/usr/bin/env bash
set -Eeuo pipefail

usage(){ echo "Uso: $0 ARCHIVE.tar.gz ARCHIVE.tar.gz.sha256 [GATEWAY_CONFIG.json]" >&2; exit 64; }
[[ $# -ge 2 && $# -le 3 ]] || usage

ARCHIVE="$(realpath "$1")"
CHECKSUM="$(realpath "$2")"
CONFIG="${3:-configs/gateway.example.json}"
CONFIG="$(realpath "$CONFIG")"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

[[ -f "$ARCHIVE" && -f "$CHECKSUM" && -f "$CONFIG" ]] || { echo "ERRO: archive/checksum/config ausente" >&2; exit 2; }
for cmd in tar grep sha256sum mktemp; do command -v "$cmd" >/dev/null 2>&1 || { echo "ERRO: comando ausente: $cmd" >&2; exit 69; }; done

expected="$(awk 'NF {print $1; exit}' "$CHECKSUM")"
actual="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
[[ "$expected" =~ ^[0-9a-fA-F]{64}$ && "${expected,,}" == "${actual,,}" ]] || { echo "ERRO: checksum da release inválido" >&2; exit 3; }

tmp="$(mktemp -d)"
cleanup(){ rm -rf "$tmp"; }
trap cleanup EXIT
listing="$tmp/listing.txt"
tar -tzf "$ARCHIVE" > "$listing"

require_entry(){
  local pattern="$1" label="$2"
  grep -Eq "$pattern" "$listing" || { echo "ERRO: release ausente: $label" >&2; exit 4; }
}

require_entry '/sbom\.cdx\.json$' 'SBOM'
require_entry '/bin/rc-gateway$' 'rc-gateway'
require_entry '/bin/rc-monitor$' 'rc-monitor'
require_entry '/bin/rc-admin$' 'rc-admin'
require_entry '/frontend/index\.html$' 'frontend/index.html'
require_entry '/frontend/assets/index-[A-Za-z0-9_-]+\.js$' 'frontend hashed JavaScript'
require_entry '/systemd/rc-gateway\.service$' 'rc-gateway.service'
require_entry '/systemd/rc-monitor\.service$' 'rc-monitor.service'
require_entry '/systemd/rc-admin\.service$' 'rc-admin.service'
for script in \
  install-release.sh install-scada-stack.sh install-rc-monitor.sh install-rc-admin.sh install-rc-frontend.sh \
  install-rc-lab-stack.sh configure-rapid-web-api.sh rc-frontend-acceptance.sh rc-admin-acceptance.sh rollback-release.sh \
  probe-usb-hid.sh collect-diagnostics.sh vm-acceptance.sh rapid-scada-acceptance.sh \
  rapid-scada-production-acceptance.sh
do
  require_entry "/scripts/${script//./\\.}$" "scripts/$script"
done
for doc in \
  USB_HID_COMAP.md RUNBOOK.md COMPATIBILITY_MATRIX.md RAPID_SCADA_INTEGRATION.md \
  GENERATOR_SCADA_PRODUCTION_READINESS.md SCADA_STACK_INSTALLER.md VM_ACCEPTANCE.md \
  THREAT_MODEL.md CONFIGURATION_COMPATIBILITY.md
do
  require_entry "/docs/${doc//./\\.}$" "docs/$doc"
done
for config in \
  rapid-scada.modbus-tcp.example.json rapid-scada.rtu-over-tcp.example.json \
  rapid-scada.rs485-multidrop.example.json scada-stack.safe.example.json
do
  require_entry "/configs/${config//./\\.}$" "configs/$config"
done
require_entry '/configs/monitor/rc-monitor\.rapid-demo-lab\.json$' 'Rapid demo lab monitor config'
require_entry '/controllers/rc-simulator/reference-controller/rapid/channels\.rapid-demo-lab\.json$' 'Rapid demo lab semantic binding'
for root_file in README.md LICENSE NOTICE THIRD_PARTY_NOTICES.md; do
  require_entry "/${root_file//./\\.}$" "$root_file"
done

if grep -q 'rc-gateway-umbrella' "$listing"; then
  echo "ERRO: legacy umbrella branding leaked into canonical release" >&2
  exit 4
fi

manifest_entry="$(grep -E '/MANIFEST$' "$listing")"
[[ "$(wc -l <<<"$manifest_entry")" == "1" ]] || { echo "ERRO: MANIFEST não é único" >&2; exit 4; }
tar -xOzf "$ARCHIVE" "$manifest_entry" > "$tmp/MANIFEST"
for line in \
  'product=rc-gateway' \
  'component=rc-monitor' \
  'component=rc-monitor-frontend' \
  'component=rc-admin' \
  'license=Proprietary-All-Rights-Reserved'
do
  grep -Fqx "$line" "$tmp/MANIFEST" || { echo "ERRO: MANIFEST ausente: $line" >&2; exit 4; }
done

grep -Eq '^node=[0-9]+\.[0-9]+\.[0-9]+$' "$tmp/MANIFEST" || { echo "ERRO: MANIFEST não registra Node exato" >&2; exit 4; }

bash "$ROOT_DIR/scripts/install-release.sh" --dry-run "$ARCHIVE" "$CHECKSUM" "$CONFIG"
echo "RELEASE ARCHIVE VALIDATION PASSED: sha256=$actual"

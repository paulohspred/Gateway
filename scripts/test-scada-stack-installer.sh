#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALLER="$ROOT_DIR/scripts/install-scada-stack.sh"
SAFE_CONFIG="$ROOT_DIR/configs/scada-stack.safe.example.json"

usage(){ echo "Uso: $0 GATEWAY.tar.gz GATEWAY.tar.gz.sha256" >&2; exit 64; }
[[ $# -eq 2 ]] || usage
ARCHIVE="$(realpath "$1")"
CHECKSUM="$(realpath "$2")"
[[ -f "$ARCHIVE" && -f "$CHECKSUM" ]] || { echo "archive/checksum ausente" >&2; exit 2; }
for cmd in dpkg-deb sha256sum python3; do command -v "$cmd" >/dev/null || { echo "$cmd ausente" >&2; exit 69; }; done

# Contract check for the production hardening learned in VM acceptance:
# Rapid SCADA upstream listens on 10000/10002 on all interfaces, so the stack
# installer must provision a persistent nftables gate that allows those ports
# only through loopback.
grep -q 'apt_install ca-certificates curl nginx unzip nftables' "$INSTALLER"
grep -q 'rc-scada-internal-firewall.service' "$INSTALLER"
grep -q 'ip daddr != 127.0.0.0/8 tcp dport { 10000, 10002 }' "$INSTALLER"
grep -q 'ip6 daddr != ::1 tcp dport { 10000, 10002 }' "$INSTALLER"
echo "stack installer Rapid internal-port firewall contract: OK"

tmp="$(mktemp -d)"
cleanup(){ rm -rf "$tmp"; }
trap cleanup EXIT

build_rapid_deb(){
  local out="$1" version="$2" package="${3:-rapidscada}"
  local pkgdir="$tmp/pkg-${version}-${package//[^A-Za-z0-9]/_}"
  rm -rf "$pkgdir"
  mkdir -p "$pkgdir/DEBIAN"
  cat > "$pkgdir/DEBIAN/control" <<EOFCONTROL
Package: $package
Version: $version
Architecture: all
Maintainer: CI Fixture <ci@example.invalid>
Description: Synthetic package used only to validate the stack installer parser.
EOFCONTROL
  dpkg-deb --build "$pkgdir" "$out" >/dev/null
}

build_rapid_zip(){
  local zip_out="$1" deb_in="$2"
  python3 - "$zip_out" "$deb_in" <<'PY'
import os
import sys
import zipfile
zip_out, deb_in = sys.argv[1:]
with zipfile.ZipFile(zip_out, "w", compression=zipfile.ZIP_DEFLATED) as zf:
    zf.write(deb_in, arcname=os.path.basename(deb_in))
PY
}

make_kit(){
  local dir="$1"
  rm -rf "$dir"
  mkdir -p "$dir"
  cp "$ARCHIVE" "$dir/"
  cp "$CHECKSUM" "$dir/$(basename "$ARCHIVE").sha256"
  cp "$SAFE_CONFIG" "$dir/rc-gateway.safe.json"
}

kit="$tmp/ok"
make_kit "$kit"
build_rapid_deb "$kit/rapidscada_6.4.7_all.deb" 6.4.7
bash "$INSTALLER" --dry-run --dir "$kit" >/dev/null

echo "stack installer positive DEB dry-run: OK"

kit="$tmp/ok-zip"
make_kit "$kit"
fixture_deb="$tmp/rapidscada_6.4.7_all.deb"
build_rapid_deb "$fixture_deb" 6.4.7
build_rapid_zip "$kit/rapidscada_6.4.7_linux_en.zip" "$fixture_deb"
zip_sha="$(sha256sum "$kit/rapidscada_6.4.7_linux_en.zip" | awk '{print $1}')"
RC_SCADA_RAPID_SHA256="$zip_sha" bash "$INSTALLER" --dry-run --dir "$kit" >/dev/null

echo "stack installer positive official-style ZIP dry-run: OK"

kit="$tmp/wrong-version"
make_kit "$kit"
build_rapid_deb "$kit/rapidscada_6.4.6_all.deb" 6.4.6
if bash "$INSTALLER" --dry-run --dir "$kit" >/dev/null 2>&1; then
  echo "ERRO: versão Rapid SCADA incorreta foi aceita" >&2
  exit 1
fi
echo "stack installer rejects wrong Rapid version: OK"

kit="$tmp/wrong-package"
make_kit "$kit"
build_rapid_deb "$kit/rapidscada_6.4.7_all.deb" 6.4.7 notrapid
if bash "$INSTALLER" --dry-run --dir "$kit" >/dev/null 2>&1; then
  echo "ERRO: pacote que não é rapidscada foi aceito" >&2
  exit 1
fi
echo "stack installer rejects wrong Debian package: OK"

kit="$tmp/duplicate"
make_kit "$kit"
build_rapid_deb "$kit/rapidscada_6.4.7_all.deb" 6.4.7
cp "$kit/rapidscada_6.4.7_all.deb" "$kit/rapidscada_6.4.7-duplicate_all.deb"
if bash "$INSTALLER" --dry-run --dir "$kit" >/dev/null 2>&1; then
  echo "ERRO: múltiplos pacotes Rapid SCADA foram aceitos" >&2
  exit 1
fi
echo "stack installer rejects ambiguous Rapid package set: OK"

kit="$tmp/bad-checksum"
make_kit "$kit"
build_rapid_deb "$kit/rapidscada_6.4.7_all.deb" 6.4.7
printf '%064d  %s\n' 0 "$(basename "$ARCHIVE")" > "$kit/$(basename "$ARCHIVE").sha256"
if bash "$INSTALLER" --dry-run --dir "$kit" >/dev/null 2>&1; then
  echo "ERRO: checksum Gateway incorreto foi aceito" >&2
  exit 1
fi
echo "stack installer rejects bad Gateway checksum: OK"

#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

candidate="$TMP/config.json"
printf '{"schema":3,"nodeId":"installer-security-test","tunnels":[]}\n' > "$candidate"

expect_reject(){
  local archive="$1"
  local expected="$2"
  sha256sum "$archive" > "$archive.sha256"
  if "$ROOT/scripts/install-release.sh" --dry-run "$archive" "$archive.sha256" "$candidate" >"$TMP/out" 2>"$TMP/err"; then
    echo "ERRO: instalador aceitou archive inseguro: $archive" >&2
    exit 1
  fi
  grep -F "$expected" "$TMP/err" >/dev/null || {
    echo "ERRO: rejeição inesperada; esperado '$expected'" >&2
    cat "$TMP/err" >&2
    exit 1
  }
}

mkdir -p "$TMP/symlink/pkg"
ln -s /etc/passwd "$TMP/symlink/pkg/escape"
tar -czf "$TMP/symlink.tar.gz" -C "$TMP/symlink" pkg
expect_reject "$TMP/symlink.tar.gz" "tipo de entrada não permitido"

mkdir -p "$TMP/multiroot/pkg"
printf 'extra\n' > "$TMP/multiroot/extra.txt"
tar -czf "$TMP/multiroot.tar.gz" -C "$TMP/multiroot" pkg extra.txt
expect_reject "$TMP/multiroot.tar.gz" "exatamente um diretório raiz"

echo "installer security tests OK"

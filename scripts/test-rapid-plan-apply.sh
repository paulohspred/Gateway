#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT/scripts/apply-rapid-plan.sh"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/rapid/Config" "$tmp/plan/files/Instance"
printf '<Config/>\n' > "$tmp/plan/files/Instance/Test.xml"
printf 'lab-noop-v1\n' > "$tmp/plan/VERSION"
( cd "$tmp/plan/files" && sha256sum Instance/Test.xml ) > "$tmp/plan/SHA256SUMS"
{ printf 'version=lab-noop-v1\n'; cat "$tmp/plan/SHA256SUMS"; } | sha256sum | awk '{print $1}' > "$tmp/plan/PLAN_SHA256"
RAPID_SCADA_ROOT="$tmp/rapid" "$SCRIPT" --dry-run "$tmp/plan" | grep -Fq 'DRY-RUN OK'
cp "$tmp/plan/SHA256SUMS" "$tmp/bad"
sed -i 's#Instance/Test.xml#../escape.xml#' "$tmp/plan/SHA256SUMS"
if RAPID_SCADA_ROOT="$tmp/rapid" "$SCRIPT" --dry-run "$tmp/plan" >/dev/null 2>&1; then
  echo 'ERRO: traversal deveria ser rejeitado' >&2; exit 1
fi
mv "$tmp/bad" "$tmp/plan/SHA256SUMS"
echo 'RAPID PLAN APPLY CONTRACT PASSED'

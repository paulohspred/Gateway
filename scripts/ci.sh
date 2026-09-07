#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/.."

require_tooling="${RC_GATEWAY_REQUIRE_TOOLING:-0}"
check_optional_tool(){
  local tool="$1"
  shift
  if command -v "$tool" >/dev/null 2>&1; then
    "$tool" "$@"
  elif [[ "$require_tooling" == "1" ]]; then
    echo "ERRO: ferramenta obrigatória ausente: $tool" >&2
    exit 69
  else
    echo "AVISO: $tool não encontrado; gate local correspondente foi pulado." >&2
  fi
}

unformatted="$(gofmt -l .)"
if [[ -n "$unformatted" ]]; then
  echo "ERRO: arquivos Go não formatados:" >&2
  printf '%s\n' "$unformatted" >&2
  exit 1
fi

go mod verify
check_optional_tool actionlint .github/workflows/*.yml
go vet ./...
check_optional_tool staticcheck ./...
go test ./... -shuffle=on -count=1 -coverprofile=coverage.out
bash scripts/check-coverage.sh coverage.out
go test -race ./... -count=1
go build -trimpath ./cmd/rc-gateway
go build -trimpath ./cmd/rc-monitor
go build -trimpath ./cmd/rc-admin
./rc-monitor --version
./rc-admin --version
./rc-admin --check-config --bind 127.0.0.1:18110 --state /tmp/rc-admin-ci-state.json

for cfg in configs/*.json; do
  ./rc-gateway --check-config --config "$cfg"
done

# Most monitor configs are source-tree portable. The LAB deployment config is
# intentionally rooted at /opt/rc-gateway/current so that moving it to /etc does
# not break profile/binding resolution. In CI, validate that same config against
# the checkout by translating only the canonical installed prefix to this repo.
repo_root="$(pwd)"
tmp_monitor_cfg="$(mktemp)"
cleanup(){ rm -f "$tmp_monitor_cfg" rc-gateway rc-monitor rc-admin coverage.out /tmp/rc-admin-ci-state.json; }
trap cleanup EXIT
for cfg in configs/monitor/*.json; do
  if grep -Fq '/opt/rc-gateway/current/' "$cfg"; then
    python3 - "$cfg" "$tmp_monitor_cfg" "$repo_root" <<'PY'
from pathlib import Path
import sys
src, dst, root = sys.argv[1:]
text = Path(src).read_text(encoding="utf-8")
text = text.replace("/opt/rc-gateway/current", root)
Path(dst).write_text(text, encoding="utf-8")
PY
    ./rc-monitor --check-config --config "$tmp_monitor_cfg"
  else
    ./rc-monitor --check-config --config "$cfg"
  fi
done

bash scripts/test-rapid-plan-apply.sh
bash -n scripts/*.sh

trap - EXIT
cleanup
echo "Gateway + RC Monitor CI local OK"

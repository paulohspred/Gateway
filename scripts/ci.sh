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
check_optional_tool actionlint .github/workflows/ci.yml .github/workflows/codeql.yml .github/workflows/release-attest.yml
go vet ./...
check_optional_tool staticcheck ./...
go test ./... -shuffle=on -count=1 -coverprofile=coverage.out
bash scripts/check-coverage.sh coverage.out
go test -race ./... -count=1
go build -trimpath ./cmd/rc-gateway

for cfg in configs/*.json; do
  ./rc-gateway --check-config --config "$cfg"
done

rm -f rc-gateway coverage.out
echo "Gateway CI local OK"

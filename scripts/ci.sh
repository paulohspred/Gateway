#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/.."

unformatted="$(gofmt -l .)"
if [[ -n "$unformatted" ]]; then
  echo "ERRO: arquivos Go não formatados:" >&2
  printf '%s\n' "$unformatted" >&2
  exit 1
fi

go vet ./...
go test ./... -shuffle=on -count=1
go test -race ./... -count=1
go build -trimpath ./cmd/rc-gateway

for cfg in configs/*.json; do
  ./rc-gateway --check-config --config "$cfg"
done

rm -f rc-gateway
echo "Gateway CI local OK"

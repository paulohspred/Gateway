#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/.."
gofmt -w .
go vet ./...
go test ./...
go build ./cmd/rc-gateway
echo "gateway-umbrella CI OK"

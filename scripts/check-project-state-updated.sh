#!/usr/bin/env bash
set -Eeuo pipefail

DOC="docs/PROJECT_STATE.md"

if [[ ! -f "$DOC" ]]; then
  echo "ERRO: documento canônico ausente: $DOC" >&2
  exit 1
fi

if [[ -n "${GITHUB_BASE_REF:-}" ]] && git rev-parse --verify "origin/${GITHUB_BASE_REF}" >/dev/null 2>&1; then
  BASE="origin/${GITHUB_BASE_REF}"
elif git rev-parse --verify HEAD^ >/dev/null 2>&1; then
  BASE="HEAD^"
else
  echo "Sem commit-base disponível; documento canônico presente."
  exit 0
fi

changed="$(git diff --name-only "$BASE"...HEAD)"
source_changed="$(printf '%s\n' "$changed" | grep -E '^(cmd/|internal/|configs/|scripts/|systemd/|go\.(mod|sum)$|\.github/workflows/)' || true)"
doc_changed="$(printf '%s\n' "$changed" | grep -Fx "$DOC" || true)"

if [[ -n "$source_changed" && -z "$doc_changed" ]]; then
  echo "ERRO: código/runtime/release mudou sem atualizar $DOC no mesmo conjunto de mudanças." >&2
  echo "Mudanças relevantes:" >&2
  printf '%s\n' "$source_changed" >&2
  echo >&2
  echo "Atualize PROJECT_STATE.md registrando mudança, validação, riscos e próximo passo." >&2
  exit 1
fi

echo "PROJECT_STATE.md está coerente com o conjunto de mudanças."

#!/usr/bin/env bash
set -Eeuo pipefail

DOC="gateway-umbrella/docs/PROJECT_STATE.md"

if [[ ! -f "$DOC" ]]; then
  echo "ERRO: documento canônico ausente: $DOC" >&2
  exit 1
fi

latest_gateway_commit="$({
  git rev-list -1 HEAD -- gateway-umbrella .github/workflows/gateway-umbrella.yml
} || true)"

latest_doc_commit="$(git rev-list -1 HEAD -- "$DOC" || true)"

if [[ -z "$latest_gateway_commit" || -z "$latest_doc_commit" ]]; then
  echo "ERRO: não foi possível determinar histórico do Gateway/documento." >&2
  exit 1
fi

if [[ "$latest_gateway_commit" != "$latest_doc_commit" ]]; then
  echo "ERRO: gateway-umbrella foi alterado depois da última atualização de $DOC." >&2
  echo "Último commit relevante do Gateway : $latest_gateway_commit" >&2
  echo "Último commit do documento          : $latest_doc_commit" >&2
  echo >&2
  echo "Atualize PROJECT_STATE.md registrando:" >&2
  echo "- o que mudou;" >&2
  echo "- por que mudou;" >&2
  echo "- validação/CI;" >&2
  echo "- riscos/limitações;" >&2
  echo "- próximo passo." >&2
  exit 1
fi

echo "PROJECT_STATE.md está sincronizado com a alteração mais recente do Gateway."

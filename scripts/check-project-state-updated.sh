#!/usr/bin/env bash
set -Eeuo pipefail

DOC="docs/PROJECT_STATE.md"

fail() {
  echo "ERRO: $*" >&2
  exit 1
}

marker_value() {
  local key="$1"
  sed -n "s/^<!-- ${key}: \(.*\) -->$/\1/p" "$DOC" | head -n1
}

status_of() {
  local wanted="$1"
  awk -F'|' -v wanted="$wanted" '
    function trim(s) { gsub(/^[[:space:]]+|[[:space:]]+$/, "", s); return s }
    /^\|/ {
      id=trim($2); status=trim($3)
      if (id == wanted) { print status; exit }
    }
  ' "$DOC"
}

[[ -f "$DOC" ]] || fail "documento canônico ausente: $DOC"

grep -Fq '<!-- PROJECT_STATE_SCHEMA: 2 -->' "$DOC" || fail "PROJECT_STATE_SCHEMA ausente ou incompatível"
grep -Fq '<!-- CANONICAL_HANDOFF: true -->' "$DOC" || fail "CANONICAL_HANDOFF precisa permanecer true"
grep -Fq '<!-- PRODUCTION_VALIDATED:' "$DOC" || fail "marcador PRODUCTION_VALIDATED ausente"
grep -Fq '<!-- PR2_MUST_REMAIN_DRAFT: true -->' "$DOC" || fail "regra de PR #2 draft ausente"
grep -Fq '<!-- CHECKLIST_START -->' "$DOC" || fail "CHECKLIST_START ausente"
grep -Fq '<!-- CHECKLIST_END -->' "$DOC" || fail "CHECKLIST_END ausente"

for invariant in \
  'BRIDGE FIRST' \
  'PROTOCOL OPTIONAL' \
  'NO DEVICE MEMORY DATABASE' \
  'NO TELEMETRY HISTORIAN'
do
  grep -Fq "$invariant" "$DOC" || fail "invariante arquitetural ausente: $invariant"
done

current_branch="$(marker_value CURRENT_CODE_BRANCH)"
current_task="$(marker_value CURRENT_DEVELOPMENT_TASK)"
external_gate="$(marker_value EXTERNAL_RUNNING_GATE)"
production_validated="$(marker_value PRODUCTION_VALIDATED)"

[[ -n "$current_branch" ]] || fail "CURRENT_CODE_BRANCH ausente"
[[ -n "$current_task" ]] || fail "CURRENT_DEVELOPMENT_TASK ausente"
[[ -n "$external_gate" ]] || fail "EXTERNAL_RUNNING_GATE ausente"
[[ "$production_validated" == "true" || "$production_validated" == "false" ]] || \
  fail "PRODUCTION_VALIDATED deve ser true ou false"

checklist="$({
  sed -n '/<!-- CHECKLIST_START -->/,/<!-- CHECKLIST_END -->/p' "$DOC"
} | awk -F'|' '
  function trim(s) { gsub(/^[[:space:]]+|[[:space:]]+$/, "", s); return s }
  /^\|/ {
    id=trim($2); status=trim($3)
    if (id != "" && id != "ID" && id !~ /^-+$/) print id "\t" status
  }
')"

[[ -n "$checklist" ]] || fail "checklist canônico está vazio"

allowed_status='^(DONE|IN_PROGRESS|NEXT|TODO|BLOCKED|DEFERRED)$'
while IFS=$'\t' read -r id status; do
  [[ "$status" =~ $allowed_status ]] || fail "status inválido no checklist: $id -> $status"
done <<< "$checklist"

duplicates="$(printf '%s\n' "$checklist" | cut -f1 | sort | uniq -d)"
[[ -z "$duplicates" ]] || fail "IDs duplicados no checklist: $duplicates"

for required_id in GW-001 VM-001 VM-006 VM-007 SOAK-001 SEM-001 MON-001 MON-002 MON-004 HIL-001 UI-001 PROD-001; do
  [[ -n "$(status_of "$required_id")" ]] || fail "task obrigatória ausente do checklist: $required_id"
done

current_task_status="$(status_of "$current_task")"
[[ "$current_task_status" == "NEXT" || "$current_task_status" == "IN_PROGRESS" ]] || \
  fail "CURRENT_DEVELOPMENT_TASK=$current_task deve estar NEXT ou IN_PROGRESS; atual=$current_task_status"

if [[ "$external_gate" != "none" && "$external_gate" != "NONE" ]]; then
  external_status="$(status_of "$external_gate")"
  [[ "$external_status" == "IN_PROGRESS" ]] || \
    fail "EXTERNAL_RUNNING_GATE=$external_gate deve estar IN_PROGRESS; atual=$external_status"
fi

next_count="$(printf '%s\n' "$checklist" | awk -F'\t' '$2 == "NEXT" {count++} END {print count+0}')"
(( next_count >= 1 )) || fail "checklist deve indicar pelo menos uma tarefa NEXT"

if [[ "$production_validated" == "true" ]]; then
  for gate in SOAK-001 SEM-001 HIL-001 PROD-001; do
    [[ "$(status_of "$gate")" == "DONE" ]] || \
      fail "PRODUCTION_VALIDATED=true exige $gate=DONE"
  done
fi

# O handoff precisa ter sido atualizado no mesmo commit material ou em commit
# posterior. Em PRs o checkout é um merge sintético; por isso a comparação usa
# explicitamente a branch head quando ela estiver disponível.
TRACK_REF="HEAD"
if [[ -n "${GITHUB_HEAD_REF:-}" ]] && git rev-parse --verify "origin/${GITHUB_HEAD_REF}" >/dev/null 2>&1; then
  TRACK_REF="origin/${GITHUB_HEAD_REF}"
fi

material_paths=(
  cmd internal configs catalog scripts systemd backend frontend web ui api
  go.mod go.sum .github/workflows
)
latest_material_commit="$(git log -1 --format=%H "$TRACK_REF" -- "${material_paths[@]}" || true)"
latest_doc_commit="$(git log -1 --format=%H "$TRACK_REF" -- "$DOC" || true)"

[[ -n "$latest_doc_commit" ]] || fail "não foi possível localizar commit de $DOC"
if [[ -n "$latest_material_commit" ]] && ! git merge-base --is-ancestor "$latest_material_commit" "$latest_doc_commit"; then
  fail "$DOC está mais antigo que a última mudança material ($latest_material_commit); atualize o handoff depois da mudança"
fi

# O conjunto proposto também precisa conter o handoff quando há mudança material
# em relação à base do PR/main. Este gate mantém o motivo explícito no diff.
if [[ -n "${GITHUB_BASE_REF:-}" ]] && git rev-parse --verify "origin/${GITHUB_BASE_REF}" >/dev/null 2>&1; then
  BASE="origin/${GITHUB_BASE_REF}"
elif [[ "${GITHUB_REF_NAME:-}" != "main" ]] && git rev-parse --verify origin/main >/dev/null 2>&1; then
  BASE="origin/main"
elif git rev-parse --verify HEAD^ >/dev/null 2>&1; then
  BASE="HEAD^"
else
  echo "Sem commit-base disponível; estrutura e freshness do documento canônico validadas."
  exit 0
fi

changed="$(git diff --name-only "$BASE"..."$TRACK_REF")"
source_changed="$(printf '%s\n' "$changed" | grep -E '^(cmd/|internal/|configs/|catalog/|scripts/|systemd/|backend/|frontend/|web/|ui/|api/|go\.(mod|sum)$|\.github/workflows/)' || true)"
doc_changed="$(printf '%s\n' "$changed" | grep -Fx "$DOC" || true)"

if [[ -n "$source_changed" && -z "$doc_changed" ]]; then
  echo "ERRO: código/runtime/release mudou sem atualizar $DOC no conjunto proposto." >&2
  echo "Mudanças relevantes:" >&2
  printf '%s\n' "$source_changed" >&2
  echo >&2
  echo "Atualize PROJECT_STATE.md registrando mudança, evidência, risco residual, checklist e próximo passo." >&2
  exit 1
fi

echo "PROJECT_STATE.md válido: schema, invariantes, checklist, ponteiros e freshness por ancestralidade OK."

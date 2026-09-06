# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/monitor-core -->
<!-- CURRENT_DEVELOPMENT_TASK: MON-005 -->
<!-- EXTERNAL_RUNNING_GATE: SOAK-001 -->
<!-- PRODUCTION_VALIDATED: false -->
<!-- PR2_MUST_REMAIN_DRAFT: true -->

Este arquivo é o handoff canônico. Mudança material exige atualização posterior deste arquivo e `scripts/check-project-state-updated.sh` verde.

## Arquitetura fixa

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN

campo -> RC GATEWAY (transporte)
      -> RAPID SCADA 6 (protocolo, registradores, polling, qualidade, histórico)
      -> RC MONITOR (domínio, API, segurança)
      -> FRONTEND
```

Regras: Gateway não contém register maps; RC Monitor recebe canais Rapid, nunca endereços Modbus físicos; zero real permanece zero; dado indefinido permanece ausente; nenhuma escrita industrial nesta fase.

## Continuidade / branches

- `hardening/standalone-10x` -> PR #2 -> `main`: deve permanecer draft/not-merged.
- `feature/monitor-core` -> PR #3 -> `hardening/standalone-10x`: deve permanecer draft.
- não alterar `main` nem mesclar PR #2/#3 sem ordem explícita do proprietário;
- `tmp-backend-finish` é somente branch de montagem/correção para fast-forward do PR #3;
- não tocar na VM durante `SOAK-001`.

Rapid baseline: **6.4.7**, source analisado `1fd36080c7830303f921672fdaee335a06e7ae50`.

## SOAK-001

```text
inicio: 2026-09-05T23:47:29Z
intervalo: 30 s
duração: 86400 s
fim esperado: 2026-09-06T23:47:29Z
```

Para DONE: `result=PASS` e todos os contadores `bad_*`, restart/session-id changes e counter regressions iguais a zero; depois repetir production preflight non-root com `DISRUPTIVE=0`.

## Política de controladoras

GenMon é referência funcional/factual GPLv2, não fonte para copiar código/JSON ao repositório proprietário. `controllers/DRAFT_PROFILES.json` contém vocabulário RC clean-room para ComAp, DSE, SmartGen, MEBAY, Kohler, Basler, PowerZone, Briggs e Generac, todos `draft` e read-only. Endereços/FC/encoding reais ficam no Rapid SCADA e só são promovidos com documentação permitida e/ou HIL.

## Backend já aprovado

```text
MON-001 DONE  fd73a495...  Gateway CI #122 / CodeQL #49 SUCCESS
MON-002 DONE  9bc9689c...  Gateway CI #126 / CodeQL #53 SUCCESS
MON-003 DONE  46f50c2b...  Gateway CI #128 / CodeQL #55 SUCCESS
MON-004 DONE  b509a382...  Gateway CI #131 / CodeQL #58 SUCCESS
```

## MON-005 / MON-006 — candidato de fechamento

Implementado no candidato:

- Rapid Web auth + current data + `GetLastAvailableEvents`;
- cookie/session e reauth única em 401/403;
- endpoint Rapid e bind do monitor obrigatoriamente loopback;
- config estrita; segredos apenas por environment variables;
- `SemanticReader` e binding de métricas, alarmes e eventos;
- condições `equals`, `one_of`, `nonzero`, `bit_set`, `gt/gte/lt/lte`;
- eventos `alarm.raised` / `alarm.cleared`;
- zero/ausência/stale/offline fail-closed;
- E2E `Rapid HTTP -> WebReader -> SemanticReader -> Provider -> Service -> /api/v1`;
- outage/recovery + mini-soak 250 ciclos;
- validação de outputs do Provider na fronteira Service;
- `/metrics` somente operacional;
- `rc-monitor.service` non-root/hardened;
- instalador próprio com config candidata validada em `/etc`, env `0600` e readiness;
- release reprodutível inclui `rc-monitor`, unit, exemplos, controllers e docs.

Histórico de gates do candidato:

```text
e2f752b39e004b43521b33c6dbb7cd7614ee0958
  CodeQL #68 SUCCESS
  Gateway CI #141 FAIL: Format somente (internal/monitor/model.go)

6dc6f7db6cea2d21eb7e10ef854bbb53958262f9
  CodeQL #69 SUCCESS
  Gateway CI #142:
    Canonical state SUCCESS
    Format SUCCESS
    Module integrity SUCCESS
    FAIL em go vet: maxProfileFileBytes undefined

3873e9f09e45efed749cbec42d3136afdf10fa76
  corrigiu gofmt do model.go

77b3f86b4687e1d63809f0da6f6f06bc502ebb14
  restaurou limite compartilhado maxProfileFileBytes=1 MiB para loaders
```

MON-005 e MON-006 permanecem `IN_PROGRESS` até Gateway CI + CodeQL ficarem verdes no mesmo HEAD.

## Próximo gate de software

Depois de MON-005/MON-006 verdes, executar MON-007: restart real do processo `rc-monitor` em CI + recovery + mini-soak. A validação systemd/VM real espera SOAK-001 terminar.

## API read-only

```text
GET /healthz
GET /readyz
GET /metrics
GET /api/v1/system/health
GET /api/v1/generators
GET /api/v1/generators/{id}
GET /api/v1/generators/{id}/telemetry
GET /api/v1/generators/{id}/alarms
GET /api/v1/generators/{id}/events
```

## Checklist canônico

<!-- CHECKLIST_START -->
| ID | Status | Critério |
|---|---|---|
| GW-001 | DONE | Gateway bridge-first. |
| GW-002 | DONE | Rapid transport comprovado. |
| GW-003 | DONE | Firewall Rapid validado. |
| VM-001 | DONE | Stack instalada/validada. |
| VM-002 | DONE | Rapid -> Gateway -> simulador comprovado. |
| VM-003 | DONE | Field outage/recovery. |
| VM-004 | DONE | Blackhole/recovery. |
| VM-005 | DONE | Cold boot. |
| VM-006 | DONE | ScadaComm non-root. |
| VM-007 | DONE | Cold boot pós-non-root. |
| HARD-001 | TODO | Codificar hardening ScadaComm non-root no installer. |
| SOAK-001 | IN_PROGRESS | Soak 24 h da VM. |
| SEM-001 | TODO | Binding semântico com canais Rapid reais. |
| MON-001 | DONE | Foundation. |
| MON-002 | DONE | API read-only. |
| MON-003 | DONE | Profiles. |
| MON-004 | DONE | RapidScadaProvider. |
| MON-005 | IN_PROGRESS | Rapid Web + semântica + E2E; aguarda gates. |
| MON-006 | IN_PROGRESS | Hardening/release/observabilidade; aguarda gates. |
| MON-007 | NEXT | Restart/recovery/soak do rc-monitor. |
| HIL-001 | BLOCKED | Primeira controladora real read-only. |
| HIL-002 | BLOCKED | Modem/VPN/meio físico. |
| CMD-001 | DEFERRED | Writes só após HIL/interlocks/auditoria. |
| UI-001 | TODO | Congelar contrato frontend. |
| UI-002 | TODO | Shell visual. |
| UI-003 | TODO | Telas + API real. |
| UI-004 | TODO | Testes frontend. |
| REL-001 | TODO | Confirmar proteção de main. |
| REL-002 | IN_PROGRESS | Release inclui monitor; aguarda release gate. |
| PROD-001 | BLOCKED | Exige SEM + HIL + soak + aprovação. |
<!-- CHECKLIST_END -->

## Próximo passo exato

```text
1. fast-forward feature/monitor-core para o HEAD desta branch temporária;
2. executar Gateway CI + CodeQL;
3. corrigir o próximo gate sem tocar na VM;
4. com ambos verdes, fechar MON-005/MON-006;
5. implementar/validar MON-007 em software;
6. somente após SOAK-001, executar SEM-001 e validação do rc-monitor na VM.
```

`PRODUCTION_VALIDATED=false`.

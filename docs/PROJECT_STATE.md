# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/monitor-core -->
<!-- CURRENT_DEVELOPMENT_TASK: MON-007 -->
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

## Backend aprovado

```text
MON-001 DONE  fd73a495...  Gateway CI #122 / CodeQL #49 SUCCESS
MON-002 DONE  9bc9689c...  Gateway CI #126 / CodeQL #53 SUCCESS
MON-003 DONE  46f50c2b...  Gateway CI #128 / CodeQL #55 SUCCESS
MON-004 DONE  b509a382...  Gateway CI #131 / CodeQL #58 SUCCESS

MON-005 DONE + MON-006 DONE
  HEAD: 67a0776565410e2513f44bab98185f774607c6ed
  Gateway CI #144: SUCCESS
  CodeQL #71: SUCCESS
```

MON-005 inclui Rapid Web API, semântica de métricas/alarmes/eventos, E2E, outage/recovery e mini-soak. MON-006 inclui config estrita, secrets por env, observabilidade local, systemd non-root, installer e release reprodutível.

## MON-007 — IN_PROGRESS

Candidato software montado em `tmp-backend-finish`:

```text
bd37bd72f8a0a4fe6d1244a2ca6aeec4a8560f45
  adiciona scripts/test-rc-monitor-process-recovery.sh

cc01d90ea37e7bfa6be7f22a12a5bd4b5eb3b809
  integra gate ao job Impairment and mini-soak
```

O gate de processo:

1. compila o binário real `cmd/rc-monitor`;
2. usa bind loopback em porta efêmera por processo;
3. executa 20 ciclos por padrão;
4. em cada ciclo prova `/healthz`, `/readyz`, `/api/v1/generators` e `/metrics`;
5. envia `SIGTERM` e exige exit status 0;
6. exige que o listener deixe de responder;
7. reinicia na mesma porta e repete.

No mesmo job de CI, o gate Rapid executa explicitamente:

```text
TestRapidWebSemanticPipelineEndToEnd
TestRapidWebSemanticPipelineMiniSoak
```

Esses testes cobrem online, stale, outage, last-known offline, recuperação, zero real, ausência, alarmes/eventos e 250 ciclos.

MON-007 só vira `DONE` em software após promoção para `feature/monitor-core` e Gateway CI + CodeQL verdes no mesmo HEAD. Validação systemd/VM do monitor continua bloqueada até `SOAK-001` liberar o host.

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
| MON-005 | DONE | Rapid Web + semântica + E2E; CI #144 + CodeQL #71. |
| MON-006 | DONE | Hardening/release/observabilidade; CI #144 + CodeQL #71. |
| MON-007 | IN_PROGRESS | Candidato restart/recovery/soak do processo rc-monitor; aguarda gates. |
| HIL-001 | BLOCKED | Primeira controladora real read-only. |
| HIL-002 | BLOCKED | Modem/VPN/meio físico. |
| CMD-001 | DEFERRED | Writes só após HIL/interlocks/auditoria. |
| UI-001 | NEXT | Congelar contrato frontend após MON-007 software. |
| UI-002 | TODO | Shell visual. |
| UI-003 | TODO | Telas + API real. |
| UI-004 | TODO | Testes frontend. |
| REL-001 | TODO | Confirmar proteção de main. |
| REL-002 | DONE | Release inclui rc-monitor e passou gate reprodutível/dry-run. |
| PROD-001 | BLOCKED | Exige SEM + HIL + soak + aprovação. |
<!-- CHECKLIST_END -->

## Próximo passo exato

```text
1. fast-forward feature/monitor-core para o HEAD desta branch temporária;
2. exigir Gateway CI + CodeQL verdes;
3. confirmar os novos passos RC Monitor Rapid outage/recovery e process restart lifecycle;
4. se verdes, marcar MON-007 software DONE;
5. manter a VM intocada até SOAK-001 terminar;
6. depois do soak, executar SEM-001 e validação systemd/VM do rc-monitor.
```

`PRODUCTION_VALIDATED=false`.

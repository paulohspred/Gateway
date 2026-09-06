# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/monitor-core -->
<!-- CURRENT_DEVELOPMENT_TASK: HARD-001 -->
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

MON-007 DONE (software)
  HEAD: b49d287cdad630abbd10461d10632d936d679096
  Gateway CI #146: SUCCESS
  CodeQL #73: SUCCESS
  RC Monitor Rapid outage and recovery: SUCCESS
  RC Monitor process restart lifecycle, 20 ciclos: SUCCESS
```

MON-005 inclui Rapid Web API, semântica de métricas/alarmes/eventos, E2E, outage/recovery e mini-soak. MON-006 inclui config estrita, secrets por env, observabilidade local, systemd non-root, installer e release reprodutível. MON-007 prova lifecycle do binário real, graceful SIGTERM/restart, recovery Rapid e mini-soak em software.

A validação systemd/VM do `rc-monitor` permanece externa e só pode começar depois de `SOAK-001` liberar o host.

## HARD-001 — IN_PROGRESS

Candidato de least privilege montado em `tmp-backend-finish`:

```text
6c4c94077369dafc7175d57e570f677d70c4e1ae
  installer: harden ScadaComm as non-root

5702cb5bbc815715a3f68b7e5767903d907b2d2a
  test: enforce ScadaComm non-root installer contract
```

O hardening codificado preserva o baseline já homologado manualmente na VM e evita ownership amplo:

- cria usuário/grupo de sistema dedicado `scadacomm` de forma idempotente;
- grava drop-in de `scadacomm6.service` com `User=scadacomm`, `Group=scadacomm`, `NoNewPrivileges=true`, `PrivateTmp=true`, `ProtectHome=true`, capability sets vazios e `UMask=0027`;
- mantém a árvore `/opt/scada` sob ownership do pacote/root; o installer não executa `chown -R /opt/scada`;
- configura `ScadaInstanceConfig.xml` para usar `/var/log/scada` como raiz de logs;
- concede escrita somente a `/var/log/scada/ScadaComm/Log`, mantendo os diretórios de aplicação/config/driver como leitura;
- após restart, verifica `scadacomm6.service` ativo, usuário efetivo `scadacomm`, leitura do worker/config e escrita no diretório de log dedicado;
- o teste de contrato exige as diretivas de hardening e falha se houver ownership recursivo da árvore Rapid.

Base técnica Rapid 6.4.7 usada na decisão: `CommDirs` exige `Lang`, `Log` e `Drv`; `Manager` carrega `ScadaInstanceConfig.xml` e aplica `LogDir`; a documentação Linux upstream define `/var/log/scada` como diretório de logs. Não foi identificada necessidade de tornar `/opt/scada` gravável pelo Communicator.

HARD-001 permanece `IN_PROGRESS` até este candidato ser promovido para `feature/monitor-core` e Gateway CI + CodeQL ficarem verdes no mesmo HEAD.

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
| HARD-001 | IN_PROGRESS | Installer ScadaComm non-root least-privilege; aguarda gates. |
| SOAK-001 | IN_PROGRESS | Soak 24 h da VM. |
| SEM-001 | TODO | Binding semântico com canais Rapid reais. |
| MON-001 | DONE | Foundation. |
| MON-002 | DONE | API read-only. |
| MON-003 | DONE | Profiles. |
| MON-004 | DONE | RapidScadaProvider. |
| MON-005 | DONE | Rapid Web + semântica + E2E; CI #144 + CodeQL #71. |
| MON-006 | DONE | Hardening/release/observabilidade; CI #144 + CodeQL #71. |
| MON-007 | DONE | Restart/recovery/mini-soak software; CI #146 + CodeQL #73. |
| HIL-001 | BLOCKED | Primeira controladora real read-only. |
| HIL-002 | BLOCKED | Modem/VPN/meio físico. |
| CMD-001 | DEFERRED | Writes só após HIL/interlocks/auditoria. |
| UI-001 | NEXT | Congelar contrato frontend após hardening operacional. |
| UI-002 | TODO | Shell visual. |
| UI-003 | TODO | Telas + API real. |
| UI-004 | TODO | Testes frontend. |
| REL-001 | TODO | Confirmar proteção de main. |
| REL-002 | DONE | Release inclui rc-monitor e passou gate reprodutível/dry-run. |
| PROD-001 | BLOCKED | Exige SEM + HIL + soak + aprovação. |
<!-- CHECKLIST_END -->

## Próximo passo exato

```text
1. fast-forward feature/monitor-core para o HEAD deste candidato;
2. exigir Gateway CI + CodeQL verdes no mesmo HEAD;
3. confirmar teste de contrato do installer e release reprodutível;
4. se verdes, marcar HARD-001 DONE;
5. manter a VM intocada até SOAK-001 terminar;
6. depois do soak, executar SEM-001 e validações systemd/VM pendentes.
```

`PRODUCTION_VALIDATED=false`.

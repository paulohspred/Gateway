# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/frontend-contract -->
<!-- CURRENT_DEVELOPMENT_TASK: UI-003 -->
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
- `feature/frontend-contract` deriva de `feature/monitor-core` e contém a especificação/implementação do frontend; não mesclar em `main` diretamente.
- não alterar `main` nem mesclar PR #2/#3 sem ordem explícita do proprietário;
- PR #4 (`feature/frontend-contract` -> `feature/monitor-core`) deve permanecer draft durante UI-003/UI-004;
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

## HARD-001 — DONE

Hardening non-root do `scadacomm6` foi codificado no installer e validado no mesmo HEAD de código:

```text
HEAD: 540ae8a60f331a57a86c5eb7429c14e27d08f4fd
Gateway CI #148: SUCCESS
CodeQL #75: SUCCESS
Gateway and Rapid SCADA stack installer contract: SUCCESS
Reproducible release build: SUCCESS
Validate release archive and installer dry-run: SUCCESS
```

Implementação:

- usuário/grupo de sistema dedicado `scadacomm`, criado de forma idempotente;
- drop-in de `scadacomm6.service` com `User=scadacomm`, `Group=scadacomm`, `NoNewPrivileges=true`, `PrivateTmp=true`, `ProtectHome=true`, capability sets vazios e `UMask=0027`;
- árvore `/opt/scada` permanece sob ownership do pacote/root; o installer não executa `chown -R /opt/scada`;
- `ScadaInstanceConfig.xml` usa `/var/log/scada` como raiz de logs;
- somente `/var/log/scada/ScadaComm/Log` recebe escrita pelo usuário dedicado;
- após restart, o installer verifica serviço ativo, usuário efetivo `scadacomm`, leitura do worker/config e escrita no diretório de log;
- o teste de contrato exige as diretivas de hardening e rejeita ownership recursivo da árvore Rapid.

Base técnica Rapid 6.4.7: `CommDirs` exige `Lang`, `Log` e `Drv`; `Manager` carrega `ScadaInstanceConfig.xml` e aplica `LogDir`; a documentação Linux upstream define `/var/log/scada` para logs. Não foi identificada necessidade de tornar `/opt/scada` gravável pelo Communicator.

A validação prática da nova automação no host real será repetida somente após `SOAK-001`; o comportamento non-root equivalente já foi comprovado manualmente em VM em `VM-006`/`VM-007`.

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

## Frontend — primeira vertical validada

A primeira implementação real está em `feature/frontend-contract` sobre a API read-only existente.

```text
frontend/
  React + TypeScript + Vite
  TanStack Query
  Zod
  Lucide

rotas:
  /
  /generators
  /generators/:id
  /alarms
  /events
  /communication
```

Características:

- shell industrial dark responsivo;
- Visão Geral e frota por telemetria real;
- lista de geradores;
- Detalhe do Gerador seguindo `docs/GENERATOR_DETAIL_CONTRACT.md`;
- sinótico sem inferir MCB/GCB/ATS/rede ausentes;
- Motor/ECU, elétrica, combustível, bateria/DC, alarmes, eventos e detalhes de quality;
- zero real permanece zero; ausência permanece `N/D`; `bad` não é exibido como valor confiável;
- sem START/STOP/RESET/TEST/TRANSFER/setpoints/acknowledge;
- polling/cache centralizados via TanStack Query;
- agregação de frota usa concorrência limitada no cliente enquanto não existe endpoint agregado;
- `capabilities/profile` read-only continua gap conhecido para distinguir `unsupported` de ausência transitória.

Gates próprios:

```text
.github/workflows/frontend-ci.yml
.github/workflows/codeql-frontend.yml
```

Evidência da implementação validada:

```text
material frontend fix: d6867dc342b4097edec9fa37409690767f11b51a
validated descendant HEAD: 98b25986f0c30e56073819e8761b5765633b1df4
Frontend CI #3: SUCCESS
  typecheck: SUCCESS
  unit tests: SUCCESS
  production build: SUCCESS
CodeQL Frontend #3: SUCCESS
CodeQL Go #79: SUCCESS
Gateway CI #152: SUCCESS
  quality/unit/race/config: SUCCESS
  stress/leak: SUCCESS
  impairment/mini-soak: SUCCESS
  security/reproducibility/release: SUCCESS
```

`UI-002` está concluído em software. Isso não significa deployment/produção; `UI-003`, `UI-004`, SEM/HIL e os gates externos continuam pendentes.

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
| HARD-001 | DONE | Installer ScadaComm non-root least-privilege; CI #148 + CodeQL #75. |
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
| UI-001 | DONE | Contratos de produto, superfícies, commissioning, ECU e Detalhe do Gerador congelados; implementação real autorizada. |
| UI-002 | DONE | Shell + primeira vertical real; Frontend CI #3, CodeQL Frontend #3, CodeQL #79 e Gateway CI #152 SUCCESS. |
| UI-003 | IN_PROGRESS | Consolidar telas/API real, capability/profile read-only e integração de produção. |
| UI-004 | NEXT | Testes frontend ampliados, regressão responsiva/visual e edge cases. |
| REL-001 | TODO | Confirmar proteção de main. |
| REL-002 | DONE | Release inclui rc-monitor e passou gate reprodutível/dry-run. |
| PROD-001 | BLOCKED | Exige SEM + HIL + soak + aprovação. |
<!-- CHECKLIST_END -->

## Próximo passo exato

```text
1. manter a VM intocada enquanto SOAK-001 estiver em execução;
2. UI-003 está ativo: consolidar integração read-only, serving/deployment do frontend e capability/profile para HMI adaptativa;
3. endurecer supply chain frontend (lockfile + runtime de Node pinado) antes de release final;
4. UI-004 é NEXT: ampliar testes de qualidade/ausência/outage e regressão responsiva;
5. após SOAK-001, verificar o relatório real antes de qualquer toque no host e então executar preflight non-disruptive;
6. executar SEM-001 com canais Rapid reais;
7. HIL-001/HIL-002 continuam bloqueados até hardware/meio físico disponível e aprovado.
```

`PRODUCTION_VALIDATED=false`.

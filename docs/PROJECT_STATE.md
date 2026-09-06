# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/monitor-core -->
<!-- CURRENT_DEVELOPMENT_TASK: MON-005 -->
<!-- EXTERNAL_RUNNING_GATE: SOAK-001 -->
<!-- PRODUCTION_VALIDATED: false -->
<!-- PR2_MUST_REMAIN_DRAFT: true -->

> **REGRA OBRIGATÓRIA:** este arquivo é o handoff canônico. Mudança material em código/runtime/release exige atualização deste arquivo depois da última mudança e `scripts/check-project-state-updated.sh` verde.

## 1. Regras de continuidade

1. confirmar `paulohspred/Gateway`, branch e HEAD reais;
2. não alterar `main` nem mesclar PR #2/#3 sem autorização explícita do proprietário;
3. não marcar `DONE` sem evidência reproduzível no mesmo HEAD;
4. nunca registrar segredos;
5. não tocar na VM enquanto `SOAK-001` estiver em execução;
6. não declarar produção antes de semântica real + HIL + soak + aprovação operacional.

Status válidos: `DONE`, `IN_PROGRESS`, `NEXT`, `TODO`, `BLOCKED`, `DEFERRED`.

## 2. Arquitetura fixa

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

```text
campo/controladora/modem/VPN/serial/TCP/USB/CAN
                    -> RC GATEWAY
                       transporte
                    -> RAPID SCADA 6
                       protocolo + registradores + polling + qualidade + histórico
                    -> RC MONITOR
                       domínio + API + segurança
                    -> FRONTEND
                       apresentação
```

Invariantes:

- Gateway transporta bytes/datagramas/frames/HID reports; não contém register maps;
- Rapid SCADA interpreta o protocolo/aplicação e mantém endereços físicos, FC, encoding, escala/polling;
- `rc-monitor` recebe canais Rapid, não registradores Modbus;
- zero real permanece zero e dado indefinido permanece ausente;
- nenhum comando industrial é habilitado nesta fase;
- frontend consumirá apenas o contrato normalizado do RC Monitor.

## 3. Branches e PRs

```text
hardening/standalone-10x
  HEAD base registrado: 494c2a7e19cda62fecac0c142bc87091ca0ac061
  PR #2 -> main
  deve permanecer open/draft/not-merged

feature/monitor-core
  PR #3 -> hardening/standalone-10x
  deve permanecer draft
```

O candidato de fechamento do backend foi montado primeiro em `tmp-backend-finish` a partir do antigo HEAD do PR #3 `4666b1defff80c54357881179d0cfa17f10be4f1`. A branch temporária é usada apenas para montagem/correção antes de fast-forward para `feature/monitor-core`.

## 4. Gateway + Rapid SCADA comprovados

Rapid baseline: **6.4.7**, source analisado `1fd36080c7830303f921672fdaee335a06e7ae50`.

Gateway bridge-first, transporte Rapid via `DrvCnlBasic` + `DrvModbus`, firewall interno, installer/recovery, systemd, supply-chain e CI já foram homologados em software/VM conforme histórico do PR #2.

VM homologada com Ubuntu 24.04.3, Rapid SCADA 6.4.7, Gateway `vm-ac5c98e047e7` / source pin `ac5c98e047e752539e7844dfa7d9d3d69565a6e6`. Não confundir esse binário da VM com HEADs posteriores do repositório.

## 5. SOAK-001

```text
inicio: 2026-09-05T23:47:29Z
intervalo: 30 s
duração: 86400 s
fim esperado: 2026-09-06T23:47:29Z
```

Enquanto estiver `IN_PROGRESS`, não reiniciar, reconfigurar ou instalar o novo backend na VM.

Critério final do relatório:

```text
result=PASS
bad_ready_samples=0
bad_session_samples=0
bad_identity_samples=0
gateway_restart_change_samples=0
scada_restart_change_samples=0
session_id_changes=0
counter_regressions=0
```

Depois do relatório PASS, repetir production preflight non-root com `DISRUPTIVE=0`.

## 6. Política clean-room de controladoras

`jgyates/genmon` é somente referência funcional/factual. Código e JSON GPLv2 não são copiados para este repositório proprietário.

Famílias levantadas para profiles RC `draft`:

- ComAp;
- Deep Sea / DSE 7320 MKII;
- SmartGen HGM4000N/HGM40x0;
- MEBAY DC4x-DC9x;
- Kohler APM603;
- Basler DGC-2020HD;
- Generac Power Zone 410;
- Briggs & Stratton GC-1032;
- Generac Evolution Liquid Cooled.

`controllers/DRAFT_PROFILES.json` contém somente vocabulário canônico RC, sem endereços físicos. Endereços/FC/encoding reais pertencem a templates/projetos Rapid e precisam de documentação permitida e/ou HIL antes de promoção de compatibilidade.

## 7. MON-001 a MON-004 — DONE

```text
MON-001 foundation/model/Provider/Service/FakeProvider
  fd73a4952c7de992075440180a959fcae1b62860
  Gateway CI #122 SUCCESS / CodeQL #49 SUCCESS

MON-002 API read-only
  9bc9689c49469b9ad442a705bd6f883618c7e7a8
  Gateway CI #126 SUCCESS / CodeQL #53 SUCCESS

MON-003 Controller Profiles
  46f50c2b606532bce638ad53fd7182467b38e723
  Gateway CI #128 SUCCESS / CodeQL #55 SUCCESS

MON-004 RapidScadaProvider + metric channel binding
  b509a38283de345f7a2fbb8e57218a95ed0f8eb0
  Gateway CI #131 SUCCESS / CodeQL #58 SUCCESS
```

## 8. MON-005 — candidato completo, aguardando gates

O candidato implementa:

- Rapid Web API auth em `POST /Api/Auth/Login`;
- current data em `GET /Api/Main/GetCurData`;
- histórico em `GET /Api/Main/GetLastAvailableEvents`;
- cookie jar e uma reautenticação em 401/403;
- response limit 2 MiB e timeout limitado;
- Rapid endpoint obrigatório em loopback já no `--check-config`;
- `AllowCommandApi` não é usado;
- `SemanticReader` separado do adapter HTTP;
- binding único de métricas + alarmes + eventos;
- condições `equals`, `one_of`, `nonzero`, `bit_set`, `gt`, `gte`, `lt`, `lte`;
- alarmes normalizados a partir de definitions do profile;
- eventos `alarm.raised` / `alarm.cleared` e eventos operacionais explicitamente vinculados;
- zero real preservado, canal status `<=0` omitido, stale e last-known offline;
- E2E sintético `Rapid HTTP -> WebReader -> SemanticReader -> Provider -> Service -> /api/v1`;
- outage/recovery e mini-soak de 250 ciclos;
- validação de `Alarm`, `Event` e `ProviderHealth` na fronteira `Service`;
- catálogo clean-room de 9 famílias continua `draft`, `remoteControl=false`.

Evidência do primeiro candidato promovido:

```text
HEAD e2f752b39e004b43521b33c6dbb7cd7614ee0958
CodeQL #68: SUCCESS
Gateway CI #141: FAIL somente no gate Format
  arquivo: internal/monitor/model.go
  canonical project state: SUCCESS
  workflow lint: SUCCESS
```

A formatação apontada foi corrigida em `3873e9f09e45efed749cbec42d3136afdf10fa76`. MON-005 permanece `IN_PROGRESS` até o novo HEAD passar Gateway CI + CodeQL.

## 9. MON-006 — candidato completo, aguardando gates

Hardening candidato:

- config JSON estrita;
- senha/usuário Rapid somente por environment variables;
- `--check-config` sem listener nem conexão Rapid;
- `/healthz`, `/readyz`, `/api/v1/system/health`;
- `/metrics` com somente uptime/request counter, sem telemetria industrial;
- `systemd/rc-monitor.service` com usuário dedicado, `NoNewPrivileges`, `PrivateDevices`, filesystem/kernel protections, capability set vazio e address families restritas;
- `scripts/install-rc-monitor.sh` idempotente no provisionamento de user/group, valida config candidata já em `/etc`, instala secrets `0600`, habilita serviço e exige readiness;
- release reprodutível passa a carregar `rc-monitor`, unit, examples, controllers e documentação;
- `build-release.sh` testa config fake/sintético e dry-run do installer na arquitetura nativa;
- `scripts/ci.sh` valida também `configs/monitor/*.json` e shell syntax.

MON-006 permanece `IN_PROGRESS` até o mesmo HEAD ficar verde.

## 10. MON-007 — próximo

Depois de MON-005/MON-006 verdes:

- acrescentar/confirmar restart de processo do próprio `rc-monitor`;
- recovery após Rapid indisponível;
- mini-soak já existente deve permanecer verde;
- executar soak/recovery real do monitor somente depois de SOAK-001 liberar a VM.

O E2E atual já cobre outage/recovery do adapter em software; MON-007 formaliza o gate operacional do processo/serviço.

## 11. API do backend

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

Somente GET. Bind padrão e produção: loopback.

## 12. Gates externos pendentes

- `SEM-001`: binding de canais Rapid reais na VM e leitura inequívoca pelo backend;
- `HIL-001`: primeira controladora real read-only por modelo/firmware;
- `HIL-002`: modem/VPN/meio físico;
- soak de campo, mínimo 24 h e alvo 7 d;
- START/STOP/transfer/reset/setpoints bloqueados até HIL/interlocks/autorização/auditoria.

Progressão:

```text
implemented
 -> software_validated
 -> software_field_test_ready
 -> vm_accepted
 -> hil_accepted
 -> soak_accepted
 -> production_validated
```

`PRODUCTION_VALIDATED=false`.

## 13. Checklist canônico

<!-- CHECKLIST_START -->
| ID | Status | Item / critério de saída |
|---|---|---|
| GW-001 | DONE | Gateway bridge-first implementado e testado. |
| GW-002 | DONE | Transporte Rapid SCADA 6.4.7 documentado/testado. |
| GW-003 | DONE | Firewall Rapid interno persistente validado. |
| VM-001 | DONE | Stack Gateway + Rapid instalada/validada. |
| VM-002 | DONE | Rapid -> Gateway -> simulador FC03 comprovado. |
| VM-003 | DONE | Field outage/recovery comprovado. |
| VM-004 | DONE | Blackhole/recovery sem restart comprovado. |
| VM-005 | DONE | Cold boot stack comprovado. |
| VM-006 | DONE | ScadaComm non-root comprovado. |
| VM-007 | DONE | Cold boot pós-non-root comprovado. |
| HARD-001 | TODO | Codificar hardening non-root do ScadaComm no installer do stack. |
| SOAK-001 | IN_PROGRESS | Soak 24 h do stack; exige report PASS + preflight final. |
| SEM-001 | TODO | E2E semântico com canais Rapid reais na VM. |
| MON-001 | DONE | Foundation/model/Provider/Service/FakeProvider. |
| MON-002 | DONE | API read-only `/api/v1`. |
| MON-003 | DONE | Controller Profiles schema/loader/profile sintético. |
| MON-004 | DONE | RapidScadaProvider + metric binding; CI #131 + CodeQL #58 PASS. |
| MON-005 | IN_PROGRESS | Candidato Rapid Web + alarm/event binding + E2E; formatação corrigida, aguarda novo CI/CodeQL. |
| MON-006 | IN_PROGRESS | Candidato hardening/systemd/release/observabilidade; aguarda novo CI/CodeQL. |
| MON-007 | NEXT | Restart/recovery/soak próprios do processo rc-monitor. |
| HIL-001 | BLOCKED | HIL read-only primeira controladora real. |
| HIL-002 | BLOCKED | HIL modem/VPN/meio físico. |
| CMD-001 | DEFERRED | Escritas industriais somente após HIL/interlocks/auditoria. |
| UI-001 | TODO | Congelar contrato frontend depois do backend software-validado. |
| UI-002 | TODO | Shell visual novo. |
| UI-003 | TODO | Telas principais com API real + E2E. |
| UI-004 | TODO | Testes frontend e estados offline/stale/N-D. |
| REL-001 | TODO | Confirmar proteção de `main`; integração não possui permissão para ler protection endpoint. |
| REL-002 | IN_PROGRESS | Release agora inclui rc-monitor; aguarda CI de release do candidato. |
| PROD-001 | BLOCKED | Exige VM + semântica real + HIL + soak + aprovação operacional. |
<!-- CHECKLIST_END -->

## 14. Próximo passo exato

```text
1. fast-forward feature/monitor-core para o novo HEAD da tmp-backend-finish;
2. observar Gateway CI + CodeQL;
3. corrigir o próximo gate sem tocar na VM;
4. quando ambos verdes no mesmo HEAD, marcar MON-005/MON-006 DONE;
5. executar MON-007 em software antes de considerar o backend encerrado;
6. manter SOAK-001 intocado até o fim e só depois executar SEM-001/VM do rc-monitor.
```

## 15. Proibições atuais

- não copiar código/controller JSON do GenMon;
- não modificar ProjetoGerador;
- não colocar Modbus/register maps físicos no Gateway ou RC Monitor;
- não reimplementar o Rapid Server wire protocol;
- não inventar zero, alarmes ou estados;
- não tratar profile `draft` como compatibilidade validada;
- não habilitar writes industriais;
- não tocar na VM durante SOAK-001;
- não mesclar PR #2/#3 em `main` sem ordem explícita.

## 16. Freshness gate

Mudanças em `cmd/`, `internal/`, `configs/`, `catalog/`, `controllers/`, `scripts/`, `systemd/`, `.github/workflows/`, `go.mod/go.sum` ou futuros backend/frontend exigem `PROJECT_STATE.md` atualizado depois da última mudança material.

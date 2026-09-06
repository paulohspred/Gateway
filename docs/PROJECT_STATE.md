# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/monitor-core -->
<!-- CURRENT_DEVELOPMENT_TASK: MON-003 -->
<!-- EXTERNAL_RUNNING_GATE: SOAK-001 -->
<!-- PRODUCTION_VALIDATED: false -->
<!-- PR2_MUST_REMAIN_DRAFT: true -->

> **REGRA OBRIGATÓRIA:** este arquivo é o handoff canônico. Novo chat, IA ou mantenedor deve lê-lo antes de alterar código, arquitetura, segurança, Rapid SCADA, backend ou frontend. Mudança material exige atualização deste arquivo e `scripts/check-project-state-updated.sh` verde.

## 1. Regras de continuidade

1. confirmar `paulohspred/Gateway`, branch e HEAD reais;
2. seguir o checklist abaixo;
3. não alterar `main` nem mesclar PR sem autorização explícita do proprietário;
4. não marcar `DONE` sem evidência reproduzível;
5. registrar mudança, evidência, risco residual e próximo passo;
6. nunca registrar segredos;
7. nunca declarar produção antes dos gates externos.

Status válidos: `DONE`, `IN_PROGRESS`, `NEXT`, `TODO`, `BLOCKED`, `DEFERRED`.

## 2. Arquitetura fixa — NÃO quebrar

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

- Gateway transporta bytes/datagramas/frames/HID reports e não inventa register map;
- transporte não implica conversão semântica;
- não existe conversão automática Modbus RTU <-> TCP nem ComAp Direct <-> Modbus;
- `length32be` é framing de transporte;
- Gateway não é historian, banco de telemetria, broker nem SCADA;
- Rapid SCADA interpreta protocolo/aplicação;
- `rc-monitor` não reimplementa bridge/Modbus/register maps físicos;
- frontend consome métricas normalizadas;
- zero real permanece zero; métrica ausente permanece ausente.

## 3. Branches e PRs

```text
repo: paulohspred/Gateway

hardening/standalone-10x
  HEAD registrado: 494c2a7e19cda62fecac0c142bc87091ca0ac061
  PR #2 -> main
  PR #2 deve permanecer open/draft/not-merged

feature/monitor-core
  base de criação: 494c2a7e19cda62fecac0c142bc87091ca0ac061
  PR #3 -> hardening/standalone-10x
  PR #3 deve permanecer draft
```

Não mesclar PR #2 ou #3 em `main` sem ordem explícita.

## 4. Gateway + Rapid SCADA já comprovados

Rapid baseline: **6.4.7**, source analisado `1fd36080c7830303f921672fdaee335a06e7ae50`.

Gateway possui TCP/TLS/Unix/serial/UDP/CAN/HID, framing explícito, limites/timeouts/allowlists, JSON estrito, admin loopback-only, command plane desabilitado, health/readiness/status/sessions/metrics, systemd notify/watchdog, installers, amd64/arm64, SBOM/provenance e gates CI/CodeQL.

Integração Rapid: `DrvCnlBasic` TCP client + `DrvModbus`; Gateway entrega transporte, Rapid interpreta.

## 5. VM de homologação

```text
Ubuntu Server 24.04.3 LTS
kernel 6.8.0-139-generic x86_64
Rapid SCADA 6.4.7
ASP.NET Core Runtime 8.0.30
Nginx 1.24.0
Gateway instalado: vm-ac5c98e047e7
source pin do binário: ac5c98e047e752539e7844dfa7d9d3d69565a6e6
```

**Não afirmar que a VM executa 494c2a7.** O branch avançou depois.

Firewall persistente protege Rapid 10000/10002 para loopback. `scadacomm6` foi homologado non-root como `scadacomm`, com `NoNewPrivileges`, `PrivateTmp`, `ProtectHome`, caps vazias e `UMask=0027`; logs em `/var/log/scada/ScadaComm/Log`. Cold boot pós-non-root passou. Pendente codificar esse hardening no installer sem `chown` amplo.

## 6. Integração VM aprovada

```text
client -> 127.0.0.1:25020 -> RC Gateway -> 127.0.0.1:15020 -> simulator
```

FC03 retornou `[2300, 2310, 2290, 5000, 1500, 805, 1250, 150, 1500, 1]`.

Rapid Line 99 usa `DrvCnlBasic`, `DrvModbus`, `TransMode=TCP`, Unit ID 1, `CmdEnabled=false`, template read-only e `<Cmds />` vazio. Polling FC03 via Gateway passou.

**Limite:** line/device usam `isBound=false`; transporte está comprovado, binding semântico ainda não.

Recovery já aprovados: field outage, blackhole, restart Rapid/Gateway, cold boot, cold boot pós-non-root e production preflight non-root.

## 7. SOAK-001 — 24 h em execução

```text
inicio: 2026-09-05T23:47:29Z
intervalo: 30 s
duração: 86400 s
fim esperado: 2026-09-06T23:47:29Z
```

Última evidência registrada: ready/sessão/bytes OK, NRestarts 0, user `scadacomm`, events somente `SOAK START`, preflight não-disruptivo PASS.

Para `DONE` o relatório deve conter:

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

Depois repetir production preflight non-root com `DISRUPTIVE=0`. Não alterar/reiniciar a VM durante o soak.

## 8. Referências externas

### Genmon

`jgyates/genmon` serve como referência funcional: controller abstraction, profiles, status, alarmes e eventos.

Os JSON de `genmon/data/controller` são material GPLv2 e **não são copiados** para este repositório proprietário. `controllers/REFERENCE_CATALOG.md` registra somente o inventário factual de famílias/modelos visto lá como `REFERENCE_ONLY`: Basler DGC-2020HD, Briggs & Stratton GC-1032, ComAp, Deep Sea, Generac Evolution Liquid Cooled, Kohler APM603, MEBAY DCxx, Power Zone 410 e SmartGen HGM4000.

Perfil real RC só pode ser promovido com documentação permitida de fabricante e/ou evidência HIL, além do binding Rapid e teste semântico.

### ProjetoGerador

`paulohspdev-cmyk/ProjetoGerador` não será modificado; referência visual/UX futura apenas.

## 9. MON-001 — foundation — DONE

Criados `monitor` model/Provider/Service/FakeProvider. Semântica: valores number/text/bool, `good/stale/offline/bad/unknown`, ausência != zero, `NaN/Inf` rejeitados. FakeProvider cobre online/offline/stale/alarme/recovery/cancel/not-found.

```text
commit funcional: fd73a4952c7de992075440180a959fcae1b62860
Gateway CI #122: SUCCESS
CodeQL #49: SUCCESS
```

## 10. MON-002 — API read-only — DONE

```text
GET /healthz
GET /readyz
GET /api/v1/generators
GET /api/v1/generators/{id}
GET /api/v1/generators/{id}/telemetry
GET /api/v1/generators/{id}/alarms
GET /api/v1/generators/{id}/events
GET /api/v1/system/health
```

Somente GET; 404/408/502/504 estáveis; métricas escalares preservam zero/ausência; `/healthz` é liveness; `/readyz` exige provider healthy; servidor fake default em `127.0.0.1:18100` e rejeita bind não-loopback.

```text
HEAD de fechamento: 9bc9689c49469b9ad442a705bd6f883618c7e7a8
Gateway CI #126: SUCCESS
CodeQL #53: SUCCESS
```

## 11. MON-003 — Controller Profiles — IN_PROGRESS

Estrutura criada:

```text
controllers/
  README.md
  REFERENCE_CATALOG.md
  rc-simulator/reference-controller/
    manifest.json
    telemetry.json
    alarms.json
    ui.json

internal/monitor/profile/
  model.go
  loader.go
  profile_test.go
```

Schema v1 original e fail-closed:

- manifest identifica fabricante/modelo/status/capabilities e componentes;
- telemetry usa somente `monitor.MetricKey` canônica + tipo/unidade/required/stale timeout;
- alarms usa código normalizado/severity/mensagem;
- UI só referencia métricas declaradas;
- unknown JSON fields, profile ID/schema divergentes, path traversal/absolute path, metric key desconhecida/duplicada, UI inconsistente e alarm severity inválida falham;
- synthetic profile usa `remoteControl=false`;
- `controllers/` foi incluído no freshness checker do handoff.

Histórico de validação deste incremento:

```text
1608ec40e5ae02057ea46cea0b45ff93ebda9741
Gateway CI #127: FAILURE somente em gofmt
  internal/monitor/profile/model.go
  internal/monitor/profile/profile_test.go
Canonical state + workflow lint: PASS antes da falha
```

A correção de `gofmt` está sendo aplicada no incremento atual. `MON-003` só vira `DONE` quando CI + CodeQL do HEAD corrigido estiverem verdes.

## 12. Próxima fila

```text
MON-004 RapidScadaProvider atrás de monitor.Provider
MON-005 Rapid -> Provider -> Service -> API com semântica E2E
MON-006 hardening rc-monitor: config/non-root/systemd/logs/observabilidade
MON-007 restart/recovery/soak próprios do rc-monitor
```

Frontend somente após modelo/API/profiles/Rapid provider e `absent/stale/offline/quality` estabilizados.

## 13. Gates externos pendentes

- `SEM-001`: canais Rapid vinculados e valores recuperados inequivocamente pelo backend;
- HIL read-only por controladora/modelo/firmware/meio físico/VPN/modem;
- soak mínimo 24 h e alvo 7 d;
- START/STOP/transfer/reset/setpoints permanecem bloqueados até HIL/interlocks/autorização/auditoria.

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

## 14. Checklist canônico

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
| HARD-001 | TODO | Codificar non-root no installer com idempotência/least privilege. |
| SOAK-001 | IN_PROGRESS | Soak 24 h; exige `.report` PASS + acceptance final. |
| SEM-001 | TODO | E2E semântico Rapid com canais vinculados. |
| MON-001 | DONE | Foundation/model/Provider/Service/FakeProvider; CI #122 + CodeQL #49 PASS. |
| MON-002 | DONE | API read-only `/api/v1`; CI #126 + CodeQL #53 PASS. |
| MON-003 | IN_PROGRESS | Controller Profile schema/loader/profile sintético; gofmt corrigido, novo CI pendente. |
| MON-004 | NEXT | Implementar `RapidScadaProvider` atrás de `monitor.Provider`. |
| MON-005 | TODO | Testar monitor contra Rapid real/sintético e fechar semântica E2E. |
| MON-006 | TODO | Hardening monitor: non-root/systemd/config/logs/observabilidade. |
| MON-007 | TODO | Soak/recovery próprios do rc-monitor. |
| HIL-001 | BLOCKED | HIL read-only primeira controladora real. |
| HIL-002 | BLOCKED | HIL modem/VPN/meio físico. |
| CMD-001 | DEFERRED | START/STOP/etc. somente após HIL/interlocks/auditoria. |
| UI-001 | TODO | Contrato frontend final após backend estável. |
| UI-002 | TODO | Shell visual novo usando ProjetoGerador só como referência. |
| UI-003 | TODO | Telas principais com API real + E2E. |
| UI-004 | TODO | Playwright/Vitest/RTL e estados offline/stale/N-D. |
| REL-001 | TODO | Confirmar proteção de `main`. |
| REL-002 | TODO | Release/installers quando rc-monitor for componente suportado. |
| PROD-001 | BLOCKED | Exige VM + semântica + HIL + soak + aprovação operacional. |
<!-- CHECKLIST_END -->

## 15. Próximo passo exato

```text
Concluir MON-003:
1. publicar correção gofmt;
2. CI + CodeQL verdes no mesmo HEAD;
3. marcar MON-003 DONE;
4. iniciar MON-004 RapidScadaProvider.
```

## 16. Proibições atuais

- não copiar código/controller JSON do Genmon;
- não modificar ProjetoGerador;
- não colocar Modbus/register maps no Gateway;
- não transformar rc-monitor em bridge;
- não inventar zero;
- não tratar `REFERENCE_ONLY` como compatibilidade;
- não habilitar writes industriais;
- não tocar na VM durante SOAK-001;
- não declarar produção antes dos gates;
- não mesclar PR #2/#3 em `main` sem ordem explícita.

## 17. Freshness gate

Mudanças em `cmd/`, `internal/`, `configs/`, `catalog/`, `controllers/`, `scripts/`, `systemd/`, `.github/workflows/`, `go.mod/go.sum` ou futuro backend/frontend exigem `PROJECT_STATE.md` atualizado depois da última mudança material.

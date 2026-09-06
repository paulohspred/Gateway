# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/monitor-core -->
<!-- CURRENT_DEVELOPMENT_TASK: MON-004 -->
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

## 11. MON-003 — Controller Profiles — DONE

Estrutura original criada:

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

Schema v1 fail-closed valida fabricante/modelo/status/capabilities, métricas canônicas, tipo/unidade/required/stale timeout, alarmes e agrupamento UI. Unknown JSON fields, IDs/schema divergentes, path traversal, métricas desconhecidas/duplicadas e UI inconsistente falham. Synthetic profile tem `remoteControl=false`.

Histórico:

```text
1608ec40e5ae02057ea46cea0b45ff93ebda9741
Gateway CI #127: FAILURE somente em gofmt

46f50c2b606532bce638ad53fd7182467b38e723
Gateway CI #128: SUCCESS
CodeQL #55: SUCCESS
```

A falha #127 foi somente formatação em `internal/monitor/profile/model.go` e `profile_test.go`; corrigida e integralmente revalidada no #128.

## 12. MON-004 — RapidScadaProvider — IN_PROGRESS

Decisão após revisar Rapid SCADA 6.4.7 upstream:

- `ScadaCommon/Client/ScadaClient` é o cliente TCP oficial do Server e declara explicitamente que não é thread-safe;
- `ScadaClient.GetCurrentData(int[] cnlNums, bool useCache, out long cnlListID)` e a variante cacheada existem para current channel data;
- `ConnectionOptions` default usa host `localhost`, port `10000`, timeout 10000 ms;
- Rapid `CnlData` considera status `> 0` definido e status `<= 0` indefinido.

**Não reimplementar o protocolo Server do Rapid em Go.** O `RapidScadaProvider` usa uma interface estreita `Reader`; MON-005 conecta essa interface ao Web API oficial do Rapid SCADA ou a outro adapter suportado, sem interpretar registradores no RC Monitor.

Incremento publicado:

```text
706353732203adb4076271ece8700ccee470f45f
internal/monitor/rapid/provider.go
internal/monitor/rapid/binding.go
internal/monitor/rapid/provider_test.go
controllers/rc-simulator/reference-controller/rapid/channels.json
CodeQL #56: SUCCESS
Gateway CI #129: FAILURE somente Staticcheck ST1005 em mensagens de erro de binding.go

5c0a00cbc0a2106b50eeb42e819c68b5e1220189
correção ST1005 publicada; nova validação CI/CodeQL pendente neste handoff
```

Contrato:

- `Reader.ReadCurrent`, `ReadAlarms`, `ReadEvents`, `Health`;
- `Provider` implementa exatamente `monitor.Provider`;
- inventário de geradores é configurado e validado contra o Controller Profile;
- `rapid/channels.json` mapeia **canais Rapid**, nunca registradores Modbus;
- transform explícito `number`, `boolean` ou `enum`;
- boolean/enum desconhecido falha fechado;
- Rapid status `<=0` omite a métrica, não produz zero;
- amostras antigas podem ser marcadas `stale` conforme profile;
- falha de leitura preserva last-known com `quality=offline` e communication `offline`;
- health do Reader indisponível produz provider `unavailable`;
- nenhum write/comando é implementado.

`MON-004` permanece `IN_PROGRESS` até CI + CodeQL verdes no mesmo HEAD.

## 13. Próxima fila

```text
MON-005 implementar adapter Rapid Web API read-only e provar
        Rapid -> Reader -> Provider -> Service -> /api/v1 com canais vinculados
MON-006 hardening rc-monitor: config/non-root/systemd/logs/observabilidade
MON-007 restart/recovery/soak próprios do rc-monitor
```

Frontend somente após modelo/API/profiles/Rapid provider e `absent/stale/offline/quality` estabilizados.

## 14. Gates externos pendentes

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

## 15. Checklist canônico

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
| MON-003 | DONE | Controller Profiles schema/loader/profile sintético; CI #128 + CodeQL #55 PASS. |
| MON-004 | IN_PROGRESS | `RapidScadaProvider`, Reader seam e binding de canais Rapid; correção CI #129 publicada, revalidação pendente. |
| MON-005 | NEXT | Adapter Rapid Web API suportado + integração semântica E2E contra Rapid real/sintético. |
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

## 16. Próximo passo exato

```text
1. confirmar CI + CodeQL verdes após 5c0a00c;
2. fechar MON-004;
3. implementar MON-005 usando o Web API oficial read-only do Rapid SCADA 6.4.7;
4. expandir modelo/perfis RC com referência factual das famílias ComAp/DSE/SmartGen/MEBAY/Kohler/Basler/PowerZone/Briggs/Generac, sem copiar JSON GPL;
5. não tocar na VM até SOAK-001 fechar.
```

## 17. Proibições atuais

- não copiar código/controller JSON do Genmon;
- não modificar ProjetoGerador;
- não colocar Modbus/register maps no Gateway;
- não reimplementar o Rapid Server wire protocol no rc-monitor;
- não inventar zero;
- não tratar `REFERENCE_ONLY` ou `draft` como compatibilidade validada;
- não habilitar writes industriais;
- não tocar na VM durante SOAK-001;
- não declarar produção antes dos gates;
- não mesclar PR #2/#3 em `main` sem ordem explícita.

## 18. Freshness gate

Mudanças em `cmd/`, `internal/`, `configs/`, `catalog/`, `controllers/`, `scripts/`, `systemd/`, `.github/workflows/`, `go.mod/go.sum` ou futuro backend/frontend exigem `PROJECT_STATE.md` atualizado depois da última mudança material.

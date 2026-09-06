# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/monitor-core -->
<!-- CURRENT_DEVELOPMENT_TASK: MON-005 -->
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
- Gateway não é historian, banco de telemetria, broker nem SCADA;
- Rapid SCADA interpreta protocolo/aplicação e mantém os mapas físicos;
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
Rapid SCADA 6.4.7
ASP.NET Core Runtime 8.0.30
Nginx 1.24.0
Gateway instalado: vm-ac5c98e047e7
source pin do binário: ac5c98e047e752539e7844dfa7d9d3d69565a6e6
```

Firewall persistente protege Rapid 10000/10002 para loopback. `scadacomm6` foi homologado non-root como `scadacomm`; cold boot pós-non-root passou. Pendente codificar esse hardening no installer sem `chown` amplo.

Integração aprovada:

```text
client -> 127.0.0.1:25020 -> RC Gateway -> 127.0.0.1:15020 -> simulator
```

FC03 via Gateway passou e Rapid Line 99 foi comprovada read-only. Binding semântico de canais na VM ainda é gate separado (`SEM-001`).

## 6. SOAK-001 — 24 h em execução

```text
inicio: 2026-09-05T23:47:29Z
intervalo: 30 s
duração: 86400 s
fim esperado: 2026-09-06T23:47:29Z
```

Não alterar/reiniciar a VM durante o soak. Para `DONE`, relatório precisa ser `PASS` sem ready/session/identity/restart/session-id/counter regressions e depois repetir production preflight non-root com `DISRUPTIVE=0`.

## 7. Referências externas e política clean-room

`jgyates/genmon` é referência funcional e factual para famílias de controladoras, métricas e comportamento. Seus JSON/código GPLv2 **não são copiados** para este repositório proprietário.

A revisão factual dos mapas `data/controller` mostrou cobertura útil para:

- ComAp;
- Deep Sea / DSE 7320 MKII;
- SmartGen HGM4000N/HGM40x0;
- MEBAY DC4x-DC9x;
- Kohler APM603;
- Basler DGC-2020HD;
- Generac Power Zone 410;
- Briggs & Stratton GC-1032;
- Generac Evolution Liquid Cooled.

Essas referências foram usadas somente para identificar **conceitos canônicos necessários**. `controllers/DRAFT_PROFILES.json` é conteúdo RC independente: não contém endereços físicos, máscaras ou sequências copiadas. Endereços Modbus e encoding ficam em templates/projetos Rapid SCADA e só são promovidos com documentação permitida e/ou HIL.

`paulohspdev-cmyk/ProjetoGerador` permanece referência visual/UX futura e não é modificado.

## 8. MON-001 — foundation — DONE

Modelo/Provider/Service/FakeProvider, quality `good/stale/offline/bad/unknown`, ausência != zero, testes.

```text
fd73a4952c7de992075440180a959fcae1b62860
Gateway CI #122: SUCCESS
CodeQL #49: SUCCESS
```

## 9. MON-002 — API read-only — DONE

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

Somente GET; servidor loopback; zero/ausência preservados.

```text
9bc9689c49469b9ad442a705bd6f883618c7e7a8
Gateway CI #126: SUCCESS
CodeQL #53: SUCCESS
```

## 10. MON-003 — Controller Profiles — DONE

Schema/loader fail-closed, profile sintético, alarmes/UI e validação estrita.

```text
46f50c2b606532bce638ad53fd7182467b38e723
Gateway CI #128: SUCCESS
CodeQL #55: SUCCESS
```

## 11. MON-004 — RapidScadaProvider — DONE

Implementado:

- `Reader` seam;
- `RapidScadaProvider` implementando `monitor.Provider`;
- binding de **canais Rapid**, nunca registradores físicos;
- transforms number/boolean/enum fail-closed;
- Rapid status `<=0` vira métrica ausente, não zero;
- stale por profile;
- last-known offline em falha de leitura;
- health fail-closed;
- sem write/comando.

Histórico:

```text
706353732203adb4076271ece8700ccee470f45f
CodeQL #56: SUCCESS
Gateway CI #129: falha somente Staticcheck ST1005

5c0a00cbc0a2106b50eeb42e819c68b5e1220189
correção ST1005

b509a38283de345f7a2fbb8e57218a95ed0f8eb0
Gateway CI #131: SUCCESS
CodeQL #58: SUCCESS
```

## 12. MON-005 — Rapid Web API + semântica E2E — IN_PROGRESS

### Adapter oficial do Rapid Web API

Publicado em `5d3f2d436d982b7141205341354ca7106da9be1a`:

- autenticação por `POST /Api/Auth/Login`;
- cookie jar/sessão preservada;
- current data por `GET /Api/Main/GetCurData?cnlNums=...`;
- parse do contrato oficial `ok/msg/data` e `cnlNum/val/stat`;
- reautenticação única em 401/403;
- resposta limitada a 2 MiB;
- endpoints somente HTTP/HTTPS em loopback;
- timeout limitado;
- nenhuma Command API;
- alarmes/eventos não são inventados: até binding semântico existir, retornam `ErrRapidSemanticBindingRequired`.

Rapid Webstation precisa habilitar explicitamente `AllowAuthApi=true`; `AllowCommandApi` deve permanecer false.

### Modelo canônico ampliado

`de2e6b7a71e9a58e92cc28c22f5dd31fd3844596` ampliou o domínio para cobrir as famílias industriais revisadas, incluindo:

- estado/RPM/óleo/coolant/horas/partidas;
- tensões fase-neutro e fase-fase;
- correntes L1/L2/L3;
- kW/kVA/kVAr/PF/energia/carga;
- rede L1/L2/L3/frequência/estado;
- controller/breakers/ATS;
- bateria e carregador;
- combustível e consumo;
- manutenção;
- identidade opcional de firmware/hardware/serial e dados nominais do grupo gerador.

### Catálogo clean-room de controladoras

`controllers/DRAFT_PROFILES.json` + `internal/monitor/profile/draft_catalog.go` modelam 9 famílias como `draft`, gerando Bundles válidos com UI e alarmes normalizados. Todos permanecem `remoteControl=false`, sem registradores físicos e sem declaração de compatibilidade HIL.

MON-005 só vira DONE após:

1. CI + CodeQL do HEAD com adapter/modelo/catálogo verdes;
2. runtime `rc-monitor` aceitar configuração `rapid-web` sem segredos em arquivo;
3. binding de alarmes read-only fail-closed;
4. E2E sintético HTTP `Rapid Web -> Reader -> Provider -> Service -> /api/v1` verde;
5. `SEM-001` real será executado após SOAK-001, sem interromper a VM antes.

## 13. Próxima fila

```text
MON-005 concluir runtime/config + alarm binding + E2E sintético
MON-006 hardening rc-monitor: non-root/systemd/config/logs/observabilidade
MON-007 restart/recovery/soak próprios do rc-monitor
```

Frontend somente após o contrato do backend ficar estável.

## 14. Gates externos pendentes

- `SEM-001`: canais Rapid reais vinculados e valores recuperados inequivocamente pelo backend;
- HIL read-only por controladora/modelo/firmware/meio físico/VPN/modem;
- soak mínimo 24 h e alvo 7 d;
- START/STOP/transfer/reset/setpoints bloqueados até HIL/interlocks/autorização/auditoria.

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
| SEM-001 | TODO | E2E semântico Rapid com canais reais vinculados. |
| MON-001 | DONE | Foundation/model/Provider/Service/FakeProvider. |
| MON-002 | DONE | API read-only `/api/v1`. |
| MON-003 | DONE | Controller Profiles schema/loader/profile sintético. |
| MON-004 | DONE | RapidScadaProvider + binding; CI #131 + CodeQL #58 PASS. |
| MON-005 | IN_PROGRESS | Rapid Web API reader, modelo/catálogo, runtime/config, alarm binding e E2E. |
| MON-006 | NEXT | Hardening monitor: non-root/systemd/config/logs/observabilidade. |
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
1. corrigir qualquer CI do adapter/modelo/catálogo;
2. implementar config estrita do rc-monitor com credenciais Rapid somente por env;
3. ligar provider rapid-web no cmd/rc-monitor;
4. implementar alarm channel binding read-only;
5. provar E2E HTTP sintético até /api/v1;
6. manter VM intocada até SOAK-001 terminar.
```

## 17. Proibições atuais

- não copiar código/controller JSON do Genmon;
- não modificar ProjetoGerador;
- não colocar Modbus/register maps no Gateway ou RC Monitor;
- não reimplementar o Rapid Server wire protocol;
- não inventar zero, alarmes ou estados;
- não tratar `REFERENCE_ONLY`/`draft` como compatibilidade validada;
- não habilitar writes industriais;
- não tocar na VM durante SOAK-001;
- não declarar produção antes dos gates;
- não mesclar PR #2/#3 em `main` sem ordem explícita.

## 18. Freshness gate

Mudanças em `cmd/`, `internal/`, `configs/`, `catalog/`, `controllers/`, `scripts/`, `systemd/`, `.github/workflows/`, `go.mod/go.sum` ou futuro backend/frontend exigem `PROJECT_STATE.md` atualizado depois da última mudança material.

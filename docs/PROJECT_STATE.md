# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/monitor-core -->
<!-- CURRENT_DEVELOPMENT_TASK: MON-002 -->
<!-- EXTERNAL_RUNNING_GATE: SOAK-001 -->
<!-- PRODUCTION_VALIDATED: false -->
<!-- PR2_MUST_REMAIN_DRAFT: true -->

> **REGRA OBRIGATÓRIA:** este é o documento canônico de continuidade. Qualquer novo chat, IA ou mantenedor deve ler este arquivo antes de alterar código, arquitetura, segurança, Rapid SCADA, backend ou frontend. Mudança material deve ser seguida por atualização deste arquivo e passar `scripts/check-project-state-updated.sh`.

## 1. Procedimento obrigatório

1. confirmar `paulohspred/Gateway`, branch e HEAD reais;
2. ler este arquivo inteiro;
3. seguir o **Checklist canônico**;
4. não alterar `main` nem mesclar PR sem autorização explícita do proprietário;
5. não declarar `DONE` sem evidência reproduzível;
6. registrar mudança, evidência, risco residual e próximo passo;
7. executar checker/CI;
8. nunca registrar segredos;
9. nunca declarar produção antes dos gates externos.

Status: `DONE`, `IN_PROGRESS`, `NEXT`, `TODO`, `BLOCKED`, `DEFERRED`.

## 2. Arquitetura fixa — NÃO quebrar

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

```text
Controladora / modem / serial / TCP / VPN / USB / CAN
                         |
                         v
                    RC GATEWAY
             transporte industrial
                         |
                         v
                  RAPID SCADA 6
        protocolo + registradores + polling
              + qualidade + histórico SCADA
                         |
                         v
                    RC MONITOR
          domínio do gerador + API + segurança
                         |
                         v
                     FRONTEND
                 interface do operador
```

Invariantes:

- Gateway transporta bytes/datagramas/frames/HID reports; não inventa register map.
- Transporte não implica conversão semântica.
- Não existe conversão automática Modbus RTU <-> TCP nem ComAp Direct <-> Modbus.
- `length32be` é framing de transporte.
- Gateway não é historian, banco de telemetria, broker nem SCADA.
- Rapid SCADA interpreta protocolo/aplicação.
- `rc-monitor` não reimplementa bridge, Modbus ou register maps físicos.
- frontend consome métricas normalizadas, não Function Code/registradores.

## 3. Repositório e branches

```text
repo: github.com/paulohspred/Gateway
```

`main`: não alterar diretamente; merge somente com autorização explícita.

### Gateway hardening

```text
branch: hardening/standalone-10x
HEAD registrado: 494c2a7e19cda62fecac0c142bc87091ca0ac061
PR #2 -> main
base SHA histórico: e0f2ecdafc5ee065428b442575fee5a9d5ab0a11
```

PR #2 deve permanecer `open=true`, `draft=true`, `merged=false`.

### RC Monitor

```text
branch: feature/monitor-core
base de criação: 494c2a7e19cda62fecac0c142bc87091ca0ac061
PR #3 -> hardening/standalone-10x
PR #3: draft
```

Não mesclar PR #3 em `main`.

## 4. Gateway/Rapid já comprovados

Gateway possui transportes TCP/TLS/Unix/serial/UDP/CAN/HID, framing explícito, limites, timeouts, allowlists, JSON estrito, admin loopback-only, command plane desabilitado, health/readiness/status/sessions/metrics, systemd notify/watchdog, installers, amd64/arm64, SBOM/provenance e gates CI/CodeQL.

Rapid baseline:

```text
Rapid SCADA 6.4.7
source analisado: 1fd36080c7830303f921672fdaee335a06e7ae50
```

Integração: `DrvCnlBasic` TCP client + `DrvModbus` com `TransMode` coerente ao payload; Gateway entrega transporte, Rapid interpreta.

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

Firewall persistente protege Rapid 10000/10002 para loopback. `scadacomm6` foi homologado non-root como `scadacomm`, com `NoNewPrivileges`, `PrivateTmp`, `ProtectHome`, caps vazias e `UMask=0027`; logs em `/var/log/scada/ScadaComm/Log`. Cold boot pós-non-root passou.

Pendente codificar esse hardening no installer sem `chown` amplo.

## 6. Integração VM já aprovada

Modbus TCP sintético:

```text
client -> 127.0.0.1:25020 -> RC Gateway -> 127.0.0.1:15020 -> simulator
```

FC03 retornou `[2300, 2310, 2290, 5000, 1500, 805, 1250, 150, 1500, 1]`.

Rapid Line 99 usa `DrvCnlBasic`, `DrvModbus`, `TransMode=TCP`, Unit ID 1, `CmdEnabled=false`, template read-only e `<Cmds />` vazio. Polling FC03 via Gateway passou.

**Limite:** `isBound=false`; transporte está comprovado, binding semântico de canais ainda não.

Recovery aprovados: field outage, blackhole, restart Rapid/Gateway, cold boot, cold boot pós-non-root e production preflight non-root.

## 7. Soak 24 h — EM EXECUÇÃO

Task `SOAK-001`:

```text
inicio: 2026-09-05T23:47:29Z
intervalo: 30 s
duração: 86400 s
fim esperado: 2026-09-06T23:47:29Z
```

Última evidência: ready, sessão e bytes OK; NRestarts 0; user `scadacomm`; events apenas `SOAK START`; preflight não-disruptivo PASS.

Para `DONE`, relatório precisa conter:

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

Depois repetir production preflight com non-root requerido e `DISRUPTIVE=0`.

## 8. Referências externas — somente referência

`jgyates/genmon`: referência funcional/de domínio (controller abstraction, profiles, status/alarmes/eventos). Não copiar código GPL, Flask/jQuery, protocolo interno ou acesso Modbus direto.

`paulohspdev-cmyk/ProjetoGerador`: não modificar; referência visual/UX apenas.

## 9. `MON-001` — foundation RC Monitor — DONE

Estrutura:

```text
internal/monitor/
├── doc.go
├── model.go
├── model_test.go
├── provider.go
├── service.go
├── service_test.go
└── fake/
    ├── provider.go
    └── provider_test.go
```

Domínio: `Generator`, `ControllerRef`, `MetricKey`, `MetricValue` tipado, `Quality`, `CommunicationState`, `TelemetrySnapshot`, `Alarm`, `Event`, `ProviderHealth`, interface `Provider` e `Service` provider-independent.

Semântica obrigatória: zero real permanece zero; métrica ausente é omitida; stale preserva last-known + `quality=stale`; offline usa `quality=offline`; `NaN/Inf` rejeitados.

FakeProvider cobre online/offline/stale/alarme/recovery/cancelamento/not-found e ausência deliberada de `fuel.level`.

Evidência funcional:

```text
commit: fd73a4952c7de992075440180a959fcae1b62860
Gateway CI #122: SUCCESS
CodeQL #49: SUCCESS
```

Run #121 falhou somente por `gofmt`; corrigido e revalidado no #122.

## 10. Checker de continuidade

`scripts/check-project-state-updated.sh` valida schema, invariantes, checklist, tarefa atual, gate externo, production flag e freshness por ancestralidade: o handoff precisa ser o mesmo commit ou descendente da última mudança material da branch real do PR.

## 11. `MON-002` — API read-only — IN_PROGRESS

Commit material atual:

```text
2ed8224bb0c42f16e89ae4db6a6178962b00c6a1
```

Implementado e aguardando validação CI final:

```text
cmd/rc-monitor/main.go
cmd/rc-monitor/main_test.go
internal/monitor/httpapi/server.go
internal/monitor/httpapi/server_test.go
```

API planejada/implementada neste incremento:

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

Decisões do contrato HTTP:

- somente GET; outros métodos -> 405 + `Allow: GET`;
- erros JSON estáveis;
- generator inexistente -> 404;
- timeout provider -> 504;
- request cancelado -> 408;
- falha provider -> 502 sem vazar detalhe interno;
- métricas são serializadas como `value` escalar number/string/bool;
- zero real continua presente;
- métrica ausente continua ausente;
- health `/healthz` é liveness de processo;
- readiness exige provider `healthy`; degraded/unavailable -> 503;
- headers `Cache-Control: no-store` e `X-Content-Type-Options: nosniff`;
- servidor de desenvolvimento usa fake provider e bind default `127.0.0.1:18100`;
- bind não-loopback é rejeitado neste estágio;
- CI foi alterado para também compilar `cmd/rc-monitor` e executar `--version`.

Testes HTTP cobrem health/readiness, list/detail, telemetry scalar, métrica ausente, zero real, stale/offline, alarms/events, 404, 405, provider error, cancelamento e timeout.

**Status:** `MON-002` permanece `IN_PROGRESS` até CI/CodeQL do HEAD com este código passarem. Não promover por expectativa.

## 12. Próxima fila

`MON-003`: Controller Profile schema + profile sintético validado. Conceito inspirado no Genmon, implementação original:

```text
controllers/<fabricante>/<modelo>/
  manifest.json
  telemetry.json
  alarms.json
  ui.json
  rapid/template.xml
  rapid/channels.json
  tests/vectors.json
```

`MON-004`: `RapidScadaProvider` implementando `monitor.Provider` sem vazar detalhes Rapid ao domínio/API.

Frontend somente após modelo, API, profiles, Rapid provider e semântica `absent/stale/offline/quality` estabilizados.

## 13. Gates externos pendentes

- `SEM-001`: canais Rapid vinculados/valores recuperados pelo backend;
- HIL read-only por controladora/firmware/meio físico/VPN/modem;
- soak 24 h mínimo e 7 d alvo;
- comandos industriais continuam bloqueados até HIL/interlocks/autorização/auditoria.

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
| MON-002 | IN_PROGRESS | API read-only `/api/v1` + contrato HTTP; código criado, CI final pendente. |
| MON-003 | NEXT | Controller Profile schema + profile sintético validado. |
| MON-004 | TODO | `RapidScadaProvider` atrás de `monitor.Provider`. |
| MON-005 | TODO | Testar monitor contra Rapid real/sintético. |
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
Concluir MON-002:
- rodar CI/CodeQL do HEAD;
- corrigir qualquer falha;
- só então marcar MON-002 DONE;
- depois iniciar MON-003.
```

Durante `SOAK-001`, não reiniciar nem alterar a VM.

## 16. O que NÃO fazer

- não copiar código Genmon;
- não modificar ProjetoGerador;
- não colocar Modbus/register maps no Gateway;
- não transformar rc-monitor em bridge;
- não inventar zero;
- não habilitar command plane/writes industriais;
- não tocar na VM durante o soak;
- não declarar produção antes dos gates;
- não mesclar PR #2/#3 em `main` sem ordem explícita.

## 17. Regra de manutenção

Mudanças em `cmd/`, `internal/`, `configs/`, `catalog/`, `scripts/`, `systemd/`, `.github/workflows/`, `go.mod/go.sum` ou futuro backend/frontend exigem handoff atualizado depois da última mudança material.

# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/monitor-core -->
<!-- CURRENT_DEVELOPMENT_TASK: MON-002 -->
<!-- EXTERNAL_RUNNING_GATE: SOAK-001 -->
<!-- PRODUCTION_VALIDATED: false -->
<!-- PR2_MUST_REMAIN_DRAFT: true -->

> **REGRA OBRIGATÓRIA:** este é o documento canônico de continuidade do projeto. Qualquer pessoa, novo chat ou IA deve ler este arquivo **antes de alterar código, arquitetura, segurança, release, integração Rapid SCADA, backend ou frontend**. Mudança material deve atualizar este arquivo no mesmo conjunto de trabalho e passar `scripts/check-project-state-updated.sh`.

## 1. Procedimento obrigatório para continuar

Antes de qualquer alteração:

1. confirmar repositório `paulohspred/Gateway`, branch e HEAD reais;
2. ler este arquivo inteiro;
3. conferir o **Checklist canônico** e executar somente a próxima tarefa compatível com os gates aprovados;
4. não alterar `main` e não mesclar PR sem autorização explícita do proprietário;
5. não promover tarefa para `DONE` sem evidência reproduzível;
6. registrar aqui mudança, evidência, risco residual e próximo passo;
7. executar `bash scripts/check-project-state-updated.sh`;
8. nunca registrar segredos neste documento;
9. nunca declarar `production_validated` antes dos gates externos obrigatórios.

Status permitidos:

```text
DONE         concluído e comprovado
IN_PROGRESS  em execução
NEXT         próxima tarefa aprovada
TODO         planejado
BLOCKED      depende de gate externo
DEFERRED     conscientemente adiado
```

## 2. Arquitetura fixa — NÃO quebrar

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

Responsabilidades:

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

- RC Gateway transporta bytes, datagramas, frames e HID reports; não inventa mapa de registradores.
- Suporte de transporte não significa conversão semântica de protocolo.
- Modbus TCP pode atravessar o Gateway como bytes.
- Modbus RTU/ASCII pode atravessar streams adequados como payload bruto.
- Não existe conversão automática Modbus RTU <-> Modbus TCP.
- Não existe conversão automática ComAp Direct <-> Modbus.
- `length32be` é framing de transporte, não é Modbus.
- Gateway não é banco de telemetria, historian, MQTT broker ou SCADA.
- Rapid SCADA interpreta protocolo/aplicação e é o consumidor industrial atual.
- `rc-monitor` não deve reimplementar bridge Modbus, transporte ou register map físico do Gateway.
- O frontend nunca deve conhecer Function Code, registrador físico ou endereço Modbus; deve consumir métricas normalizadas.

## 3. Repositório, branches e PRs

Repositório de trabalho:

```text
github.com/paulohspred/Gateway
```

### `main`

- não alterar diretamente;
- não mesclar automaticamente;
- qualquer merge exige autorização explícita do proprietário.

### Gateway hardening

```text
branch: hardening/standalone-10x
HEAD registrado: 494c2a7e19cda62fecac0c142bc87091ca0ac061
PR: #2
base: main
base SHA histórico: e0f2ecdafc5ee065428b442575fee5a9d5ab0a11
```

PR #2 deve permanecer `open=true`, `draft=true`, `merged=false`. Não marcar ready nem fazer merge apenas porque CI está verde.

### RC Monitor

```text
branch: feature/monitor-core
base de criação: 494c2a7e19cda62fecac0c142bc87091ca0ac061
PR: #3
base do PR #3: hardening/standalone-10x
PR #3: draft
```

PR #3 existe para desenvolver a camada de monitoramento separadamente do PR #2. Não mesclar em `main`.

## 4. Estado do RC Gateway já implementado

O Gateway já possui TCP listen/connect/reverse TCP, TLS 1.3/mTLS, Unix stream/`unixpacket`, RS232/422/485 raw, UDP, SocketCAN/CAN-FD, Linux USB HID, framing explícito `length32be`, limites de concorrência, timeouts, allowlists, configuração JSON estrita, admin loopback-only, command plane desabilitado, CAN TX/HID write bloqueados por padrão, health/readiness/status/sessions/metrics, systemd notify/watchdog, installer/rollback, builds amd64/arm64, SBOM/provenance e gates CI de qualidade/segurança/stress/soak.

Transport support continua distinto de interpretação semântica.

## 5. Rapid SCADA — baseline

```text
Rapid SCADA: 6.4.7
upstream source commit analisado:
1fd36080c7830303f921672fdaee335a06e7ae50
```

Integração nativa atual:

- `DrvCnlBasic` como `TcpClient`;
- `Behavior=Master`;
- `ConnectionMode=Shared` quando apropriado;
- `DrvModbus` com `TransMode=TCP`, `RTU` ou `ASCII` conforme payload real;
- Gateway entrega transporte; Rapid SCADA interpreta o payload.

Documentos complementares:

- `docs/RAPID_SCADA_INTEGRATION.md`;
- `docs/GENERATOR_SCADA_PRODUCTION_READINESS.md`;
- `docs/SCADA_STACK_INSTALLER.md`;
- `docs/RUNBOOK.md`.

## 6. VM de homologação — estado comprovado

Ambiente:

```text
Ubuntu Server 24.04.3 LTS
kernel 6.8.0-139-generic x86_64
Rapid SCADA 6.4.7
ASP.NET Core Runtime 8.0.30
Nginx 1.24.0 Ubuntu
```

Gateway instalado na VM foi construído do source pin:

```text
ac5c98e047e752539e7844dfa7d9d3d69565a6e6
installed version: vm-ac5c98e047e7
```

**Não afirmar que a VM executa o binário 494c2a7.** O hardening branch avançou depois; a VM usa `ac5c...` mais configurações/hardenings aplicados durante homologação.

Serviços validados: `rc-gateway`, `scadaagent6`, `scadaserver6`, `scadacomm6`, `scadaweb6`, `nginx`, firewall Rapid interno e simulador Modbus quando ativo.

Portas Rapid 10000/10002 estão protegidas por firewall persistente para loopback. O bloqueio por interface não-loopback e host externo foi comprovado.

### Rapid Communicator non-root

Perfil homologado:

```text
User=scadacomm
Group=scadacomm
NoNewPrivileges=yes
PrivateTmp=yes
ProtectHome=yes
CapabilityBoundingSet=
AmbientCapabilities=
UMask=0027
```

Diretório gravável validado:

```text
/var/log/scada/ScadaComm/Log
owner/group: scadacomm:scadacomm
mode dir: 0750
mode files: 0640
```

Sobreviveu a cold boot com polling/logs. Pendente codificar isso no installer de forma segura/idempotente e sem `chown` amplo de `/opt/scada`.

## 7. Integração VM já aprovada

### Modbus TCP sintético via Gateway — PASS

```text
Python Modbus client
  -> 127.0.0.1:25020
  -> RC Gateway transparent TCP bridge
  -> 127.0.0.1:15020
  -> Python Modbus TCP simulator
```

FC03 retornou:

```text
[2300, 2310, 2290, 5000, 1500, 805, 1250, 150, 1500, 1]
```

### Rapid SCADA -> Gateway -> simulador — PASS de transporte

Linha sintética Rapid `99`: `DrvCnlBasic`, TCP `127.0.0.1:25020`, `DrvModbus`, `TransMode=TCP`, Unit ID 1, `CmdEnabled=false`, template read-only e `<Cmds />` vazio.

Rapid originou FC03 repetidos e recebeu respostas via Gateway.

**Limite da evidência:** line/device usam `isBound=false`; isso prova transporte/polling Modbus, mas ainda não prova E2E semântico de canais Rapid vinculados, armazenados e recuperados pelo backend.

### Recovery/preflight — PASS

Comprovados: field outage/recovery, network blackhole/recovery sem restart, restart Rapid, restart Gateway, cold boot, cold boot pós-non-root e production preflight com `RAPID_SCADA_REQUIRE_NON_ROOT=1`.

## 8. Soak sintético 24 h — EM EXECUÇÃO

Task `SOAK-001`:

```text
inicio:   2026-09-05T23:47:29Z
intervalo: 30 s
duração:  86400 s
fim esperado: 2026-09-06T23:47:29Z
```

Coletor:

```text
rc-scada-soak.service
/usr/local/sbin/rc-scada-soak-24h.sh
/var/log/rc-scada-soak/
```

Última observação registrada: `ready=1`, state `active`, sessão presente, bytes crescendo nos dois sentidos, Gateway/ScadaComm `NRestarts=0`, usuário `scadacomm`, events apenas `SOAK START`, preflight não-disruptivo PASS.

Para `SOAK-001 = DONE`, relatório final deve existir e conter no mínimo:

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

Depois repetir production preflight com `RAPID_SCADA_REQUIRE_NON_ROOT=1` e `RAPID_SCADA_DISRUPTIVE=0`.

## 9. Referências externas — SOMENTE referência

### `jgyates/genmon`

Usar como referência funcional/de domínio: separação núcleo/UI, abstração de controladoras, controller profiles, estado comum de gerador, alarmes, eventos, manutenção e consumidores de interface central.

Não copiar Flask/jQuery, protocolo interno, acesso Modbus/serial direto ou código. Genmon é GPL v2; nossa implementação é original.

### `paulohspdev-cmyk/ProjetoGerador`

Não modificar. Usar apenas como referência visual/UX: identidade, shell, cards, gauges/power-flow, tabelas, filtros, busca, responsividade, tema e estados `N/D/stale/quality`.

Não herdar backend antigo, bridge Modbus própria, mega-contexts ou dados fake/legacy.

## 10. RC Monitor — foundation implementada (`MON-001`)

`MON-001` foi implementada na branch `feature/monitor-core` sem tocar na VM do soak e sem frontend.

### Estrutura criada

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

### Contrato canônico criado

O domínio agora possui:

- `Generator` e `ControllerRef`;
- `MetricKey` com namespace canônico;
- `MetricValue` estritamente tipado como número, texto ou booleano;
- `Quality`: `good`, `stale`, `offline`, `bad`, `unknown`;
- `CommunicationState`;
- `TelemetrySnapshot`;
- `Alarm`, `Event` e `ProviderHealth`;
- validação de valor exatamente de um tipo;
- rejeição de `NaN`/`Inf`;
- zero numérico preservado como valor real;
- métrica ausente representada por ausência no mapa, nunca por zero inventado.

Namespaces iniciais incluem `engine.*`, `generator.*`, `mains.*`, `controller.mode`, `breaker.*`, `battery.voltage` e `fuel.level`.

### Provider boundary

Foi criada uma única interface `Provider` com operações read-only para inventário, gerador, telemetria, alarmes, eventos e health. `Service` depende apenas dessa interface. Isso é a fronteira que o futuro `RapidScadaProvider` deverá implementar sem vazar detalhes Rapid para a API/domínio.

### FakeProvider

Provider sintético determinístico cobre:

- online;
- stale com last-known preservado;
- offline com `quality=offline`;
- alarme crítico ativo;
- recovery offline -> online;
- contexto cancelado;
- gerador inexistente com sentinel estável;
- `fuel.level` deliberadamente ausente para provar que ausência não vira `0`.

Ele não implementa Modbus, não é historian e não altera o Gateway.

### Evidência de validação de `MON-001`

Commit funcional formatado:

```text
fd73a4952c7de992075440180a959fcae1b62860
```

Sobre esse commit:

```text
Gateway CI #122: SUCCESS
CodeQL #49:       SUCCESS
```

No CI #122 passaram `gofmt`, module integrity, `go vet`, Staticcheck, unit/integration tests (incluindo `internal/monitor/...`), coverage threshold, race detector, build/config validation, stress/churn, mini-soak e release/security gates existentes.

O primeiro run #121 falhou somente no gate de `gofmt` de `internal/monitor/model.go`; a diferença foi corrigida em `fd73a495...` e o run #122 passou integralmente. Registrar essa falha é intencional para preservar rastreabilidade.

### Checker de continuidade reforçado

`scripts/check-project-state-updated.sh` agora exige que o commit mais recente de `docs/PROJECT_STATE.md` seja o mesmo ou descendente da última mudança material na branch real do PR. Isso evita que uma atualização antiga do handoff satisfaça indefinidamente o freshness gate.

### Risco residual após `MON-001`

Ainda não existe API HTTP do rc-monitor, Controller Profile schema, RapidScadaProvider, persistência/configuração de produto, systemd do monitor nem autenticação. A foundation é somente read-only e provider-independent.

## 11. Próxima etapa — `MON-002`

Implementar API read-only `/api/v1` e testes de contrato HTTP sobre `monitor.Service` + `FakeProvider`.

Contrato planejado:

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

Regras para `MON-002`:

- somente leitura;
- sem START/STOP/transfer/reset/setpoint;
- sem Modbus/parser/register map;
- sem frontend;
- API deve distinguir 404, erro interno/provider e contexto cancelado/timeout;
- JSON precisa preservar `0` real e não inventar métrica ausente;
- health/readiness devem ter semântica explícita;
- testes HTTP devem usar `httptest` e provider determinístico;
- não tocar na VM do soak.

## 12. Controller Profiles e Rapid provider — etapas posteriores

`MON-003`: schema original de Controller Profiles, inspirado somente conceitualmente no Genmon:

```text
controllers/
  <fabricante>/
    <modelo>/
      manifest.json
      telemetry.json
      alarms.json
      ui.json
      rapid/
        template.xml
        channels.json
      tests/
        vectors.json
```

Profiles declaram capacidades reais; frontend não presume medidas universais.

`MON-004`: `RapidScadaProvider` implementando exatamente a interface `monitor.Provider`. O domínio/API não deve saber se internamente a fonte usa DLL, socket, API ou outro mecanismo Rapid suportado.

## 13. Frontend — somente depois do backend estabilizado

Não iniciar integração visual antes de:

1. modelo canônico aprovado;
2. FakeProvider testado;
3. API `/api/v1` testada;
4. Controller Profile schema validado;
5. RapidScadaProvider integrado/testado;
6. `absent/stale/offline/quality` estabilizados.

Primeiras telas futuras: Login, Visão Geral, Sites/Clientes, Geradores, Detalhe do Gerador, Alarmes/Eventos, Usuários e Saúde do Sistema.

## 14. Gates externos ainda pendentes

### Semântica Rapid E2E

Comprovar valores reais/sintéticos vinculados a canais Rapid e recuperados inequivocamente pelo backend. Transporte já está provado; binding semântico ainda não.

### HIL

Obrigatório por modelo/firmware/caminho físico: controladora, modem/serial server quando aplicável, VPN/rede, RS232/422/485/multidrop, USB HID/ComAp quando usado, power cycle/reconnect e validação de mapa/escala/qualidade.

Primeiro HIL é read-only. Comandos industriais continuam bloqueados até interlocks, permissões e auditoria próprios.

### Promotion chain

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
| GW-001 | DONE | Gateway bridge-first implementado e coberto por testes automatizados de transporte/segurança. |
| GW-002 | DONE | Integração de transporte Rapid SCADA 6.4.7 documentada e testada em software. |
| GW-003 | DONE | Firewall interno Rapid 10000/10002 persistente e validado na VM. |
| VM-001 | DONE | Stack Gateway + Rapid SCADA instalada e serviços básicos validados. |
| VM-002 | DONE | Rapid -> Gateway -> simulador Modbus FC03 comprovado. |
| VM-003 | DONE | Field outage/recovery automático comprovado. |
| VM-004 | DONE | Network blackhole/recovery comprovado sem restart de serviços. |
| VM-005 | DONE | Cold boot de stack comprovado. |
| VM-006 | DONE | Rapid Communicator non-root comprovado no perfil read-only atual. |
| VM-007 | DONE | Cold boot pós-non-root comprovado com polling e logs. |
| HARD-001 | TODO | Codificar no installer a configuração non-root validada, segura/idempotente e sem chown amplo. |
| SOAK-001 | IN_PROGRESS | Soak sintético 24 h; concluir somente após `.report` PASS e acceptance final. |
| SEM-001 | TODO | Provar E2E semântico Rapid com canais vinculados/valores esperados. |
| MON-001 | DONE | Foundation `rc-monitor`, modelo canônico, Provider/Service, FakeProvider e testes; CI #122 + CodeQL #49 PASS. |
| MON-002 | NEXT | Implementar API read-only `/api/v1` e testes de contrato HTTP. |
| MON-003 | TODO | Implementar Controller Profile schema + profile sintético validado. |
| MON-004 | TODO | Implementar `RapidScadaProvider` atrás da interface `monitor.Provider`. |
| MON-005 | TODO | Testar rc-monitor contra Rapid SCADA real/sintético sem alterar invariantes do Gateway. |
| MON-006 | TODO | Hardening rc-monitor: non-root, systemd, config, logs, health/readiness e observabilidade. |
| MON-007 | TODO | Soak e recovery/restart gates próprios do rc-monitor. |
| HIL-001 | BLOCKED | HIL read-only com primeira controladora real + mapa/escala/qualidade. |
| HIL-002 | BLOCKED | HIL de modem/VPN/meio físico real aplicável. |
| CMD-001 | DEFERRED | START/STOP/etc.; somente após HIL/interlocks/autorização/auditoria. |
| UI-001 | TODO | Definir contrato frontend final após API estabilizar. |
| UI-002 | TODO | Implementar shell visual novo usando ProjetoGerador apenas como referência UX. |
| UI-003 | TODO | Geradores/detalhe/alarmes/sites/health com dados reais da API e testes E2E. |
| UI-004 | TODO | Adicionar Playwright/Vitest/RTL e fluxos offline/stale/N-D/permissão. |
| REL-001 | TODO | Confirmar política/proteção de `main` antes de história de produção. |
| REL-002 | TODO | Atualizar documentação/release/installers quando rc-monitor virar componente suportado. |
| PROD-001 | BLOCKED | `production_validated`: exige VM + semântica + HIL + soak + aprovação operacional. |
<!-- CHECKLIST_END -->

## 16. Próximo passo exato

Enquanto `SOAK-001` continua rodando, desenvolvimento é permitido somente no GitHub/ambiente separado, sem reiniciar ou alterar a VM do soak.

```text
MON-002
Implementar API read-only /api/v1 sobre monitor.Service + FakeProvider.
Adicionar testes de contrato HTTP.
Não implementar frontend.
Não implementar comandos industriais.
Não tocar em main.
Não alterar a VM de soak.
```

## 17. O que NÃO fazer

- não copiar código do Genmon;
- não modificar `ProjetoGerador`;
- não colocar Modbus parser/register maps no core Gateway;
- não transformar rc-monitor em bridge;
- não inventar `0` para métrica ausente;
- não habilitar command plane do Gateway;
- não habilitar writes industriais no primeiro HIL;
- não reiniciar a VM durante o soak atual;
- não declarar tarefa `DONE` por expectativa;
- não declarar produção antes dos gates externos;
- não mesclar PR #2, PR #3 ou `feature/monitor-core` em `main` sem ordem explícita.

## 18. Regra de manutenção deste handoff

Mudança material em `cmd/`, `internal/`, `configs/`, `catalog/`, `scripts/`, `systemd/`, `.github/workflows/`, `go.mod/go.sum` ou futuro backend/frontend deve ser seguida por atualização deste arquivo.

O checker valida estrutura, invariantes, checklist, ponteiros de tarefa e ancestralidade do handoff em relação à última mudança material.

Ao fechar tarefa:

1. mudar status no checklist;
2. registrar evidência objetiva;
3. indicar risco residual;
4. selecionar próximo `NEXT`;
5. rodar checker/CI;
6. preservar histórico crítico para o próximo mantenedor/chat/IA.

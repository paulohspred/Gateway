# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/monitor-core -->
<!-- CURRENT_DEVELOPMENT_TASK: MON-001 -->
<!-- EXTERNAL_RUNNING_GATE: SOAK-001 -->
<!-- PRODUCTION_VALIDATED: false -->
<!-- PR2_MUST_REMAIN_DRAFT: true -->

> **REGRA OBRIGATÓRIA:** este é o documento canônico de continuidade do projeto. Qualquer pessoa, novo chat ou IA deve ler este arquivo **antes de alterar código, arquitetura, segurança, release, integração Rapid SCADA, backend ou frontend**. Mudança material deve atualizar este arquivo no mesmo conjunto de mudanças e deve passar `scripts/check-project-state-updated.sh`.

## 1. Como continuar o projeto sem perder contexto

Antes de qualquer alteração:

1. confirmar o repositório `paulohspred/Gateway`, a branch e o HEAD reais no GitHub;
2. ler este arquivo inteiro;
3. conferir a tabela **Checklist canônico** e executar somente a próxima tarefa compatível com os gates já aprovados;
4. não alterar `main` e não mesclar PR sem autorização explícita do proprietário;
5. não promover uma tarefa para `DONE` sem evidência reproduzível (teste, log, CI, HIL ou relatório de soak conforme o caso);
6. registrar neste arquivo: mudança, evidência, risco residual e próximo passo;
7. executar `bash scripts/check-project-state-updated.sh` antes de considerar o trabalho concluído;
8. nunca registrar senha, token, chave privada ou segredo neste documento;
9. nunca declarar `production_validated` enquanto os gates externos obrigatórios permanecerem pendentes.

Status permitidos no checklist:

```text
DONE         concluído e comprovado
IN_PROGRESS  em execução neste momento
NEXT         próxima tarefa aprovada para começar
TODO         planejado, ainda não iniciado
BLOCKED      depende de requisito/gate externo
DEFERRED     conscientemente adiado
```

## 2. Decisões de arquitetura que NÃO podem ser quebradas

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

Responsabilidades fixas:

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

### Invariantes

- RC Gateway transporta bytes, datagramas, frames e HID reports; não inventa mapa de registradores.
- Suporte de transporte **não** significa conversão semântica de protocolo.
- Modbus TCP pode atravessar o Gateway como bytes.
- Modbus RTU/ASCII pode atravessar streams adequados como payload bruto.
- Não existe conversão automática Modbus RTU <-> Modbus TCP.
- Não existe conversão automática ComAp Direct <-> Modbus.
- `length32be` é framing de transporte, não é Modbus.
- Gateway não é banco de telemetria, historian, MQTT broker ou SCADA.
- Rapid SCADA interpreta protocolo/aplicação e é o consumidor industrial atual.
- O futuro backend (`rc-monitor`) não deve reimplementar bridge Modbus nem competir com o Gateway.
- O frontend nunca deve conhecer registrador físico, Function Code ou endereço Modbus; deve consumir métricas normalizadas da API.

## 3. Repositório, branches e regra de merge

Repositório proprietário de trabalho:

```text
github.com/paulohspred/Gateway
```

### `main`

- não alterar diretamente;
- não mesclar automaticamente;
- qualquer merge exige autorização explícita do proprietário.

### Hardening do Gateway

```text
branch: hardening/standalone-10x
HEAD registrado: 494c2a7e19cda62fecac0c142bc87091ca0ac061
PR: #2
base: main
base SHA histórico: e0f2ecdafc5ee065428b442575fee5a9d5ab0a11
```

PR #2 deve permanecer:

```text
open:   true
draft:  true
merged: false
```

Não marcar como ready e não fazer merge somente porque CI está verde.

### Desenvolvimento da próxima camada

```text
branch: feature/monitor-core
base de criação: 494c2a7e19cda62fecac0c142bc87091ca0ac061
```

Esta branch foi criada para desenvolver o backend/monitor sem misturar a implementação inicial com o PR de hardening. No momento deste snapshot, **nenhum código do rc-monitor foi implementado ainda**; somente a branch e este plano de continuidade existem.

## 4. Estado do RC Gateway já implementado

O core do Gateway já possui, entre outros:

- TCP listen/connect e reverse TCP;
- TLS 1.3 e mTLS;
- Unix stream e Unix `SOCK_SEQPACKET` (`unixpacket`);
- RS232/RS422/RS485 raw;
- UDP com sessões/limites;
- SocketCAN clássico e CAN-FD;
- Linux USB HID via `hidraw` com seleção estável VID/PID/serial;
- framing explícito `length32be` para transições packet <-> stream;
- limites globais e por tunnel;
- timeouts, half-close drain, keepalive/NODELAY e CIDR allowlists;
- configuração JSON estrita;
- admin HTTP loopback-only;
- command plane desabilitado;
- CAN TX e HID write desabilitados por padrão;
- health/readiness/status/sessions/metrics;
- systemd `Type=notify` + watchdog;
- installer/rollback health-gated;
- builds Linux amd64/arm64, checksums, SBOM e provenance/attestation;
- CI com lint, vet, Staticcheck, testes, coverage, race, stress, churn, mini-soak, govulncheck e CodeQL;
- contratos automatizados de transporte para Rapid SCADA/Modbus.

## 5. Rapid SCADA — baseline e contrato

Baseline homologado para integração:

```text
Rapid SCADA: 6.4.7
upstream source commit analisado:
1fd36080c7830303f921672fdaee335a06e7ae50
```

Integração nativa usada:

- `DrvCnlBasic` como `TcpClient`;
- `Behavior=Master`;
- `ConnectionMode=Shared` quando apropriado;
- `DrvModbus` com `TransMode=TCP`, `RTU` ou `ASCII` conforme o payload real;
- Gateway entrega transporte; Rapid SCADA interpreta o payload.

Documentos especializados que complementam este estado:

- `docs/RAPID_SCADA_INTEGRATION.md`;
- `docs/GENERATOR_SCADA_PRODUCTION_READINESS.md`;
- `docs/SCADA_STACK_INSTALLER.md`;
- `docs/RUNBOOK.md`.

## 6. VM de homologação — estado comprovado

Ambiente atual de laboratório:

```text
Ubuntu Server 24.04.3 LTS
kernel 6.8.0-139-generic x86_64
Rapid SCADA 6.4.7
ASP.NET Core Runtime 8.0.30
Nginx 1.24.0 Ubuntu
```

### Importante sobre a versão realmente instalada na VM

O binário Gateway atualmente instalado na VM foi construído do source pin:

```text
ac5c98e047e752539e7844dfa7d9d3d69565a6e6
installed version: vm-ac5c98e047e7
```

O branch de hardening avançou depois até `494c2a7...`. Portanto **não afirmar que a VM está executando binário 494c2a7**. A VM possui o binário `ac5c...` mais hardenings/configurações aplicados depois durante a homologação.

### Serviços comprovados ativos/persistentes

- `rc-gateway.service`;
- `scadaagent6.service`;
- `scadaserver6.service`;
- `scadacomm6.service`;
- `scadaweb6.service`;
- `nginx.service`;
- `rc-scada-internal-firewall.service`;
- simulador Modbus de laboratório quando o perfil sintético está ativo.

### Segurança Rapid interna

As portas Rapid internas TCP `10000` e `10002` são upstream wildcard, mas foram isoladas por firewall persistente para permitir somente loopback. Foi comprovado bloqueio pelo endereço não-loopback, inclusive a partir de host externo de laboratório, mantendo SSH funcional.

### Rapid Communicator non-root

O `scadacomm6.service` foi homologado manualmente com:

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

Somente o diretório efetivo de logs do Communicator foi tornado gravável pelo usuário dedicado:

```text
/var/log/scada/ScadaComm/Log
owner/group: scadacomm:scadacomm
mode dir: 0750
mode files: 0640
```

O hardening non-root sobreviveu a reboot completo e o Communicator voltou executando como UID/GID dedicado, com polling e logs funcionando.

**Pendente:** transformar esse hardening manual em comportamento suportado/idempotente do installer ou opção explicitamente homologada no repositório. Não fazer `chown` amplo de `/opt/scada`.

## 7. Testes de integração VM já aprovados

### Modbus TCP sintético através do Gateway — PASS

Topologia comprovada:

```text
Python Modbus client
  -> 127.0.0.1:25020
  -> RC Gateway transparent TCP bridge
  -> 127.0.0.1:15020
  -> Python Modbus TCP simulator
```

Leitura FC03 retornou os 10 registradores esperados:

```text
[2300, 2310, 2290, 5000, 1500, 805, 1250, 150, 1500, 1]
```

Foi observado request Modbus TCP de 12 bytes e response de 29 bytes, com contadores de sessão do Gateway coerentes.

### Rapid SCADA -> Gateway -> simulador — PASS de transporte

Foi criada uma linha sintética Rapid SCADA isolada (`Line 99`) com:

- `DrvCnlBasic` TCP client para `127.0.0.1:25020`;
- `DrvModbus`;
- `TransMode=TCP`;
- Unit ID 1;
- somente leitura;
- `CmdEnabled=false`;
- template com 10 Holding Registers read-only;
- `<Cmds />` vazio.

Rapid originou FC03 repetidos e recebeu as respostas via Gateway.

**Limite da evidência:** essa linha/device usa `isBound=false`; portanto o teste prova transporte e parser Modbus em operação, mas ainda **não prova E2E semântico de canais Rapid vinculados/armazenados/exibidos**.

### Failure/recovery — PASS

Comprovado:

- parada do endpoint de campo -> sessão cai -> Gateway permanece ready -> endpoint volta -> sessão recupera automaticamente;
- blackhole de rede específico -> requests continuam saindo, respostas param -> sem restart do Gateway/Rapid -> remoção do blackhole -> respostas retomam;
- restart Rapid -> recuperação;
- restart Gateway -> recuperação;
- cold boot de stack completo -> serviços, firewall e polling retornam;
- cold boot **após non-root** -> `scadacomm` continua non-root, logs e polling continuam funcionais.

### Production preflight sintético — PASS

`rapid-scada-production-acceptance.sh` já passou, inclusive anteriormente com gate disruptivo e depois com:

```text
RAPID_SCADA_REQUIRE_NON_ROOT=1
```

Esse preflight não substitui HIL nem soak longo.

## 8. Soak sintético de 24 h — EM EXECUÇÃO

Task ID: `SOAK-001`.

Início registrado:

```text
2026-09-05T23:47:29Z
intervalo: 30 s
duração planejada: 86400 s
fim esperado: 2026-09-06T23:47:29Z
```

Coletor em execução como:

```text
rc-scada-soak.service
/usr/local/sbin/rc-scada-soak-24h.sh
/var/log/rc-scada-soak/
```

Primeiras amostras comprovaram:

- `ready=1`;
- state `active`;
- `sessions=1`;
- bytes crescendo nos dois sentidos;
- Gateway `NRestarts=0`;
- Rapid `NRestarts=0`;
- usuário `scadacomm`;
- `.events` somente com `SOAK START` na última observação;
- production preflight non-disruptivo passou durante o período.

### Critério para `SOAK-001 = DONE`

O relatório final deve existir e, no mínimo, registrar:

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

Além disso, executar novamente o production preflight com `RAPID_SCADA_REQUIRE_NON_ROOT=1` e `RAPID_SCADA_DISRUPTIVE=0`.

Até esse relatório final existir, o soak permanece `IN_PROGRESS` e não pode ser contabilizado como concluído.

## 9. Referências externas estudadas — SOMENTE referência

### `jgyates/genmon`

Usar como referência funcional/de domínio, não como base de código.

Conceitos úteis observados:

- separação entre núcleo de monitoramento e consumidores/UI;
- abstração de controladoras;
- controller profiles/configs por fabricante/modelo;
- modelo comum de status, engine, utility, transfer, battery, fuel, maintenance, alarms, logs e commands;
- UI orientada às capacidades reais da controladora;
- extensões/add-ons consumindo uma interface central.

Não copiar implementação Flask/jQuery, daemon/protocolo interno, acesso Modbus/serial direto ou parser de controladora. Genmon é GPL v2; nossa implementação deve ser original e compatível com a licença/propriedade do nosso projeto.

### `paulohspdev-cmyk/ProjetoGerador`

Não modificar esse repositório. Usar somente como referência visual/UX do frontend criado anteriormente pelo proprietário.

Partes visuais/conceituais que podem inspirar o frontend novo:

- identidade visual;
- sidebar/topbar;
- cards de gerador;
- gauges/power-flow;
- filtros, busca, tabela e layouts responsivos;
- dark/light theme;
- componentes UI;
- tratamento visual de `N/D`, stale e qualidade.

Não herdar automaticamente o backend antigo, a bridge Modbus própria, mega-contexts, dados fake/legacy ou toda a quantidade de telas antigas.

## 10. Plano do RC Monitor — próximo desenvolvimento

Task atual planejada: `MON-001` (`NEXT`).

O backend será criado do zero no nosso repositório, inicialmente sem frontend e sem tocar na VM em soak.

### Foundation alvo

Serviço separado do `rc-gateway`, inicialmente read-only:

```text
rc-monitor
├── health/readiness
├── inventário de geradores
├── estado do gerador
├── telemetria normalizada
├── alarmes
├── eventos
├── saúde de comunicação
└── controller profiles
```

Primeiro contrato HTTP previsto:

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

### Provider architecture

Começar com provider sintético determinístico:

```text
FakeProvider ---------+
                      +--> Monitor Core --> API
RapidScadaProvider ---+
```

O `FakeProvider` deve testar, no mínimo:

- online;
- offline;
- telemetria válida;
- métrica ausente;
- stale/last-known;
- alarme ativo;
- recuperação de comunicação.

Depois implementar `RapidScadaProvider` com a mesma interface. O restante do backend não deve conhecer detalhes de DLL/socket/API usados para obter dados Rapid.

### Modelo canônico de métricas

Exemplos de namespace:

```text
engine.rpm
engine.oil_pressure
engine.coolant_temperature
engine.run_hours

generator.voltage_l1
generator.voltage_l2
generator.voltage_l3
generator.frequency
generator.current_l1
generator.power_kw
generator.power_kva
generator.power_kvar
generator.power_factor

mains.voltage_l1
mains.frequency

controller.mode
breaker.gcb
breaker.mcb
battery.voltage
fuel.level
```

Regra semântica obrigatória:

```text
valor real      -> value
métrica ausente -> absent/null, nunca zero inventado
stale           -> último valor + stale
offline         -> qualidade offline
```

### Controller Profiles

Inspirados no conceito do Genmon, mas implementados do zero e adaptados ao Rapid SCADA:

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

Profiles devem declarar capacidades, não fazer o frontend presumir medidas universais.

## 11. Frontend — somente depois do backend/API estabilizados

Não iniciar refactor visual antes de:

1. modelo canônico aprovado;
2. FakeProvider testado;
3. API `/api/v1` testada;
4. RapidScadaProvider integrado e testado;
5. contratos de `absent/stale/offline/quality` estabilizados.

Primeiro conjunto de telas do produto novo:

```text
Login
Visão Geral
Sites/Clientes
Geradores
Detalhe do Gerador
Alarmes/Eventos
Usuários
Saúde do Sistema
```

Telas como manutenção avançada, relatórios, mapa, automação, webhooks, MQTT/OPC UA e integrações ERP/BMS ficam para fases posteriores.

## 12. Gates ainda pendentes antes de produção

### Semântica Rapid E2E

Pendente comprovar dados da controladora/simulador realmente vinculados a canais Rapid SCADA e recuperados de maneira inequívoca pela integração backend. Log detalhado pode ser evidência de parser, mas não substitui canal vinculado/armazenado quando esse for o critério.

### HIL real

Obrigatório por modelo/firmware e caminho físico real:

- controladora real;
- modem/serial server quando aplicável;
- VPN/rede real;
- RS232/422/485 e multidrop quando usados;
- USB HID/ComAp quando usado;
- power cycle/reconnect;
- validação de mapa, escala e qualidade.

Primeiro HIL deve ser read-only. START/STOP/transfer/reset/setpoint ficam bloqueados até validação própria de interlocks, permissões, auditoria e comportamento da controladora.

### Soak

- mínimo: 24 h;
- alvo: 7 dias antes de promoção ampla;
- soak sintético não substitui HIL.

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

`production_validated=false` neste snapshot.

## 13. Checklist canônico

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
| HARD-001 | TODO | Codificar no installer a configuração non-root validada, de forma segura/idempotente e sem chown amplo. |
| SOAK-001 | IN_PROGRESS | Soak sintético 24 h; só concluir após `.report` PASS e acceptance final. |
| SEM-001 | TODO | Provar E2E semântico Rapid com canais vinculados/valores esperados, não apenas transporte. |
| MON-001 | NEXT | Criar foundation `rc-monitor`, modelo canônico, FakeProvider e testes; sem frontend. |
| MON-002 | TODO | Implementar API read-only `/api/v1` e testes de contrato HTTP. |
| MON-003 | TODO | Implementar Controller Profile schema + profile sintético validado. |
| MON-004 | TODO | Implementar `RapidScadaProvider` atrás da interface do provider. |
| MON-005 | TODO | Testar rc-monitor contra Rapid SCADA real/sintético sem alterar invariantes do Gateway. |
| MON-006 | TODO | Hardening do rc-monitor: identity non-root, systemd, config, logs, health, readiness e observabilidade. |
| MON-007 | TODO | Soak do rc-monitor e recovery/restart gates próprios. |
| HIL-001 | BLOCKED | HIL read-only com primeira controladora real + mapa/escala/qualidade. |
| HIL-002 | BLOCKED | HIL de modem/VPN/meio físico real aplicável. |
| CMD-001 | DEFERRED | Comandos industriais START/STOP/etc.; somente depois de HIL/interlocks/autorização/auditoria. |
| UI-001 | TODO | Definir contrato frontend final após API estabilizar. |
| UI-002 | TODO | Implementar shell visual novo usando ProjetoGerador apenas como referência UX. |
| UI-003 | TODO | Geradores/detalhe/alarmes/sites/health com dados reais da API e testes E2E. |
| UI-004 | TODO | Adicionar Playwright/Vitest/RTL e fluxos offline/stale/N-D/permissão. |
| REL-001 | TODO | Confirmar política/proteção de `main` antes de história de produção. |
| REL-002 | TODO | Atualizar documentação/release/installers quando rc-monitor virar componente suportado. |
| PROD-001 | BLOCKED | `production_validated`: exige gates VM + semântica + HIL + soak + aprovação operacional. |
<!-- CHECKLIST_END -->

## 14. Próximo passo exato

Enquanto `SOAK-001` continua rodando, é permitido desenvolver **somente no GitHub/ambiente separado**, sem reiniciar ou alterar a VM do soak.

Próxima tarefa de código aprovada:

```text
MON-001
Criar foundation do rc-monitor + modelo canônico + FakeProvider + testes.
Não implementar frontend.
Não implementar comandos industriais.
Não tocar em main.
Não alterar a VM de soak.
```

Antes de iniciar `MON-001`, confirmar que esta branch ainda deriva do hardening esperado e que nenhuma mudança externa alterou os invariantes acima.

## 15. O que NÃO fazer

- não copiar código do Genmon para o projeto;
- não modificar `ProjetoGerador`;
- não colocar Modbus parser ou register maps no core do Gateway;
- não transformar rc-monitor em outra bridge;
- não inventar `0` quando uma métrica estiver ausente;
- não habilitar command plane do Gateway;
- não habilitar writes industriais no primeiro HIL;
- não reiniciar a VM enquanto o soak atual estiver em execução;
- não atualizar status para `DONE` apenas por expectativa;
- não declarar 100% produção antes dos gates externos;
- não mesclar PR #2 nem `feature/monitor-core` em `main` sem ordem explícita.

## 16. Regra de manutenção deste handoff

Toda mudança material em:

```text
cmd/
internal/
configs/
catalog/
scripts/
systemd/
.github/workflows/
go.mod / go.sum
futuro backend/frontend
```

deve atualizar `docs/PROJECT_STATE.md` no mesmo conjunto de mudanças.

Ao fechar uma tarefa:

1. mudar seu status no checklist;
2. registrar evidência objetiva na seção correspondente;
3. indicar risco residual;
4. selecionar o próximo `NEXT`;
5. rodar o checker;
6. nunca apagar histórico crítico que um futuro mantenedor precisa para entender por que a decisão foi tomada.

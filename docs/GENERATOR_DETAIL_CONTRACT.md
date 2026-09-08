# RC Monitor — contrato da tela Detalhe do Gerador

Status: **UI-001 / layout e dados congelados antes de React**.

Este documento define a primeira tela principal do frontend novo. O objetivo é impedir divergência entre referência visual, implementação e dados realmente disponíveis.

A implementação deve seguir este contrato. Mudanças de região, hierarquia, dado ou comportamento precisam alterar este documento antes do código.

## 1. Finalidade operacional

Ao abrir um gerador, o operador deve conseguir responder rapidamente, nesta ordem:

1. estou olhando o ativo correto?
2. a cadeia de comunicação está atual?
3. qual é o estado da rede?
4. qual é o estado do motor/gerador?
5. MCB/GCB/ATS estão em qual estado conhecido?
6. o gerador está produzindo e com quais grandezas?
7. motor/ECU apresentam condição anormal?
8. há alarme ativo?
9. quais eventos recentes explicam a situação?
10. o dado mostrado é atual, stale, offline, bad, unknown ou ausente?

A tela não deve tentar responder tarefas de manutenção, gestão comercial, commissioning ou comando remoto dentro da área operacional.

## 2. Escopo v1

A primeira versão é **100% read-only**.

Não entram como controles funcionais:

```text
START
STOP
RESET
AUTO
TEST
MCB close/open
GCB close/open
TRANSFER
setpoints
acknowledge alarm
```

Essas ações não devem ser mostradas como botões aparentemente utilizáveis. Espaços reservados para funções futuras também devem ser evitados se causarem expectativa operacional.

## 3. Endpoints atuais que sustentam a tela

A tela pode usar apenas contratos reais do RC Monitor:

```text
GET /api/v1/generators/{id}
GET /api/v1/generators/{id}/telemetry
GET /api/v1/generators/{id}/alarms
GET /api/v1/generators/{id}/events
GET /api/v1/system/health
```

### `GET /api/v1/generators/{id}`

Fonte para:
- `id`;
- `name`;
- `siteId`;
- fabricante/modelo da controladora;
- firmware/hardware/serial quando disponíveis;
- `GeneratorSpec` quando cadastrado: potência nominal, tensão nominal, frequência nominal, RPM nominal e número de fases.

### `GET /telemetry`

Fonte para:
- `generatorId`;
- `capturedAt`;
- `communication`;
- mapa de métricas;
- `value`;
- `unit`;
- `quality`;
- `observedAt`.

### `/alarms`

Fonte para:
- código;
- severidade;
- mensagem;
- ativo/inativo;
- `raisedAt`;
- `clearedAt` quando houver.

### `/events`

Fonte para:
- tipo;
- mensagem;
- `occurredAt`.

### `/system/health`

É saúde do provider/sistema, não substitui o estado individual do gerador.

## 4. Gap read-only necessário para HMI adaptativa

O backend atual não expõe ao frontend o contrato de capacidade por gerador/profile.

Ausência de uma MetricKey no snapshot pode significar:

```text
profile não suporta
canal opcional não foi bindado
fonte retornou status inválido
leitura está temporariamente ausente
```

A HMI não deve tentar adivinhar qual dessas situações ocorreu.

Antes da implementação final da UI adaptativa, criar um contrato read-only equivalente a:

```text
GET /api/v1/generators/{id}/capabilities
```

ou estender o recurso `Generator` com metadados equivalentes.

Conteúdo mínimo esperado:

```json
{
  "generatorId": "...",
  "profileId": "...",
  "profileStatus": "homologated|lab|draft|synthetic",
  "metrics": [
    {
      "key": "engine.rpm",
      "kind": "number",
      "unit": "rpm",
      "required": true,
      "staleAfterSeconds": 10,
      "sourceClass": "controller|ecu_j1939|external_sensor|rapid_derived|manual_spec",
      "supportLevel": "homologated|lab|experimental"
    }
  ],
  "alarms": true,
  "events": true,
  "remoteControl": false
}
```

Este endpoint **não contém registradores físicos**. O mapa físico continua exclusivamente no Rapid SCADA.

Até esse contrato existir, a primeira implementação pode usar `N/D` para ausência, mas não pode rotular a causa como `unsupported`.

## 5. Resolução de referência

### Desktop principal

```text
1600 x 900
```

Também deve permanecer funcional em:

```text
1366 x 768
1920 x 1080
```

A tela não deve depender de uma única resolução nem criar cards gigantes para preencher espaço.

### Tablet

A partir de aproximadamente 1024 px:
- sinótico permanece inteiro;
- tabelas podem empilhar;
- motor/ECU passa de múltiplas colunas para duas;
- alarmes/eventos permanecem legíveis.

### Mobile

Mobile é consulta de contingência, não estação principal de operação.

Ordem mobile:

```text
Header
Comunicação
Sinótico
Alarmes ativos
Motor/ECU
Elétrica
Combustível/DC
Eventos
Detalhes técnicos
```

Não comprimir o sinótico até ficar ilegível; usar uma composição vertical própria.

## 6. Wireframe estrutural desktop

```text
┌──────────────────────────────────────────────────────────────────────────────┐
│ GER-0147 / Gerador Emergência 01        ONLINE      AUTO       capturado ...│
│ Site: hospital-central   Controller: DSE 8610 MKII   Profile: ...            │
├──────────────────────────────────────────────────────────────────────────────┤
│ COMUNICAÇÃO / QUALIDADE                                                     │
│ Provider healthy   equipamento online   dados atuais / stale / offline       │
├──────────────────────────────────────────────────────────────────────────────┤
│                              SINÓTICO ELÉTRICO                              │
│                                                                              │
│ REDE ─── [MCB] ───────┐                                                      │
│                       ├──── BARRAMENTO / CARGA                               │
│ GERADOR ─ [GCB] ──────┘                                                      │
│                                                                              │
│ Mains: estado / Hz / V        Generator: estado / Hz / kW        ATS: ...    │
├──────────────────────────────────────┬───────────────────────────────────────┤
│ MOTOR / ECU                          │ ELÉTRICA                              │
│ RPM                                  │ Generator V L-N / L-L                  │
│ estado motor                         │ A L1/L2/L3                             │
│ óleo                                 │ Hz                                     │
│ coolant                              │ kW / kVA / kVAr / PF / load %         │
│ horas / partidas                     │ Rede V / Hz                            │
│ futuras ECU homologadas              │ kWh                                    │
├──────────────────────────────────────┼───────────────────────────────────────┤
│ COMBUSTÍVEL                          │ DC / BATERIA                          │
│ nível                                │ tensão / corrente                      │
│ consumo instantâneo                  │ charger V / A                          │
│ consumo total                        │                                       │
├──────────────────────────────────────┴───────────────────────────────────────┤
│ ALARMES ATIVOS                                                               │
│ severidade | código | mensagem | desde                                      │
├──────────────────────────────────────────────────────────────────────────────┤
│ EVENTOS RECENTES                                                              │
│ hora | tipo | mensagem                                                       │
├──────────────────────────────────────────────────────────────────────────────┤
│ DETALHES TÉCNICOS / QUALIDADE / PROVENIÊNCIA (expansível)                    │
└──────────────────────────────────────────────────────────────────────────────┘
```

A disposição é uma hierarquia funcional, não uma obrigação de pixel exato. A proporção e a presença das regiões são obrigatórias.

## 7. Header do ativo

### Região esquerda

Mostrar:
- `Generator.name` como título principal;
- ID/tag em posição secundária;
- `siteId`;
- fabricante + modelo da controladora.

Se firmware/hardware/serial existirem, ficam em detalhes técnicos, não competindo com o título.

### Região direita

Mostrar:
- comunicação do gerador;
- `controller.mode`, se presente;
- `controller.status`, se presente;
- timestamp `capturedAt` rotulado como **captura do snapshot**, não como horário garantido de cada métrica.

Não mostrar `ONLINE` apenas porque `/system/health` está healthy. O estado individual vem de `telemetry.communication`.

## 8. Comunicação e qualidade

Qualidade é parte da operação e deve ser visível.

### Comunicação

```text
online   comunicação operacional segundo provider
offline  equipamento/caminho indisponível; pode haver last-known
unknown  estado não determinado
```

### Quality por métrica

| Quality | Renderização operacional |
|---|---|
| `good` | valor normal + unidade |
| `stale` | último valor visível, visualmente atenuado + badge `STALE` + idade quando útil |
| `offline` | último valor conhecido atenuado + badge `OFFLINE`; nunca parecer atual |
| `bad` | não usar o número para decisão visual; mostrar `Dado inválido`/`—` e permitir inspeção técnica |
| `unknown` | mostrar `N/D` ou `Qualidade desconhecida`; sem cor de normalidade |
| ausência | `N/D` enquanto capability não distinguir a causa; nunca converter para zero |

### Zero real

Exemplo:

```text
generator.power_kw = 0, quality=good
```

deve aparecer como `0 kW`.

Nunca substituir zero válido por `—`, `N/D` ou falsey fallback.

## 9. Sinótico elétrico

Esta é a região visual principal.

### Topologia base

```text
Mains source ─ MCB ─┐
                    ├─ Bus / Load
Gen source   ─ GCB ─┘
```

`ATS` é informação de transferência e deve ser integrada ao sinótico somente quando `ats.state` estiver realmente disponível e seu vocabulário estiver normalizado pelo profile.

### Dados permitidos

Rede:

```text
mains.state
mains.frequency
mains.voltage_l1/l2/l3
mains.voltage_l1_l2/l2_l3/l3_l1
```

Gerador:

```text
generator.status
generator.frequency
generator.power_kw
```

Disjuntores/transferência:

```text
breaker.mcb
breaker.gcb
ats.state
```

### Regras de desenho

1. MCB/GCB só aparecem como `OPEN/CLOSED` quando a respectiva métrica booleana estiver presente e com qualidade utilizável.
2. Ausência de MCB/GCB não pode ser inferida por tensão ou potência.
3. Rede não fica verde apenas porque há tensão numérica > 0; limiar de presença não é inventado no frontend.
4. Gerador não é declarado `RUNNING` apenas porque RPM > 0, se houver `engine.state`/`generator.status` disponível para representar estado explicitamente.
5. Se apenas RPM existir, pode-se mostrar `RPM 1800`; não inventar o enum `RUNNING`.
6. Não animar fluxo de potência na v1.
7. Não inferir direção de potência pelo sinal de kW sem convenção de sinal homologada.
8. Estado desconhecido usa símbolo neutro/cinza, nunca verde.

### Quando profile não possui rede

A área de mains pode ser omitida ou representada como `Rede N/D` conforme capability. Não desenhar um MCB fictício.

### Quando profile não possui ATS

O sinótico permanece com fontes/barramento disponíveis. Não colocar um ícone ATS decorativo.

## 10. Motor / ECU

### Métricas canônicas atuais

Ordem visual recomendada:

```text
engine.state
engine.rpm
engine.oil_pressure
engine.coolant_temperature
engine.oil_temperature
engine.oil_level
engine.coolant_level
engine.run_hours
engine.starts
```

### Apresentação

- números grandes apenas para 2–4 grandezas críticas, não para tudo;
- demais valores em grade técnica compacta;
- unidade sempre vem do backend/profile;
- não aplicar barras percentuais a pressão/temperatura sem limites homologados;
- não colorir normal/anormal comparando com limites inventados.

### ECU/J1939 futura

Quando homologadas e adicionadas ao domínio:

```text
engine.load_percent
engine.torque_percent
engine.intake_air_temperature
engine.intake_manifold_pressure
engine.boost_pressure
engine.exhaust_temperature
engine.coolant_pressure
fuel.pressure
fuel.temperature
ecu.supply_voltage
ecu.communication_state
```

Essas chaves não devem existir no código do frontend antes de existir contrato no backend/profile.

### Diagnósticos ECU

DTC/SPN/FMI não entra nesta grade como métricas artificiais.

Futuro contrato estruturado:

```text
GET /api/v1/generators/{id}/diagnostics
```

Até existir, não mostrar tabela DTC fictícia.

## 11. Elétrica do gerador

Preferir tabela técnica + resumo, não painel de gauges.

### Tensões

Agrupar conforme disponibilidade:

```text
L1-N  L2-N  L3-N
L1-L2 L2-L3 L3-L1
```

MetricKeys:

```text
generator.voltage_l1
generator.voltage_l2
generator.voltage_l3
generator.voltage_l1_l2
generator.voltage_l2_l3
generator.voltage_l3_l1
```

### Correntes

```text
generator.current_l1
generator.current_l2
generator.current_l3
```

### Potência/energia

```text
generator.frequency
generator.power_kw
generator.power_kva
generator.power_kvar
generator.power_factor
generator.energy_kwh
generator.load_percent
```

### Nominais

`GeneratorSpec` pode fornecer:

```text
ratedPowerKw
nominalVoltage
nominalFrequency
nominalRpm
phaseCount
```

Só quando `GeneratorSpec` estiver presente.

Um gauge de carga pode usar `generator.load_percent` diretamente quando homologado. Não calcular percentual por `power_kw / ratedPowerKw` no frontend como substituto silencioso.

## 12. Rede elétrica

A rede pode compartilhar o bloco Elétrica ou aparecer ao lado do sinótico, conforme espaço.

MetricKeys:

```text
mains.state
mains.voltage_l1
mains.voltage_l2
mains.voltage_l3
mains.voltage_l1_l2
mains.voltage_l2_l3
mains.voltage_l3_l1
mains.frequency
```

Não calcular `mains.state` a partir das tensões.

## 13. Combustível

MetricKeys atuais:

```text
fuel.level
fuel.consumption_rate
fuel.total_consumption
```

### Regras

- renderizar a unidade recebida; não assumir `%` ou `L`;
- `0 L/h` good é um valor real;
- consumo total não é integrado no navegador;
- não estimar consumo por kW;
- não calcular autonomia sem contrato de volume útil + consumo + qualidade/timestamp;
- se `fuel.level` for `%`, uma barra pode representar literalmente 0–100%; se a unidade for outra, não usar barra percentual automaticamente.

Eficiência só entra depois de contrato homologado; não calcular `kWh/L` no browser.

## 14. DC / bateria / carregador

MetricKeys:

```text
battery.voltage
battery.current
battery.charger_voltage
battery.charger_current
```

Apresentação compacta.

Não classificar tensão como `boa`, `baixa` ou `alta` por limiar criado no frontend. Alarmes de bateria vêm da fonte de alarmes/profile.

## 15. Alarmes ativos

### Posição

Alarmes ativos ficam antes dos eventos recentes e permanecem visualmente evidentes sem dominar toda a tela quando não há condição ativa.

### Conteúdo v1

```text
severity
code
message
raisedAt
```

`clearedAt` é relevante para histórico/lista completa, não para uma condição ainda ativa.

### Severidade

Usar exatamente:

```text
info
warning
critical
```

Não promover/diminuir severidade no frontend.

### Sem alarmes

Mostrar estado discreto:

```text
Nenhum alarme ativo retornado
```

Não escrever `Sistema normal` se outras qualidades estiverem stale/offline/bad.

## 16. Eventos recentes

Mostrar uma lista curta, por exemplo 5–10 linhas conforme espaço:

```text
occurredAt
type
message
```

Sem inventar:
- operador;
- responsável;
- causa;
- ação executada;
- localidade;

quando não existirem no evento.

## 17. Detalhes técnicos expansíveis

Região secundária, fechada por padrão para operador.

Pode mostrar dados reais disponíveis:

```text
generator ID
siteId
controller manufacturer/model
firmware
hardware version
serial
GeneratorSpec
capturedAt
metric observedAt
metric quality
profile/capability metadata quando o contrato existir
sourceClass/proveniência quando disponível
```

Não mostrar registradores físicos Modbus no frontend do RC Monitor.

## 18. Design visual

### Direção

Usar como referência:
- a identidade escura/larda das imagens fornecidas pelo proprietário;
- hierarquia de processo do Rapid SCADA;
- densidade e organização por motor/gerador dos produtos ComAp/DSE/SmartGen;
- princípios ISA-101 de consistência, estado e navegação.

### Evitar

- gradientes decorativos sem função;
- glows em todos os cards;
- gauges circulares para toda grandeza;
- ícones enormes;
- excesso de arredondamento tipo dashboard financeiro;
- cards com 80% de espaço vazio;
- cor verde como cor padrão de decoração;
- animação contínua sem significado de processo;
- números fictícios para preencher layout.

### Cor deve carregar significado

Cores de estado devem ser semânticas e consistentes.

Exemplo de intenção, sem congelar hex agora:

```text
normal/good       neutro + indicação discreta
online            verde restrito a estado conhecido
warning           âmbar
critical          vermelho
offline/bad       vermelho/cinza conforme contexto
stale             âmbar/cinza
unknown/N-D       cinza
selection/action  laranja RC
```

A cor de marca não deve transformar tudo em estado de alarme.

## 19. Dados que ficam explicitamente fora da v1

Mesmo que apareçam nas referências visuais antigas:

```text
health score
availability 30d
MTBF
MTTR
OS abertas
plano preventivo
próxima manutenção calculada pelo frontend
tendência 24h
consumo mensal
energia mensal
estimativa de autonomia
mapa
responsável pelo alarme
causa provável gerada automaticamente
```

Só entram quando existir backend/fonte auditável.

## 20. Histórico e trends

Não criar séries em memória no React a partir de polling corrente.

Gráficos históricos dependem de uma API histórica real ligada ao Rapid SCADA.

Estado:

```text
DEFER
```

Quando esse contrato existir, a mesma tela poderá receber uma aba/área `Tendências` sem redesenhar a hierarquia principal.

## 21. Loading, erro e recuperação

### Loading inicial

Usar skeleton estrutural que preserve a geometria da tela. Não mostrar números `0` durante carregamento.

### Falha de uma requisição

As regiões devem falhar de forma independente quando possível:
- generator metadata;
- telemetry;
- alarms;
- events.

Falha de eventos não deve apagar telemetria válida.

### Outage depois de dados válidos

Quando o backend fornecer last-known offline/stale:
- preservar o último valor;
- marcar claramente a qualidade;
- não fazer a tela piscar para `0` ou desaparecer;
- recuperação retorna ao estado `good` automaticamente quando a API voltar a fornecê-lo.

## 22. Acessibilidade e operação

- nenhuma informação crítica depende apenas de cor;
- badges possuem texto/ícone além da cor;
- contraste compatível com leitura prolongada;
- foco de teclado visível;
- tabelas têm cabeçalhos semânticos;
- tooltips não são a única forma de descobrir informação crítica;
- timestamps devem usar formato consistente e timezone definido pela aplicação.

## 23. Performance

A tela não deve disparar um request por métrica.

Modelo esperado:

```text
metadata       1 request
telemetry      1 request por ciclo
alarms         1 request por ciclo/cadência própria
events         1 request por cadência própria
system health  compartilhado pela aplicação
```

TanStack Query deve centralizar cache/retry/deduplicação; componentes visuais não criam polling independentes.

Cadências ainda serão definidas no contrato de integração, não neste documento.

## 24. Critérios de aceite visual e funcional

A tela só é considerada fiel ao contrato quando testes comprovarem:

1. zero real (`0`) é exibido como zero;
2. métrica ausente não vira zero;
3. `stale` não parece good;
4. `offline` preserva last-known quando fornecido;
5. `bad` não é apresentado como valor confiável;
6. ausência de MCB/GCB/ATS não cria estado fictício;
7. ausência de mains não cria rede verde;
8. alarmes usam severidade do backend;
9. eventos mantêm timestamps do backend;
10. nenhuma ação industrial de escrita aparece habilitada;
11. nenhum gráfico histórico é fabricado com polling local;
12. o layout funciona em 1366x768, 1600x900 e 1920x1080;
13. loading/erro de eventos não derruba telemetria;
14. o mesmo componente funciona com profiles que expõem conjuntos de métricas diferentes;
15. todos os dados exibidos possuem origem identificável no contrato/API.

## 25. Regra de fidelidade

O mockup aprovado desta tela deverá ser salvo como referência versionada antes da implementação.

Depois da aprovação:
- regiões principais não podem mudar por conveniência de implementação;
- informação não pode ser removida silenciosamente;
- componentes podem ser refatorados internamente sem mudar o contrato visual;
- qualquer diferença intencional precisa ser revisada primeiro neste documento.

## 26. Próximo artefato

Com este contrato aceito, o próximo passo é produzir **um único mockup visual do Detalhe do Gerador**, usando dados explicitamente demonstrativos rotulados como mockup, sem confundi-los com dados de produção.

O mockup deve seguir exatamente as regiões e estados definidos aqui. Só após aprovação visual começa o shell/design system e a implementação React.
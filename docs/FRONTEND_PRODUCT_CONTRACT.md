# RC Monitor Frontend — contrato de produto e HMI

Status: **proposta de arquitetura de produto para aprovação antes da implementação**.

Este documento reduz o risco de divergência entre promessa visual e software entregue. O frontend não deve avançar por improvisação de componentes: cada tela precisa ter finalidade operacional, fonte de dados, estados e critério de aceite conhecidos.

## Princípio de produto

RC Monitor deve parecer e se comportar como um sistema industrial de monitoramento de grupos geradores, não como um dashboard SaaS genérico.

A interface deve priorizar:

1. situação do processo;
2. topologia elétrica;
3. estado de comunicação e qualidade do dado;
4. alarmes e eventos;
5. grandezas elétricas e de motor/ECU;
6. diagnóstico;
7. informação administrativa.

Decoração nunca substitui estado operacional.

## Referências de HMI

A direção é inspirada por padrões e produtos industriais, sem copiar identidade visual ou código proprietário:

- Rapid SCADA — frota, mapas/mimics, canais, históricos e eventos;
- ComAp InteliSCADA/WebSupervisor — monitoramento de gensets, diagramas, trends, alarmes, fleet management;
- DSE8005 — SCADA multi-controladora e organização por system/engine/generator/alarm/trend;
- HMIs SmartGen/Kohler/Basler — densidade útil de grandezas e separação entre operação e engenharia;
- ISA-101 — hierarquia, navegação, convenções gráficas, alarmes, segurança e lifecycle de HMI.

Referências públicas:
- https://rapidscada.org/projects/
- https://rapidscada.net/docs/en/latest/configuration/channels
- https://www.comap-control.com/products/software-tools/monitoring-tools/pc-monitoring-tools/inteliscada/
- https://www.comap-control.com/products/software-tools/monitoring-tools/online-monitoring-tools/websupervisor/
- https://www.deepseaelectronics.com/genset/software-tools/dse8005/software
- https://www.isa.org/standards-and-publications/isa-standards/isa-101-standards

## Separação principal do produto

O menu não deve expor as 54 superfícies históricas de uma vez.

```text
OPERAÇÃO
ENGENHARIA
ADMINISTRAÇÃO
```

Cada área responde a uma função diferente.

### Operação

Voltada ao operador que precisa entender rapidamente o estado do parque.

Proposta de navegação v0:

```text
Visão Geral
Sites
Geradores
Alarmes
Eventos
Comunicação
```

### Engenharia

Voltada a commissioning, perfis e diagnóstico.

```text
Commissioning
Controladoras / Profiles
Rapid SCADA
Canais / Bindings
Diagnóstico
```

Esses módulos só entram quando houver backend real que os sustente. Não são mocks permanentes.

### Administração

```text
Usuários / Perfis
Auditoria
Configurações
Saúde do sistema
Versão
```

Autenticação/RBAC ainda precisam de contrato próprio no produto novo antes de serem implementados.

## Regra de navegação operacional

O operador deve conseguir responder rapidamente:

```text
1. Existe algo crítico no parque?
2. Onde está?
3. Qual gerador?
4. Há comunicação atual?
5. Rede está presente?
6. Motor está rodando?
7. Gerador está produzindo?
8. GCB/MCB/ATS estão em qual estado?
9. Qual carga/potência?
10. Qual alarme/evento explica a situação?
```

Se uma tela não ajuda a responder uma dessas perguntas ou uma tarefa de engenharia/administração claramente definida, ela não deve ocupar o menu principal.

# Telas operacionais

## 1. Visão Geral

Objetivo: situational awareness da frota.

### Regiões obrigatórias

1. cabeçalho com identificação do produto, busca e estado global;
2. faixa compacta de KPIs do parque;
3. lista/grade de ativos que exigem atenção;
4. alarmes recentes/críticos;
5. comunicação/health;
6. acesso rápido aos geradores.

### KPIs permitidos com o backend atual

Podem ser calculados no frontend a partir de respostas atuais quando a semântica for inequívoca:
- número de geradores cadastrados no RC Monitor;
- quantidade com comunicação online/offline/unknown;
- quantidade com alarmes ativos retornados pela API, respeitando disponibilidade dos endpoints.

Não exibir sem backend apropriado:
- disponibilidade 30 dias;
- consumo mensal;
- energia produzida no mês;
- MTBF/MTTR;
- health score;
- OS/manutenção;
- tendências 24h.

### Regra visual

KPIs devem ser compactos. A maior parte da tela deve mostrar situação operacional, não números decorativos.

## 2. Sites

Objetivo: agrupar ativos por instalação/unidade.

### Status

`FUTURE` no produto novo até existir contrato real de site/metadata.

O backend atual possui `siteId` no Generator, mas não fornece hoje um recurso rico de sites com nome, localização, cliente ou coordenadas.

Até existir esse contrato, não inventar mapa, cidade, região, cliente ou disponibilidade do site.

## 3. Geradores

Objetivo: localizar rapidamente um ativo e reconhecer seu estado.

### Formas de visualização

A referência antiga de três modos é útil:
- operacional/card;
- compacto;
- tabela/lista.

A primeira implementação pode começar por tabela + card operacional, mantendo o terceiro modo como melhoria posterior se necessário.

### Informação mínima por ativo

Quando disponível:
- nome/tag;
- site ID;
- controladora;
- comunicação;
- engine state/RPM;
- generator status;
- kW;
- Hz;
- alarme ativo mais relevante;
- freshness/quality.

Não ocupar card com métricas que não existem no profile.

## 4. Detalhe do Gerador

É a tela mais importante do produto e deve ser aprovada antes das demais telas complexas.

### Ordem de leitura

```text
Identidade + comunicação
        ↓
Sinótico REDE -> MCB/ATS -> GERADOR/GCB -> CARGA
        ↓
Estado do motor / ECU
        ↓
Grandezas elétricas
        ↓
Combustível / DC
        ↓
Alarmes e eventos
        ↓
Diagnóstico / detalhes técnicos
```

### Cabeçalho

Mostrar:
- nome/tag;
- site ID;
- fabricante/modelo da controladora;
- communication state;
- controller mode/status quando disponíveis;
- timestamp/freshness.

Sem botões START/STOP/TEST/TRANSFER neste ciclo.

### Sinótico elétrico

Referência visual prioritária:

```text
REDE ---- MCB/ATS ---- BARRAMENTO/CARGA
                     |
                 GCB |
                     |
                  GERADOR
```

O desenho deve se adaptar ao que o profile suporta.

Exemplos:
- sem `breaker.mcb`: não desenhar MCB como conhecido;
- sem `ats.state`: não declarar ATS automático/normal;
- sem mains metrics: mostrar rede como `N/D`, não verde;
- sem GCB: não inferir estado do breaker por kW.

Fluxo animado só pode existir se o estado necessário for conhecido. Animação não pode inferir direção de potência a partir de estética.

### Motor / ECU

Mostrar as métricas realmente suportadas conforme `TELEMETRY_ECU_CATALOG.md`.

Prioridade:
- RPM;
- oil pressure;
- coolant temperature;
- fuel;
- run hours;
- battery;
- futuras métricas ECU/J1939 quando homologadas.

### Elétrica

Preferir uma tabela/agrupamento legível a dezenas de gauges.

Mostrar:
- V fase/neutro e fase/fase conforme suporte;
- A;
- Hz;
- kW/kVA/kVAr;
- PF;
- load %;
- kWh.

Gauge só deve ser usado quando escala/nominal é conhecida e operacionalmente útil.

### Alarmes/eventos

Lista curta no detalhe com acesso à tela completa.

Sem reconhecimento de alarme enquanto o backend não expuser uma operação homologada para isso.

### Tendências

Não implementar gráfico histórico com buffer do navegador.

Estado: `DEFER` até existir endpoint histórico real sustentado pelo Rapid SCADA.

## 5. Alarmes

Objetivo: apresentar condições ativas e contexto suficiente para ação humana.

Com o contrato atual:
- ID;
- generatorId;
- code;
- severity;
- message;
- active;
- raisedAt;
- clearedAt quando houver.

### Layout recomendado

- KPIs compactos por severidade apenas se derivados de dados reais;
- filtros de gerador/severidade/status no cliente quando volume permitir;
- tabela principal;
- painel de detalhe opcional.

Não mostrar:
- responsável;
- causa provável;
- ações recomendadas;
- reconhecimento;

sem fonte real para esses dados.

## 6. Eventos

Objetivo: linha temporal factual do que o RC Monitor recebeu do Rapid.

Dados atuais:
- ID;
- generatorId;
- type;
- message;
- occurredAt.

Não inventar operador, origem humana ou categoria se não vier do backend.

## 7. Comunicação

Objetivo: separar problemas de processo de problemas de observabilidade.

### Primeira versão

Usar:
- `/api/v1/system/health`;
- communication state de cada gerador;
- metric quality/observedAt.

Pode mostrar:
- provider healthy/degraded/unavailable;
- geradores online/offline/unknown;
- última observação por equipamento/métrica.

Não mostrar ainda:
- RSSI de modem;
- consumo de dados;
- operadora;
- latência/ping;
- VPN status;

sem endpoints reais.

# Engenharia

## 8. Commissioning Queue

Esta tela é futura, mas deve existir no desenho do produto antes de `Adicionar gerador`.

Estados:
- draft;
- inventory;
- commissioning + etapa atual;
- blocked/failed;
- rollback required;
- commissioned;
- suspended;
- retired.

Um ativo em commissioning não entra nos KPIs operacionais.

Detalhes em `COMMISSIONING_LIFECYCLE.md`.

## 9. Wizard de commissioning

Não é CRUD.

Etapas aprovadas conceitualmente:

```text
Ativo
Controladora / ECU
Transporte
Preflight
Rapid plan
Validação de telemetria
Evidências
Comissionar
```

A interface deve mostrar o resultado objetivo de cada gate.

## 10. Controladoras / Profiles

Objetivo: catálogo de capacidades, não biblioteca decorativa.

Mostrar por profile:
- fabricante/modelo;
- profile ID/versão;
- status (`draft`, `lab`, `homologated` etc.);
- métricas requeridas/opcionais;
- alarmes/eventos;
- firmware/hardware compatível conhecido;
- remote control status;
- evidência HIL.

## 11. Rapid SCADA / Bindings

Deve ser área de engenharia, não menu do operador.

Mostrar futuramente:
- número de device/object quando necessário;
- canais Rapid atribuídos;
- metric key vinculada;
- quality;
- profile/binding version;
- diff/staged plan.

Nunca mostrar ou editar registradores físicos no RC Monitor frontend; esses pertencem ao projeto Rapid.

## 12. Diagnóstico

Objetivo: explicar por que uma cadeia não está funcional.

Separar:

```text
Gateway transport
Rapid provider
Rapid channels
controller
ECU/CAN
metric quality
```

Isso evita a mensagem genérica `offline` para problemas totalmente diferentes.

# Administração

As telas administrativas do legado são referência de necessidades futuras, não escopo automático.

Antes de implementar Usuários/Perfis/Auditoria, definir:
- autenticação;
- sessão;
- RBAC;
- auditoria;
- política de acesso à engenharia.

O RC Monitor atual não possui esse contrato web completo.

# Matriz preliminar das superfícies antigas

Esta matriz é uma decisão de arquitetura de informação v0 e deve ser refinada antes de implementar cada módulo.

| Superfície antiga | Decisão v0 | Destino |
|---|---|---|
| Visão Geral | REDESIGN | Operação |
| Geradores | REDESIGN | Operação |
| Central de Operação | MERGE | Visão Geral + Geradores |
| Sites | DEFER/REDESIGN | Operação quando houver site API |
| Mapa | DEFER | depende de geodados reais |
| Alarmes | REDESIGN | Operação |
| Eventos | REDESIGN | Operação |
| Tendências | DEFER | exige histórico real |
| Histórico | DEFER | exige API histórica |
| Relatórios | DEFER | backend futuro |
| Rede | MERGE | sinótico/site/gerador |
| Energia - Geradores | MERGE | detalhe do gerador |
| Carga | MERGE | sinótico/site |
| Transferência | MERGE/DEFER | sinótico; operação futura |
| Paralelismo | DEFER | só para aplicação/profile compatível |
| Manutenção | DEFER | domínio futuro |
| Combustível | MERGE | detalhe + futuro analytics |
| Baterias | MERGE | detalhe do gerador |
| Horímetros | MERGE | detalhe + futuro relatório |
| Agenda | DEFER | domínio futuro |
| Controladoras | REDESIGN | Engenharia |
| Modems | MERGE/DEFER | Comunicação futura |
| Gateways | MERGE/REDESIGN | Comunicação/Engenharia |
| Conectividade | REDESIGN | Comunicação |
| Comunicação | REDESIGN | Comunicação |
| Regras | DEFER | automação futura |
| Exercício automático | DEFER | comando/automação futura |
| Agendamentos | DEFER | automação futura |
| Notificações | DEFER | serviço futuro |
| Escalonamento | DEFER | serviço futuro |
| Canais | REDESIGN | Engenharia |
| Tags | MERGE | Engenharia / canais |
| Templates | MERGE | Profiles/Rapid plan |
| Rapid SCADA | REDESIGN | Engenharia |
| Diagnóstico | REDESIGN | Engenharia |
| Fabricantes | MERGE | Profiles |
| Biblioteca Controladoras | MERGE | Profiles |
| Protocolos | REFERENCE_ONLY | engenharia/documentação |
| Controller Packs | REDESIGN | Profiles |
| Laboratório | DEFER | HIL/Engenharia |
| Clientes | DEFER | asset management futuro |
| Unidades | DEFER/REDESIGN | Sites futuro |
| Usuários | DEFER | Administração/RBAC |
| Perfis e permissões | DEFER | Administração/RBAC |
| Auditoria | DEFER/REQUIRED BEFORE WRITES | Administração |
| API | REFERENCE_ONLY | documentação/admin |
| Webhooks | DEFER | integração futura |
| E-mail | DEFER | integração futura |
| WhatsApp | DEFER | integração futura |
| ERP/BMS | DEFER | integração futura |
| Configurações | DEFER | Administração |
| Backups | DEFER | Administração/ops |
| Saúde do sistema | REDESIGN | Comunicação/Admin |
| Versão | KEEP SMALL | Admin/about |
| Login | DEFER | depende de auth real |
| Detalhe do gerador | REDESIGN PRIORITY | Operação |

`MERGE` significa preservar a informação útil sem manter necessariamente uma rota/menu separado.

# Guardrail contra divergência visual

Antes de codificar uma tela complexa:

1. escrever o screen contract;
2. produzir wireframe/layout aprovado;
3. associar cada região a endpoint/campo real;
4. declarar estados loading/empty/error/offline/stale/absent;
5. aprovar referência visual;
6. só então implementar;
7. gerar screenshot do build real;
8. comparar com a referência;
9. registrar diferenças intencionais.

Nenhum componente pode `preencher espaço` com valor demonstrativo na versão operacional.

## Baselines visuais futuros

Depois da aprovação dos primeiros mockups, manter no repositório uma pasta de referência, por exemplo:

```text
frontend/reference/
  overview.desktop.png
  generators.desktop.png
  generator-detail.desktop.png
  alarms.desktop.png
  events.desktop.png
```

E adicionar regressão visual em CI (por exemplo Playwright) quando o frontend existir.

A referência deve ser screenshot aprovado do **produto RC**, não screenshot de ComAp/DSE/Rapid ou de outro fabricante.

# Direção visual

As imagens de conceito fornecidas pelo usuário são úteis para acabamento:
- dark industrial;
- azul-marinho quase preto;
- laranja RC como destaque;
- bordas discretas;
- tabelas densas;
- sidebar profissional;
- tipografia clara;
- informação bem agrupada.

Mas a linguagem de HMI deve ser refinada para evitar excesso de cor.

Princípios:
- cor forte prioritariamente para estado anormal, energia/fluxo ou ação deliberada;
- condição normal pode ser discreta;
- vermelho reservado para criticidade real;
- amarelo/âmbar para atenção;
- azul/cinza para informação;
- `N/D`, stale e offline visíveis e inequívocos;
- animação mínima e funcional.

# Contrato de responsividade

Prioridade:

1. desktop de operação;
2. notebook;
3. tablet;
4. mobile como consulta, não como substituto do posto de operação.

A versão desktop deve suportar alta densidade sem exigir scroll excessivo para descobrir o estado principal.

# Critério para iniciar React

Não iniciar implementação das telas principais enquanto não houver aprovação explícita de:

- este contrato;
- `FRONTEND_SURFACE_MAP.md`;
- `COMMISSIONING_LIFECYCLE.md`;
- `TELEMETRY_ECU_CATALOG.md`;
- wireframe da tela **Detalhe do Gerador**;
- wireframe da **Visão Geral**.

Depois disso, implementar primeiro o shell/design system e uma única vertical completa `Geradores -> Detalhe`, integrada ao backend real, antes de expandir para o restante do menu.

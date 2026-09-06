# RC Monitor Frontend — índice da especificação

Status: **UI-001 / definição antes da implementação**.

Este é o ponto de entrada para qualquer trabalho futuro de frontend. Nenhuma tela deve ser implementada ignorando estes contratos.

## Documentos obrigatórios

1. [`FRONTEND_SURFACE_MAP.md`](./FRONTEND_SURFACE_MAP.md)
   - inventário auditado do frontend legado;
   - menus visíveis, admin, ocultos e superfície órfã;
   - impede que uma tela histórica desapareça ou reapareça por acidente.

2. [`FRONTEND_PRODUCT_CONTRACT.md`](./FRONTEND_PRODUCT_CONTRACT.md)
   - arquitetura de informação proposta;
   - Operação x Engenharia x Administração;
   - contrato das telas;
   - matriz preliminar `REDESIGN / MERGE / DEFER / REFERENCE_ONLY`;
   - guardrail de fidelidade visual.

3. [`COMMISSIONING_LIFECYCLE.md`](./COMMISSIONING_LIFECYCLE.md)
   - define que cadastro não é commissioning;
   - state machine e gates;
   - critérios para um gerador entrar na frota operacional;
   - staging, evidência e rollback;
   - separa futuras escritas de configuração do RC Monitor read-only.

4. [`TELEMETRY_ECU_CATALOG.md`](./TELEMETRY_ECU_CATALOG.md)
   - catálogo canônico atual;
   - consumo de combustível;
   - expansão futura ECU/J1939;
   - DTCs estruturados;
   - proveniência e quality;
   - proibição de métricas inventadas no navegador.

5. [`GENERATOR_DETAIL_CONTRACT.md`](./GENERATOR_DETAIL_CONTRACT.md)
   - congela a primeira HMI principal antes de React;
   - define regiões, wireframe e hierarquia operacional;
   - mapeia cada bloco aos endpoints e MetricKeys reais;
   - define comportamento para `good`, `stale`, `offline`, `bad`, `unknown`, ausência e zero real;
   - proíbe inferências elétricas e industriais não sustentadas;
   - identifica o pequeno gap read-only de capability/profile necessário para uma UI adaptativa profissional.

## Ordem de execução

```text
inventário auditado
      ↓
contrato de produto
      ↓
commissioning
      ↓
catálogo de telemetria/ECU
      ↓
contrato Detalhe do Gerador
      ↓
mockup Detalhe do Gerador
      ↓
aprovação visual
      ↓
wireframe/mockup Visão Geral
      ↓
aprovação visual
      ↓
shell + design system
      ↓
vertical Geradores -> Detalhe integrada ao backend real
      ↓
regressão visual
      ↓
expansão das demais telas
```

## Gate de implementação

React ainda não está autorizado por estes documentos sozinho.

Antes de iniciar a implementação das telas principais devem existir duas referências de layout aprovadas:

- **Detalhe do Gerador**;
- **Visão Geral**.

Cada referência precisa indicar:

- regiões fixas da tela;
- dados reais usados em cada região;
- comportamento para `good`, `stale`, `offline`, `bad`, `unknown` e ausência;
- elementos futuros que ficam ocultos/desabilitados;
- resolução desktop de referência;
- comportamento responsivo esperado.

## Regra de mudança

Mudança posterior de menu, hierarquia ou região principal de uma tela deve primeiro alterar a especificação correspondente. O código acompanha o contrato; o contrato não é reescrito depois para justificar uma implementação diferente.

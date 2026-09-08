# RC Monitor Frontend — índice da especificação

Status: **UI-001 congelado; implementação real autorizada pelo proprietário**.

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
   - congela a primeira HMI principal antes da implementação;
   - define regiões, wireframe e hierarquia operacional;
   - mapeia cada bloco aos endpoints e MetricKeys reais;
   - define comportamento para `good`, `stale`, `offline`, `bad`, `unknown`, ausência e zero real;
   - proíbe inferências elétricas e industriais não sustentadas;
   - identifica o pequeno gap read-only de capability/profile necessário para uma UI adaptativa profissional.

## Autorização de implementação

Em 2026-09-06 o proprietário determinou explicitamente que a próxima entrega deve ser **o frontend funcional no repositório**, e não imagens/mockups gerados.

Portanto:

- `GENERATOR_DETAIL_CONTRACT.md` e os contratos de produto são a referência normativa;
- imagens fornecidas anteriormente continuam `REFERENCE_ONLY` para identidade visual;
- mockup gerado por imagem **não é gate de implementação**;
- esta decisão supersede o requisito de mockup obrigatório descrito nas seções finais de `GENERATOR_DETAIL_CONTRACT.md`;
- qualquer divergência estrutural futura continua exigindo atualização prévia do contrato.

## Ordem de execução atual

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
shell + design system real
      ↓
vertical Geradores -> Detalhe integrada ao backend read-only
      ↓
Visão Geral / Alarmes / Eventos / Comunicação
      ↓
regressão funcional e responsiva
      ↓
expansão das demais telas conforme backend real
```

## Gate de implementação

A implementação pode avançar desde que:

- use somente endpoints realmente existentes;
- não invente dados para preencher layout;
- preserve `good`, `stale`, `offline`, `bad`, `unknown`, ausência e zero real;
- não exponha ações industriais de escrita nesta fase;
- mantenha os registradores físicos fora do frontend/RC Monitor;
- possua typecheck, testes e build automatizados;
- qualquer função que dependa de backend inexistente permaneça ausente, não simulada como funcional.

## Regra de mudança

Mudança posterior de menu, hierarquia ou região principal de uma tela deve primeiro alterar a especificação correspondente. O código acompanha o contrato; o contrato não é reescrito depois para justificar uma implementação diferente.

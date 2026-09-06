# RC Gateway / RC Monitor — lifecycle de commissioning de geradores

Status: **especificação de produto e engenharia**. Este documento não implementa escrita industrial, não habilita comandos e não altera o RC Monitor read-only existente.

## Objetivo

Impedir o erro do produto legado em que um gerador podia aparecer na interface após um simples cadastro, embora transporte, controladora, Rapid SCADA e canais ainda não estivessem funcionais.

No RC, **cadastrar um ativo não significa comissioná-lo**.

Um gerador só pode entrar na frota operacional depois que a cadeia abaixo for comprovada:

```text
controladora / ECU
      -> meio físico / modem / VPN
      -> RC Gateway
      -> Rapid SCADA
      -> canais Rapid reais
      -> RC Monitor
      -> API
      -> frontend
```

## Invariantes

- Gateway continua `BRIDGE FIRST`, `PROTOCOL OPTIONAL`, `NO DEVICE MEMORY DATABASE`, `NO TELEMETRY HISTORIAN`.
- Rapid SCADA continua proprietário de protocolo físico, polling, mapas de registradores, canais, quality e histórico.
- RC Monitor continua proprietário do domínio read-only e recebe apenas números de canais Rapid.
- O frontend nunca acessa controladora, ECU, modem ou Rapid diretamente.
- Nenhum endereço físico de registrador entra no RC Monitor ou no frontend.
- Primeiro ciclo de homologação permanece read-only.
- `START`, `STOP`, transferência, reset e setpoints permanecem fora deste lifecycle até um gate futuro específico de comandos.

## Quatro eixos que nunca devem ser confundidos

O produto deve manter estados independentes. Um único campo `status` não é suficiente.

### 1. Lifecycle do ativo

```text
DRAFT
INVENTORY
COMMISSIONING
COMMISSIONED
SUSPENDED
RETIRED
```

### 2. Etapa de commissioning

```text
IDENTITY
TRANSPORT
CONTROLLER
PROFILE
RAPID_PLAN
RAPID_APPLY
TELEMETRY_VALIDATION
EVIDENCE
```

### 3. Estado de comunicação operacional

Após `COMMISSIONED`:

```text
online
offline
unknown
```

### 4. Quality por métrica

```text
good
stale
offline
bad
unknown
absent
```

`absent` significa que a métrica não foi fornecida. Não equivale a zero.

## Regra de visibilidade da frota

- `DRAFT`, `INVENTORY` e `COMMISSIONING` aparecem em **Engenharia > Commissioning**.
- Esses estados **não contam** como geradores online/offline nos KPIs operacionais.
- Somente `COMMISSIONED` entra em **Operação > Geradores**.
- `SUSPENDED` permanece rastreável, mas deve ser visualmente separado da frota ativa.
- `RETIRED` sai das telas operacionais e permanece no histórico/auditoria.

Um equipamento que nunca foi validado não deve aparecer como `offline`. Ele deve aparecer como `commissioning incomplete`, com a etapa e a causa da falha.

## State machine de alto nível

```text
DRAFT
  |
  v
INVENTORY
  |
  v
COMMISSIONING:IDENTITY
  |
  v
COMMISSIONING:TRANSPORT
  |
  v
COMMISSIONING:CONTROLLER
  |
  v
COMMISSIONING:PROFILE
  |
  v
COMMISSIONING:RAPID_PLAN
  |
  v
COMMISSIONING:RAPID_APPLY
  |
  v
COMMISSIONING:TELEMETRY_VALIDATION
  |
  v
COMMISSIONING:EVIDENCE
  |
  v
COMMISSIONED
```

Qualquer etapa pode resultar em `BLOCKED` ou `FAILED`, mas o lifecycle do ativo permanece `COMMISSIONING`; o operador deve conseguir ver causa, evidência e ação de recuperação.

Mudança parcial do Rapid que não possa ser automaticamente revertida deve produzir `ROLLBACK_REQUIRED` e bloquear promoção a `COMMISSIONED`.

## Gates obrigatórios

### G0 — autorização e escopo

Entrada:
- identidade do solicitante;
- site/unidade;
- justificativa/ordem de trabalho quando exigida pela política;
- nível de suporte do Controller Pack.

Saída:
- commissioning ID imutável;
- ator;
- timestamp;
- escopo read-only explicitamente registrado.

Falha:
- nenhuma alteração de configuração.

### G1 — identidade do ativo

Dados recomendados:
- tag;
- nome;
- site ID;
- potência nominal;
- tensão nominal;
- frequência nominal;
- número de fases;
- RPM nominal;
- fabricante/modelo/serial do grupo quando conhecidos;
- fabricante/modelo/firmware/hardware/serial da controladora quando conhecidos;
- fabricante/modelo/serial do motor/ECU quando disponíveis.

Dados desconhecidos permanecem desconhecidos. A interface não inventa nominal, serial ou modelo.

### G2 — transporte

Validar somente o necessário ao meio escolhido.

Exemplos:

TCP:
- gateway/rota esperado;
- host/porta quando pertencem ao desenho aprovado;
- conexão TCP;
- timeout;
- Unit ID quando aplicável.

Serial/RTU:
- porta;
- baud rate;
- paridade;
- stop bits;
- endereço do dispositivo.

VPN/modem:
- existência do caminho;
- sessão/rota necessária;
- sem exigir que o Gateway interprete protocolo industrial.

Passar neste gate prova **transporte**, não prova semântica de telemetria.

### G3 — controladora

Objetivo: confirmar que o equipamento do outro lado é compatível com o perfil selecionado.

Pode usar, quando tecnicamente disponível e homologado:
- identificação reportada pelo protocolo;
- modelo/firmware;
- assinatura de pontos read-only;
- evidência manual/HIL quando o protocolo não oferece identificação inequívoca.

Nunca promover com base apenas em `porta TCP aberta`.

### G4 — Controller Pack / perfil

O commissioning deve registrar:
- profile ID;
- versão do profile;
- manufacturer/model;
- lifecycle do profile (`draft`, `lab`, `homologated`, etc.);
- conjunto de métricas requeridas;
- conjunto de métricas opcionais;
- alarmes/eventos suportados;
- compatibilidade de firmware conhecida;
- `remoteControl=false` enquanto comandos estiverem diferidos.

`draft` não pode ser tratado como compatibilidade de produção.

### G5 — Rapid plan

Antes de alterar a instância, construir um plano reproduzível contendo o que será criado/alterado no Rapid:
- objeto lógico quando necessário;
- communication line/device;
- driver;
- polling;
- channels;
- archive/event settings necessários;
- binding de números de canais para o RC Monitor.

O plano deve possuir:
- versão;
- hash;
- configuração base;
- diff;
- validação de integridade;
- rollback previsto.

O navegador não escreve XML/DAT do Rapid diretamente.

### G6 — Rapid apply

Aplicação deve ser uma operação privilegiada de engenharia, fora do RC Monitor read-only.

Requisitos:
- aplicar somente um plano previamente validado;
- registrar ator e revisão;
- manter snapshot/versão anterior;
- validar o resultado da configuração;
- manter comandos do Rapid desabilitados no ciclo read-only.

### G7 — validação de telemetria

Um gerador só pode ser promovido quando:
- todos os canais `required=true` do profile/binding existem;
- todos os required são recuperáveis pelo RC Monitor;
- os tipos são válidos;
- unidades são as homologadas;
- valores são finitos quando numéricos;
- timestamps são válidos;
- quality não está `bad`/`unknown` para a amostra de aceitação;
- pelo menos uma janela curta de observação prova atualização contínua dos required;
- ausência continua distinta de zero.

Métricas opcionais ausentes não bloqueiam commissioning. Elas ficam `unsupported/absent`.

### G8 — alarmes e eventos

Quando o profile declara suporte:
- provar ao menos os bindings homologados;
- provar timestamp/origem conforme a semântica disponível;
- não inventar alarme quando canal está ausente ou inválido.

Teste real de condição perigosa não deve ser provocado apenas para satisfazer commissioning. HIL/simulação controlada e evidência de fabricante podem ser usadas conforme o plano de homologação.

### G9 — evidência e promoção

O pacote de evidência deve registrar no mínimo:
- commissioning ID;
- asset ID/tag/site;
- Controller Pack + versão;
- Rapid plan + hash;
- números de canais vinculados;
- resultado de cada gate;
- métricas requeridas observadas;
- timestamps;
- ator(es);
- falhas e retries;
- configuração anterior e rollback;
- aprovação final.

Somente então o lifecycle muda para `COMMISSIONED`.

## Arquitetura futura de escrita de configuração

O RC Monitor atual permanece GET-only. Commissioning deve ser uma capacidade separada.

```text
Frontend
   |
   +--> RC Monitor (operação, GET/read-only)
   |
   +--> RC Commissioning (engenharia, futuro)
             |
             +--> validação e plano
             +--> configuração versionada do Rapid
             +--> configuração versionada do RC Monitor
             +--> evidência / auditoria / rollback
```

`RC Commissioning` é um nome arquitetural provisório, não autorização para implementação imediata.

## Contrato do wizard de novo gerador

O wizard futuro não deve ter um botão genérico `Salvar e concluir`.

Fluxo:

1. **Ativo** — identidade e nominais.
2. **Controladora/ECU** — fabricante, modelo, profile e nível de suporte.
3. **Transporte** — caminho físico/lógico.
4. **Preflight** — testes read-only de conectividade/identidade.
5. **Rapid plan** — canais e diff de configuração.
6. **Validação** — telemetria requerida e quality.
7. **Evidências** — resumo técnico.
8. **Comissionar** — promoção explícita.

A tela deve permitir sair e retomar sem publicar o ativo na operação.

## Falhas que devem ser exibidas claramente

Exemplos:
- gateway inacessível;
- TCP aberto, mas dispositivo não identificado;
- Unit ID não responde;
- profile incompatível;
- firmware não homologado;
- Rapid channel ausente;
- required metric com status inválido;
- stale durante validação;
- binding divergente;
- configuração Rapid parcialmente aplicada;
- rollback necessário.

Mensagens como `offline` ou `erro desconhecido` não são suficientes para commissioning.

## Rollback e reconfiguração

Reconfiguração de um ativo já `COMMISSIONED` deve criar novo commissioning/change ID e repetir os gates afetados.

Nunca editar silenciosamente o ativo de produção.

Regras:
- configuração anterior preservada;
- diff visível antes da aplicação;
- confirmação explícita;
- rollback automático quando seguro;
- se rollback falhar, estado `ROLLBACK_REQUIRED` e alerta operacional/engenharia;
- histórico do ativo preservado em troca de profile ou transporte.

## Retirada do ativo

Retirar não significa apagar.

`RETIRED` deve:
- remover o ativo da frota ativa;
- preservar identidade e evidências;
- preservar histórico de eventos/alarmes conforme política de retenção;
- desabilitar polling/configuração operacional de forma controlada;
- registrar quem retirou, quando e por quê.

## Referências de desenho industrial

- Rapid SCADA recomenda sequência explícita de objetos/linhas/dispositivos, polling, canais e upload do projeto: https://rapidscada.net/docs/en/latest/configuration/configuration-basics
- Rapid SCADA associa current data, historical data e events a canais numerados: https://rapidscada.net/docs/en/latest/configuration/channels
- Rapid SCADA Communicator separa communication line, driver, device e polling: https://rapidscada.net/docs/en/latest/configuration/device-polling
- Rapid SCADA recomenda Git para versionamento dos arquivos de projeto: https://rapidscada.net/docs/en/latest/software-overview/applications
- Ignition também trata configuração do device e estado de conexão como etapas observáveis antes do uso de tags: https://www.docs.inductiveautomation.com/docs/8.3/ignition-modules/opc-ua/opc-ua-drivers

## Critério de aceite desta especificação

Antes de implementar commissioning:
- definir o schema persistente dos estados acima;
- definir autenticação/RBAC;
- definir mecanismo de staging/apply/rollback do Rapid SCADA 6.4.7;
- definir como a configuração do RC Monitor será gerada/ativada;
- criar HIL para a primeira controladora real;
- manter `CMD-001` separado e diferido.

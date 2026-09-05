# Referência: ThingsBoard IoT Gateway vs RC Universal Gateway

> Leia primeiro [`PROJECT_STATE.md`](./PROJECT_STATE.md).

Este documento registra o que foi aproveitado como referência do projeto público `thingsboard/thingsboard-gateway` e, principalmente, onde o RC Universal Gateway deliberadamente diverge.

## O que o ThingsBoard IoT Gateway faz

O ThingsBoard IoT Gateway possui uma arquitetura baseada em connectors, converters, storage e um cliente da plataforma ThingsBoard.

Os connectors estabelecem comunicação com fontes/protocolos como MQTT, Modbus, OPC UA, BACnet, CAN, BLE, SNMP, sockets e outros. Os converters transformam os dados recebidos no modelo ThingsBoard e o storage segura eventos antes do envio à plataforma.

Isso é apropriado para o objetivo do ThingsBoard: integrar equipamentos heterogêneos e produzir telemetria ThingsBoard.

## O que queremos aproveitar

- arquitetura modular por connector;
- connectors customizáveis;
- lifecycle independente por integração;
- reconnect automático;
- supervisão de processos/adapters;
- configuração declarativa;
- métricas e diagnóstico;
- separação entre conectividade e integração de destino;
- extensibilidade sem alterar o core inteiro.

## O que NÃO queremos copiar para o core

O RC Universal Gateway tem objetivo diferente: **ser uma ponte universal de conexão**.

Portanto o core não deve:

- manter converters de telemetria por modelo de equipamento;
- possuir mapas de memória Modbus por DSE/ComAp/PLC/IHM;
- fazer polling de registradores apenas para descobrir RPM, pressão, tensão etc.;
- manter banco/histórico de telemetria;
- converter tudo para um modelo de dados proprietário;
- depender de uma plataforma específica;
- assumir que precisa entender semanticamente o protocolo para transportar a conexão.

## Comparação direta

| Tema | ThingsBoard IoT Gateway | RC Universal Gateway |
|---|---|---|
| Objetivo principal | integrar dados ao ThingsBoard | transportar conexões entre campo e qualquer destino |
| Connector modular | sim | sim |
| Converter semântico | central | fora do core |
| Polling Modbus | sim | apenas se um adapter externo específico exigir; não no core bridge |
| Mapas de registradores | configuração de integração | não pertencem ao Gateway |
| Storage de telemetria | sim | não |
| Plataforma obrigatória | ThingsBoard | nenhuma |
| Payload desconhecido | normalmente precisa de converter para uso | pode ser transportado byte-transparent |
| SCADA como destino | plataforma ThingsBoard | Rapid, FUXA, software de fabricante, outro SCADA, qualquer endpoint |
| Protocolo proprietário desconhecido | custom connector/converter | pode ser bridge transparente sem conhecer semântica |

## Regra de decisão

Se a função necessária é:

> "Receber uma conexão do equipamento/modem e fazer o software de destino conversar com esse equipamento"

ela pertence ao RC Universal Gateway.

Se a função é:

> "Ler registrador X e transformar em RPM/pressão/tensão/alarme"

ela pertence ao driver/SCADA/aplicação de destino, não ao core do Gateway.

## Exemplo PUSR + Rapid SCADA

```text
Controladora
   |
RS485 / Modbus RTU
   |
PUSR TCP Client
   |
Internet
   |
MikroTik
   |
RC Universal Gateway
   |
virtual endpoint / bridge
   |
Rapid SCADA
```

O Rapid envia a requisição Modbus. O Gateway transporta. O PUSR entrega à controladora. A resposta volta pelo mesmo caminho.

O Gateway pode conhecer framing/CRC/Unit ID se isso for necessário para multiplexar ou proteger o stream, mas não precisa conhecer o significado dos registradores.

## Consequência para o código existente

Adapters anteriores de OPC UA read, SNMP read, CoAP GET, MQTT data ingestion e outros são experimentos úteis de biblioteca/conectividade, mas não definem o core bridge-first.

A evolução deve classificar cada componente em uma destas categorias:

1. **transport/bridge core** — fica;
2. **framing/routing helper** — fica se necessário para transportar corretamente;
3. **semantic data reader/converter** — mover para plugin opcional externo ou remover do produto principal;
4. **telemetry persistence** — remover do core;
5. **operations/metrics** — fica, desde que monitore o Gateway e não o processo industrial.

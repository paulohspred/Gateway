# Rapid SCADA -> RC Monitor semantic binding

## Regra de arquitetura

O arquivo de binding contém **números de canais Rapid SCADA**, nunca endereços físicos Modbus.

```text
registrador/coil físico
        |
        | configurado no Rapid SCADA
        v
canal Rapid
        |
        | binding RC
        v
métrica / alarme / evento canônico
```

Function code, endereço, tipo de dado, endian/word swap e aquisição são responsabilidade do projeto Rapid SCADA. Isso mantém o Gateway byte-transparent e impede um segundo banco de memórias de dispositivos no RC Monitor.

## Schema

Schema atual: `1`.

Estrutura:

```json
{
  "schema": 1,
  "profileId": "rc-simulator.reference-controller",
  "metrics": [],
  "alarms": [],
  "events": []
}
```

Campos desconhecidos são rejeitados.

## Métricas

Exemplo numérico:

```json
{"key":"engine.rpm","channelNumber":101,"transform":{"kind":"number"}}
```

Número com escala/offset:

```json
{"key":"battery.voltage","channelNumber":111,"transform":{"kind":"number","scale":0.1}}
```

Booleano:

```json
{
  "key":"breaker.gcb",
  "channelNumber":113,
  "transform":{"kind":"boolean","trueValues":[1],"falseValues":[0]}
}
```

Enum:

```json
{
  "key":"controller.mode",
  "channelNumber":112,
  "transform":{"kind":"enum","enumValues":{"0":"off","1":"manual","2":"auto"}}
}
```

Um valor discreto não mapeado falha fechado.

## Alarmes

O código precisa existir no `alarms.json` do profile.

```json
{
  "code":"LOW_OIL_PRESSURE",
  "channelNumber":115,
  "active":{"kind":"equals","value":1}
}
```

Condições suportadas:

- `equals`;
- `one_of`;
- `nonzero`;
- `bit_set` com bit 0..31;
- `gt`;
- `gte`;
- `lt`;
- `lte`.

Exemplo de alarm word:

```json
{
  "code":"EMERGENCY_STOP",
  "channelNumber":220,
  "active":{"kind":"bit_set","bit":3}
}
```

Assim, coils individuais, alarm words e estados numéricos podem ser normalizados sem código específico de fabricante.

## Eventos

Eventos operacionais podem ser vinculados explicitamente:

```json
{
  "type":"controller.test_event",
  "channelNumber":116,
  "condition":{"kind":"nonzero"},
  "message":"Synthetic controller event"
}
```

O histórico é recuperado pela Web API oficial do Rapid (`GetLastAvailableEvents`) e filtrado pelos canais do binding. Alarmes também produzem eventos canônicos de transição `alarm.raised` e `alarm.cleared` quando a evidência histórica permite identificar a mudança.

## Estado definido e ausência

Rapid status `> 0` significa valor definido para o adapter RC. Status `<= 0` significa indefinido. Um canal indefinido não vira `0`, `false` nem `unknown` arbitrário: a métrica/alarme é omitida ou tratada conforme o contrato fail-closed.

## Profiles draft

`controllers/DRAFT_PROFILES.json` registra o vocabulário canônico útil para ComAp, DSE, SmartGen, MEBAY, Kohler, Basler, PowerZone, Briggs e Generac. Ele não contém register maps.

Para criar um binding de equipamento real:

1. configurar os registradores/coils no Rapid SCADA usando documentação permitida e/ou HIL;
2. criar canais Rapid estáveis;
3. mapear esses canais para um profile RC;
4. testar zero/ausência/escala/enums/alarmes;
5. executar E2E semântico;
6. somente então promover o profile conforme o processo de homologação.

## Escritas

Este schema é exclusivamente read-only. Não há binding de comando. START/STOP/transfer/reset/setpoint continuam fora do backend atual.

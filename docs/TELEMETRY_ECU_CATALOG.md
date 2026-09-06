# RC Monitor — catálogo de telemetria, ECU e proveniência

Status: **contrato de informação para produto/UI e futura expansão do domínio**. Este documento não adiciona registradores físicos ao RC Monitor e não altera o contrato read-only existente.

## Objetivo

Definir quais grandezas um produto profissional de monitoramento de grupos geradores deve ser capaz de representar, distinguindo:

- o que já existe no modelo canônico do RC Monitor;
- o que deve ser considerado para futuras ECU/J1939;
- o que é dado estruturado e não deve ser forçado em uma métrica simples;
- o que é medido, derivado, estático ou não suportado.

A regra é: **a interface mostra somente o que a cadeia real fornece e o profile homologado declara**.

## Proveniência obrigatória como conceito

Uma grandeza pode ter o mesmo significado operacional e vir de fontes diferentes. A engenharia precisa saber a origem.

Classes de fonte propostas:

```text
controller        valor medido/calculado pela controladora de grupo gerador
ecu_j1939         valor originado na ECU e exposto pela controladora/CAN
external_sensor   sensor externo homologado
rapid_derived     cálculo configurado e homologado no Rapid SCADA
manual_spec       dado nominal/placa cadastrado e não telemetria
```

O frontend operacional não precisa exibir a origem em todos os cards, mas a tela de engenharia/diagnóstico deve conseguir mostrá-la.

Nunca rotular um valor como `ECU` se a proveniência real for desconhecida.

## Modelo existente no RC Monitor

O domínio atual já possui as seguintes famílias canônicas.

### Motor

```text
engine.rpm
engine.state
engine.oil_pressure
engine.oil_temperature
engine.oil_level
engine.coolant_temperature
engine.coolant_level
engine.run_hours
engine.starts
```

### Gerador elétrico

```text
generator.status
generator.voltage_l1
generator.voltage_l2
generator.voltage_l3
generator.voltage_l1_l2
generator.voltage_l2_l3
generator.voltage_l3_l1
generator.frequency
generator.current_l1
generator.current_l2
generator.current_l3
generator.power_kw
generator.power_kva
generator.power_kvar
generator.power_factor
generator.energy_kwh
generator.load_percent
```

### Rede

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

### Controladora / transferência

```text
controller.mode
controller.status
controller.temperature
breaker.gcb
breaker.mcb
ats.state
```

### DC / bateria

```text
battery.voltage
battery.current
battery.charger_voltage
battery.charger_current
```

### Combustível

```text
fuel.level
fuel.consumption_rate
fuel.total_consumption
```

### Manutenção

```text
maintenance.hours_remaining
maintenance.due
```

O fato de uma chave existir no domínio **não significa que todo Controller Pack precisa fornecê-la**.

## Regra de presença e zero

Exemplo correto:

```text
fuel.consumption_rate = 0.0 L/h, quality=good
```

significa que a fonte informou zero.

Ausência da chave significa:

```text
unsupported / not supplied / not bound
```

e nunca deve ser renderizada como `0`.

## Consumo de combustível

Esta área precisa ser tratada com rigor porque fontes distintas não são equivalentes.

### `fuel.level`

Nível do tanque ou grandeza equivalente. Pode vir de:
- entrada resistiva/analógica da controladora;
- sensor externo;
- ECU apenas em aplicações específicas.

Unidade esperada pode ser `%`, `L` ou outra unidade homologada pelo profile. A UI não deve converter sem contrato explícito.

### `fuel.consumption_rate`

Consumo instantâneo. Exemplos de unidade:
- `L/h`;
- `gal/h`.

Preferência de confiança:

1. dado ECU/J1939 exposto pela controladora;
2. medição homologada por sensor/medidor dedicado;
3. cálculo homologado no Rapid;
4. estimativa por curva do fabricante, somente se explicitamente classificada como estimativa.

A UI deve distinguir `medido` de `estimado` quando essa distinção existir.

### `fuel.total_consumption`

Totalizador acumulado. Não deve ser calculado no navegador integrando amostras instantâneas.

Se a fonte não expõe totalizador, eventual integração deve ocorrer em camada apropriada com persistência/histórico e especificação própria.

### Eficiência

Possíveis grandezas futuras:

```text
fuel.efficiency_kwh_per_l
fuel.specific_consumption_g_per_kwh
```

Elas **não entram automaticamente** no modelo.

Só devem ser criadas quando houver:
- fórmula aprovada;
- fontes com timestamps/quality compatíveis;
- unidade conhecida;
- densidade/configuração necessária quando aplicável;
- validação HIL ou comparação com fabricante.

O navegador nunca inventa eficiência.

## Extensões futuras de ECU / J1939

Estas chaves são **candidatas de domínio**, não implementação aprovada. Cada uma deve passar pelo processo de profile/fabricante/HIL antes de entrar no código.

### Carga e torque do motor

```text
engine.load_percent
engine.torque_percent
```

Uso operacional:
- avaliar carregamento real do motor;
- comparar carga elétrica com esforço mecânico;
- diagnóstico de sobrecarga/subcarga.

### Ar de admissão / turbo

```text
engine.intake_air_temperature
engine.intake_manifold_pressure
engine.boost_pressure
```

### Escape

```text
engine.exhaust_temperature
```

Quando houver múltiplos bancos/sensores, não reduzir para um único número sem uma regra explícita. Pode ser necessário um modelo indexado futuro.

### Fluidos adicionais

```text
engine.coolant_pressure
fuel.pressure
fuel.temperature
```

### ECU

```text
ecu.supply_voltage
ecu.communication_state
```

`ecu.communication_state` é distinto da comunicação RC Monitor/Rapid. O Gateway/Rapid pode estar online enquanto a controladora reporta falha no CAN/ECU.

### Aftertreatment / emissões

Aplicável apenas a motores/ECUs que realmente exponham esses dados.

Candidatos:

```text
aftertreatment.def_level
aftertreatment.dpf_soot_load
aftertreatment.regeneration_state
aftertreatment.scr_state
```

Essas chaves não devem aparecer para controladoras/motores sem aftertreatment.

## Diagnósticos ECU não são simples métricas

DTCs J1939 formam uma coleção estruturada e podem conter, quando a fonte disponibiliza:

```text
SPN
FMI
occurrence count
lamp state
active/inactive
timestamp
source address
texto normalizado do fabricante
```

Não modelar isso como dezenas de chaves `ecu.dtc_1`, `ecu.dtc_2`, etc.

Contrato futuro recomendado:

```text
GET /api/v1/generators/{id}/diagnostics
```

Exemplo conceitual:

```json
{
  "generatorId": "GER-0147",
  "capturedAt": "...",
  "source": "ecu_j1939",
  "dtcs": [
    {
      "spn": 100,
      "fmi": 1,
      "occurrenceCount": 2,
      "active": true,
      "message": "..."
    }
  ]
}
```

Esse endpoint não existe hoje e não deve ser simulado no frontend.

## Métricas de operação x dados nominais

Dados como:

```text
potência nominal
voltagem nominal
frequência nominal
RPM nominal
número de fases
fabricante/modelo/serial
```

são `manual_spec` ou identidade, não telemetria.

O modelo atual já possui `GeneratorSpec` para nominais. Eles só aparecem se cadastrados/validados.

Não derivar potência nominal do maior kW observado.

## Quality e timestamp

Cada valor operacional deve conservar:
- `value`;
- `unit` quando aplicável;
- `quality`;
- `observedAt`.

O frontend deve renderizar qualidade como informação de processo, não como detalhe técnico escondido.

Regras:

- `good`: valor atual confiável segundo a cadeia.
- `stale`: última leitura conhecida, mas fora da janela esperada.
- `offline`: última leitura conhecida de equipamento/comunicação offline.
- `bad`: fonte reportou dado inválido/ruim.
- `unknown`: qualidade não pôde ser determinada.
- ausência: chave não existe no snapshot.

`stale` e `offline` não devem parecer valores atuais.

## Controller Pack como contrato de capacidade

Cada profile deve evoluir para declarar, no mínimo:

```text
metric key
kind
unit
required / optional
staleAfterSeconds
sourceClass
supportLevel
```

Onde `supportLevel` pode distinguir, por exemplo:

```text
homologated
lab
experimental
unsupported
```

A proveniência não contém o endereço físico do registrador. O mapa físico continua no Rapid SCADA.

## Grupos de tela recomendados para detalhe do gerador

A UI não deve simplesmente listar todas as métricas em uma grade única.

### Sinótico

- mains state;
- MCB;
- GCB;
- ATS state;
- engine/generator state;
- kW;
- Hz;
- comunicação/quality.

### Motor / ECU

- RPM;
- oil pressure/temp/level;
- coolant temp/level;
- load/torque quando suportado;
- intake/boost/exhaust quando suportado;
- ECU communication;
- DTC summary.

### Elétrica

- phase/line voltages;
- currents;
- frequency;
- kW/kVA/kVAr;
- PF;
- energy;
- load percent.

### Combustível

- level;
- instantaneous consumption;
- total consumption;
- efficiency apenas quando homologada.

### DC

- battery voltage/current;
- charger voltage/current.

### Histórico e tendências

Só deve existir quando o backend oferecer uma API histórica real. O frontend atual não pode fabricar séries com snapshots locais.

## Dados que não devem ser calculados no frontend

- consumo total por integração local;
- disponibilidade de 30 dias;
- MTBF/MTTR;
- eficiência combustível;
- health score;
- potência nominal;
- limites de alarme;
- percentuais de barra baseados em limites inventados;
- manutenção restante;
- severidade de DTC.

Esses valores precisam vir de uma fonte/algoritmo homologado e auditável.

## Referências técnicas usadas para o catálogo

- ComAp InteliLite suporta EFI/J1939 e mensagens diagnósticas: https://www.comap-control.com/products/controllers/single-gen-set-controllers/intelilite/intelilite-nt-amf-25/
- DSE descreve coleta de kW, uso de combustível e horas da ECU para consumo/eficiência em DSE86xx MKII: https://www.deepseaelectronics.com/public/uploads/linkedfiles/Technical%20Focus%2086XX%20MKII.pdf
- SmartGen documenta, em controladores com J1939, coolant pressure/level, fuel temperature/pressure, inlet/exhaust temperature, turbo pressure, fuel consumption e total fuel consumption: https://www.smartgen.cn/yun/data/download/APC715_en.pdf

## Critério antes de adicionar uma nova MetricKey

1. necessidade operacional clara;
2. definição e unidade inequívocas;
3. fonte fabricante/controller/ECU documentada;
4. representação possível no Rapid;
5. profile declara suporte;
6. binding real existe;
7. HIL ou evidência de homologação disponível;
8. teste de ausência/zero/quality;
9. nenhuma duplicação semântica com chave existente.

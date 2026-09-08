# Integração Rapid SCADA v6 — RC Universal Gateway

## Objetivo

Integrar o RC Universal Gateway ao Rapid SCADA **sem incorporar ou copiar o código do Rapid SCADA** e sem alterar a filosofia bridge-first do Gateway.

A integração usa os canais e drivers nativos do Rapid SCADA. O Gateway entrega um endpoint TCP local previsível; o Rapid SCADA Communicator conecta nesse endpoint e continua responsável pelo protocolo de aplicação, endereços Modbus, templates e tags.

Referência de compatibilidade revisada: `RapidScada/scada-v6`, branch `master`, commit `1fd36080c7830303f921672fdaee335a06e7ae50`.

Rapid SCADA Standard é Apache-2.0 e permanece um produto de terceiros independente. Nenhum binário ou fonte Rapid SCADA é incorporado ao release do RC Universal Gateway.

## Compatibilidade confirmada por contrato de software

No Rapid SCADA v6 revisado:

- `DrvCnlBasic` oferece canais `SerialPort`, `TcpClient`, `TcpServer` e `Udp`;
- o canal TCP client aceita `Host`, `TcpPort`, `ReconnectAfter`, `StayConnected`, `DisconnectOnError`, `Behavior` e `ConnectionMode`;
- `Behavior=Master` é apropriado para polling de controladoras;
- o driver `DrvModbus` aceita `TransMode=RTU`, `ASCII` ou `TCP`;
- em Linux, o Communicator roda como `scadacomm6.service` na instalação padrão documentada pelo projeto.

Isso permite integrar o Gateway sem plugin proprietário obrigatório para Modbus TCP e Modbus RTU/ASCII transportado de forma transparente.

## Regra principal de mapeamento

O `TransMode` do Rapid SCADA deve representar o protocolo que realmente existe **dentro do stream entregue pelo Gateway**.

| Bytes entregues ao Rapid SCADA | Rapid SCADA `TransMode` | Observação |
|---|---|---|
| Modbus TCP com MBAP | `TCP` | dispositivo/modem realmente fala Modbus TCP |
| Modbus RTU raw dentro de TCP | `RTU` | serial-over-TCP / RTU-over-TCP, CRC permanece no payload |
| Modbus ASCII raw dentro de TCP | `ASCII` | framing ASCII permanece no payload |

O Gateway não tenta detectar nem converter esses formatos automaticamente.

## Perfil Rapid SCADA recomendado

Para cada tunnel TCP do Gateway exposto ao Rapid SCADA, configure no Communicator:

```text
Channel type:       TcpClient
Channel driver:     DrvCnlBasic
Behavior:           Master
Host:               127.0.0.1
TcpPort:            <porta consumer do Gateway>
ReconnectAfter:     5
StayConnected:      true
DisconnectOnError:  false
ConnectionMode:     Shared
```

`ConnectionMode=Shared` é a recomendação padrão para um tunnel do Gateway. Em especial para RS485 multidrop, ele evita criar várias conexões TCP concorrentes para o mesmo barramento serial.

Depois configure o dispositivo com o driver de aplicação correspondente. Para Modbus, use `DrvModbus` e selecione o `TransMode` conforme a tabela acima.

## Cenário A — dispositivo Modbus TCP direto/VPN

```text
Controladora :502
      |
VPN / LAN
      |
Gateway field connect
      |
Gateway consumer 127.0.0.1:25020
      |
Rapid SCADA TcpClient + DrvModbus TransMode=TCP
```

Config de Gateway: `configs/rapid-scada.modbus-tcp.example.json`.

Rapid SCADA:

```text
Host=127.0.0.1
TcpPort=25020
Behavior=Master
ConnectionMode=Shared
TransMode=TCP
```

O endereço Modbus do equipamento e o template de registradores continuam no Rapid SCADA.

## Cenário B — modem/serial server enviando Modbus RTU raw sobre TCP

```text
Controladora RS232/RS485
      |
modem / serial server
      |
TCP contendo RTU raw
      |
Gateway field listen :15003
      |
Gateway consumer 127.0.0.1:25003
      |
Rapid SCADA TcpClient + DrvModbus TransMode=RTU
```

Config de Gateway: `configs/rapid-scada.rtu-over-tcp.example.json`.

Rapid SCADA:

```text
Host=127.0.0.1
TcpPort=25003
Behavior=Master
ConnectionMode=Shared
TransMode=RTU
```

Se o modem fizer conversão real RTU↔Modbus TCP, altere o Rapid SCADA para `TransMode=TCP`. Não confundir serial-over-TCP com Modbus TCP.

## Cenário C — RS485 local multidrop com várias controladoras

```text
Rapid SCADA
   |
127.0.0.1:25100
   |
Gateway
   |
/dev/ttyUSB0 RS485
   |
   +-- ID 1
   +-- ID 2
   +-- ID 3
   +-- ID 4
   +-- ID 5
```

Config de Gateway: `configs/rapid-scada.rs485-multidrop.example.json`.

Rapid SCADA:

```text
Channel type=TcpClient
Driver=DrvCnlBasic
Host=127.0.0.1
TcpPort=25100
Behavior=Master
ConnectionMode=Shared
TransMode=RTU
```

Adicione todas as controladoras na **mesma communication line** do Rapid SCADA, cada uma com seu endereço Modbus numérico exclusivo. Exemplos: 1, 2, 3, 4 e 5.

Modelos diferentes podem compartilhar o mesmo barramento desde que:

- usem o mesmo padrão elétrico RS485;
- usem parâmetros seriais compatíveis (baud, bits, paridade e stop bits);
- o protocolo permita coexistência no mesmo barramento;
- os endereços sejam exclusivos;
- o cabeamento, terminação e bias estejam corretos;
- exista apenas um mestre efetivo controlando o barramento.

Cada modelo pode usar seu próprio template de registradores no Rapid SCADA.

## Cenário D — controladora RS232 usando conversor RS232↔RS485 e modem RS485↔TCP

```text
Controladora RS232
      |
conversor ativo RS232 <-> RS485
      |
modem RS485 <-> TCP
      |
Gateway
      |
Rapid SCADA
```

É compatível quando o conversor é eletricamente correto e transparente e o modem preserva o protocolo esperado. O Rapid SCADA deve usar `RTU` se o TCP contém Modbus RTU raw, ou `TCP` se o modem efetivamente converte para Modbus TCP.

## Reverse TCP e pareamento

Para modem que conecta no Gateway e Rapid SCADA que também conecta no Gateway, o tunnel é `listen↔listen`. Esse modelo mantém pareamento conservador e não permite paralelismo ambíguo no mesmo tunnel.

Use **um tunnel/porta por modem ou conexão de campo que precise identidade própria**. Não use uma única porta `listen↔listen` para vários modems esperando que o Gateway adivinhe qual conexão pertence a qual linha Rapid SCADA.

## Concorrência e RS485

Não aumente `maxConcurrentPairs` em um tunnel que representa um único barramento serial RS485. Uma linha Modbus RTU multidrop deve ser tratada como recurso compartilhado serializado pelo mestre.

Para muitos equipamentos IP independentes, prefira tunnels independentes ou uma topologia `listen↔connect` com limites explícitos quando a identidade do target for inequívoca.

## Comandos de escrita

O Gateway é bidirecional. Se o Rapid SCADA tiver comandos habilitados e o protocolo/driver permitir escrita, os bytes de comando podem atravessar o tunnel.

Portanto, autorização de comandos deve ser controlada no Rapid SCADA, no projeto SCADA e na rede. O `commandPlaneEnabled=false` do Gateway significa que o próprio Gateway não oferece uma API industrial genérica de comando; isso não transforma uma ponte duplex em read-only.

## USB HID / ComAp

Rapid SCADA `DrvModbus` não deve ser conectado diretamente ao stream `length32be` de HID esperando Modbus. O envelope `length32be` é framing de transporte do Gateway, não um protocolo Rapid SCADA.

Para ComAp por USB HID, continua necessário confirmar em HIL:

- enumeração Linux;
- VID/PID/serial;
- report descriptor;
- tamanhos/report IDs;
- protocolo de aplicação;
- necessidade de adapter ComAp Direct.

## Instalação na mesma VM

A instalação Linux documentada pelo Rapid SCADA utiliza `/opt/scada` e serviços como:

```text
scadaagent6.service
scadaserver6.service
scadacomm6.service
scadaweb6.service
```

O Gateway usa `/opt/rc-gateway` e `rc-gateway.service`, portanto não há colisão de layout prevista.

Para co-localização, mantenha os endpoints consumer do Gateway em loopback (`127.0.0.1`) para que o tráfego SCADA↔Gateway não seja exposto na rede.

## Acceptance test

Depois de instalar e configurar o Rapid SCADA:

```bash
sudo RAPID_SCADA_COMM_SERVICE=scadacomm6.service \
  /opt/rc-gateway/current/scripts/rapid-scada-acceptance.sh
```

O teste verifica:

- serviço do Gateway ativo;
- `/healthz` e `/readyz`;
- `scadacomm6.service` ativo quando solicitado;
- command plane do Gateway desabilitado;
- pelo menos uma sessão ativa entre consumidor e Gateway quando `RAPID_SCADA_EXPECT_SESSION=1`.

Para Rapid SCADA em outra máquina:

```bash
sudo RAPID_SCADA_REQUIRE_LOCAL_SERVICE=0 \
  RAPID_SCADA_EXPECT_SESSION=1 \
  /opt/rc-gateway/current/scripts/rapid-scada-acceptance.sh
```

## Estado de validação

A compatibilidade acima é **integração por contrato** baseada nas interfaces nativas do Rapid SCADA v6 e nos transportes já validados do Gateway.

Ela só passa para `vm_accepted` depois de executar o Rapid SCADA real contra o artifact real na VM. Comunicação com controladoras físicas continua sujeita ao HIL específico da combinação.

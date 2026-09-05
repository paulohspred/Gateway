# USB HID / ComAp — transporte, identidade estável e HIL

## Escopo

O Gateway implementa um provider Linux para dispositivos USB HID expostos como `/dev/hidrawN`. O provider trata USB HID como **transporte de reports**, preservando a fronteira de cada report através de `AF_UNIX/SOCK_SEQPACKET` (`unixpacket`).

Ele não interpreta registradores, não converte ComAp Direct para Modbus e não tenta emular um dispositivo USB para software Windows.

## Identificação do equipamento

O número `/dev/hidrawN` não é uma identidade estável: pode mudar entre boots, hotplug ou alterações na enumeração USB. Por isso o provider aceita duas formas de seleção.

### Caminho explícito

```json
{
  "id": "comap-il4-amf9-usb",
  "socket": "/run/rc-gateway/comap-il4-amf9.hid.sock",
  "device": "/dev/hidraw0",
  "maxReportBytes": 4096,
  "allowWrite": false
}
```

### Seletor estável VID/PID/serial

Os valores abaixo mostram apenas o **formato**; use os valores obtidos na controladora real pelo probe.

```json
{
  "id": "comap-il4-amf9-usb",
  "socket": "/run/rc-gateway/comap-il4-amf9.hid.sock",
  "vendorId": "1a2b",
  "productId": "003c",
  "serialNumber": "SERIAL-DA-UNIDADE",
  "maxReportBytes": 4096,
  "allowWrite": false
}
```

Regras:

- `vendorId` e `productId` são IDs USB de 16 bits em hexadecimal e devem ser configurados juntos;
- `serialNumber`, quando usado, exige VID/PID;
- sem `device`, o Gateway procura em `/sys/class/hidraw` e exige **exatamente um** match;
- múltiplos equipamentos com o mesmo VID/PID exigem `serialNumber` ou `device` explícito;
- é permitido informar `device` **e** VID/PID/serial; nesse caso o Gateway verifica a identidade e falha se o nó não corresponder ao equipamento esperado;
- `socket` deve ser absoluto e é publicado com modo `0660`;
- `maxReportBytes` usa 4096 por padrão e aceita no máximo 16384;
- `allowWrite=false` é o padrão fail-closed;
- symlink no caminho `/dev/hidrawN` é rejeitado;
- o nó precisa existir e ser character device **antes de o provider declarar readiness**.

A resolução por VID/PID/serial foi implementada diretamente sobre sysfs/hidraw, sem incorporar código do projeto externo `hid2tcp` e sem adicionar dependência Python/libusb ao runtime.

## Probe de bancada

Execute:

```bash
bash scripts/probe-usb-hid.sh
```

O probe informa, quando disponíveis:

- `/dev/hidrawN`;
- `HID_ID`, VID/PID normalizados, nome e serial HID;
- VID/PID, fabricante, produto e serial do dispositivo USB pai;
- número da interface USB;
- tamanho e SHA-256 do HID report descriptor;
- permissões/owner/group do nó hidraw.

Isso permite transformar uma configuração temporária baseada em `/dev/hidraw0` em uma configuração estável baseada na identidade real do equipamento.

## Contrato de transporte

O socket do provider é `unixpacket`. Cada mensagem corresponde a exatamente um report HID.

```text
/dev/hidrawN -> unixpacket = field_to_consumer
unixpacket -> /dev/hidrawN = consumer_to_field
```

Quando `allowWrite=false`, qualquer tentativa de enviar um report ao dispositivo encerra a sessão com erro de escrita desabilitada.

O bridge genérico do Gateway também entende `network: "unixpacket"`. A validação de produção conhece o tipo real dos sockets de provider:

- serial publica `unix`/stream;
- USB HID publica `unixpacket`;
- SocketCAN publica `unixpacket`.

Uma configuração que tente consumir um provider HID/CAN usando `network: "unix"` é rejeitada antes do startup.

## Packet framing ao cruzar para TCP/stream

`unixpacket` preserva mensagens; TCP e `unix` são streams. Converter entre esses modelos sem framing destruiria a fronteira dos reports. O Gateway portanto **não faz isso silenciosamente**.

Se um túnel possuir exatamente uma ponta `unixpacket` e outra ponta stream, a configuração exige:

```json
{
  "packetFraming": "length32be"
}
```

No lado stream, cada pacote é codificado como:

```text
4 bytes: comprimento uint32 big-endian
N bytes: payload original, sem alteração
```

O sentido inverso decodifica o mesmo envelope e produz um único pacote `unixpacket` por frame. O limite atual é 64 KiB por frame.

Esse framing é um contrato do transporte do Gateway. **Rapid SCADA, FUXA ou um software Modbus comum não entendem esse envelope automaticamente.** Ele é útil para adapter dedicado, cliente remoto compatível ou transporte controlado entre componentes que conhecem o framing.

## InteliLite 4 AMF 9

A InteliLite 4 AMF 9 possui USB A↔B para conexão local com ferramentas ComAp. O provider cobre o caso em que essa interface é enumerada pelo Linux como hidraw.

**Não declarar a combinação InteliLite 4 AMF 9 + USB como production validated antes do HIL.** Ainda precisamos confirmar na unidade física:

1. enumeração USB real no Linux;
2. VID/PID, nome e número de série;
3. interface e HID report descriptor;
4. tamanho e report IDs usados pela interface;
5. comportamento de leitura/escrita durante uma sessão inofensiva do InteliConfig;
6. se o protocolo sobre USB é ComAp Direct e qual framing/handshake ele utiliza;
7. comportamento de reconexão após remoção/reinserção e power-cycle;
8. permissões/udev corretas para executar o serviço sem ampliar privilégios desnecessariamente.

Também podem ser úteis durante HIL:

```bash
ls -l /dev/hidraw*
udevadm info --query=all --name=/dev/hidraw0
```

## Relação com Rapid SCADA / FUXA

Rapid SCADA ou FUXA normalmente esperam Modbus TCP/RTU ou outro protocolo de aplicação. O provider HID sozinho não transforma ComAp Direct em Modbus.

Caminhos válidos:

```text
A) ComAp + módulo Ethernet/RS232 -> Modbus -> Gateway -> SCADA
B) ComAp USB HID -> provider HID -> adapter ComAp Direct -> SCADA
C) ComAp USB HID -> provider HID -> consumidor que já entenda ComAp Direct
D) ComAp USB HID -> unixpacket -> length32be/TCP -> cliente compatível -> adapter/consumidor
```

O caminho B continua sendo o alvo caso a controladora fale ComAp Direct pela USB. O adapter deve permanecer separado do core bridge-first e só deve ser implementado com documentação suficiente ou captura/HIL controlado da comunicação real.

## Referência arquitetural externa

O projeto `embyt/hid2tcp` foi analisado como referência conceitual. Ele demonstrou a utilidade de identificar hardware por VID/PID e de expor USB sobre rede, mas o Gateway não copia seu código. Além de o repositório externo usar GPL-3.0, o desenho do Gateway mantém controles adicionais: fail-closed, socket local por padrão, identidade verificável, framing explícito na transição packet↔stream, métricas, readiness, testes e integração com os gates de release.

## Próximo gate de bancada

Conectar uma InteliLite 4 AMF 9 real por USB a um host Linux, executar o probe e registrar os metadados. Depois capturar uma sessão de leitura inofensiva do InteliConfig para determinar o contrato de aplicação. Até isso ocorrer, o status correto é **USB HID transport implemented / ComAp application adapter pending HIL**.

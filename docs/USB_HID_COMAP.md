# USB HID / ComAp — transporte e HIL

## Escopo

O Gateway implementa um provider Linux para dispositivos expostos como `/dev/hidrawN`. O provider trata USB HID como **transporte de reports**, preservando a fronteira de cada report através de um socket Unix `unixpacket`.

Ele não interpreta registradores, não converte ComAp Direct para Modbus e não tenta emular um dispositivo USB para software Windows.

## Configuração

Exemplo:

```json
{
  "usbHidProviders": [
    {
      "id": "comap-il4-amf9-usb",
      "socket": "/run/rc-gateway/comap-il4-amf9.hid.sock",
      "device": "/dev/hidraw0",
      "maxReportBytes": 4096,
      "allowWrite": false
    }
  ]
}
```

Regras:

- `device` deve ser `/dev/hidrawN`;
- `socket` deve ser absoluto e é publicado com modo `0660`;
- `maxReportBytes` usa 4096 por padrão e aceita no máximo 16384;
- `allowWrite=false` é o padrão fail-closed;
- o arquivo de dispositivo é validado como character device quando um consumidor abre uma sessão;
- symlink no caminho do dispositivo é rejeitado no runtime;
- somente um consumidor é atendido por vez por provider, seguindo a mesma disciplina de acesso exclusivo dos providers serial/CAN.

## Contrato de transporte

O socket do provider é `AF_UNIX/SOCK_SEQPACKET` (`unixpacket`). Cada mensagem do socket corresponde a exatamente um report HID.

Direções:

```text
/dev/hidrawN -> unixpacket = field_to_consumer
unixpacket -> /dev/hidrawN = consumer_to_field
```

Quando `allowWrite=false`, qualquer tentativa de enviar um report ao dispositivo encerra a sessão com erro de escrita desabilitada.

## InteliLite 4 AMF 9

A InteliLite 4 AMF 9 possui USB A↔B para conexão local com ferramentas ComAp. O provider deste projeto cobre o caso em que essa interface é enumerada pelo Linux como `hidraw`.

**Não declarar a combinação InteliLite 4 AMF 9 + USB como production validated antes do HIL.** Ainda precisamos confirmar na unidade física:

1. enumeração USB real no Linux;
2. VID/PID, nome e número de série;
3. tamanho e report IDs usados pela interface;
4. comportamento de leitura/escrita durante InteliConfig;
5. se o protocolo sobre USB é ComAp Direct e qual framing/handshake ele utiliza;
6. comportamento de reconexão após remoção/reinserção e power-cycle.

Use:

```bash
bash scripts/probe-usb-hid.sh
```

Também são úteis durante HIL:

```bash
ls -l /dev/hidraw*
udevadm info --query=all --name=/dev/hidraw0
```

## Relação com Rapid SCADA / FUXA

Rapid SCADA ou FUXA normalmente esperam Modbus TCP/RTU ou outro protocolo de aplicação. O provider HID sozinho não transforma ComAp Direct em Modbus.

Existem três caminhos válidos:

```text
A) ComAp + módulo Ethernet/RS232 -> Modbus -> Gateway -> SCADA
B) ComAp USB HID -> provider HID -> adapter ComAp Direct -> SCADA
C) ComAp USB HID -> provider HID -> consumidor que já entenda ComAp Direct
```

O caminho B exige um adapter de protocolo separado. Ele deve ser implementado somente com documentação pública suficiente ou captura/HIL controlado da comunicação real, sem inserir polling ou banco de registradores no core do Gateway.

## Próximo gate de bancada

Conectar uma InteliLite 4 AMF 9 real por USB a um host Linux, executar o probe e registrar os metadados. Depois capturar uma sessão de leitura inofensiva do InteliConfig para determinar o contrato de aplicação. Até isso ocorrer, o status correto é **USB HID transport implemented / ComAp application adapter pending HIL**.

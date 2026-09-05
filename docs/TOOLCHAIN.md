# Toolchain

O módulo standalone é `github.com/paulohspred/Gateway` e declara **Go 1.27** (`go 1.27.0`). O CI usa **Go 1.27.1**.

## Dependências Go de runtime

- `go.bug.st/serial v1.8.0` — acesso serial RS232/RS422/RS485;
- `golang.org/x/sys v0.43.0` — syscalls Linux usadas pelo provider SocketCAN/CAN-FD.

TCP, TLS, Unix sockets, UDP, USB HID via `hidraw`, concorrência, configuração, HTTP administrativo e métricas usam a biblioteca padrão. O provider USB HID não adiciona CGO/libusb/hidapi, preservando o build Linux amd64/arm64 atual.

## Ferramentas e dependências de supply chain

- `govulncheck` pinado em `8fcedea455d953a0f8470e1f41420bb6f2e72665` por compatibilidade com Go 1.27.1;
- `cyclonedx-gomod v1.12.0` para SBOM CycloneDX;
- `actions/checkout`, `actions/setup-go` e `actions/upload-artifact` usam tags de versão principal no workflow atual;
- `actions/checkout` roda com `persist-credentials: false`.

Pinagem imutável das GitHub Actions por commit SHA continua desejável, mas deve ser reintroduzida somente junto de um workflow validado. Novas dependências só devem entrar quando um provider concreto exigir, com versão fixada, `govulncheck`, revisão de licença/proveniência e atualização deste documento.

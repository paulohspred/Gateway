# Toolchain

O módulo standalone é `github.com/paulohspred/Gateway` e declara **Go 1.27** (`go 1.27.0`). O CI usa **Go 1.27.1**.

## Dependências Go de runtime

- `go.bug.st/serial v1.8.0` — acesso serial RS232/RS422/RS485;
- `golang.org/x/sys v0.43.0` — syscalls Linux usadas pelo provider SocketCAN/CAN-FD.

TCP, TLS, Unix sockets, UDP, concorrência, configuração, HTTP administrativo e métricas usam a biblioteca padrão.

## Ferramentas de supply chain no CI

- `govulncheck` pinado em `8fcedea455d953a0f8470e1f41420bb6f2e72665` por compatibilidade com Go 1.27.1;
- `cyclonedx-gomod v1.12.0` para SBOM CycloneDX;
- GitHub artifact provenance attestation para releases promovidas em `main`.

Novas dependências só devem entrar quando um provider concreto exigir, com versão fixada, `govulncheck`, revisão de licença/proveniência e atualização deste documento.

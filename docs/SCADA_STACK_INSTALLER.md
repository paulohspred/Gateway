# Instalador único — RC Gateway + Rapid SCADA v6.4.7

## Objetivo

Este instalador prepara uma VM **Ubuntu Server 24.04** para a fase atual do SCADA de geradores:

```text
controladora / modem / VPN / RS485 / Ethernet
                 |
            RC Gateway
                 |
             loopback TCP
                 |
       Rapid SCADA v6.4.7
```

Backend próprio, frontend próprio e demais serviços ficam fora deste instalador.

O Rapid SCADA continua sendo software de terceiro e **não é incorporado ao código ou ao pacote proprietário do RC Gateway**. O operador coloca o pacote Linux oficial do Rapid SCADA no mesmo diretório do kit.

## Arquivos no diretório

Para uma VM amd64, o diretório pode ficar assim:

```text
scada-kit/
├── install-scada-stack.sh
├── rc-gateway_<versao>_linux_amd64.tar.gz
├── rc-gateway_<versao>_linux_amd64.tar.gz.sha256
├── rapidscada_6.4.7_all.deb
└── rc-gateway.json                 # opcional
```

O instalador também aceita o ZIP Linux oficial do Rapid SCADA no lugar do `.deb`, desde que `unzip` esteja instalado ou a VM tenha acesso aos repositórios APT.

Se `rc-gateway.json` não existir, o instalador usa `rc-gateway.safe.json` quando presente ou gera uma configuração segura com **zero tunnels de campo**. Isso instala e inicia o stack sem tentar se conectar a uma controladora ainda não homologada.

## Instalação em uma VM limpa

Entre no diretório e execute:

```bash
sudo bash install-scada-stack.sh
```

O script:

1. detecta `amd64` ou `arm64` e escolhe exatamente um archive compatível;
2. confere SHA-256 do RC Gateway;
3. rejeita paths, links e tipos inseguros no archive do Gateway;
4. extrai o instalador seguro existente dentro do próprio artifact e executa seu dry-run;
5. valida `Package=rapidscada`, arquitetura e versão `6.4.7` do `.deb`;
6. instala `ca-certificates`, `curl`, `nginx`, `unzip` e ASP.NET Core Runtime 8.0 quando necessário;
7. instala Rapid SCADA usando o pacote local;
8. instala o RC Gateway pelo installer health-gated existente;
9. habilita `scadaagent6`, `scadaserver6`, `scadacomm6`, `scadaweb6`, `nginx` e `rc-gateway` no boot;
10. força o Webstation upstream a ouvir em `127.0.0.1:10008` por systemd override;
11. configura Nginx somente em `127.0.0.1:80`;
12. testa serviços, Gateway `/readyz`, Webstation e proxy Nginx;
13. grava evidência da instalação em `/var/lib/rc-scada-stack/install-state.env`.

## Segurança por padrão

O instalador **não expõe o Rapid SCADA Webstation na LAN ou Internet**. O serviço upstream v6.4.7 inicia em `0.0.0.0:10008`; o stack aplica um override para `127.0.0.1:10008`, e o Nginx também fica em loopback.

Essa decisão é proposital porque a documentação do Rapid SCADA informa credenciais iniciais padrão. Troque as credenciais e defina posteriormente a política de TLS/reverse proxy/firewall antes de qualquer exposição remota.

O installer não ativa start/stop de gerador, transferência, reset ou setpoints. Esses comandos pertencem à configuração do Rapid SCADA e só devem ser habilitados após HIL específico por modelo/firmware e revisão de interlocks.

## Configuração do Gateway

Para instalar já com uma topologia de teste, crie no kit:

```text
rc-gateway.json
```

ou informe explicitamente:

```bash
sudo bash install-scada-stack.sh \
  --gateway-config /caminho/rc-gateway.json
```

Para a primeira instalação da VM, é aceitável instalar com o baseline seguro sem tunnels e depois substituir `/etc/rc-gateway.json` por um dos perfis homologados.

## Dry-run antes da instalação

Sem alterar o host:

```bash
bash install-scada-stack.sh --dry-run
```

O dry-run valida o Gateway, o checksum, o package metadata do Rapid SCADA e a configuração candidata do Gateway.

## Pacote Rapid SCADA

Baseline desta fase:

```text
Rapid SCADA v6.4.7
```

O pacote oficial Linux pode ser baixado separadamente do site do fornecedor. Depois de extrair o pacote Linux, coloque `rapidscada_6.4.7_all.deb` no diretório do kit.

Para kit de produção, registre o SHA-256 do arquivo de origem de uma das duas formas:

```text
rapidscada_6.4.7_all.deb.sha256
```

ou:

```bash
sudo RC_SCADA_RAPID_SHA256='<64-hex>' bash install-scada-stack.sh
```

O checksum local protege contra corrupção/substituição do arquivo após o kit ser congelado; ele não substitui a validação da origem do fornecedor.

## Dependências e modo offline

Por padrão, o script usa apenas os repositórios APT **já configurados na VM**. Ele não adiciona repositórios externos automaticamente.

O Rapid SCADA v6.4.7 requer ASP.NET Core Runtime 8.0.x no Linux. Se `aspnetcore-runtime-8.0` não estiver disponível no APT configurado, instale o runtime 8.0 segundo a documentação oficial e execute o installer novamente.

Para impedir qualquer download:

```bash
sudo bash install-scada-stack.sh --offline
```

Nesse modo `curl`, `nginx`, `unzip` e ASP.NET Core Runtime 8.0 já precisam existir no host e as dependências do `.deb` Rapid SCADA precisam estar satisfeitas.

## Atualização

Em instalação existente, o script bloqueia por padrão para evitar sobrescrever um SCADA em operação. Depois de backup e janela autorizada:

```bash
sudo bash install-scada-stack.sh --upgrade
```

O mecanismo de release do Gateway continua usando instalação health-gated e rollback próprio. Atualizações do Rapid SCADA devem seguir também a política de backup e compatibilidade do projeto Rapid SCADA.

## Nginx já existente

Em VM limpa, o installer substitui apenas o link do site default do Nginx pelo site `rc-scada`.

Se encontrar outros sites habilitados, falha para não modificar uma instalação de terceiros. O override explícito para ambiente conhecido é:

```bash
sudo RC_SCADA_ALLOW_EXISTING_NGINX=1 bash install-scada-stack.sh
```

Use isso somente depois de revisar conflitos de portas e proxy.

## Pós-instalação

Confirme:

```bash
systemctl status rc-gateway.service --no-pager
systemctl status scadacomm6.service --no-pager
systemctl status scadaserver6.service --no-pager
systemctl status scadaweb6.service --no-pager
curl -fsS http://127.0.0.1:18080/readyz
curl -fsSL http://127.0.0.1/ >/dev/null
cat /var/lib/rc-scada-stack/install-state.env
```

Depois configure a communication line no Rapid SCADA e execute:

```bash
sudo /opt/rc-gateway/current/scripts/rapid-scada-acceptance.sh
```

Para o preflight de produção, informe as portas consumer reais do Gateway:

```bash
sudo RAPID_SCADA_GATEWAY_PORTS='25020' \
  /opt/rc-gateway/current/scripts/rapid-scada-production-acceptance.sh
```

A instalação bem-sucedida significa **stack instalado e serviços saudáveis**. O status `production_validated` continua dependendo de VM acceptance, sessão real Rapid SCADA↔Gateway, HIL da controladora/modem/VPN e soak de 24 h/7 dias.

# Instaladores RC — Gateway + Rapid SCADA 6.4.7 + RC Monitor + Frontend

## Escopo

A release RC agora contém o runtime completo da aplicação própria:

```text
RC Gateway
RC Monitor
Frontend React/Vite pré-compilado
systemd units
configs/perfis de laboratório
installers e acceptance scripts
```

O **Rapid SCADA 6.4.7 continua sendo pacote de terceiro separado** e nunca é incorporado ao artifact proprietário.

Há dois fluxos deliberadamente distintos:

1. `install-scada-stack.sh`: instala o baseline Gateway + Rapid SCADA + hardening interno;
2. `install-rc-lab-stack.sh`: orquestra o baseline e, em seguida, configura Rapid Web API read-only, RC Monitor, frontend/Nginx e acceptance de laboratório.

O segundo fluxo é **SIMULATION_TEST_ONLY**. Ele não homologa controladora física e não altera `PRODUCTION_VALIDATED=false`.

## Kit de uma VM amd64

```text
scada-kit/
├── install-scada-stack.sh
├── install-rc-lab-stack.sh
├── rc-gateway_<versao>_linux_amd64.tar.gz
├── rc-gateway_<versao>_linux_amd64.tar.gz.sha256
├── rapidscada_6.4.7_linux_en.zip         # ZIP Linux oficial recomendado
├── rc-gateway.safe.json
└── rc-monitor.rapid.env                   # criado localmente, NÃO versionar
```

O archive RC contém `bin/rc-gateway`, `bin/rc-monitor`, `frontend/index.html`, assets hashed, systemd units, perfis, configs e scripts. O installer recusa releases onde `rc-monitor` não é executável ou o frontend não está presente.


### Integridade do Rapid SCADA

A instalação é **fail-closed** para o pacote Rapid SCADA: pacote sem checksum confiável é recusado.

Para o ZIP oficial Linux 6.4.7 usado e validado no ciclo LAB, o instalador fixa:

```text
rapidscada_6.4.7_linux_en.zip
SHA256=48e8c8c33b8380fddc7c6012d6856123cb6328a1645e784e97a29aa525311180
```

Para um `.deb` direto ou qualquer fonte diferente, forneça `RC_SCADA_RAPID_SHA256=<sha256>` ou um arquivo `<pacote>.sha256` adjacente. O hash deve vir de uma fonte previamente confiável; não gere o sidecar a partir de um pacote não verificado no momento da instalação.

## Credencial Rapid local

Crie o arquivo somente na VM/kit protegido:

```bash
sudo install -m 0600 -o root -g root /dev/null /root/rc-monitor.rapid.env
sudoedit /root/rc-monitor.rapid.env
```

Conteúdo:

```text
RC_RAPID_USER=<usuario_read_only>
RC_RAPID_PASSWORD=<senha_local>
```

O arquivo precisa estar em modo `0600` ou `0400`. O instalador copia o segredo para `/etc/rc-monitor.env` em modo `0600`; nenhum segredo entra no JSON, no frontend ou no repositório.

## Instalação completa do laboratório

Para manter o frontend somente em loopback:

```bash
sudo bash install-rc-lab-stack.sh \
  --dir /caminho/scada-kit \
  --rapid-env /root/rc-monitor.rapid.env
```

Para uma VM descartável acessível pela LAN, a exposição HTTP precisa ser explícita:

```bash
sudo bash install-rc-lab-stack.sh \
  --dir /caminho/scada-kit \
  --rapid-env /root/rc-monitor.rapid.env \
  --frontend-bind 0.0.0.0:80
```

Esse fluxo executa, em ordem:

1. checksum e validação estrutural do artifact RC;
2. validação da versão/package metadata do Rapid SCADA 6.4.7;
3. instalação do Rapid e do Gateway;
4. hardening non-root do `scadacomm6` usando `/opt/scada/Config/ScadaInstanceConfig.xml`;
5. nftables para impedir acesso não-loopback às portas Rapid 10000/10002;
6. Webstation Rapid fixado em `127.0.0.1:10008`;
7. `AllowAuthApi=true` e **`AllowCommandApi=false`** por edição validada e com backup;
8. instalação transacional do RC Monitor com provider `rapid-web`;
9. binding semântico **de laboratório** sobre os canais do projeto demo Rapid;
10. instalação do frontend pré-compilado e Nginx;
11. acceptance SPA/API/read-only/cache/headers/listeners;
12. registro `/var/lib/rc-scada-stack/lab-mode.env` com `SIMULATION_TEST_ONLY` e `production_validated=false`.

A topologia resultante é:

```text
Browser -> Nginx :80
             ├─ /api/*   -> RC Monitor 127.0.0.1:18100
             ├─ /healthz -> RC Monitor
             ├─ /readyz  -> RC Monitor
             └─ SPA      -> /opt/rc-gateway/current/frontend

RC Monitor -> Rapid Web API 127.0.0.1:10008
RC Gateway admin -> 127.0.0.1:18080
Rapid Server/Agent -> portas 10000/10002 protegidas por nftables
```

O Rapid Webstation não é publicado pelo Nginx quando o frontend RC está instalado.

## Binding Rapid demo de laboratório

A instalação limpa do Rapid 6.4.7 traz canais de simulador (`Sine`, `Square`, `Triangle`, `Array`). Esses canais **não possuem semântica de gerador por si só**.

Por isso o laboratório usa um binding dedicado:

```text
controllers/rc-simulator/reference-controller/rapid/channels.rapid-demo-lab.json
configs/monitor/rc-monitor.rapid-demo-lab.json
```

As transformações produzem faixas plausíveis apenas para exercitar HMI, quality, alarmes e recuperação. Os valores devem ser tratados como **simulação**, nunca como medição de campo ou evidência HIL. O alarme `SIMULATED_DIGITAL_ALARM` existe exclusivamente para esse teste.

## Installer-base

Para instalar somente Rapid + Gateway:

```bash
sudo bash install-scada-stack.sh --dir /caminho/scada-kit
```

Se `rc-gateway.json` não existir, o script usa `rc-gateway.safe.json` quando presente ou gera configuração segura com zero tunnels de campo.

Em instalação existente, qualquer alteração fica bloqueada por padrão. Rerun/upgrade exige autorização explícita:

```bash
sudo bash install-scada-stack.sh --dir /caminho/scada-kit --upgrade
```

No orquestrador de laboratório:

```bash
sudo bash install-rc-lab-stack.sh \
  --dir /caminho/scada-kit \
  --rapid-env /root/rc-monitor.rapid.env \
  --upgrade
```

## Dry-run

O baseline pode ser validado sem alterar a VM:

```bash
bash install-scada-stack.sh --dir /caminho/scada-kit --dry-run
```

O release interno também executa `install-release.sh --dry-run`, que agora exige simultaneamente Gateway, RC Monitor executável e frontend compilado.

## Rapid Web API read-only

O script:

```bash
sudo /opt/rc-gateway/current/scripts/configure-rapid-web-api.sh
```

altera somente:

```xml
<Option name="AllowCommandApi" value="false" />
<Option name="AllowAuthApi" value="true" />
```

Antes da alteração cria backup em `/var/lib/rc-scada-stack`. A operação exige exatamente uma ocorrência de cada opção e recusa configuração ambígua.

## RC Monitor transacional

`install-rc-monitor.sh` valida a configuração antes da troca. Configurações `rapid-web` destinadas a `/etc` devem usar caminhos absolutos para `profileDir` e `rapidBinding`; caminhos relativos são rejeitados antes do restart.

Se a nova configuração não atingir `/readyz`, o instalador restaura configuração, env e unit anteriores e reinicia o estado anterior. Isso evita deixar o serviço degradado após credencial/config inválida.

## Frontend e Nginx

`install-rc-frontend.sh` não compila Node na VM. O frontend já vem dentro da release. O script:

- exige `frontend/index.html` e bundle JS hashed;
- usa `/opt/rc-gateway/current/frontend` como document root;
- mantém `/api`, `/healthz` e `/readyz` no RC Monitor;
- não publica Rapid Web;
- usa `no-store` no shell SPA e `public, immutable` por um ano nos assets hashed;
- instala CSP, `nosniff`, `DENY`, Referrer-Policy e Permissions-Policy;
- usa `systemctl restart nginx` depois de `nginx -t`;
- mantém backup e rollback da configuração Nginx.

HSTS não é aplicado pelo installer HTTP. Em produção, TLS/HSTS pertencem ao reverse proxy/terminador TLS aprovado.

## Acceptance do frontend instalado

```bash
sudo /opt/rc-gateway/current/scripts/rc-frontend-acceptance.sh
```

O gate valida:

- Rapid Web e RC Monitor em loopback;
- rotas SPA e deep-link de gerador;
- API do RC Monitor através do Nginx;
- POST em API read-only retornando 405;
- política de cache;
- headers HTTP;
- ausência de wildcard exposure em 10008, 18080 e 18100.

Os gates Rapid/Gateway existentes continuam separados:

```bash
sudo /opt/rc-gateway/current/scripts/vm-acceptance.sh
sudo RAPID_SCADA_EXPECT_SESSION=0 /opt/rc-gateway/current/scripts/rapid-scada-acceptance.sh
```

`RAPID_SCADA_EXPECT_SESSION=0` é válido somente quando ainda não existe sessão de campo. Não prova Controller→Gateway.

## Supply chain do frontend

A release exige:

```text
.node-version = 22.23.2
frontend/package-lock.json (lockfileVersion 3)
npm ci
```

O Gateway CI configura o mesmo Node antes da build reprodutível. A release falha se o runtime Node divergir ou se o lockfile estiver ausente.

## Segurança e limites da evidência

Nenhum installer deste fluxo habilita START/STOP/RESET/TEST/TRANSFER/setpoints ou acknowledge. `AllowCommandApi` permanece `false`.

A instalação bem-sucedida comprova apenas a stack de software/laboratório. Continuam externos:

- relatório real de `SOAK-001` da VM original;
- `SEM-001` com canais Rapid reais de equipamento;
- `HIL-001` com primeira controladora real read-only;
- `HIL-002` modem/VPN/meio físico;
- TLS e política de exposição de produção;
- aprovação de `PROD-001`.

`PRODUCTION_VALIDATED=false` até que esses gates estejam efetivamente concluídos.

# RC Universal Gateway — Canonical Project State

> Read this file before changing runtime, security, release or transport contracts. Material changes must keep this handoff accurate.

## Product decision

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

RC Universal Gateway transports bytes, datagrams, frames and HID reports between field equipment and the software that understands the application protocol. It does not silently invent register maps, convert proprietary protocols, authorize industrial writes or act as a historian/broker/SCADA.

Current product focus: a generator SCADA acquisition stack using **RC Gateway + Rapid SCADA v6.4.7**. Backend and frontend owned by this project remain a later phase.

## Repository and product identity

Repository: `github.com/paulohspred/Gateway`.

Canonical pre-v1 runtime identity:

```text
binary:   rc-gateway
service:  rc-gateway.service
root:     /opt/rc-gateway
config:   /etc/rc-gateway.json
runtime:  /run/rc-gateway
```

The old `rc-gateway-umbrella` identity is not part of the canonical release. The installer detects a legacy installation and requires explicit migration instead of taking it over silently.

## License

Original RC Universal Gateway material is proprietary, **All Rights Reserved**. The controlling terms are in `LICENSE`; `NOTICE` makes explicit that public source visibility does not grant permission to use, execute, copy for reuse, modify, distribute, deploy, train models on, or create derivatives from the original project without prior written permission.

Third-party software is not relicensed by those proprietary terms. Required notices are in `THIRD_PARTY_NOTICES.md`, and release artifacts contain a CycloneDX SBOM.

Rapid SCADA remains an external third-party product under its own license. The Gateway release and the new stack installer do not embed Rapid SCADA source or binaries; the operator supplies the official Linux package separately.

## Implemented transport plane

- TCP listen/connect and reverse-TCP modem paths;
- TLS 1.3 and mTLS;
- Unix stream sockets;
- Unix `SOCK_SEQPACKET` (`unixpacket`);
- RS232/RS422/RS485 raw serial provider;
- UDP with peer sessions, idle expiry and bounded session/datagram limits;
- SocketCAN classic and CAN-FD with packet preservation;
- Linux USB HID via `/dev/hidrawN`;
- USB HID stable discovery by VID/PID with optional serial, fail-closed ambiguity handling and re-resolution after re-enumeration;
- explicit `length32be` framing when packet transports cross a stream boundary;
- bounded stream concurrency with global and per-tunnel pair limits;
- pair timeout, write timeout, half-close drain, TCP keepalive/NODELAY and CIDR allowlists.

Transport support is not semantic protocol conversion. Modbus TCP, Modbus RTU/ASCII over appropriate transports, MQTT, OPC UA, IEC-104, DNP3 and proprietary TCP protocols can pass transparently when the endpoints understand them. J1939/CANopen and ComAp Direct remain application-layer responsibilities.

## Security invariants

- admin HTTP is loopback-only in this release;
- command plane is disabled and `commandPlaneEnabled=true` is rejected;
- non-loopback TCP/UDP listeners require `allowedCidrs`;
- TLS options are fail-closed; mTLS listeners require CA material;
- CAN transmit defaults to disabled;
- USB HID write defaults to disabled;
- provider paths/sockets are canonicalized and cannot overwrite ordinary files;
- HID explicit paths reject symlinks and non-character devices;
- HID identity selectors must resolve unambiguously and must agree with an explicit path when both are configured;
- serial provider sockets use Unix stream; HID/CAN provider sockets use `unixpacket`;
- packet↔stream transitions require explicit framing and never silently discard message boundaries;
- configuration uses strict JSON and a single file snapshot in `LoadStrict`;
- active stream pairs and UDP sessions are bounded;
- release archives reject path traversal, links and special entries;
- installation validates the candidate config before activation and rolls back when readiness fails.

## Runtime health and observability

Local GET-only admin endpoints exist at both legacy and `/v1` aliases where implemented:

```text
/healthz   /v1/healthz
/readyz    /v1/readyz
/status    /v1/status
/sessions  /v1/sessions
/metrics   /v1/metrics
```

Readiness means configured local runtime components initialized successfully; it does not mean a remote controller completed its application-protocol handshake.

The systemd unit uses `Type=notify`, readiness notification and watchdog operation. Linux `NOTIFY_SOCKET` pathname and abstract-namespace forms are supported. Watchdog timing is derived directly from `WATCHDOG_USEC` and is process-scoped when `WATCHDOG_PID` is provided.

## Automated quality and supply-chain gates

The exact candidate commit must pass:

1. canonical-state consistency gate;
2. GitHub Actions workflow lint (`actionlint`);
3. `gofmt`;
4. `go mod verify` plus tidy-diff check;
5. `go vet`;
6. Staticcheck;
7. shuffled unit/integration tests with coverage;
8. explicit Rapid SCADA Modbus transport contract tests;
9. minimum total coverage threshold;
10. race detector;
11. build and validation of every `configs/*.json` example;
12. real-socket parallel/concurrency tests plus 1,000-pair stress;
13. 1,000 TCP churn cycles and leak gate;
14. impairment and mini-soak gate;
15. `govulncheck`;
16. CodeQL Go analysis;
17. shell syntax and malicious installer-archive tests;
18. deterministic/reproducible Linux amd64 and arm64 release builds;
19. SHA256 checksums and CycloneDX SBOM;
20. Gateway installer dry-run against the real release archive;
21. Gateway + Rapid SCADA stack-installer positive dry-run and negative tests for wrong Rapid version, wrong Debian package identity, ambiguous Rapid package sets and bad Gateway checksum;
22. artifact content checks, including proprietary/third-party notices, Rapid SCADA production-readiness tooling and the standalone stack deployment kit;
23. provenance attestation workflow for versioned releases.

Third-party GitHub Actions are pinned to immutable commit SHAs. Dependabot is configured to propose Go-module and GitHub Actions updates; updates still require the full validation chain.

## Rapid SCADA v6 integration contract

Rapid SCADA is an external consumer, not part of the Gateway core and not bundled into the proprietary Gateway artifact. Compatibility was reviewed against Rapid SCADA **v6.4.7**, `RapidScada/scada-v6` `master` commit `1fd36080c7830303f921672fdaee335a06e7ae50`.

The supported native integration uses Rapid SCADA `DrvCnlBasic` as `TcpClient` and lets the Rapid SCADA application driver interpret the payload. For Modbus, `DrvModbus` supports `TransMode=RTU`, `ASCII` and `TCP`.

Canonical mapping:

- native Modbus TCP delivered by the Gateway -> Rapid SCADA `TransMode=TCP`;
- Modbus RTU raw encapsulated in TCP -> Rapid SCADA `TransMode=RTU`;
- Modbus ASCII raw encapsulated in TCP -> Rapid SCADA `TransMode=ASCII`.

For one Gateway tunnel, especially a single RS485 multidrop bus, `ConnectionMode=Shared` is the recommended Rapid SCADA channel mode. Multiple Modbus slave IDs belong to the same Rapid SCADA communication line and are not modeled inside the Gateway.

Repository integration assets:

- `docs/RAPID_SCADA_INTEGRATION.md`;
- `docs/GENERATOR_SCADA_PRODUCTION_READINESS.md`;
- `docs/SCADA_STACK_INSTALLER.md`;
- `configs/rapid-scada.modbus-tcp.example.json`;
- `configs/rapid-scada.rtu-over-tcp.example.json`;
- `configs/rapid-scada.rs485-multidrop.example.json`;
- `configs/scada-stack.safe.example.json`;
- `internal/bridge/rapid_scada_test.go`;
- `scripts/rapid-scada-acceptance.sh`;
- `scripts/rapid-scada-production-acceptance.sh`;
- `scripts/install-scada-stack.sh`;
- `scripts/test-scada-stack-installer.sh`.

The dedicated software contract covers Modbus TCP read/write/exception frames, Modbus RTU CRC preservation, five Unit IDs on one shared stream, TCP fragmentation/coalescing and 1,000 sustained polling cycles. These tests prove byte-stream behavior; they do not claim a semantic Modbus implementation inside the Gateway.

The upstream Rapid SCADA `scadacomm6.service` baseline has `Type=notify`/`Restart=always` and no explicit `User=`. Non-root hardening must be tested with the real installation before it can become a production invariant; the production preflight can require this using `RAPID_SCADA_REQUIRE_NON_ROOT=1`.

The upstream Rapid SCADA v6.4.7 Webstation unit starts with `--urls=http://0.0.0.0:10008`. The stack installer intentionally overrides that service to `127.0.0.1:10008` and configures Nginx on `127.0.0.1:80` only. This prevents accidental remote exposure before Rapid SCADA credentials and the final TLS/reverse-proxy policy are configured.

## One-directory stack installer contract

The standalone deployment kit is designed for a clean Ubuntu Server 24.04 VM. The CI artifact contains both Gateway architecture archives, their checksums, `install-scada-stack.sh`, `rc-gateway.safe.json` and `SCADA_STACK_INSTALLER.md`. Rapid SCADA itself remains separate; the operator places exactly one official `rapidscada_*_all.deb` or supported Linux ZIP in the same directory.

Running:

```bash
sudo bash install-scada-stack.sh
```

performs fail-closed artifact selection, Gateway SHA256 verification, archive-type/path validation, embedded Gateway installer dry-run, Rapid Debian package identity/version/architecture validation, dependency installation from already-configured APT repositories, ASP.NET Core Runtime 8 detection/installation, Rapid SCADA installation, safe loopback Webstation/Nginx configuration, Gateway health-gated installation, systemd enablement and post-install health checks.

If no `rc-gateway.json` is present, the installer uses the safe zero-tunnel baseline. It does not invent a controller topology or expose a field listener automatically.

The stack installer records installed versions and source hashes in `/var/lib/rc-scada-stack/install-state.env`. A Rapid package checksum may be supplied as a sibling `.sha256` or pinned with `RC_SCADA_RAPID_SHA256`; absence is warned because a local hash alone is not publisher authenticity evidence.

The installer targets fresh deployment by default. Existing Gateway/Rapid installations require explicit `--upgrade` after backup and an authorized maintenance window. Existing non-default Nginx sites are not overwritten unless the operator explicitly allows that condition.

## Configuration compatibility

Current canonical pre-v1 schema: `schema: 3`.

The policy is documented in `CONFIGURATION_COMPATIBILITY.md`. Before v1, incompatible experimental changes must be explicit. After v1, incompatible configuration semantics require an explicit schema change and migration path. Security-sensitive configuration is never silently guessed or reinterpreted.

## Release contract

Canonical Gateway artifacts are named:

```text
rc-gateway_<version>_linux_amd64.tar.gz
rc-gateway_<version>_linux_arm64.tar.gz
```

A Gateway release includes the binary, validated examples, systemd unit, install/rollback/diagnostic/VM scripts, Rapid SCADA acceptance/preflight/stack-installer tooling, generator-SCADA readiness documentation, operational documentation, `LICENSE`, `NOTICE`, `THIRD_PARTY_NOTICES.md`, `MANIFEST`, `VERSION` and SBOM. The manifest identifies `product=rc-gateway` and `license=Proprietary-All-Rights-Reserved`.

The CI artifact also publishes top-level deployment-kit files:

```text
install-scada-stack.sh
rc-gateway.safe.json
SCADA_STACK_INSTALLER.md
```

Rapid SCADA is intentionally not embedded in those Gateway artifacts. Version-tag builds can be provenance-attested through GitHub OIDC. SHA256 is an integrity checksum; provenance/attestation is the publisher/build-chain evidence.

## Current validation checkpoint

The previous exact HEAD `fceb0982d29b3fd5f942238d8a5f2e1d327331bd` passed Gateway CI `33987179973` and CodeQL `33987179972` and was `software_field_test_ready` for the Gateway + Rapid SCADA transport contract.

The new one-directory stack-installer increment must pass the complete CI and CodeQL chain on its own exact HEAD before it inherits that status. Repository-only tests validate packaging and dry-run behavior; the real Rapid SCADA `.deb`, systemd services, Nginx override and ASP.NET runtime still require the clean-VM acceptance run.

## Current promotion rule

```text
implemented
  -> software_validated
  -> software_field_test_ready
  -> vm_accepted
  -> hil_accepted
  -> soak_accepted
  -> production_validated
```

No state is inferred from an older successful commit. `software_field_test_ready` requires all automated gates to be green for the exact HEAD being promoted.

## Remaining gates that cannot be completed by repository-only automation

### Repository-owner administration

The `main` branch protection/ruleset must be confirmed by the repository owner according to `GITHUB_PROTECTION.md` before treating `main` as protected production history. The connected integration can verify some repository state but cannot write administrative branch-protection settings.

Because the repository is public, third parties can technically read/clone the source even though the proprietary license grants no permission to reuse it. If the requirement is to prevent source access rather than merely prohibit licensed use, the repository owner must change visibility to private.

### VM acceptance

Install the exact CI/release artifact on a clean Ubuntu Server 24.04 amd64 VM. The preferred initial path is now the one-directory stack installer, followed by `VM_ACCEPTANCE.md` / `scripts/vm-acceptance.sh`.

The VM acceptance must prove the actual Rapid SCADA v6.4.7 package installation, ASP.NET Core Runtime 8, `scadaagent6`, `scadaserver6`, `scadacomm6`, `scadaweb6`, Nginx, Gateway service, loopback-only Webstation exposure, reboot persistence and the stack state record.

After configuring a real Rapid communication line, execute both `scripts/rapid-scada-acceptance.sh` and `scripts/rapid-scada-production-acceptance.sh`. The production preflight requires explicit consumer ports, checks loopback exposure, time synchronization, disk headroom, restart counters and real session recovery. Disruptive restart testing is opt-in and must run only in an authorized VM/window.

### Hardware-in-the-loop

Production claims remain matrix-specific and require real equipment for the claimed path: PUSR/USR/Teltonika or equivalent modem, MikroTik/VPN/4G, RS232/422/485 including multidrop/half-duplex, USB HID/ComAp InteliLite 4 AMF 9, UDP where used, CAN/CAN-FD, power-cycle/reconnect and consumer/Gateway restarts.

For generator projects, the initial commissioning gate is read-oriented. Start/stop, transfer, reset and setpoint writes must be enabled only after model/firmware-specific HIL, interlock review, authorization and audit behavior are proven in the SCADA layer.

For ComAp USB, HIL must confirm actual Linux enumeration, VID/PID/serial, HID report descriptor/report sizes and the application protocol. Do not claim automatic ComAp Direct↔Modbus conversion.

## Merge rule

PR #2 remains draft and must not be merged into `main` merely because code exists. Keep it draft until the exact candidate is green and the owner is ready for the VM/HIL acceptance sequence. Production validation is recorded only for the hardware/network/consumer combinations actually tested.

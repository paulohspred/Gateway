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
8. minimum total coverage threshold;
9. race detector;
10. build and validation of every `configs/*.json` example;
11. real-socket parallel/concurrency tests plus 1,000-pair stress;
12. 1,000 TCP churn cycles and leak gate;
13. impairment and mini-soak gate;
14. `govulncheck`;
15. CodeQL Go analysis;
16. shell syntax and malicious installer-archive tests;
17. deterministic/reproducible Linux amd64 and arm64 release builds;
18. SHA256 checksums and CycloneDX SBOM;
19. installer dry-run against the real release archive;
20. artifact content checks, including proprietary and third-party notices;
21. provenance attestation workflow for versioned releases.

Third-party GitHub Actions are pinned to immutable commit SHAs. Dependabot is configured to propose Go-module and GitHub Actions updates; updates still require the full validation chain.

## Configuration compatibility

Current canonical pre-v1 schema: `schema: 3`.

The policy is documented in `CONFIGURATION_COMPATIBILITY.md`. Before v1, incompatible experimental changes must be explicit. After v1, incompatible configuration semantics require an explicit schema change and migration path. Security-sensitive configuration is never silently guessed or reinterpreted.

## Release contract

Canonical artifacts are named:

```text
rc-gateway_<version>_linux_amd64.tar.gz
rc-gateway_<version>_linux_arm64.tar.gz
```

A release includes the binary, validated examples, systemd unit, install/rollback/diagnostic/VM scripts, operational documentation, `LICENSE`, `NOTICE`, `THIRD_PARTY_NOTICES.md`, `MANIFEST`, `VERSION` and SBOM. The manifest identifies `product=rc-gateway` and `license=Proprietary-All-Rights-Reserved`.

Version-tag builds can be provenance-attested through GitHub OIDC. SHA256 is an integrity checksum; provenance/attestation is the publisher/build-chain evidence.

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

The `main` branch is currently not protected by GitHub branch protection/rulesets. The connected integration can verify this state but cannot write repository administration. Before treating `main` as production history, apply the settings in `GITHUB_PROTECTION.md`.

Because the repository is public, third parties can technically read/clone the source even though the proprietary license grants no permission to reuse it. If the requirement is to prevent source access rather than merely prohibit licensed use, the repository owner must change visibility to private.

### VM acceptance

Install the exact CI/release artifact on a clean Ubuntu Server 24.04 amd64 VM and execute `VM_ACCEPTANCE.md` / `scripts/vm-acceptance.sh`, including reboot persistence, watchdog, failure recovery, upgrade, rollback, negative config tests, resource observation and a 24-hour soak. A 7-day soak is the target before broad production rollout.

### Hardware-in-the-loop

Production claims remain matrix-specific and require real equipment for the claimed path: PUSR/USR/Teltonika or equivalent modem, MikroTik/VPN/4G, RS232/422/485 including multidrop/half-duplex, USB HID/ComAp InteliLite 4 AMF 9, UDP where used, CAN/CAN-FD, power-cycle/reconnect and consumer/Gateway restarts.

For ComAp USB, HIL must confirm actual Linux enumeration, VID/PID/serial, HID report descriptor/report sizes and the application protocol. Do not claim automatic ComAp Direct↔Modbus conversion.

## Merge rule

PR #2 remains draft and must not be merged into `main` merely because code exists. Keep it draft until the exact candidate is green and the owner is ready for the VM/HIL acceptance sequence. Production validation is recorded only for the hardware/network/consumer combinations actually tested.

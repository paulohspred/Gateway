# Threat Model — RC Universal Gateway

## Security objective

Preserve the confidentiality, integrity and availability properties that the configured transports can provide **without silently changing industrial payloads**. The Gateway is not a semantic safety controller and a working transport path does not authorize process writes.

## Assets

- integrity of bytes, datagrams, CAN frames and HID reports;
- availability of field↔consumer communication;
- configuration integrity;
- TLS private keys/certificates and network credentials external to the process;
- host privileges and device access;
- release/update integrity;
- operational metadata that may reveal field topology.

## Trust boundaries

1. **Field network → Gateway data listener** — potentially untrusted or legacy devices/modems.
2. **Gateway → consumer endpoint** — SCADA, manufacturer software or adapter.
3. **Local IPC → physical providers** — Unix sockets for serial/CAN/HID.
4. **Admin HTTP → local operator** — loopback-only management/observability boundary.
5. **Configuration/filesystem → daemon** — root/operator-controlled deployment boundary.
6. **Release artifact → installer → systemd service** — software supply-chain boundary.
7. **Host kernel → serial/HID/CAN device nodes** — OS/device boundary.

## Threats and controls

### Spoofing / unauthorized peers

Risks: hostile clients reaching a public listener, wrong HID selected after enumeration, unauthenticated plaintext legacy transports.

Controls: non-loopback TCP/UDP listeners require CIDR allowlists; TLS 1.3/mTLS is available; HID can be selected and verified by VID/PID/serial; admin remains loopback-only; VPN/firewall segmentation remains an outer network control.

Residual risk: CIDR allowlists are not cryptographic identity. Legacy plaintext devices should be isolated by VPN/firewall and must not be treated as authenticated merely because an IP is allowed.

### Tampering

Risks: payload modification, unsafe packet↔stream conversion, replacement of device/socket paths, malicious release archives.

Controls: bridge-first byte/frame/report preservation; explicit reversible `length32be` framing; provider socket type validation; symlink/non-character-device checks; refusal to overwrite regular files as stale sockets; release archive traversal/link/special-entry rejection; SHA-256 and reproducible-build checks.

Residual risk: SHA-256 alone proves integrity, not publisher identity. Signed provenance/release authentication is a pre-v1 promotion item.

### Repudiation / insufficient operational evidence

Risks: inability to determine which tunnel failed, when sessions opened/closed or whether recovery occurred.

Controls: structured logs, session registry, per-transport metrics, version/commit metadata, readiness and release manifest.

Residual risk: logs are not an immutable audit ledger. The core intentionally does not implement a generic industrial command plane, reducing the scope of command auditing required here.

### Information disclosure

Risks: logging secrets, exposing admin/status remotely, leaking topology through diagnostics.

Controls: admin loopback-only; no credential fields are intentionally logged; support/security policy forbids public secrets; deployment permissions restrict config/device/socket access.

Residual risk: addresses and device identities in logs can still be sensitive operational metadata. Diagnostic bundles must be sanitized before external sharing.

### Denial of service

Risks: connection storms, unbounded sessions, slow peers, reconnect loops, oversized datagrams/reports/frames, FD/goroutine leaks.

Controls: global `limits.maxActivePairs`, per-tunnel `maxConcurrentPairs`, UDP `maxSessions`, size limits, pair/write/drain/dial/reconnect timeouts, keepalive, bounded admin headers/timeouts, stress/churn/leak tests, race detector, fail-closed configuration.

Parallel stream pairs are permitted only for an unambiguous triggered topology with exactly one `listen` side and one `connect` side. Multi-pair `listen↔listen` remains rejected to avoid incorrect temporal pairing between unrelated clients.

Residual risk: kernel listen backlog and per-source connection rate limiting are partly OS/firewall responsibilities. VM/HIL tests must include connection/reconnect storms and resource monitoring.

### Elevation of privilege / unsafe actuation

Risks: process gains unnecessary host access; a consumer transmits CAN/HID commands unintentionally.

Controls: non-root systemd service, `NoNewPrivileges`, filesystem/kernel hardening, restricted address families, least-privilege device groups, CAN transmit disabled by default, USB HID write disabled by default, command plane rejected.

Residual risk: serial protocols are inherently duplex and some devices treat writes as control. Authorization remains an installation/application responsibility; transport reachability is not process-command authorization.

## Supply-chain threats

Controls currently include pinned GitHub Actions, pinned Go security/supply-chain tools, `govulncheck`, Staticcheck, CodeQL, SBOM, deterministic archives, SHA-256 and installer dry-run/security tests.

Pre-v1 target: signed release artifacts and build provenance/attestation with installer-side verification or an equivalently strong authenticated release process.

## Validation requirements

Automated CI is necessary but insufficient. Production validation additionally requires VM acceptance, real modem/VPN paths, physical serial/HID/CAN where claimed, power-cycle/reconnect tests, network impairment and long-duration soak with objective resource thresholds.

## Review trigger

Update this document whenever a new listener, privilege, write/transmit capability, transport, remote management surface, update mechanism, dependency class or trust boundary is introduced.

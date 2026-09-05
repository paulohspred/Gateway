# Changelog

All notable changes to RC Universal Gateway are recorded here. The project is pre-v1; versioning will follow Semantic Versioning when the first release is promoted.

## Unreleased

### Added

- standalone repository layout and production-oriented release pipeline;
- TCP listen/connect, TLS 1.3/mTLS, Unix stream and Unix `SOCK_SEQPACKET` bridging;
- bounded parallel stream pairs with global and per-tunnel limits and real-socket concurrency tests;
- RS232/RS422/RS485, UDP, SocketCAN/CAN-FD and Linux USB HID providers;
- USB HID stable VID/PID/serial discovery and re-enumeration handling;
- explicit `length32be` framing for packet↔stream transport boundaries;
- health/readiness, versioned admin aliases, sessions and Prometheus-format metrics;
- systemd `Type=notify` readiness/watchdog integration including abstract `NOTIFY_SOCKET` support;
- strict fail-closed configuration, resource collision checks and data-plane safety controls;
- race, stress, churn, impairment, mini-soak, vulnerability, reproducibility and installer security gates;
- `actionlint`, Staticcheck, minimum coverage gate and CodeQL v4 workflow;
- immutable SHA pinning for GitHub Actions and weekly Dependabot proposals;
- SBOM, deterministic release artifacts, health-gated install and rollback;
- provenance/attestation workflow for versioned release artifacts;
- canonical pre-v1 `rc-gateway` service/package/filesystem identity;
- proprietary All Rights Reserved `LICENSE`/`NOTICE` plus third-party redistribution notices;
- configuration-compatibility, repository-protection, VM-acceptance, threat-model and professionalization documentation.

### Security

- admin interface restricted to loopback;
- public TCP/UDP listeners require CIDR allowlists;
- TLS/mTLS options validated fail-closed;
- CAN transmit and USB HID write remain disabled by default;
- provider/device/socket path hardening and malicious archive rejection;
- bounded active stream pairs and UDP sessions to limit resource exhaustion;
- workflow dependency pinning and module-integrity checks;
- release installer requires canonical product/license metadata and legal notice files.

### Changed

- the obsolete `rc-gateway-umbrella` identity is excluded from canonical new release artifacts; legacy installations require explicit migration;
- CodeQL moved from v3 lineage to v4 lineage;
- local CI now mirrors coverage/module/static/workflow checks when the pinned tools are available/required.

### Validation status

Automated validation applies only to the exact commit that passed CI. Hardware, VM, network and long-soak acceptance remain separate promotion gates until explicitly recorded as passed.

## Release policy

A release entry will include version, commit, compatibility/migration notes, security-relevant changes, license identity and validation level. `production validated` will never be inferred solely from automated CI.

# Changelog

All notable changes to RC Universal Gateway are recorded here. The project is pre-v1; versioning will follow Semantic Versioning when the first release is promoted.

## Unreleased

### Added

- standalone repository layout and production-oriented release pipeline;
- TCP listen/connect, TLS 1.3/mTLS, Unix stream and Unix `SOCK_SEQPACKET` bridging;
- bounded parallel stream pairs with global and per-tunnel limits;
- RS232/RS422/RS485, UDP, SocketCAN/CAN-FD and Linux USB HID providers;
- USB HID stable VID/PID/serial discovery and re-enumeration handling;
- explicit `length32be` framing for packet↔stream transport boundaries;
- health/readiness, sessions and Prometheus-format metrics;
- strict fail-closed configuration, resource collision checks and data-plane safety controls;
- race, stress, churn, impairment, mini-soak, vulnerability, reproducibility and installer security gates;
- SBOM, deterministic release artifacts, health-gated install and rollback;
- pinned GitHub Actions, Staticcheck, minimum coverage gate and CodeQL workflow;
- security, support, contribution and issue/PR governance files;
- Rapid SCADA v6 native interoperability profile using `DrvCnlBasic/TcpClient` without bundling Rapid SCADA;
- Gateway examples for native Modbus TCP, RTU-over-TCP and RS485 multidrop with Rapid SCADA;
- Rapid SCADA VM acceptance script requiring a real Communicator session before consumer acceptance.

### Security

- admin interface restricted to loopback;
- public TCP/UDP listeners require CIDR allowlists;
- TLS/mTLS options validated fail-closed;
- CAN transmit and USB HID write remain disabled by default;
- provider/device/socket path hardening and malicious archive rejection;
- bounded active stream pairs to limit resource exhaustion;
- Rapid SCADA consumer endpoints are documented loopback-first for co-located deployments.

### Validation status

Automated validation applies only to the exact commit that passed CI. Hardware, VM, network and long-soak acceptance remain separate promotion gates until explicitly recorded as passed.

Rapid SCADA compatibility is currently a software integration contract reviewed against `RapidScada/scada-v6` master commit `1fd36080c7830303f921672fdaee335a06e7ae50`; actual `scadacomm6.service` interoperability remains a VM acceptance gate.

## Release policy

A release entry will include version, commit, compatibility/migration notes, security-relevant changes and validation level. `production validated` will never be inferred solely from automated CI.
